// Package app wires the application's HTTP server and domain services.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"go.mongodb.org/mongo-driver/mongo"

	"launchpad/internal/assignments"
	"launchpad/internal/audit"
	"launchpad/internal/auth"
	"launchpad/internal/departments"
	"launchpad/internal/employees"
	"launchpad/internal/journeys"
	"launchpad/internal/notifications"
	"launchpad/internal/organizations"
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

	router := newRouter(cfg, buildHandlers(db, deps, cfg))
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

func buildHandlers(db *mongo.Database, deps Dependencies, cfg config.Config) handlers {
	auditStore := audit.NewStore(db)
	orgStore := organizations.NewStore(db)
	userStore := auth.NewUserStore(db)
	departmentStore := departments.NewStore(db)
	employeeStore := employees.NewStore(db)
	journeyStore := journeys.NewStore(db)
	assignmentStore := assignments.NewStore(db)
	notificationStore := notifications.NewStore(db)

	auditSvc := audit.NewService(auditStore)
	orgSvc := organizations.NewService(orgStore)
	departmentSvc := departments.NewService(departmentStore)
	employeeSvc := employees.NewService(employeeStore, departmentSvc)
	journeySvc := journeys.NewService(journeyStore)
	notificationSvc := notifications.NewService(notificationStore)
	assignmentSvc := assignments.NewService(assignmentStore, journeySvc, employeeSvc, notificationSvc)
	sessionStore := auth.NewSessionStore(deps.Redis.RDB(), cfg.RefreshTTL)
	authSvc := auth.NewService(userStore, orgSvc, auditSvc, sessionStore, auth.Config{
		JWTSecret:      cfg.JWTSecret,
		AccessTTL:      cfg.AccessTTL,
		RefreshTTL:     cfg.RefreshTTL,
		PasswordMinLen: cfg.PasswordMinLen,
	})

	return handlers{
		auth:        auth.NewHandler(authSvc),
		orgs:        organizations.NewHandler(orgSvc, auditSvc),
		audit:       audit.NewHandler(auditSvc),
		departments: departments.NewHandler(departmentSvc, auditSvc),
		employees: employees.NewHandler(
			employeeSvc,
			auditSvc,
			accountCreatorAdapter{auth: authSvc},
			memberAdderAdapter{orgs: orgSvc},
		),
		journeys:      journeys.NewHandler(journeySvc, auditSvc),
		assignments:   assignments.NewHandler(assignmentSvc, auditSvc),
		notifications: notifications.NewHandler(notificationSvc),
	}
}

func ensureIndexes(ctx context.Context, db *mongo.Database) error {
	indexers := []struct {
		name string
		fn   func(context.Context) error
	}{
		{name: "audit", fn: audit.NewStore(db).EnsureIndexes},
		{name: "organization", fn: organizations.NewStore(db).EnsureIndexes},
		{name: "user", fn: auth.NewUserStore(db).EnsureIndexes},
		{name: "department", fn: departments.NewStore(db).EnsureIndexes},
		{name: "employee", fn: employees.NewStore(db).EnsureIndexes},
		{name: "journey", fn: journeys.NewStore(db).EnsureIndexes},
		{name: "assignment", fn: assignments.NewStore(db).EnsureIndexes},
		{name: "notification", fn: notifications.NewStore(db).EnsureIndexes},
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
	router.Use(chimw.RealIP)
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
}

func registerPrivateRoutes(api chi.Router, cfg config.Config, routeHandlers handlers) {
	api.Group(func(private chi.Router) {
		private.Use(middleware.Authenticate(cfg.JWTSecret))
		private.Post("/auth/logout", routeHandlers.auth.HandleLogout)
		private.Get("/auth/me", routeHandlers.auth.HandleMe)

		private.Group(func(orgRoutes chi.Router) {
			orgRoutes.Use(middleware.RequireOrganization)
			orgRoutes.Get("/organizations/current", routeHandlers.orgs.HandleGetCurrent)
			orgRoutes.Patch("/organizations/current", routeHandlers.orgs.HandleUpdateCurrent)
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
		})
	})
}

func newServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}
}
