// Package app wires the application's HTTP server and domain services.
package app

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"launchpad/internal/analytics"
	"launchpad/internal/assessments"
	assessmentsmongo "launchpad/internal/assessments/mongo"
	"launchpad/internal/assignments"
	assignmentsmongo "launchpad/internal/assignments/mongo"
	"launchpad/internal/assistant"
	assistantanthropic "launchpad/internal/assistant/anthropic"
	assistantembed "launchpad/internal/assistant/embed"
	assistantextractive "launchpad/internal/assistant/extractive"
	assistantmongo "launchpad/internal/assistant/mongo"
	"launchpad/internal/audit"
	auditmongo "launchpad/internal/audit/mongo"
	"launchpad/internal/auth"
	authmongo "launchpad/internal/auth/mongo"
	authredis "launchpad/internal/auth/redis"
	"launchpad/internal/billing"
	billingmongo "launchpad/internal/billing/mongo"
	billingpaystack "launchpad/internal/billing/paystack"
	"launchpad/internal/cms"
	cmsmongo "launchpad/internal/cms/mongo"
	"launchpad/internal/departments"
	departmentsmongo "launchpad/internal/departments/mongo"
	"launchpad/internal/email"
	"launchpad/internal/employees"
	employeesmongo "launchpad/internal/employees/mongo"
	"launchpad/internal/featureflags"
	featureflagsmongo "launchpad/internal/featureflags/mongo"
	"launchpad/internal/hris"
	hrisbamboohr "launchpad/internal/hris/bamboohr"
	hrismongo "launchpad/internal/hris/mongo"
	"launchpad/internal/integrations"
	integrationsmongo "launchpad/internal/integrations/mongo"
	"launchpad/internal/jobs"
	"launchpad/internal/journeys"
	journeysmongo "launchpad/internal/journeys/mongo"
	"launchpad/internal/knowledge"
	knowledgemongo "launchpad/internal/knowledge/mongo"
	"launchpad/internal/leads"
	leadsmongo "launchpad/internal/leads/mongo"
	"launchpad/internal/marketplace"
	marketplacemongo "launchpad/internal/marketplace/mongo"
	"launchpad/internal/meetings"
	meetingsmongo "launchpad/internal/meetings/mongo"
	"launchpad/internal/notifications"
	notificationsmongo "launchpad/internal/notifications/mongo"
	notificationswebhook "launchpad/internal/notifications/webhook"
	"launchpad/internal/organizations"
	organizationsmongo "launchpad/internal/organizations/mongo"
	"launchpad/internal/platform"
	platformmongo "launchpad/internal/platform/mongo"
	"launchpad/internal/privacy"
	"launchpad/internal/requests"
	requestsmongo "launchpad/internal/requests/mongo"
	"launchpad/internal/roles"
	rolesmongo "launchpad/internal/roles/mongo"
	"launchpad/internal/scim"
	scimmongo "launchpad/internal/scim/mongo"
	"launchpad/internal/sms"
	"launchpad/internal/sso"
	ssomongo "launchpad/internal/sso/mongo"
	ssooidc "launchpad/internal/sso/oidc"
	ssoredis "launchpad/internal/sso/redis"
	"launchpad/internal/support"
	supportmongo "launchpad/internal/support/mongo"
	"launchpad/pkg/config"
	"launchpad/pkg/httpx"
	"launchpad/pkg/middleware"
	mongox "launchpad/pkg/mongo"
	"launchpad/pkg/observability"
	redisx "launchpad/pkg/redis"
	"launchpad/pkg/security"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	// writeTimeout must exceed the Anthropic client timeout (60s) since it
	// bounds the whole /assistant/ask handler (embeddings + vector search +
	// LLM call); a slow-but-successful answer must still be writable.
	writeTimeout    = 120 * time.Second
	idleTimeout     = 120 * time.Second
	shutdownTimeout = 10 * time.Second

	readyzPingTimeout = 2 * time.Second

	publicRateLimit  = 20
	publicRateWindow = time.Minute

	// assistantRateLimit bounds calls into the paid LLM per client IP.
	assistantRateLimit  = 10
	assistantRateWindow = time.Minute
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
	auth            *auth.Handler
	orgs            *organizations.Handler
	roles           *roles.Handler
	audit           *audit.Handler
	departments     *departments.Handler
	employees       *employees.Handler
	journeys        *journeys.Handler
	assignments     *assignments.Handler
	assignmentRules *assignments.RuleHandler
	notifications   *notifications.Handler
	knowledge       *knowledge.Handler
	assistant       *assistant.Handler
	scim            *scim.Handler
	sso             *sso.Handler
	hris            *hris.Handler
	integrations    *integrations.Handler
	platform        *platform.Handler
	leads           *leads.Handler
	featureflags    *featureflags.Handler
	billing         *billing.Handler
	support         *support.Handler
	requests        *requests.Handler
	privacy         *privacy.Handler
	analytics       *analytics.Handler
	cms             *cms.Handler
	marketplace     *marketplace.Handler
	assessments     *assessments.Handler
	meetings        *meetings.Handler
	jobs            *jobs.Handler
}

type wiredServices struct {
	auth         *auth.Service
	platform     *platform.Service
	featureflags *featureflags.Service
	billing      *billing.Service
	sessions     middleware.SessionChecker
	permissions  middleware.PermissionResolver
	scheduler    *jobs.Scheduler
	handlers     handlers
}

// sessionCheckerAdapter adapts the auth Redis session store to the
// middleware.SessionChecker port: a missing session maps to (false, nil) so
// the middleware fails closed with 401; store errors propagate so it can
// fail closed with 503.
type sessionCheckerAdapter struct {
	sessions *authredis.SessionStore
}

func (a sessionCheckerAdapter) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	exists, err := a.sessions.Exists(ctx, sessionID)
	if err != nil {
		return false, fmt.Errorf("check session: %w", err)
	}

	return exists, nil
}

type accountCreatorAdapter struct {
	auth *auth.Service
}

type marketplaceJourneyInstaller struct {
	journeys *journeys.Service
}

func (a marketplaceJourneyInstaller) InstallMarketplaceTemplate(
	ctx context.Context,
	organizationID, createdBy, name, description string,
	steps []marketplace.Step,
) (string, error) {
	importSteps := make([]journeys.ImportStep, 0, len(steps))
	for _, step := range steps {
		importSteps = append(importSteps, journeys.ImportStep{
			StepType: step.StepType, Title: step.Title, Instructions: step.Instructions,
			DueOffsetDays: step.DueOffsetDays, Config: step.Config,
		})
	}
	template, err := a.journeys.ImportTemplate(ctx, organizationID, createdBy, name, description, importSteps)
	if err != nil {
		return "", fmt.Errorf("import marketplace journey: %w", err)
	}
	return template.ID, nil
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

// FindUserIDByEmail resolves an existing account's user id by email, letting
// the employees provisioner reuse the account a failed attempt left behind
// instead of bricking the retry on EMAIL_TAKEN.
func (a accountCreatorAdapter) FindUserIDByEmail(ctx context.Context, email string) (string, error) {
	user, err := a.auth.GetUserByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("find user by email: %w", err)
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

// FindUserByEmail resolves an existing account's user id by email so a
// retried invite can reuse the account a previous attempt created.
func (a inviteAccountCreator) FindUserByEmail(ctx context.Context, email string) (string, error) {
	user, err := a.auth.GetUserByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("find invite user by email: %w", err)
	}

	return user.ID, nil
}

type memberAdderAdapter struct {
	orgs *organizations.Service
}

func (a memberAdderAdapter) AddEmployeeMember(ctx context.Context, organizationID, userID string) error {
	_, err := a.orgs.AddMember(ctx, organizationID, userID, organizations.RoleEmployee())
	if err != nil {
		if errors.Is(err, organizations.ErrAlreadyMember) {
			// A previous provisioning attempt already created the membership;
			// the goal state holds, so the retry must succeed.
			return nil
		}

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

// memberUserReader adapts the auth user store to the organizations
// MemberUserReader port for member listings.
type memberUserReader struct {
	users auth.UserRepository
}

func (a memberUserReader) GetMemberUser(ctx context.Context, userID string) (organizations.MemberUser, error) {
	user, err := a.users.GetByID(ctx, userID)
	if err != nil {
		return organizations.MemberUser{}, fmt.Errorf("get member user: %w", err)
	}

	return organizations.MemberUser{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Status:      user.Status,
	}, nil
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

// scimProvisionerAdapter provides the primitive account/membership operations
// the SCIM service composes. The cross-tenant safety decision lives in the SCIM
// service, not here — this adapter only performs side effects.
type scimProvisionerAdapter struct {
	auth *auth.Service
	orgs *organizations.Service
}

func (a scimProvisionerAdapter) CreateAccount(ctx context.Context, email, displayName string) (string, error) {
	password, err := security.NewRefreshToken()
	if err != nil {
		return "", fmt.Errorf("generate provisioning password: %w", err)
	}

	user, err := a.auth.CreateUserAccount(ctx, email, displayName, password)
	if err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			return "", scim.ErrAccountExists
		}

		return "", fmt.Errorf("create provisioned account: %w", err)
	}

	return user.ID, nil
}

func (a scimProvisionerAdapter) FindOrgMember(ctx context.Context, organizationID, email string) (string, error) {
	user, err := a.auth.GetUserByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("load account: %w", err)
	}

	exists, err := a.orgs.HasMembership(ctx, organizationID, user.ID)
	if err != nil {
		return "", fmt.Errorf("check membership: %w", err)
	}

	if !exists {
		return "", scim.ErrNotOrgMember
	}

	return user.ID, nil
}

func (a scimProvisionerAdapter) AddMember(ctx context.Context, organizationID, userID string) error {
	if _, err := a.orgs.AddMember(ctx, organizationID, userID, organizations.RoleEmployee()); err != nil {
		return fmt.Errorf("add membership: %w", err)
	}

	return nil
}

func (a scimProvisionerAdapter) SetActive(ctx context.Context, organizationID, userID string, active bool) error {
	if err := a.orgs.SetMembershipStatus(ctx, organizationID, userID, active); err != nil {
		return fmt.Errorf("set membership status: %w", err)
	}

	return nil
}

// ssoSessionIssuer issues a LaunchPad session for a federated user via the auth
// service's password-less FederatedLogin.
type ssoSessionIssuer struct {
	auth *auth.Service
}

func (a ssoSessionIssuer) IssueFederatedSession(
	ctx context.Context,
	email, organizationID string,
) (sso.Session, error) {
	result, err := a.auth.FederatedLogin(ctx, email, organizationID)
	if err != nil {
		if errors.Is(err, auth.ErrNotProvisioned) {
			return sso.Session{}, sso.ErrNotProvisioned
		}

		return sso.Session{}, fmt.Errorf("federated login: %w", err)
	}

	return sso.Session{
		AccessToken:  result.Tokens.AccessToken,
		RefreshToken: result.Tokens.RefreshToken,
		TokenType:    result.Tokens.TokenType,
		ExpiresIn:    result.Tokens.ExpiresIn,
	}, nil
}

// ssoOrgResolver maps an organization slug to its id for the SSO start flow.
type ssoOrgResolver struct {
	orgs *organizations.Service
}

func (a ssoOrgResolver) OrganizationIDBySlug(ctx context.Context, slug string) (string, error) {
	org, err := a.orgs.GetBySlug(ctx, slug)
	if err != nil {
		return "", fmt.Errorf("resolve organization slug: %w", err)
	}

	return org.ID, nil
}

func (a ssoOrgResolver) OrganizationSlugByID(ctx context.Context, organizationID string) (string, error) {
	org, err := a.orgs.Get(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("resolve organization id: %w", err)
	}

	return org.Slug, nil
}

// maxHRISApplyErrors bounds the per-entry error messages returned by an apply,
// so a large directory with many failures cannot produce an unbounded payload.
const maxHRISApplyErrors = 50

// applyOutcome is the result of processing a single directory entry.
type applyOutcome int

const (
	outcomeCreated applyOutcome = iota
	outcomeSkipped
	outcomeFailed
)

// hrisDirectoryApplier maps an HRIS directory snapshot into the employees
// domain: for each entry whose work email is not already present it creates a
// minimal invited employee, mapping the entry's department NAME to this
// tenant's department ID. Everything is organization-scoped, and re-applying is
// idempotent (already-present emails are skipped).
type hrisDirectoryApplier struct {
	employees   *employees.Service
	departments *departments.Service
}

func (a hrisDirectoryApplier) Apply(
	ctx context.Context,
	organizationID string,
	entries []hris.DirectoryEntry,
) (hris.ApplyResult, error) {
	depts, err := a.departments.ListDepartments(ctx, organizationID)
	if err != nil {
		return hris.ApplyResult{}, fmt.Errorf("list departments for hris apply: %w", err)
	}

	deptIDByName := make(map[string]string, len(depts))
	for _, dept := range depts {
		deptIDByName[strings.ToLower(strings.TrimSpace(dept.Name))] = dept.ID
	}

	result := hris.ApplyResult{
		Total:   len(entries),
		Created: 0,
		Skipped: 0,
		Failed:  0,
		Errors:  nil,
	}

	for _, entry := range entries {
		outcome, msg := a.applyEntry(ctx, organizationID, entry, deptIDByName)
		switch outcome {
		case outcomeCreated:
			result.Created++
		case outcomeSkipped:
			result.Skipped++
		case outcomeFailed:
			result.Failed++
			if len(result.Errors) < maxHRISApplyErrors {
				result.Errors = append(result.Errors, msg)
			}
		}
	}

	return result, nil
}

// applyEntry processes one directory entry against this organization, returning
// the outcome and (for failures) a safe, human-readable message.
func (a hrisDirectoryApplier) applyEntry(
	ctx context.Context,
	organizationID string,
	entry hris.DirectoryEntry,
	deptIDByName map[string]string,
) (applyOutcome, string) {
	email := strings.ToLower(strings.TrimSpace(entry.Email))
	if email == "" || !strings.Contains(email, "@") {
		return outcomeFailed, entry.ExternalID + ": missing or invalid email"
	}

	_, lookupErr := a.employees.GetByWorkEmail(ctx, organizationID, email)
	if lookupErr == nil {
		return outcomeSkipped, ""
	}

	if !errors.Is(lookupErr, employees.ErrNotFound) {
		return outcomeFailed, email + ": lookup failed"
	}

	_, createErr := a.employees.Create(ctx, organizationID, employees.CreateInput{
		EmployeeNumber:    entry.ExternalID,
		FirstName:         entry.FirstName,
		LastName:          entry.LastName,
		WorkEmail:         email,
		JobRoleID:         "",
		DepartmentID:      deptIDByName[strings.ToLower(strings.TrimSpace(entry.Department))],
		ManagerEmployeeID: "",
		BuddyEmployeeID:   "",
		StartDate:         time.Now().UTC(),
	})

	switch {
	case createErr == nil:
		return outcomeCreated, ""
	case errors.Is(createErr, employees.ErrEmailTaken):
		// A concurrent apply created it first; the goal state still holds.
		return outcomeSkipped, ""
	case errors.Is(createErr, employees.ErrInvalidInput):
		return outcomeFailed, email + ": incomplete record (name required)"
	default:
		return outcomeFailed, email + ": could not be created"
	}
}

// hrisProviders is the HRIS provider registry, resolving a config's provider
// name to its adapter.
type hrisProviders struct {
	bamboo *hrisbamboohr.Client
}

//nolint:ireturn // registry returns the selected Provider implementation
func (p hrisProviders) For(name string) (hris.Provider, error) {
	if name == hrisbamboohr.Provider {
		return p.bamboo, nil
	}

	return nil, hris.ErrUnsupportedProvider
}

func newHRISService(
	db *drivermongo.Database,
	applier hris.DirectoryApplier,
	auditSvc *audit.Service,
) *hris.Service {
	return hris.NewService(
		hrismongo.NewConfigStore(db),
		hrismongo.NewStore(db),
		hrisProviders{bamboo: hrisbamboohr.NewClient(nil)},
		applier,
		auditSvc,
	)
}

// integrationServices bundles the enterprise-integration services so
// buildHandlers stays small.
type integrationServices struct {
	scim        *scim.Service
	sso         *sso.Service
	saml        *sso.SAMLService
	hris        *hris.Service
	connections *integrations.Service
	ssoStates   sso.StateStore
}

func newIntegrationServices(
	db *drivermongo.Database,
	deps Dependencies,
	authSvc *auth.Service,
	orgSvc *organizations.Service,
	auditSvc *audit.Service,
	hrisApplier hris.DirectoryApplier,
	cfg config.Config,
) integrationServices {
	stateStore := ssoredis.NewStore(deps.Redis.RDB())
	return integrationServices{
		scim: newScimService(db, authSvc, orgSvc, auditSvc),
		sso:  newSSOService(db, stateStore, authSvc, orgSvc),
		saml: sso.NewSAMLService(
			ssomongo.NewSAMLStore(db), stateStore, ssoSessionIssuer{auth: authSvc},
			ssoOrgResolver{orgs: orgSvc}, cfg.APIPublicURL, cfg.SAMLSPPrivateKey, cfg.SAMLSPCertificate,
		),
		hris:        newHRISService(db, hrisApplier, auditSvc),
		connections: newConnectionsService(db, auditSvc),
		ssoStates:   stateStore,
	}
}

func newConnectionsService(db *drivermongo.Database, auditSvc *audit.Service) *integrations.Service {
	return integrations.NewService(
		integrationsmongo.NewStore(db),
		auditSvc,
		integrations.NewGitHubClient(nil),
		integrations.NewJiraClient(nil),
	)
}

func newSSOService(
	db *drivermongo.Database,
	stateStore sso.StateStore,
	authSvc *auth.Service,
	orgSvc *organizations.Service,
) *sso.Service {
	return sso.NewService(
		ssomongo.NewStore(db),
		stateStore,
		ssooidc.NewClient(nil),
		ssoSessionIssuer{auth: authSvc},
		ssoOrgResolver{orgs: orgSvc},
	)
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

	go wired.scheduler.Start(ctx)

	router := newRouter(cfg, deps, wired.handlers, wired.sessions, wired.permissions)
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

// domainStores bundles the Mongo persistence adapters so buildHandlers stays
// focused on service composition.
type domainStores struct {
	audit              *auditmongo.Store
	org                *organizationsmongo.Store
	user               *authmongo.UserStore
	invitation         *authmongo.InvitationStore
	passwordReset      *authmongo.PasswordResetStore
	department         *departmentsmongo.Store
	employee           *employeesmongo.Store
	journey            *journeysmongo.Store
	assignment         *assignmentsmongo.Store
	platform           *platformmongo.Store
	lead               *leadsmongo.Store
	featureFlag        *featureflagsmongo.Store
	billing            *billingmongo.Store
	support            *supportmongo.Store
	requests           *requestsmongo.Store
	cms                *cmsmongo.Store
	knowledge          *knowledgemongo.Store
	role               *rolesmongo.Store
	marketplace        *marketplacemongo.Store
	assessment         *assessmentsmongo.Store
	meeting            *meetingsmongo.Store
	calendarConnection *meetingsmongo.ConnectionStore
}

func newDomainStores(db *drivermongo.Database) domainStores {
	return domainStores{
		audit:              auditmongo.NewStore(db),
		org:                organizationsmongo.NewStore(db),
		user:               authmongo.NewUserStore(db),
		invitation:         authmongo.NewInvitationStore(db),
		department:         departmentsmongo.NewStore(db),
		employee:           employeesmongo.NewStore(db),
		journey:            journeysmongo.NewStore(db),
		assignment:         assignmentsmongo.NewStore(db),
		platform:           platformmongo.NewStore(db),
		lead:               leadsmongo.NewStore(db),
		featureFlag:        featureflagsmongo.NewStore(db),
		billing:            billingmongo.NewStore(db),
		support:            supportmongo.NewStore(db),
		requests:           requestsmongo.NewStore(db),
		cms:                cmsmongo.NewStore(db),
		knowledge:          knowledgemongo.NewStore(db),
		role:               rolesmongo.NewStore(db),
		marketplace:        marketplacemongo.NewStore(db),
		assessment:         assessmentsmongo.NewStore(db),
		meeting:            meetingsmongo.NewStore(db),
		calendarConnection: meetingsmongo.NewConnectionStore(db),
		passwordReset:      authmongo.NewPasswordResetStore(db),
	}
}

const appEnvLocal = "local"

func newKnowledgeStack(
	db *drivermongo.Database,
	cfg config.Config,
	stores domainStores,
	assignmentSvc *assignments.Service,
	employeeSvc *employees.Service,
	deptSvc *departments.Service,
) (*knowledge.Service, *assistant.Service, *analytics.Service) {
	knowledgeSvc, assistantSvc := newKnowledgeServices(db, cfg, stores.knowledge)
	analyticsSvc := analytics.NewService(assignmentSvc, employeeSvc).WithSources(deptSvc, assistantSvc)

	return knowledgeSvc, assistantSvc, analyticsSvc
}

func newPlatformService(
	stores domainStores,
	orgSvc *organizations.Service,
	leadSvc *leads.Service,
	supportSvc *support.Service,
	deps Dependencies,
	cfg config.Config,
	billingSvc *billing.Service,
	featureFlagSvc *featureflags.Service,
) *platform.Service {
	return platform.NewService(stores.platform, orgSvc, leadSvc, supportSvc).
		WithReadiness(newReadinessDeps(deps, cfg, billingSvc, featureFlagSvc))
}

func newIntegrationStack(
	db *drivermongo.Database,
	deps Dependencies,
	authSvc *auth.Service,
	orgSvc *organizations.Service,
	auditSvc *audit.Service,
	employeeSvc *employees.Service,
	deptSvc *departments.Service,
	cfg config.Config,
) (accountCreatorAdapter, integrationServices) {
	accounts := accountCreatorAdapter{auth: authSvc}
	hrisApplier := hrisDirectoryApplier{employees: employeeSvc, departments: deptSvc}
	entIntegrations := newIntegrationServices(db, deps, authSvc, orgSvc, auditSvc, hrisApplier, cfg)

	return accounts, entIntegrations
}

func newAssignmentRuleService(
	stores domainStores,
	journeySvc *journeys.Service,
	employeeSvc *employees.Service,
	assignmentSvc *assignments.Service,
) *assignments.RuleService {
	ruleSvc := assignments.NewRuleService(stores.assignment, journeySvc, employeeSvc, assignmentSvc)
	employeeSvc.SetRuleApplier(ruleSvc)

	return ruleSvc
}

func newPrivacyServices(
	db *drivermongo.Database,
	stores domainStores,
	assignmentSvc *assignments.Service,
	employeeSvc *employees.Service,
	auditSvc *audit.Service,
) (*requests.Service, *privacy.ExportService, *privacy.PurgeService) {
	requestsSvc := requests.NewService(stores.requests, employeeSvc)
	assignmentSvc.SetRequestCreator(requestsSvc)

	exportSvc := privacy.NewExportService(
		stores.org, stores.employee, stores.department, stores.journey, stores.assignment, stores.audit,
	)
	purgeSvc := newPurgeService(db, stores, auditSvc)

	return requestsSvc, exportSvc, purgeSvc
}

func newPurgeService(db *drivermongo.Database, stores domainStores, auditSvc *audit.Service) *privacy.PurgeService {
	return privacy.NewPurgeService(stores.org, stores.employee, privacy.PurgeStores{
		Organizations:          stores.org,
		Employees:              stores.employee,
		Departments:            stores.department,
		Journeys:               stores.journey,
		Assignments:            stores.assignment,
		Notifications:          notificationsmongo.NewStore(db),
		NotificationChannels:   notificationsmongo.NewChannelStore(db),
		NotificationDeliveries: notificationsmongo.NewDeliveryStore(db),
		Knowledge:              stores.knowledge,
		AssistantChunks:        assistantmongo.NewVectorStore(db),
		AssistantInteractions:  assistantmongo.NewConversationStore(db),
		Audit:                  stores.audit,
		Roles:                  stores.role,
		Integrations:           integrationsmongo.NewStore(db),
		HRISConfigs:            hrismongo.NewConfigStore(db),
		HRISState:              hrismongo.NewStore(db),
		SSOConfigs:             ssomongo.NewStore(db),
		SAMLConfigs:            ssomongo.NewSAMLStore(db),
		SCIMUsers:              scimmongo.NewStore(db),
		SCIMTokens:             scimmongo.NewTokenStore(db),
		SCIMGroups:             scimmongo.NewGroupStore(db),
		BillingSubscriptions:   stores.billing,
		Support:                stores.support,
		FeatureFlagOverrides:   stores.featureFlag,
		Invitations:            stores.invitation,
		PasswordResets:         stores.passwordReset,
		MFAEnrollments:         authmongo.NewMFAStore(db),
		Requests:               stores.requests,
	}, auditSvc)
}

func newScheduler(
	cfg config.Config,
	stores domainStores,
	employeeSvc *employees.Service,
	notificationSvc *notifications.Service,
) *jobs.Scheduler {
	scheduler := jobs.NewScheduler(cfg.SchedulerInterval, jobs.DefaultSweepTimeout)
	scheduler.Register("due-notifications",
		jobs.NewDueNotificationSweep(stores.assignment, employeeSvc, notificationSvc, jobs.DefaultDueSoonHorizon))
	scheduler.Register("meeting-reminders",
		jobs.NewMeetingReminderSweep(stores.meeting, employeeSvc, notificationSvc, jobs.DefaultMeetingReminderHorizon))
	scheduler.Register("notification-delivery-retries", notificationSvc.RetryDueDeliveries)

	return scheduler
}

func buildHandlers(db *drivermongo.Database, deps Dependencies, cfg config.Config) wiredServices {
	stores := newDomainStores(db)

	auditSvc := audit.NewService(stores.audit)
	orgSvc, roleSvc, inviteAccounts := newOrgAndRoleServices(stores.org, stores.role, stores.user)
	deptSvc := departments.NewService(stores.department)
	employeeSvc := employees.NewService(stores.employee, deptSvc)
	journeySvc := journeys.NewService(stores.journey)
	marketplaceSvc := marketplace.NewService(stores.marketplace, marketplaceJourneyInstaller{journeys: journeySvc})
	notificationSvc := newNotificationService(db, cfg, stores.user, employeeSvc)
	supportSvc := support.NewService(stores.support)
	assignmentSvc := assignments.NewService(stores.assignment, journeySvc, employeeSvc, notificationSvc, supportSvc)
	assessmentSvc := assessments.NewService(stores.assessment, employeeSvc)
	meetingSvc := meetings.NewService(
		stores.meeting,
		stores.calendarConnection,
		employeeSvc,
		meetings.NewGoogleCalendarClient(nil),
	).WithMicrosoftCalendar(meetings.NewMicrosoftCalendarClient(nil))
	googleCalendarOAuth := meetings.NewOAuthClient(meetings.ProviderGoogle, meetings.OAuthConfig{
		ClientID: cfg.GoogleCalendarClientID, ClientSecret: cfg.GoogleCalendarClientSecret,
		RedirectURI: cfg.APIPublicURL + "/api/v1/calendar/oauth/callback",
	}, nil)
	microsoftCalendarOAuth := meetings.NewOAuthClient(meetings.ProviderMicrosoft, meetings.OAuthConfig{
		ClientID: cfg.MicrosoftCalendarClientID, ClientSecret: cfg.MicrosoftCalendarClientSecret,
		RedirectURI: cfg.APIPublicURL + "/api/v1/calendar/oauth/callback", Tenant: cfg.MicrosoftCalendarTenant,
	}, nil)
	meetingSvc.WithOAuthClients(googleCalendarOAuth, microsoftCalendarOAuth)
	assignmentSvc.SetAssessmentVerifier(assessmentSvc)
	assignmentSvc.SetMeetingScheduler(meetingSvc)
	assignmentRuleSvc := newAssignmentRuleService(stores, journeySvc, employeeSvc, assignmentSvc)
	requestsSvc, privacyExportSvc, privacyPurgeSvc := newPrivacyServices(db, stores, assignmentSvc, employeeSvc, auditSvc)
	leadSvc := leads.NewService(stores.lead)
	scheduler := newScheduler(cfg, stores, employeeSvc, notificationSvc)
	billingOrg := billingOrgAdapter{orgs: orgSvc}
	billingSvc := billing.NewService(stores.billing, billingOrg, billingOrg)
	if cfg.PaystackSecretKey != "" {
		billingSvc.SetPayments(
			billingpaystack.NewClient(cfg.PaystackSecretKey, cfg.PaystackBaseURL, nil),
			cfg.PaystackWebhookSecret,
		)
	}
	scheduler.Register("billing_dunning", billingSvc.RunDunning)
	featureFlagSvc := featureflags.NewService(stores.featureFlag, orgPlanCodeReader{orgs: orgSvc})
	platformSvc := newPlatformService(stores, orgSvc, leadSvc, supportSvc, deps, cfg, billingSvc, featureFlagSvc)
	platformSvc.WithBusinessMetrics(
		func(ctx context.Context) (platform.RevenueMetrics, error) {
			summary, err := billingSvc.RevenueSummary(ctx)
			return platform.RevenueMetrics{
				MRRTotalCents: summary.MRRTotalCents, ARRTotalCents: summary.ARRTotalCents,
				ActiveSubscriptions: summary.ActiveSubscriptions,
			}, err
		},
		func(ctx context.Context) (platform.SupportMetrics, error) {
			summary, err := supportSvc.Summary(ctx)
			return platform.SupportMetrics{Overdue: summary.Overdue, Urgent: summary.Urgent}, err
		},
	)
	platformSvc.WithStorageMetrics(func(ctx context.Context) (platform.StorageOverview, error) {
		var stats struct {
			Collections int64 `bson:"collections"`
			Objects     int64 `bson:"objects"`
			DataSize    int64 `bson:"dataSize"`
			StorageSize int64 `bson:"storageSize"`
			IndexSize   int64 `bson:"indexSize"`
		}
		if err := deps.Mongo.DB().RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}, {Key: "scale", Value: 1}}).Decode(&stats); err != nil {
			return platform.StorageOverview{}, fmt.Errorf("database storage stats: %w", err)
		}
		return platform.StorageOverview{
			Collections: stats.Collections, Objects: stats.Objects, DataSizeBytes: stats.DataSize,
			StorageSizeBytes: stats.StorageSize, IndexSizeBytes: stats.IndexSize,
		}, nil
	})
	cmsSvc := cms.NewService(stores.cms)
	scheduler.Register("cms_scheduled_publish", cmsSvc.PublishDue)
	knowledgeSvc, assistantSvc, analyticsSvc := newKnowledgeStack(db, cfg, stores, assignmentSvc, employeeSvc, deptSvc)
	knowledgeSvc.WithConnector(knowledge.NewHTTPConnector()).WithNotifier(notificationSvc)
	scheduler.Register("knowledge-stale-alerts", knowledgeSvc.NotifyStale)
	sessionStore := authredis.NewSessionStore(deps.Redis.RDB(), cfg.RefreshTTL)
	authSvc := newAuthService(cfg, stores.user, stores.invitation, sessionStore, orgSvc, auditSvc, platformSvc)
	authSvc = authSvc.WithPermissionResolver(roleSvc)
	authSvc = authSvc.WithMFA(authmongo.NewMFAStore(db), authmongo.NewMFATicketStore(db))
	authSvc = authSvc.
		WithPasswordResets(stores.passwordReset).
		WithMailer(
			email.NewSender(email.Config{APIKey: cfg.EmailAPIKey, From: cfg.EmailFrom, BaseURL: cfg.EmailBaseURL}),
			cfg.OrgAdminURL+"/accept-invitation",
			cfg.OrgAdminURL+"/reset-password",
		)
	inviteAccounts.auth = authSvc
	accounts, entIntegrations := newIntegrationStack(db, deps, authSvc, orgSvc, auditSvc, employeeSvc, deptSvc, cfg)

	return wiredServices{
		auth:         authSvc,
		platform:     platformSvc,
		featureflags: featureFlagSvc,
		billing:      billingSvc,
		sessions:     sessionCheckerAdapter{sessions: sessionStore},
		permissions:  roleSvc,
		scheduler:    scheduler,
		handlers: handlers{
			auth:            auth.NewHandler(authSvc).WithSecureCookies(cfg.AppEnv != appEnvLocal),
			orgs:            organizations.NewHandler(orgSvc, auditSvc),
			roles:           roles.NewHandler(roleSvc, auditSvc),
			audit:           audit.NewHandler(auditSvc),
			departments:     departments.NewHandler(deptSvc, auditSvc),
			employees:       employees.NewHandler(employeeSvc, auditSvc, accounts, memberAdderAdapter{orgs: orgSvc}),
			journeys:        journeys.NewHandler(journeySvc, auditSvc),
			assignments:     assignments.NewHandler(assignmentSvc, auditSvc),
			assignmentRules: assignments.NewRuleHandler(assignmentRuleSvc, auditSvc),
			notifications:   notifications.NewHandler(notificationSvc).WithAudit(auditSvc),
			knowledge:       knowledge.NewHandler(knowledgeSvc, auditSvc),
			assistant:       assistant.NewHandler(assistantSvc),
			scim:            scim.NewHandler(entIntegrations.scim),
			sso: sso.NewHandler(entIntegrations.sso).
				WithSAML(entIntegrations.saml, ssoOrgResolver{orgs: orgSvc}, cfg.OrgAdminURL).
				WithSessionCookies(cfg.RefreshTTL, cfg.AppEnv != appEnvLocal),
			hris:         hris.NewHandler(entIntegrations.hris),
			integrations: integrations.NewHandler(entIntegrations.connections),
			platform:     platform.NewHandler(platformSvc, auditSvc),
			leads:        leads.NewHandler(leadSvc),
			featureflags: featureflags.NewHandler(featureFlagSvc, auditSvc),
			billing:      billing.NewHandler(billingSvc, auditSvc),
			support:      support.NewHandler(supportSvc, auditSvc),
			requests:     requests.NewHandler(requestsSvc, auditSvc),
			privacy:      privacy.NewHandler(privacyExportSvc, privacyPurgeSvc, auditSvc),
			analytics:    analytics.NewHandler(analyticsSvc),
			cms:          cms.NewHandler(cmsSvc, auditSvc),
			marketplace:  marketplace.NewHandler(marketplaceSvc, auditSvc),
			assessments:  assessments.NewHandler(assessmentSvc, auditSvc),
			meetings: meetings.NewHandler(meetingSvc, auditSvc).WithOAuth(
				ssoredis.NewStore(deps.Redis.RDB()), googleCalendarOAuth, microsoftCalendarOAuth, cfg.OrgAdminURL,
			),
			jobs: jobs.NewHandler(scheduler, auditSvc),
		},
	}
}

// newOrgAndRoleServices builds the organizations and roles services,
// late-binding the role checker to break their construction cycle: the roles
// service reads plan codes from the organizations service (enterprise gate)
// while the organizations service validates custom role codes through the
// roles service. Same late-binding pattern as the invite AccountCreator.
func newOrgAndRoleServices(
	orgStore *organizationsmongo.Store,
	roleStore *rolesmongo.Store,
	users auth.UserRepository,
) (*organizations.Service, *roles.Service, *inviteAccountCreator) {
	inviteAccounts := &inviteAccountCreator{auth: nil}
	orgSvc := organizations.NewService(orgStore, inviteAccounts, memberUserReader{users: users}, nil)
	roleSvc := roles.NewService(roleStore, orgPlanCodeReader{orgs: orgSvc})
	orgSvc.SetRoleChecker(roleSvc)

	return orgSvc, roleSvc, inviteAccounts
}

// newReadinessDeps adapts process dependencies and seeded-data services to
// the platform launch-readiness signals. Secrets are reduced to booleans so
// the endpoint can never leak a key.
func newReadinessDeps(
	deps Dependencies,
	cfg config.Config,
	billingSvc *billing.Service,
	featureFlagSvc *featureflags.Service,
) platform.ReadinessDeps {
	return platform.ReadinessDeps{
		MongoPing: func(ctx context.Context) error {
			if err := deps.Mongo.DB().Client().Ping(ctx, readpref.Primary()); err != nil {
				return fmt.Errorf("ping mongo: %w", err)
			}

			return nil
		},
		RedisPing: func(ctx context.Context) error {
			if err := deps.Redis.RDB().Ping(ctx).Err(); err != nil {
				return fmt.Errorf("ping redis: %w", err)
			}

			return nil
		},
		CountPlans: func(ctx context.Context) (int, error) {
			plans, err := billingSvc.ListPlans(ctx, false)
			if err != nil {
				return 0, fmt.Errorf("count billing plans: %w", err)
			}

			return len(plans), nil
		},
		CountFlags: func(ctx context.Context) (int, error) {
			flags, err := featureFlagSvc.ListFlags(ctx)
			if err != nil {
				return 0, fmt.Errorf("count feature flags: %w", err)
			}

			return len(flags), nil
		},
		AppEnv:           cfg.AppEnv,
		CORSOrigins:      cfg.CORSOrigins,
		EncryptionKeySet: cfg.EncryptionKey != "",
		AnthropicKeySet:  cfg.AnthropicAPIKey != "",
	}
}

// newAnswerGenerator selects the Claude-backed generator when an API key is
// configured, falling back to the offline extractive generator otherwise.
//
//nolint:ireturn // factory intentionally selects an AnswerGenerator implementation
func newAnswerGenerator(cfg config.Config) assistant.AnswerGenerator {
	if cfg.AnthropicAPIKey == "" {
		return assistantextractive.NewGenerator()
	}

	return assistantanthropic.NewClient(assistantanthropic.Config{
		APIKey:     cfg.AnthropicAPIKey,
		Model:      cfg.AssistantModel,
		BaseURL:    "",
		HTTPClient: nil,
	})
}

// newKnowledgeServices builds the knowledge module and the AI assistant that
// indexes into it, sharing one embedding provider and vector store.
func newKnowledgeServices(
	db *drivermongo.Database,
	cfg config.Config,
	knowledgeStore knowledge.Repository,
) (*knowledge.Service, *assistant.Service) {
	embeddings := assistantembed.NewProvider()
	vectorStore := assistantmongo.NewVectorStore(db)
	indexer := assistant.NewIndexer(embeddings, vectorStore)

	knowledgeSvc := knowledge.NewService(knowledgeStore, indexer)
	assistantSvc := assistant.NewService(
		embeddings,
		vectorStore,
		newAnswerGenerator(cfg),
		assistantmongo.NewConversationStore(db),
	)

	return knowledgeSvc, assistantSvc
}

func newNotificationService(
	db *drivermongo.Database,
	cfg config.Config,
	users auth.UserRepository,
	employeesSvc *employees.Service,
) *notifications.Service {
	service := notifications.NewService(
		notificationsmongo.NewStore(db),
		notificationsmongo.NewChannelStore(db),
		notificationswebhook.NewDispatcher(nil),
	).WithEmailDispatcher(notificationEmailDispatcher{
		users:   users,
		sender:  email.NewSender(email.Config{APIKey: cfg.EmailAPIKey, From: cfg.EmailFrom, BaseURL: cfg.EmailBaseURL}),
		baseURL: cfg.OrgAdminURL,
	}).WithDeliveryStore(notificationsmongo.NewDeliveryStore(db))
	smsSender := sms.NewSender(sms.Config{APIKey: cfg.SMSAPIKey, From: cfg.SMSFrom, BaseURL: cfg.SMSBaseURL})
	if smsSender.Configured() {
		service.WithSMSDispatcher(notificationSMSDispatcher{employees: employeesSvc, sender: smsSender})
	}
	return service
}

type notificationEmailDispatcher struct {
	users   auth.UserRepository
	sender  email.Sender
	baseURL string
}

type notificationSMSDispatcher struct {
	employees *employees.Service
	sender    *sms.Sender
}

func (d notificationSMSDispatcher) DispatchSMS(
	ctx context.Context,
	notification notifications.Notification,
) error {
	employee, err := d.employees.GetByUserID(ctx, notification.OrganizationID, notification.UserID)
	if err != nil {
		return fmt.Errorf("resolve SMS recipient: %w", err)
	}
	if employee.MobilePhone == "" {
		return fmt.Errorf("SMS recipient has no mobile phone")
	}
	message := notification.Title + ": " + notification.Body
	if len(message) > 320 {
		message = message[:320]
	}
	return d.sender.Send(ctx, employee.MobilePhone, message)
}

func (d notificationEmailDispatcher) DispatchEmail(
	ctx context.Context,
	notification notifications.Notification,
) error {
	user, err := d.users.GetByID(ctx, notification.UserID)
	if err != nil {
		return fmt.Errorf("resolve notification recipient: %w", err)
	}
	body := "<p>" + html.EscapeString(notification.Body) + "</p>"
	if notification.Link != "" {
		body += `<p><a href="` + html.EscapeString(d.baseURL+notification.Link) + `">Open in LaunchPad</a></p>`
	}
	if err := d.sender.Send(ctx, user.Email, notification.Title, body); err != nil {
		return fmt.Errorf("send notification email: %w", err)
	}

	return nil
}

func newScimService(
	db *drivermongo.Database,
	authSvc *auth.Service,
	orgSvc *organizations.Service,
	auditSvc *audit.Service,
) *scim.Service {
	return scim.NewService(
		scimmongo.NewStore(db),
		scimmongo.NewTokenStore(db),
		scimProvisionerAdapter{auth: authSvc, orgs: orgSvc},
		scimmongo.NewGroupStore(db),
		auditSvc,
	)
}

func newAuthService(
	cfg config.Config,
	users auth.UserRepository,
	invitations auth.InvitationStore,
	sessions auth.SessionRepository,
	orgs *organizations.Service,
	auditSvc *audit.Service,
	platformSvc *platform.Service,
) *auth.Service {
	return auth.NewService(
		users,
		orgs,
		auditSvc,
		sessions,
		invitations,
		auth.Config{
			JWTSecret:      cfg.JWTSecret,
			AccessTTL:      cfg.AccessTTL,
			RefreshTTL:     cfg.RefreshTTL,
			InviteTTL:      cfg.InviteTTL,
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
	for _, indexer := range MongoIndexers(db) {
		if err := indexer.Ensure(ctx); err != nil {
			return fmt.Errorf("%s indexes: %w", indexer.Name, err)
		}
	}

	return nil
}

func newRouter(
	cfg config.Config,
	deps Dependencies,
	routeHandlers handlers,
	sessions middleware.SessionChecker,
	permissions middleware.PermissionResolver,
) http.Handler {
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	// RealIP rewrites RemoteAddr from X-Forwarded-For/X-Real-IP so the rate
	// limiter keys on the real client IP; only deploy behind a trusted load
	// balancer that overwrites those headers, never directly internet-exposed.
	router.Use(middleware.RealIP)
	router.Use(audit.Middleware)
	router.Use(middleware.RequestLogger)
	externalExporter := observability.NewExternalExporter(
		cfg.ErrorTrackingURL, cfg.TracingExportURL, cfg.ObservabilityToken,
	)
	observability.SetErrorSink(externalExporter.ErrorSink)
	router.Use(externalExporter.Middleware)

	metricsRegistry := observability.NewRegistry().WithPingChecks(
		func(ctx context.Context) error {
			if err := deps.Mongo.DB().Client().Ping(ctx, readpref.Primary()); err != nil {
				return fmt.Errorf("mongo ping: %w", err)
			}

			return nil
		},
		func(ctx context.Context) error {
			if err := deps.Redis.RDB().Ping(ctx).Err(); err != nil {
				return fmt.Errorf("redis ping: %w", err)
			}

			return nil
		},
	)
	router.Use(metricsRegistry.Middleware)

	router.Use(chimw.Recoverer)
	router.Use(middleware.SecurityHeadersWithConfig(cfg.AppEnv))
	router.Use(middleware.CORS(cfg.CORSOrigins))

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"}); err != nil {
			slog.ErrorContext(r.Context(), "write healthz response", "error", err)
		}
	})

	router.Get("/readyz", readinessHandler(deps))
	router.Get("/metrics", metricsRegistry.Handler())

	router.Route("/api/v1", func(api chi.Router) {
		registerPublicRoutes(api, routeHandlers)
		registerScimRoutes(api, routeHandlers)
		registerPrivateRoutes(api, cfg, routeHandlers, sessions, permissions)
	})

	return router
}

// readinessHandler pings Mongo and Redis so a pod with a dead dependency
// fails its readiness probe and is pulled from rotation.
func readinessHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), readyzPingTimeout)
		defer cancel()

		status := http.StatusOK

		if err := deps.Mongo.DB().Client().Ping(pingCtx, readpref.Primary()); err != nil {
			slog.ErrorContext(r.Context(), "readyz mongo ping", "error", err)

			status = http.StatusServiceUnavailable
		}

		if err := deps.Redis.RDB().Ping(pingCtx).Err(); err != nil {
			slog.ErrorContext(r.Context(), "readyz redis ping", "error", err)

			status = http.StatusServiceUnavailable
		}

		body := map[string]string{"status": "ok"}
		if status != http.StatusOK {
			body["status"] = "unavailable"
		}

		if err := httpx.WriteJSON(w, status, body); err != nil {
			slog.ErrorContext(r.Context(), "write readyz response", "error", err)
		}
	}
}

func registerPublicRoutes(api chi.Router, routeHandlers handlers) {
	// Sensitive unauthenticated endpoints are rate-limited per client IP to blunt
	// password brute-force / credential-stuffing, lead spam, and outbound OIDC
	// token-exchange amplification via the SSO callback.
	api.Group(func(limited chi.Router) {
		limited.Use(middleware.RateLimit(publicRateLimit, publicRateWindow))
		limited.Post("/auth/register", routeHandlers.auth.HandleRegister)
		limited.Post("/auth/login", routeHandlers.auth.HandleLogin)
		limited.Post("/auth/login/mfa", routeHandlers.auth.HandleLoginMFA)
		limited.Post("/auth/refresh", routeHandlers.auth.HandleRefresh)
		limited.Post("/auth/invitations/accept", routeHandlers.auth.HandleAcceptInvitation)
		limited.Post("/auth/password-reset/request", routeHandlers.auth.HandleRequestPasswordReset)
		limited.Post("/auth/password-reset/confirm", routeHandlers.auth.HandleConfirmPasswordReset)
		limited.Post("/leads", routeHandlers.leads.HandleCreate)
		limited.Get("/cms/pages/{slug}", routeHandlers.cms.HandlePublicGetBySlug)
		limited.Get("/cms/navigation", routeHandlers.cms.HandlePublicNavigation)
		limited.Get("/marketplace/templates", routeHandlers.marketplace.HandlePublicList)
		limited.Get("/auth/sso/{orgSlug}/start", routeHandlers.sso.HandleStart)
		limited.Get("/auth/sso/callback", routeHandlers.sso.HandleCallback)
		limited.Get("/auth/saml/{orgSlug}/start", routeHandlers.sso.HandleSAMLStart)
		limited.Get("/auth/saml/{orgSlug}/metadata", routeHandlers.sso.HandleSAMLMetadata)
		limited.Post("/auth/saml/{orgSlug}/acs", routeHandlers.sso.HandleSAMLACS)
		limited.Get("/calendar/oauth/callback", routeHandlers.meetings.HandleCalendarOAuthCallback)
	})
}

func registerPrivateRoutes(
	api chi.Router,
	cfg config.Config,
	routeHandlers handlers,
	sessions middleware.SessionChecker,
	permissions middleware.PermissionResolver,
) {
	api.Group(func(private chi.Router) {
		private.Use(middleware.Authenticate(cfg.JWTSecret, sessions, auth.AccessTokenCookieName))
		private.Use(middleware.CSRF(cfg.AppEnv != appEnvLocal))
		private.Post("/auth/logout", routeHandlers.auth.HandleLogout)
		private.Post("/auth/mfa/enroll", routeHandlers.auth.HandleMFAEnroll)
		private.Post("/auth/mfa/confirm", routeHandlers.auth.HandleMFAConfirm)
		private.Post("/auth/mfa/disable", routeHandlers.auth.HandleMFADisable)
		private.Get("/auth/me", routeHandlers.auth.HandleMe)
		private.Get("/auth/organizations", routeHandlers.auth.HandleListOrganizations)
		private.Post("/auth/switch-organization", routeHandlers.auth.HandleSwitchOrganization)

		private.Group(func(platformRoutes chi.Router) {
			platformRoutes.Use(middleware.RequirePlatform)
			registerPlatformRoutes(platformRoutes, routeHandlers)
		})

		private.Group(func(orgRoutes chi.Router) {
			orgRoutes.Use(middleware.RequireOrganization)
			registerOrganizationRoutes(orgRoutes, routeHandlers, permissions)
		})
	})
}

// permit wraps a single route handler with the RBAC permission gate,
// resolving the caller's role to a permission set per request.
func permit(
	permissions middleware.PermissionResolver,
	permission string,
	next http.HandlerFunc,
) http.HandlerFunc {
	return middleware.RequirePermission(permissions, permission)(next).ServeHTTP
}

func registerScimRoutes(api chi.Router, routeHandlers handlers) {
	const userByID = "/scim/v2/Users/{userID}"

	api.Group(func(scimRoutes chi.Router) {
		scimRoutes.Use(routeHandlers.scim.Authenticate)
		scimRoutes.Get("/scim/v2/ServiceProviderConfig", routeHandlers.scim.HandleServiceProviderConfig)
		scimRoutes.Get("/scim/v2/Users", routeHandlers.scim.HandleListUsers)
		scimRoutes.Post("/scim/v2/Users", routeHandlers.scim.HandleCreateUser)
		scimRoutes.Get(userByID, routeHandlers.scim.HandleGetUser)
		scimRoutes.Put(userByID, routeHandlers.scim.HandleReplaceUser)
		scimRoutes.Patch(userByID, routeHandlers.scim.HandlePatchUser)
		scimRoutes.Delete(userByID, routeHandlers.scim.HandleDeleteUser)

		const groupByID = "/scim/v2/Groups/{groupID}"

		scimRoutes.Get("/scim/v2/Groups", routeHandlers.scim.HandleListGroups)
		scimRoutes.Post("/scim/v2/Groups", routeHandlers.scim.HandleCreateGroup)
		scimRoutes.Get(groupByID, routeHandlers.scim.HandleGetGroup)
		scimRoutes.Put(groupByID, routeHandlers.scim.HandleReplaceGroup)
		scimRoutes.Patch(groupByID, routeHandlers.scim.HandlePatchGroup)
		scimRoutes.Delete(groupByID, routeHandlers.scim.HandleDeleteGroup)
	})
}

func registerPlatformRoutes(platformRoutes chi.Router, routeHandlers handlers) {
	platformRoutes.Get("/platform/overview", routeHandlers.platform.HandleOverview)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), platform.RoleAdmin(),
	)).Get("/platform/staff", routeHandlers.platform.HandleListStaff)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), platform.RoleAdmin(),
	)).Post("/platform/staff", routeHandlers.platform.HandleCreateStaff)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), platform.RoleAdmin(),
	)).Patch("/platform/staff/{staffID}", routeHandlers.platform.HandleUpdateStaffRole)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), platform.RoleAdmin(),
	)).Post("/platform/staff/{staffID}/deactivate", routeHandlers.platform.HandleDeactivateStaff)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), platform.RoleAdmin(),
	)).Post("/platform/staff/{staffID}/reactivate", routeHandlers.platform.HandleReactivateStaff)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), platform.RoleAdmin(), "security_admin",
	)).Get("/platform/security/access-review", routeHandlers.platform.HandleAccessReview)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), "security_admin",
	)).Post("/platform/security/access-review/{staffID}/attest", routeHandlers.platform.HandleAttestAccess)
	platformRoutes.With(middleware.RequirePlatformRole(platform.RoleOwner())).Post(
		"/platform/security/break-glass/{staffID}", routeHandlers.platform.HandleGrantBreakGlass,
	)
	platformRoutes.With(middleware.RequirePlatformRole(platform.RoleOwner())).Post(
		"/platform/security/break-glass/{staffID}/revoke", routeHandlers.platform.HandleRevokeBreakGlass,
	)
	platformRoutes.Get("/platform/launch-readiness", routeHandlers.platform.HandleLaunchReadiness)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), platform.RoleAdmin(), "security_admin",
	)).Get("/platform/storage", routeHandlers.platform.HandleStorageOverview)
	platformRoutes.Get("/platform/audit-events", routeHandlers.audit.HandlePlatformList)
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
	platformRoutes.With(middleware.RequirePlatformRole()).Post(
		"/platform/organizations/{organizationID}/close",
		routeHandlers.platform.HandleCloseOrganization,
	)
	platformRoutes.Post(
		"/platform/organizations/{organizationID}/purge",
		routeHandlers.privacy.HandlePurgeOrganization,
	)
	platformRoutes.Get("/platform/leads", routeHandlers.leads.HandleList)
	platformRoutes.Get("/platform/feature-flags", routeHandlers.featureflags.HandlePlatformList)
	platformRoutes.Post("/platform/feature-flags", routeHandlers.featureflags.HandlePlatformCreate)
	platformRoutes.Get("/platform/marketplace/templates", routeHandlers.marketplace.HandlePlatformList)
	platformRoutes.Post("/platform/marketplace/templates", routeHandlers.marketplace.HandlePlatformCreate)
	platformRoutes.Post("/platform/marketplace/templates/{templateID}/publish", routeHandlers.marketplace.HandlePublish)
	platformRoutes.Post("/platform/marketplace/templates/{templateID}/remove", routeHandlers.marketplace.HandleRemove)
	platformRoutes.Post("/platform/marketplace/templates/{templateID}/version", routeHandlers.marketplace.HandleVersion)
	platformRoutes.Put("/platform/marketplace/templates/{templateID}/featured", routeHandlers.marketplace.HandleFeature)
	platformRoutes.Patch(
		"/platform/feature-flags/{key}",
		routeHandlers.featureflags.HandlePlatformPatch,
	)
	platformRoutes.Get(
		"/platform/feature-flags/{key}/history",
		routeHandlers.featureflags.HandlePlatformHistory,
	)
	platformRoutes.Put(
		"/platform/organizations/{organizationID}/feature-flags/{key}",
		routeHandlers.featureflags.HandlePlatformSetOverride,
	)
	platformRoutes.Get("/platform/plans", routeHandlers.billing.HandlePlatformListPlans)
	platformRoutes.Post("/platform/plans", routeHandlers.billing.HandlePlatformCreatePlan)
	platformRoutes.Patch("/platform/plans/{code}", routeHandlers.billing.HandlePlatformPatchPlan)
	platformRoutes.Get("/platform/subscriptions", routeHandlers.billing.HandlePlatformListSubscriptions)
	platformRoutes.Get("/platform/invoices", routeHandlers.billing.HandlePlatformListInvoices)
	platformRoutes.Get("/platform/coupons", routeHandlers.billing.HandlePlatformListCoupons)
	platformRoutes.Post("/platform/coupons", routeHandlers.billing.HandlePlatformCreateCoupon)
	platformRoutes.Patch("/platform/invoices/{invoiceID}", routeHandlers.billing.HandlePlatformAdjustInvoice)
	platformRoutes.Post("/platform/invoices/{invoiceID}/refund", routeHandlers.billing.HandlePlatformRefundInvoice)
	platformRoutes.Post(
		"/platform/organizations/{organizationID}/subscription",
		routeHandlers.billing.HandlePlatformSetOrganizationSubscription,
	)
	platformRoutes.Get("/platform/support/tickets", routeHandlers.support.HandlePlatformList)
	platformRoutes.Get("/platform/support/summary", routeHandlers.support.HandlePlatformSummary)
	platformRoutes.Get(
		"/platform/support/tickets/{ticketID}",
		routeHandlers.support.HandlePlatformGet,
	)
	platformRoutes.Post(
		"/platform/support/tickets/{ticketID}/status",
		routeHandlers.support.HandlePlatformUpdateStatus,
	)
	platformRoutes.Post("/platform/support/tickets/{ticketID}/messages", routeHandlers.support.HandlePlatformAddMessage)
	platformRoutes.Post("/platform/support/tickets/{ticketID}/escalate", routeHandlers.support.HandlePlatformEscalate)
	platformRoutes.Get("/platform/cms/pages", routeHandlers.cms.HandlePlatformList)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), platform.RoleAdmin(), "security_admin",
	)).Get("/platform/jobs", routeHandlers.jobs.HandleList)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), platform.RoleAdmin(), "security_admin",
	)).Get("/platform/deliveries", routeHandlers.notifications.HandlePlatformDeliveries)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), platform.RoleAdmin(),
	)).Post("/platform/deliveries/{deliveryID}/retry", routeHandlers.notifications.HandlePlatformRetryDelivery)
	platformRoutes.With(middleware.RequirePlatformRole(
		platform.RoleOwner(), platform.RoleAdmin(),
	)).Post("/platform/jobs/{name}/run", routeHandlers.jobs.HandleRun)
	platformRoutes.Post("/platform/cms/pages", routeHandlers.cms.HandlePlatformCreate)
	platformRoutes.Get("/platform/cms/pages/{pageID}", routeHandlers.cms.HandlePlatformGet)
	platformRoutes.Patch("/platform/cms/pages/{pageID}", routeHandlers.cms.HandlePlatformUpdate)
	platformRoutes.Post(
		"/platform/cms/pages/{pageID}/publish",
		routeHandlers.cms.HandlePlatformPublish,
	)
	platformRoutes.Post(
		"/platform/cms/pages/{pageID}/schedule",
		routeHandlers.cms.HandlePlatformSchedule,
	)
}

func registerOrganizationRoutes(
	orgRoutes chi.Router,
	routeHandlers handlers,
	permissions middleware.PermissionResolver,
) {
	orgRoutes.Get("/organizations/current", routeHandlers.orgs.HandleGetCurrent)
	orgRoutes.Patch("/organizations/current", routeHandlers.orgs.HandleUpdateCurrent)
	orgRoutes.Patch("/organizations/current/setup", routeHandlers.orgs.HandleUpdateSetupProgress)
	orgRoutes.Post(
		"/marketplace/templates/submit",
		permit(permissions, roles.PermissionJourneysCreate, routeHandlers.marketplace.HandleSubmit),
	)
	orgRoutes.Post(
		"/marketplace/templates/{templateID}/install",
		permit(permissions, roles.PermissionJourneysCreate, routeHandlers.marketplace.HandleInstall),
	)
	orgRoutes.Put("/marketplace/templates/{templateID}/rating", routeHandlers.marketplace.HandleRate)
	orgRoutes.Get("/assessments", permit(permissions, roles.PermissionAssessmentsManage, routeHandlers.assessments.HandleList))
	orgRoutes.Post("/assessments", permit(permissions, roles.PermissionAssessmentsManage, routeHandlers.assessments.HandleCreate))
	orgRoutes.Get("/assessments/{assessmentID}", permit(permissions, roles.PermissionAssessmentsManage, routeHandlers.assessments.HandleGet))
	orgRoutes.Patch("/assessments/{assessmentID}", permit(permissions, roles.PermissionAssessmentsManage, routeHandlers.assessments.HandleUpdate))
	orgRoutes.Post("/assessments/{assessmentID}/publish", permit(permissions, roles.PermissionAssessmentsManage, routeHandlers.assessments.HandlePublish))
	orgRoutes.Post("/assessments/{assessmentID}/archive", permit(permissions, roles.PermissionAssessmentsManage, routeHandlers.assessments.HandleArchive))
	orgRoutes.Get("/assessments/{assessmentID}/attempts", permit(permissions, roles.PermissionAssessmentsManage, routeHandlers.assessments.HandleListAttempts))
	orgRoutes.Post("/assessments/{assessmentID}/attempts/{attemptID}/review", permit(permissions, roles.PermissionAssessmentsManage, routeHandlers.assessments.HandleReviewAttempt))
	orgRoutes.Get("/assessments/{assessmentID}/take", routeHandlers.assessments.HandleTake)
	orgRoutes.Post("/assessments/{assessmentID}/attempts", routeHandlers.assessments.HandleSubmitAttempt)
	orgRoutes.Get("/me/certificates", routeHandlers.assessments.HandleListMyCertificates)
	orgRoutes.Get("/me/meetings", routeHandlers.meetings.HandleListMine)
	orgRoutes.Post("/me/meetings/{meetingID}/cancel", routeHandlers.meetings.HandleCancelMine)
	orgRoutes.Get("/meetings", permit(permissions, roles.PermissionEmployeesRead, routeHandlers.meetings.HandleList))
	orgRoutes.Post("/meetings", permit(permissions, roles.PermissionMeetingsManage, routeHandlers.meetings.HandleCreate))
	orgRoutes.Post("/meetings/{meetingID}/complete", permit(permissions, roles.PermissionMeetingsManage, routeHandlers.meetings.HandleComplete))
	orgRoutes.Post("/meetings/{meetingID}/cancel", permit(permissions, roles.PermissionMeetingsManage, routeHandlers.meetings.HandleCancel))
	orgRoutes.Patch("/meetings/{meetingID}", permit(permissions, roles.PermissionMeetingsManage, routeHandlers.meetings.HandleReschedule))
	orgRoutes.Get("/calendar/connection", permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.meetings.HandleGetCalendarConnection))
	orgRoutes.Put("/calendar/connection", permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.meetings.HandleConnectCalendar))
	orgRoutes.Delete("/calendar/connection", permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.meetings.HandleDisconnectCalendar))
	orgRoutes.Get("/calendar/oauth/{provider}/start", permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.meetings.HandleStartCalendarOAuth))
	orgRoutes.Get(
		"/organizations/current/export",
		permit(permissions, roles.PermissionDataExport, routeHandlers.privacy.HandleExport),
	)
	orgRoutes.Post(
		"/organizations/current/scim-token",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.scim.HandleGenerateToken),
	)
	orgRoutes.Get(
		"/organizations/current/sso",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.sso.HandleGetConfig),
	)
	orgRoutes.Get(
		"/organizations/current/saml",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.sso.HandleGetSAMLConfig),
	)
	orgRoutes.Put(
		"/organizations/current/saml",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.sso.HandleSetSAMLConfig),
	)
	orgRoutes.Put(
		"/organizations/current/sso",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.sso.HandleSetConfig),
	)

	registerOrgMemberRoutes(orgRoutes, routeHandlers, permissions)
	registerOrgIntegrationRoutes(orgRoutes, routeHandlers, permissions)
	registerOrgPeopleRoutes(orgRoutes, routeHandlers, permissions)
	registerOrgContentRoutes(orgRoutes, routeHandlers, permissions)
}

// registerOrgIntegrationRoutes registers the HRIS integration routes and the
// tenant audit log, each gated by its permission.
func registerOrgIntegrationRoutes(
	orgRoutes chi.Router,
	routeHandlers handlers,
	permissions middleware.PermissionResolver,
) {
	orgRoutes.Get(
		"/organizations/current/hris",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.hris.HandleGetConfig),
	)
	orgRoutes.Put(
		"/organizations/current/hris",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.hris.HandleSetConfig),
	)
	orgRoutes.Post(
		"/organizations/current/hris/sync",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.hris.HandleSync),
	)
	orgRoutes.Get(
		"/organizations/current/hris/directory",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.hris.HandleGetState),
	)
	orgRoutes.Post(
		"/organizations/current/hris/apply",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.hris.HandleApply),
	)

	orgRoutes.Get(
		"/integrations",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.integrations.HandleList),
	)
	orgRoutes.Post(
		"/integrations/{provider}/connect",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.integrations.HandleConnect),
	)
	orgRoutes.Delete(
		"/integrations/{provider}/connect",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.integrations.HandleDisconnect),
	)
	orgRoutes.Post(
		"/integrations/{provider}/health",
		permit(permissions, roles.PermissionIntegrationsManage, routeHandlers.integrations.HandleHealth),
	)

	orgRoutes.Get(
		"/audit-events",
		permit(permissions, roles.PermissionAuditRead, routeHandlers.audit.HandleList),
	)
}

// registerOrgPeopleRoutes registers the people-operations routes: reads are
// open to any org member (handlers keep their existing actor checks), writes
// are gated by their permission.
func registerOrgPeopleRoutes(
	orgRoutes chi.Router,
	routeHandlers handlers,
	permissions middleware.PermissionResolver,
) {
	orgRoutes.Get("/departments", routeHandlers.departments.HandleListDepartments)
	orgRoutes.Post(
		"/departments",
		permit(permissions, roles.PermissionDepartmentsManage, routeHandlers.departments.HandleCreateDepartment),
	)
	orgRoutes.Get("/job-roles", routeHandlers.departments.HandleListJobRoles)
	orgRoutes.Post(
		"/job-roles",
		permit(permissions, roles.PermissionDepartmentsManage, routeHandlers.departments.HandleCreateJobRole),
	)

	orgRoutes.Get("/employees", routeHandlers.employees.HandleList)
	orgRoutes.Get("/me/contacts", routeHandlers.employees.HandleMyContacts)
	orgRoutes.Post("/employees/import", permit(permissions, roles.PermissionEmployeesCreate, routeHandlers.employees.HandleImportCSV))
	orgRoutes.Post(
		"/employees",
		permit(permissions, roles.PermissionEmployeesCreate, routeHandlers.employees.HandleCreate),
	)
	orgRoutes.Get("/employees/{employeeID}", routeHandlers.employees.HandleGet)
	orgRoutes.Patch(
		"/employees/{employeeID}",
		permit(permissions, roles.PermissionEmployeesUpdate, routeHandlers.employees.HandleUpdate),
	)
	orgRoutes.Post(
		"/employees/{employeeID}/provision",
		permit(permissions, roles.PermissionEmployeesUpdate, routeHandlers.employees.HandleProvisionAccess),
	)

	registerOrgJourneyRoutes(orgRoutes, routeHandlers, permissions)

	orgRoutes.Get("/assignments", routeHandlers.assignments.HandleList)
	orgRoutes.Post(
		"/assignments",
		permit(permissions, roles.PermissionJourneysAssign, routeHandlers.assignments.HandleAssign),
	)
	orgRoutes.Post(
		"/assignments/department",
		permit(permissions, roles.PermissionJourneysAssign, routeHandlers.assignments.HandleAssignDepartment),
	)
	registerOrgAssignmentRuleRoutes(orgRoutes, routeHandlers, permissions)
	orgRoutes.Get("/assignments/{assignmentID}", routeHandlers.assignments.HandleGet)
	orgRoutes.Get("/assignments/{assignmentID}/steps", routeHandlers.assignments.HandleListSteps)
	orgRoutes.Get("/me/assignments", routeHandlers.assignments.HandleListMine)
	orgRoutes.Post("/step-assignments/{stepAssignmentID}/complete", routeHandlers.assignments.HandleCompleteStep)
	orgRoutes.Post("/step-assignments/{stepAssignmentID}/start", routeHandlers.assignments.HandleStartStep)
	orgRoutes.Post("/step-assignments/{stepAssignmentID}/submit", routeHandlers.assignments.HandleSubmitStep)
	orgRoutes.Post(
		"/step-assignments/{stepAssignmentID}/override",
		permit(permissions, roles.PermissionAssignmentsManage, routeHandlers.assignments.HandleOverrideStep),
	)

	orgRoutes.Get("/approvals", routeHandlers.assignments.HandleListApprovals)
	orgRoutes.Post(
		"/approvals/{approvalID}/decide",
		permit(permissions, roles.PermissionApprovalsDecide, routeHandlers.assignments.HandleDecideApproval),
	)

	registerOrgManagerRoutes(orgRoutes, routeHandlers, permissions)
	registerOrgNotificationRoutes(orgRoutes, routeHandlers)
}

// registerOrgJourneyRoutes registers journey template and versioning routes.
func registerOrgJourneyRoutes(
	orgRoutes chi.Router,
	routeHandlers handlers,
	permissions middleware.PermissionResolver,
) {
	orgRoutes.Get("/journeys", routeHandlers.journeys.HandleList)
	orgRoutes.Post(
		"/journeys",
		permit(permissions, roles.PermissionJourneysCreate, routeHandlers.journeys.HandleCreate),
	)
	orgRoutes.Get("/journeys/{journeyID}", routeHandlers.journeys.HandleGet)
	orgRoutes.Get("/journeys/{journeyID}/steps", routeHandlers.journeys.HandleListSteps)
	orgRoutes.Post(
		"/journeys/{journeyID}/steps",
		permit(permissions, roles.PermissionJourneysCreate, routeHandlers.journeys.HandleAddStep),
	)
	orgRoutes.Post(
		"/journeys/{journeyID}/publish",
		permit(permissions, roles.PermissionJourneysPublish, routeHandlers.journeys.HandlePublish),
	)
	orgRoutes.Delete(
		"/journeys/{journeyID}/steps/{stepID}",
		permit(permissions, roles.PermissionJourneysCreate, routeHandlers.journeys.HandleDeleteStep),
	)
	orgRoutes.Get("/journeys/{journeyID}/versions", routeHandlers.journeys.HandleListVersions)
	orgRoutes.Post(
		"/journeys/{journeyID}/versions",
		permit(permissions, roles.PermissionJourneysCreate, routeHandlers.journeys.HandleCreateVersion),
	)
	orgRoutes.Post(
		"/journeys/{journeyID}/clone",
		permit(permissions, roles.PermissionJourneysCreate, routeHandlers.journeys.HandleClone),
	)
	orgRoutes.Post(
		"/journeys/{journeyID}/rollback",
		permit(permissions, roles.PermissionJourneysPublish, routeHandlers.journeys.HandleRollback),
	)
}

// registerOrgAssignmentRuleRoutes registers the assignment rule CRUD and run routes.
func registerOrgAssignmentRuleRoutes(
	orgRoutes chi.Router,
	routeHandlers handlers,
	permissions middleware.PermissionResolver,
) {
	orgRoutes.Get(
		"/assignment-rules",
		permit(permissions, roles.PermissionJourneysAssign, routeHandlers.assignmentRules.HandleListRules),
	)
	orgRoutes.Post(
		"/assignment-rules",
		permit(permissions, roles.PermissionJourneysAssign, routeHandlers.assignmentRules.HandleCreateRule),
	)
	orgRoutes.Patch(
		"/assignment-rules/{ruleID}",
		permit(permissions, roles.PermissionJourneysAssign, routeHandlers.assignmentRules.HandleUpdateRule),
	)
	orgRoutes.Delete(
		"/assignment-rules/{ruleID}",
		permit(permissions, roles.PermissionJourneysAssign, routeHandlers.assignmentRules.HandleDeleteRule),
	)
	orgRoutes.Post(
		"/assignment-rules/{ruleID}/run",
		permit(permissions, roles.PermissionJourneysAssign, routeHandlers.assignmentRules.HandleRunRule),
	)
}

// registerOrgRequestRoutes registers equipment and access request routes.
func registerOrgRequestRoutes(
	orgRoutes chi.Router,
	routeHandlers handlers,
	permissions middleware.PermissionResolver,
) {
	orgRoutes.Post("/me/requests", routeHandlers.requests.HandleCreateMine)
	orgRoutes.Get("/me/requests", routeHandlers.requests.HandleListMine)
	orgRoutes.Post("/me/requests/{requestID}/cancel", routeHandlers.requests.HandleCancelMine)
	orgRoutes.Get(
		"/requests",
		permit(permissions, roles.PermissionEmployeesRead, routeHandlers.requests.HandleList),
	)
	orgRoutes.Post(
		"/requests/{requestID}/decide",
		permit(permissions, roles.PermissionApprovalsDecide, routeHandlers.requests.HandleDecide),
	)
	orgRoutes.Post(
		"/requests/{requestID}/fulfill",
		permit(permissions, roles.PermissionApprovalsDecide, routeHandlers.requests.HandleFulfill),
	)
}

// registerOrgManagerRoutes registers the manager-scoped rollup and blocker routes.
func registerOrgManagerRoutes(
	orgRoutes chi.Router,
	routeHandlers handlers,
	permissions middleware.PermissionResolver,
) {
	orgRoutes.Get(
		"/manager/team",
		permit(permissions, roles.PermissionAssignmentsRead, routeHandlers.assignments.HandleTeamRollup),
	)
	orgRoutes.Get(
		"/manager/blockers",
		permit(permissions, roles.PermissionAssignmentsRead, routeHandlers.assignments.HandleListTeamBlockers),
	)
	orgRoutes.Post("/me/blockers", routeHandlers.assignments.HandleReportBlocker)
}

// registerOrgNotificationRoutes registers the notification list/read and
// tenant channel configuration routes.
func registerOrgNotificationRoutes(orgRoutes chi.Router, routeHandlers handlers) {
	orgRoutes.Get("/notifications", routeHandlers.notifications.HandleList)
	orgRoutes.Post("/notifications/{id}/read", routeHandlers.notifications.HandleMarkRead)
	orgRoutes.Get("/notifications/channels", routeHandlers.notifications.HandleGetChannels)
	orgRoutes.Put("/notifications/channels", routeHandlers.notifications.HandleSetChannels)
}

// registerOrgMemberRoutes registers member administration and role management
// routes, each gated by its members.* permission.
func registerOrgMemberRoutes(
	orgRoutes chi.Router,
	routeHandlers handlers,
	permissions middleware.PermissionResolver,
) {
	orgRoutes.Post(
		"/organizations/current/members",
		permit(permissions, roles.PermissionMembersInvite, routeHandlers.orgs.HandleInviteMember),
	)
	orgRoutes.Get(
		"/organizations/current/members",
		permit(permissions, roles.PermissionMembersRead, routeHandlers.orgs.HandleListMembers),
	)
	orgRoutes.Patch(
		"/organizations/current/members/{userID}",
		permit(permissions, roles.PermissionMembersUpdate, routeHandlers.orgs.HandleUpdateMemberRole),
	)
	orgRoutes.Post(
		"/organizations/current/invitations",
		permit(permissions, roles.PermissionMembersInvite, routeHandlers.auth.HandleIssueInvitation),
	)
	orgRoutes.Get(
		"/organizations/current/invitations",
		permit(permissions, roles.PermissionMembersRead, routeHandlers.auth.HandleListInvitations),
	)
	orgRoutes.Post(
		"/organizations/current/invitations/{invitationID}/resend",
		permit(permissions, roles.PermissionMembersInvite, routeHandlers.auth.HandleResendInvitation),
	)
	orgRoutes.Delete(
		"/organizations/current/invitations/{invitationID}",
		permit(permissions, roles.PermissionMembersInvite, routeHandlers.auth.HandleRevokeInvitation),
	)

	orgRoutes.Get(
		"/organizations/current/roles",
		permit(permissions, roles.PermissionMembersRead, routeHandlers.roles.HandleList),
	)
	orgRoutes.Post(
		"/organizations/current/roles",
		permit(permissions, roles.PermissionMembersUpdate, routeHandlers.roles.HandleCreate),
	)
	orgRoutes.Patch(
		"/organizations/current/roles/{roleID}",
		permit(permissions, roles.PermissionMembersUpdate, routeHandlers.roles.HandleUpdate),
	)
	orgRoutes.Delete(
		"/organizations/current/roles/{roleID}",
		permit(permissions, roles.PermissionMembersUpdate, routeHandlers.roles.HandleDelete),
	)
}

func registerOrgContentRoutes(
	orgRoutes chi.Router,
	routeHandlers handlers,
	permissions middleware.PermissionResolver,
) {
	orgRoutes.Get("/knowledge/documents", routeHandlers.knowledge.HandleList)
	orgRoutes.Post(
		"/knowledge/documents",
		permit(permissions, roles.PermissionKnowledgeManage, routeHandlers.knowledge.HandleCreate),
	)
	orgRoutes.Get("/knowledge/documents/{documentID}", routeHandlers.knowledge.HandleGet)
	orgRoutes.Get("/knowledge/documents/{documentID}/history", routeHandlers.knowledge.HandleHistory)
	orgRoutes.Patch(
		"/knowledge/documents/{documentID}",
		permit(permissions, roles.PermissionKnowledgeManage, routeHandlers.knowledge.HandleUpdate),
	)
	orgRoutes.Post(
		"/knowledge/documents/{documentID}/versions",
		permit(permissions, roles.PermissionKnowledgeManage, routeHandlers.knowledge.HandleCreateNewVersion),
	)
	orgRoutes.Post(
		"/knowledge/documents/{documentID}/sync",
		permit(permissions, roles.PermissionKnowledgeManage, routeHandlers.knowledge.HandleSync),
	)
	orgRoutes.Post(
		"/knowledge/documents/{documentID}/approve",
		permit(permissions, roles.PermissionKnowledgeManage, routeHandlers.knowledge.HandleApprove),
	)
	orgRoutes.Post(
		"/knowledge/documents/{documentID}/index",
		permit(permissions, roles.PermissionKnowledgeManage, routeHandlers.knowledge.HandleIndex),
	)
	orgRoutes.Post(
		"/knowledge/documents/{documentID}/archive",
		permit(permissions, roles.PermissionKnowledgeManage, routeHandlers.knowledge.HandleArchive),
	)

	// The assistant fans out to a paid LLM, so it gets its own per-IP budget
	// on top of authentication to bound cost amplification.
	orgRoutes.Group(func(assistantRoutes chi.Router) {
		assistantRoutes.Use(middleware.RateLimit(assistantRateLimit, assistantRateWindow))
		assistantRoutes.Post("/assistant/ask", routeHandlers.assistant.HandleAsk)
		assistantRoutes.Post("/assistant/interactions/{interactionID}/feedback", routeHandlers.assistant.HandleFeedback)
	})

	orgRoutes.Get("/feature-flags", routeHandlers.featureflags.HandleOrgList)
	orgRoutes.Get(
		"/billing/plans",
		permit(permissions, roles.PermissionBillingRead, routeHandlers.billing.HandleOrgListPlans),
	)
	orgRoutes.Get(
		"/billing/subscription",
		permit(permissions, roles.PermissionBillingRead, routeHandlers.billing.HandleOrgGetSubscription),
	)
	orgRoutes.Get("/support/tickets", routeHandlers.support.HandleOrgList)
	registerOrgRequestRoutes(orgRoutes, routeHandlers, permissions)
	orgRoutes.Post("/support/tickets", routeHandlers.support.HandleOrgCreate)
	orgRoutes.Get("/support/tickets/{ticketID}", routeHandlers.support.HandleOrgGet)
	orgRoutes.Post("/support/tickets/{ticketID}/messages", routeHandlers.support.HandleOrgAddMessage)
	orgRoutes.Get(
		"/analytics/onboarding",
		permit(permissions, roles.PermissionAnalyticsRead, routeHandlers.analytics.HandleOnboardingSummary),
	)
	orgRoutes.Get(
		"/analytics/onboarding/breakdown",
		permit(permissions, roles.PermissionAnalyticsRead, routeHandlers.analytics.HandleOnboardingBreakdown),
	)
	orgRoutes.Get(
		"/analytics/onboarding/funnel",
		permit(permissions, roles.PermissionAnalyticsRead, routeHandlers.analytics.HandleFunnelReport),
	)
	orgRoutes.Get(
		"/analytics/assistant",
		permit(permissions, roles.PermissionAnalyticsRead, routeHandlers.analytics.HandleAssistantReport),
	)
}

func newServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	}
}
