// Package app wires the application's HTTP server and domain services.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"

	"launchpad/internal/analytics"
	"launchpad/internal/assignments"
	assignmentsmongo "launchpad/internal/assignments/mongo"
	"launchpad/internal/audit"
	auditmongo "launchpad/internal/audit/mongo"
	"launchpad/internal/auth"
	authmongo "launchpad/internal/auth/mongo"
	authredis "launchpad/internal/auth/redis"
	"launchpad/internal/billing"
	billingmongo "launchpad/internal/billing/mongo"
	"launchpad/internal/cms"
	cmsmongo "launchpad/internal/cms/mongo"
	"launchpad/internal/departments"
	departmentsmongo "launchpad/internal/departments/mongo"
	"launchpad/internal/employees"
	employeesmongo "launchpad/internal/employees/mongo"
	"launchpad/internal/featureflags"
	featureflagsmongo "launchpad/internal/featureflags/mongo"
	"launchpad/internal/journeys"
	journeysmongo "launchpad/internal/journeys/mongo"
	"launchpad/internal/leads"
	leadsmongo "launchpad/internal/leads/mongo"
	"launchpad/internal/notifications"
	notificationsmongo "launchpad/internal/notifications/mongo"
	"launchpad/internal/organizations"
	organizationsmongo "launchpad/internal/organizations/mongo"
	"launchpad/internal/platform"
	platformmongo "launchpad/internal/platform/mongo"
	"launchpad/internal/support"
	supportmongo "launchpad/internal/support/mongo"
	"launchpad/pkg/config"
	"launchpad/pkg/httpx"
	"launchpad/pkg/middleware"
	mongox "launchpad/pkg/mongo"
	redisx "launchpad/pkg/redis"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

var (
	errMongoDependencyRequired = errors.New("mongo dependency is required")
	errRedisDependencyRequired = errors.New("redis dependency is required")
)

// Dependencies are process-level connections owned by main.
type Dependencies struct {
	Mongo *mongox.Database
	Redis *redisx.Client
}

type handlers struct {
	auth          *auth.Handler
	orgs          *organizations.Handler
	audit         *audit.Handler
	departments   *departments.Handler
	employees     *employees.Handler
	journeys      *journeys.Handler
	assignments   *assignments.Handler
	notifications *notifications.Handler
	platform      *platform.Handler
	leads         *leads.Handler
	featureflags  *featureflags.Handler
	billing       *billing.Handler
	support       *support.Handler
	analytics     *analytics.Handler
	cms           *cms.Handler
}

type wiredServices struct {
	auth         *auth.Service
	platform     *platform.Service
	featureflags *featureflags.Service
	billing      *billing.Service
	handlers     handlers
}

type accountCreatorAdapter struct {
	auth *auth.Service
}

func (a accountCreatorAdapter) CreateUserAccount(
	ctx context.Context,
	email, displayName, password string,
) (string, error) {
	user, err := a.auth.CreateUserAccount(ctx, email, displayName, password)
	if err != nil {
		return "", fmt.Errorf("create user account: %w", err)
	}

	return user.ID, nil
}

type inviteAccountCreator struct {
	auth *auth.Service
}

func (a inviteAccountCreator) CreateUserAccount(
	ctx context.Context,
	email, displayName, password string,
) (string, error) {
	user, err := a.auth.CreateUserAccount(ctx, email, displayName, password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidInput):
			return "", organizations.ErrInviteInvalidInput
		case errors.Is(err, auth.ErrWeakPassword):
			return "", organizations.ErrInviteWeakPassword
		case errors.Is(err, auth.ErrEmailTaken):
			return "", organizations.ErrInviteEmailTaken
		default:
			return "", fmt.Errorf("create invite user account: %w", err)
		}
	}

	return user.ID, nil
}

type memberAdderAdapter struct {
	orgs *organizations.Service
}

func (a memberAdderAdapter) AddEmployeeMember(ctx context.Context, organizationID, userID string) error {
	_, err := a.orgs.AddMember(ctx, organizationID, userID, organizations.RoleEmployee())
	if err != nil {
		return fmt.Errorf("add employee member: %w", err)
	}

	return nil
}

func (a memberAdderAdapter) AddMember(ctx context.Context, organizationID, userID, roleCode string) error {
	_, err := a.orgs.AddMember(ctx, organizationID, userID, roleCode)
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}

	return nil
}

type platformStaffReader struct {
	svc *platform.Service
}

func (a platformStaffReader) GetByUserID(ctx context.Context, userID string) (string, error) {
	roleCode, err := a.svc.StaffRoleByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, platform.ErrNotFound) {
			return "", auth.ErrPlatformStaffNotFound
		}

		return "", fmt.Errorf("get platform staff role: %w", err)
	}

	return roleCode, nil
}

type orgPlanCodeReader struct {
	orgs *organizations.Service
}

func (a orgPlanCodeReader) PlanCode(ctx context.Context, organizationID string) (string, error) {
	org, err := a.orgs.Get(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("get organization plan code: %w", err)
	}

	return org.PlanCode, nil
}

type billingOrgAdapter struct {
	orgs *organizations.Service
}

func (a billingOrgAdapter) Get(ctx context.Context, id string) (billing.OrganizationSummary, error) {
	org, err := a.orgs.Get(ctx, id)
	if err != nil {
		return billing.OrganizationSummary{}, fmt.Errorf("get organization: %w", err)
	}

	return billing.OrganizationSummary{
		ID:       org.ID,
		PlanCode: org.PlanCode,
		Status:   org.Status,
	}, nil
}

func (a billingOrgAdapter) SetPlanCode(ctx context.Context, id, planCode string) (billing.OrganizationSummary, error) {
	org, err := a.orgs.SetPlanCode(ctx, id, planCode)
	if err != nil {
		return billing.OrganizationSummary{}, fmt.Errorf("set organization plan code: %w", err)
	}

	return billing.OrganizationSummary{
		ID:       org.ID,
		PlanCode: org.PlanCode,
		Status:   org.Status,
	}, nil
}

// Run wires domain services and serves HTTP until ctx is cancelled.
func Run(ctx context.Context, cfg config.Config, deps Dependencies) error {
	if deps.Mongo == nil {
		return errMongoDependencyRequired
	}

	if deps.Redis == nil {
		return errRedisDependencyRequired
	}

	db := deps.Mongo.DB()
	if err := ensureIndexes(ctx, db); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}

	wired := buildHandlers(db, deps, cfg)
	seedDefaults(ctx, wired)
	bootstrapPlatformOwner(ctx, cfg, wired.auth, wired.platform)

	router := newRouter(cfg, wired.handlers)
	server := newServer(cfg.HTTPAddr, router)
	errCh := make(chan error, 1)

	go func() {
		slog.Info("launchpad api listening", "addr", cfg.HTTPAddr)

		listenErr := server.ListenAndServe()
		if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen and serve: %w", listenErr)

			return
		}

		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}

		if err := <-errCh; err != nil {
			return fmt.Errorf("serve API: %w", err)
		}

		return nil
	case err := <-errCh:
		return err
	}
}

func buildHandlers(db *drivermongo.Database, deps Dependencies, cfg config.Config) wiredServices {
	auditStore := auditmongo.NewStore(db)
	orgStore := organizationsmongo.NewStore(db)
	userStore := authmongo.NewUserStore(db)
	departmentStore := departmentsmongo.NewStore(db)
	employeeStore := employeesmongo.NewStore(db)
	journeyStore := journeysmongo.NewStore(db)
	assignmentStore := assignmentsmongo.NewStore(db)
	notificationStore := notificationsmongo.NewStore(db)
	platformStore := platformmongo.NewStore(db)
	leadStore := leadsmongo.NewStore(db)
	featureFlagStore := featureflagsmongo.NewStore(db)
	billingStore := billingmongo.NewStore(db)
	supportStore := supportmongo.NewStore(db)
	cmsStore := cmsmongo.NewStore(db)

	auditSvc := audit.NewService(auditStore)
	orgSvc := organizations.NewService(orgStore)
	departmentSvc := departments.NewService(departmentStore)
	employeeSvc := employees.NewService(employeeStore, departmentSvc)
	journeySvc := journeys.NewService(journeyStore)
	notificationSvc := notifications.NewService(notificationStore)
	assignmentSvc := assignments.NewService(assignmentStore, journeySvc, employeeSvc, notificationSvc)
	leadSvc := leads.NewService(leadStore)
	supportSvc := support.NewService(supportStore)
	billingOrg := billingOrgAdapter{orgs: orgSvc}
	billingSvc := billing.NewService(billingStore, billingOrg, billingOrg)
	featureFlagSvc := featureflags.NewService(featureFlagStore, orgPlanCodeReader{orgs: orgSvc})
	platformSvc := platform.NewService(platformStore, orgSvc, leadSvc, supportSvc)
	analyticsSvc := analytics.NewService(assignmentSvc, employeeSvc)
	cmsSvc := cms.NewService(cmsStore)
	authSvc := newAuthService(deps, cfg, userStore, orgSvc, auditSvc, platformSvc)
	accounts := accountCreatorAdapter{auth: authSvc}
	inviteAccounts := inviteAccountCreator{auth: authSvc}
	members := memberAdderAdapter{orgs: orgSvc}

	return wiredServices{
		auth:         authSvc,
		platform:     platformSvc,
		featureflags: featureFlagSvc,
		billing:      billingSvc,
		handlers: handlers{
			auth:          auth.NewHandler(authSvc),
			orgs:          organizations.NewHandler(orgSvc, auditSvc, inviteAccounts, members),
			audit:         audit.NewHandler(auditSvc),
			departments:   departments.NewHandler(departmentSvc, auditSvc),
			employees:     employees.NewHandler(employeeSvc, auditSvc, accounts, members),
			journeys:      journeys.NewHandler(journeySvc, auditSvc),
			assignments:   assignments.NewHandler(assignmentSvc, auditSvc),
			notifications: notifications.NewHandler(notificationSvc),
			platform:      platform.NewHandler(platformSvc),
			leads:         leads.NewHandler(leadSvc),
			featureflags:  featureflags.NewHandler(featureFlagSvc),
			billing:       billing.NewHandler(billingSvc, auditSvc),
			support:       support.NewHandler(supportSvc),
			analytics:     analytics.NewHandler(analyticsSvc),
			cms:           cms.NewHandler(cmsSvc),
		},
	}
}

func newAuthService(
	deps Dependencies,
	cfg config.Config,
	users auth.UserRepository,
	orgs *organizations.Service,
	auditSvc *audit.Service,
	platformSvc *platform.Service,
) *auth.Service {
	return auth.NewService(
		users,
		orgs,
		auditSvc,
		authredis.NewSessionStore(deps.Redis.RDB(), cfg.RefreshTTL),
		auth.Config{
			JWTSecret:      cfg.JWTSecret,
			AccessTTL:      cfg.AccessTTL,
			RefreshTTL:     cfg.RefreshTTL,
			PasswordMinLen: cfg.PasswordMinLen,
		},
		platformStaffReader{svc: platformSvc},
	)
}

func seedDefaults(ctx context.Context, wired wiredServices) {
	if err := wired.featureflags.SeedDefaults(ctx); err != nil {
		slog.Warn("seed feature flags", "error", err)
	}

	if err := wired.billing.SeedDefaults(ctx); err != nil {
		slog.Warn("seed billing plans", "error", err)
	}
}

func bootstrapPlatformOwner(
	ctx context.Context,
	cfg config.Config,
	authSvc *auth.Service,
	platformSvc *platform.Service,
) {
	email := strings.TrimSpace(cfg.PlatformOwnerEmail)

	password := cfg.PlatformOwnerPassword
	if email == "" || password == "" {
		return
	}

	displayName := strings.TrimSpace(cfg.PlatformOwnerName)
	if displayName == "" {
		displayName = "Platform Owner"
	}

	user, err := authSvc.CreateUserAccount(ctx, email, displayName, password)
	if err != nil {
		if !errors.Is(err, auth.ErrEmailTaken) {
			slog.Warn("bootstrap platform owner: create user", "error", err)

			return
		}

		user, err = authSvc.GetUserByEmail(ctx, email)
		if err != nil {
			slog.Warn("bootstrap platform owner: load existing user", "error", err)

			return
		}
	}

	if _, err := platformSvc.EnsureStaff(ctx, user.ID, platform.RoleOwner()); err != nil {
		slog.Warn("bootstrap platform owner: ensure staff", "error", err)

		return
	}

	slog.Info("platform owner bootstrapped", "email", email)
}

func ensureIndexes(ctx context.Context, db *drivermongo.Database) error {
	indexers := []struct {
		name string
		fn   func(context.Context) error
	}{
		{name: "audit", fn: auditmongo.NewStore(db).EnsureIndexes},
		{name: "organization", fn: organizationsmongo.NewStore(db).EnsureIndexes},
		{name: "user", fn: authmongo.NewUserStore(db).EnsureIndexes},
		{name: "department", fn: departmentsmongo.NewStore(db).EnsureIndexes},
		{name: "employee", fn: employeesmongo.NewStore(db).EnsureIndexes},
		{name: "journey", fn: journeysmongo.NewStore(db).EnsureIndexes},
		{name: "assignment", fn: assignmentsmongo.NewStore(db).EnsureIndexes},
		{name: "notification", fn: notificationsmongo.NewStore(db).EnsureIndexes},
		{name: "platform", fn: platformmongo.NewStore(db).EnsureIndexes},
		{name: "leads", fn: leadsmongo.NewStore(db).EnsureIndexes},
		{name: "featureflags", fn: featureflagsmongo.NewStore(db).EnsureIndexes},
		{name: "billing", fn: billingmongo.NewStore(db).EnsureIndexes},
		{name: "support", fn: supportmongo.NewStore(db).EnsureIndexes},
		{name: "cms", fn: cmsmongo.NewStore(db).EnsureIndexes},
	}

	for _, indexer := range indexers {
		if err := indexer.fn(ctx); err != nil {
			return fmt.Errorf("%s indexes: %w", indexer.name, err)
		}
	}

	return nil
}

func newRouter(cfg config.Config, routeHandlers handlers) http.Handler {
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.Use(middleware.RequestLogger)
	router.Use(chimw.Recoverer)
	router.Use(middleware.CORS(cfg.CORSOrigins))

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"}); err != nil {
			slog.ErrorContext(r.Context(), "write healthz response", "error", err)
		}
	})

	router.Route("/api/v1", func(api chi.Router) {
		registerPublicRoutes(api, routeHandlers)
		registerPrivateRoutes(api, cfg, routeHandlers)
	})

	return router
}

func registerPublicRoutes(api chi.Router, routeHandlers handlers) {
	api.Post("/auth/register", routeHandlers.auth.HandleRegister)
	api.Post("/auth/login", routeHandlers.auth.HandleLogin)
	api.Post("/auth/refresh", routeHandlers.auth.HandleRefresh)
	api.Post("/leads", routeHandlers.leads.HandleCreate)
	api.Get("/cms/pages/{slug}", routeHandlers.cms.HandlePublicGetBySlug)
}

func registerPrivateRoutes(api chi.Router, cfg config.Config, routeHandlers handlers) {
	api.Group(func(private chi.Router) {
		private.Use(middleware.Authenticate(cfg.JWTSecret))
		private.Post("/auth/logout", routeHandlers.auth.HandleLogout)
		private.Get("/auth/me", routeHandlers.auth.HandleMe)

		private.Group(func(platformRoutes chi.Router) {
			platformRoutes.Use(middleware.RequirePlatform)
			registerPlatformRoutes(platformRoutes, routeHandlers)
		})

		private.Group(func(orgRoutes chi.Router) {
			orgRoutes.Use(middleware.RequireOrganization)
			registerOrganizationRoutes(orgRoutes, routeHandlers)
		})
	})
}

func registerPlatformRoutes(platformRoutes chi.Router, routeHandlers handlers) {
	platformRoutes.Get("/platform/overview", routeHandlers.platform.HandleOverview)
	platformRoutes.Get("/platform/organizations", routeHandlers.platform.HandleListOrganizations)
	platformRoutes.Get(
		"/platform/organizations/{organizationID}",
		routeHandlers.platform.HandleGetOrganization,
	)
	platformRoutes.Post(
		"/platform/organizations/{organizationID}/suspend",
		routeHandlers.platform.HandleSuspendOrganization,
	)
	platformRoutes.Post(
		"/platform/organizations/{organizationID}/activate",
		routeHandlers.platform.HandleActivateOrganization,
	)
	platformRoutes.Get("/platform/leads", routeHandlers.leads.HandleList)
	platformRoutes.Get("/platform/feature-flags", routeHandlers.featureflags.HandlePlatformList)
	platformRoutes.Post("/platform/feature-flags", routeHandlers.featureflags.HandlePlatformCreate)
	platformRoutes.Patch(
		"/platform/feature-flags/{key}",
		routeHandlers.featureflags.HandlePlatformPatch,
	)
	platformRoutes.Put(
		"/platform/organizations/{organizationID}/feature-flags/{key}",
		routeHandlers.featureflags.HandlePlatformSetOverride,
	)
	platformRoutes.Get("/platform/plans", routeHandlers.billing.HandlePlatformListPlans)
	platformRoutes.Post("/platform/plans", routeHandlers.billing.HandlePlatformCreatePlan)
	platformRoutes.Patch("/platform/plans/{code}", routeHandlers.billing.HandlePlatformPatchPlan)
	platformRoutes.Get("/platform/subscriptions", routeHandlers.billing.HandlePlatformListSubscriptions)
	platformRoutes.Post(
		"/platform/organizations/{organizationID}/subscription",
		routeHandlers.billing.HandlePlatformSetOrganizationSubscription,
	)
	platformRoutes.Get("/platform/support/tickets", routeHandlers.support.HandlePlatformList)
	platformRoutes.Get(
		"/platform/support/tickets/{ticketID}",
		routeHandlers.support.HandlePlatformGet,
	)
	platformRoutes.Post(
		"/platform/support/tickets/{ticketID}/status",
		routeHandlers.support.HandlePlatformUpdateStatus,
	)
	platformRoutes.Get("/platform/cms/pages", routeHandlers.cms.HandlePlatformList)
	platformRoutes.Post("/platform/cms/pages", routeHandlers.cms.HandlePlatformCreate)
	platformRoutes.Get("/platform/cms/pages/{pageID}", routeHandlers.cms.HandlePlatformGet)
	platformRoutes.Patch("/platform/cms/pages/{pageID}", routeHandlers.cms.HandlePlatformUpdate)
	platformRoutes.Post(
		"/platform/cms/pages/{pageID}/publish",
		routeHandlers.cms.HandlePlatformPublish,
	)
}

func registerOrganizationRoutes(orgRoutes chi.Router, routeHandlers handlers) {
	orgRoutes.Get("/organizations/current", routeHandlers.orgs.HandleGetCurrent)
	orgRoutes.Patch("/organizations/current", routeHandlers.orgs.HandleUpdateCurrent)
	orgRoutes.Post("/organizations/current/members", routeHandlers.orgs.HandleInviteMember)
	orgRoutes.Get("/audit-events", routeHandlers.audit.HandleList)

	orgRoutes.Get("/departments", routeHandlers.departments.HandleListDepartments)
	orgRoutes.Post("/departments", routeHandlers.departments.HandleCreateDepartment)
	orgRoutes.Get("/job-roles", routeHandlers.departments.HandleListJobRoles)
	orgRoutes.Post("/job-roles", routeHandlers.departments.HandleCreateJobRole)

	orgRoutes.Get("/employees", routeHandlers.employees.HandleList)
	orgRoutes.Post("/employees", routeHandlers.employees.HandleCreate)
	orgRoutes.Get("/employees/{employeeID}", routeHandlers.employees.HandleGet)
	orgRoutes.Post("/employees/{employeeID}/provision", routeHandlers.employees.HandleProvisionAccess)

	orgRoutes.Get("/journeys", routeHandlers.journeys.HandleList)
	orgRoutes.Post("/journeys", routeHandlers.journeys.HandleCreate)
	orgRoutes.Get("/journeys/{journeyID}", routeHandlers.journeys.HandleGet)
	orgRoutes.Get("/journeys/{journeyID}/steps", routeHandlers.journeys.HandleListSteps)
	orgRoutes.Post("/journeys/{journeyID}/steps", routeHandlers.journeys.HandleAddStep)
	orgRoutes.Post("/journeys/{journeyID}/publish", routeHandlers.journeys.HandlePublish)

	orgRoutes.Get("/assignments", routeHandlers.assignments.HandleList)
	orgRoutes.Post("/assignments", routeHandlers.assignments.HandleAssign)
	orgRoutes.Get("/assignments/{assignmentID}", routeHandlers.assignments.HandleGet)
	orgRoutes.Get("/assignments/{assignmentID}/steps", routeHandlers.assignments.HandleListSteps)
	orgRoutes.Get("/me/assignments", routeHandlers.assignments.HandleListMine)
	orgRoutes.Post("/step-assignments/{stepAssignmentID}/complete", routeHandlers.assignments.HandleCompleteStep)

	orgRoutes.Get("/approvals", routeHandlers.assignments.HandleListApprovals)
	orgRoutes.Post("/approvals/{approvalID}/decide", routeHandlers.assignments.HandleDecideApproval)

	orgRoutes.Get("/notifications", routeHandlers.notifications.HandleList)
	orgRoutes.Post("/notifications/{id}/read", routeHandlers.notifications.HandleMarkRead)

	orgRoutes.Get("/feature-flags", routeHandlers.featureflags.HandleOrgList)
	orgRoutes.Get("/billing/plans", routeHandlers.billing.HandleOrgListPlans)
	orgRoutes.Get("/billing/subscription", routeHandlers.billing.HandleOrgGetSubscription)
	orgRoutes.Get("/support/tickets", routeHandlers.support.HandleOrgList)
	orgRoutes.Post("/support/tickets", routeHandlers.support.HandleOrgCreate)
	orgRoutes.Get("/support/tickets/{ticketID}", routeHandlers.support.HandleOrgGet)
	orgRoutes.Get("/analytics/onboarding", routeHandlers.analytics.HandleOnboardingSummary)
}

func newServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}
}
