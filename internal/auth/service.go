package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/audit"
	"launchpad/internal/organizations"
	"launchpad/pkg/security"
)

const fieldEmail = "email"

// fieldSessionID is the me-response key carrying the auth session id (or,
// under impersonation, the support session id).
const fieldSessionID = "sessionId"

// Config holds auth service settings.
type Config struct {
	JWTSecret      string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	InviteTTL      time.Duration
	PasswordMinLen int
}

// PlatformStaffReader loads platform staff roles for authentication.
type PlatformStaffReader interface {
	GetByUserID(ctx context.Context, userID string) (roleCode string, err error)
}

// Service implements authentication use cases.
type Service struct {
	users          UserRepository
	orgs           OrgDirectory
	audit          *audit.Service
	sessions       SessionRepository
	invitations    InvitationStore
	passwordResets PasswordResetStore
	mfa            MFAStore
	mfaTickets     MFATicketStore
	platformStaff  PlatformStaffReader
	permissions    PermissionResolver
	mailer         MailSender
	// inviteAcceptBaseURL and passwordResetBaseURL are the link bases emailed
	// to users (the token is appended as ?token=...).
	inviteAcceptBaseURL  string
	passwordResetBaseURL string
	cfg                  Config
}

// PermissionResolver resolves a role code to its effective permission set.
// Satisfied by roles.Service; the auth package only reads through it.
type PermissionResolver interface {
	ResolvePermissions(ctx context.Context, organizationID, roleCode string) (map[string]struct{}, error)
}

// MailSender sends transactional email. Satisfied by email.Sender; the auth
// package depends on the interface so tests can substitute a fake.
type MailSender interface {
	Send(ctx context.Context, to, subject, html string) error
}

// Result is returned after successful registration or login.
type Result struct {
	User         UserPublic          `json:"user"`
	Organization *OrganizationPublic `json:"organization"`
	Tokens       TokenPair           `json:"tokens"`
	// MFARequired and MFATicket are set INSTEAD of user/organization/tokens
	// when the account has MFA enabled: the password was valid but the second
	// factor is still owed. The single-use ticket is exchanged via
	// CompleteMFALogin (POST /auth/login/mfa).
	MFARequired bool   `json:"mfaRequired,omitempty"`
	MFATicket   string `json:"mfaTicket,omitempty"`
}

// NewService constructs an auth Service.
func NewService(
	users UserRepository,
	orgs OrgDirectory,
	auditSvc *audit.Service,
	sessions SessionRepository,
	invitations InvitationStore,
	cfg Config,
	platformStaff PlatformStaffReader,
) *Service {
	return &Service{
		users:                users,
		orgs:                 orgs,
		audit:                auditSvc,
		sessions:             sessions,
		invitations:          invitations,
		passwordResets:       nil,
		mfa:                  nil,
		mfaTickets:           nil,
		platformStaff:        platformStaff,
		permissions:          nil,
		mailer:               nil,
		inviteAcceptBaseURL:  "",
		passwordResetBaseURL: "",
		cfg:                  cfg,
	}
}

// WithPermissionResolver attaches the RBAC resolver so Me can report the
// caller's effective permissions. Chainable; nil-safe (Me omits permissions).
func (s *Service) WithPermissionResolver(resolver PermissionResolver) *Service {
	s.permissions = resolver

	return s
}

// WithPasswordResets attaches the password-reset token store. Chainable;
// without it the password-reset use cases refuse to run.
func (s *Service) WithPasswordResets(resets PasswordResetStore) *Service {
	s.passwordResets = resets

	return s
}

// WithMFA attaches the MFA enrollment and login-ticket stores. Chainable;
// without them the MFA use cases refuse to run and logins never challenge
// for a second factor.
func (s *Service) WithMFA(mfa MFAStore, tickets MFATicketStore) *Service {
	s.mfa = mfa
	s.mfaTickets = tickets

	return s
}

// WithMailer attaches the transactional mail sender and the base URLs used to
// build invitation-accept and password-reset links. Chainable; nil-safe —
// without a mailer no email is sent and tokens keep being returned in API
// responses only.
func (s *Service) WithMailer(mailer MailSender, inviteAcceptBaseURL, passwordResetBaseURL string) *Service {
	s.mailer = mailer
	s.inviteAcceptBaseURL = strings.TrimRight(inviteAcceptBaseURL, "/")
	s.passwordResetBaseURL = strings.TrimRight(passwordResetBaseURL, "/")

	return s
}

// Register creates a user, organization, and session.
func (s *Service) Register(ctx context.Context, in RegisterInput) (Result, error) {
	email, displayName, organizationName, err := s.validateRegistration(in)
	if err != nil {
		return Result{}, err
	}

	// Reject taken emails up front so a failed attempt burns neither the
	// email nor the organization slug.
	if _, err := s.users.GetByEmail(ctx, email); err == nil {
		return Result{}, ErrEmailTaken
	}

	user, err := s.buildUser(email, displayName, in.Password)
	if err != nil {
		return Result{}, err
	}

	// The organization is created before the user: an org-creation failure
	// must not leave an orphan user whose email then blocks re-registration.
	org, err := s.createOrganization(ctx, in, organizationName, user.ID)
	if err != nil {
		return Result{}, err
	}

	if err := s.users.Create(ctx, user); err != nil {
		return Result{}, fmt.Errorf("create user: %w", err)
	}

	if err := s.recordRegistration(ctx, user, org); err != nil {
		return Result{}, err
	}

	tokens, err := s.issueTokens(ctx, user, org.ID, roleOrganizationOwner)
	if err != nil {
		return Result{}, fmt.Errorf("issue registration tokens: %w", err)
	}

	return Result{User: toPublic(user), Organization: toOrganizationPublicPtr(org), Tokens: tokens}, nil
}

// Login authenticates a user and returns tokens.
func (s *Service) Login(ctx context.Context, in LoginInput) (Result, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		s.recordLoginFailure(ctx, "", email)

		return Result{}, ErrInvalidCredentials
	}

	if !security.CheckPassword(user.PasswordHash, in.Password) {
		s.recordLoginFailure(ctx, user.ID, email)

		return Result{}, ErrInvalidCredentials
	}

	organizationID := strings.TrimSpace(in.OrganizationID)

	orgID, roleCode, err := s.resolveLoginMembership(ctx, user.ID, organizationID)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) && organizationID == "" {
			return s.loginAsPlatformStaff(ctx, user)
		}

		return Result{}, fmt.Errorf("resolve login membership: %w", err)
	}

	org, err := s.orgs.Get(ctx, orgID)
	if err != nil || !organizationAllowsLogin(org.Status) {
		s.recordLoginFailure(ctx, user.ID, email)

		return Result{}, ErrInvalidCredentials
	}

	mfaRequired, err := s.mfaRequiredFor(ctx, orgID, user.ID)
	if err != nil {
		return Result{}, err
	}

	if mfaRequired {
		return s.challengeMFA(ctx, user, orgID, roleCode)
	}

	if err := s.audit.Record(ctx, &orgID, user.ID, "auth.login", "user", user.ID, nil); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	tokens, err := s.issueTokens(ctx, user, org.ID, roleCode)
	if err != nil {
		return Result{}, fmt.Errorf("issue login tokens: %w", err)
	}

	return Result{User: toPublic(user), Organization: toOrganizationPublicPtr(org), Tokens: tokens}, nil
}

// FederatedLogin issues a session for an already-provisioned user who has been
// authenticated by an external identity provider (SSO). Unlike Login it takes
// no password; the user must already exist and be an active member of the
// organization (e.g. provisioned via SCIM or invited).
func (s *Service) FederatedLogin(ctx context.Context, email, organizationID string) (Result, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	organizationID = strings.TrimSpace(organizationID)
	if email == "" || organizationID == "" {
		return Result{}, ErrInvalidInput
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return Result{}, ErrNotProvisioned
	}

	membership, err := s.orgs.Membership(ctx, organizationID, user.ID)
	if err != nil {
		return Result{}, ErrNotProvisioned
	}

	org, err := s.orgs.Get(ctx, organizationID)
	if err != nil || !organizationAllowsLogin(org.Status) {
		return Result{}, ErrNotProvisioned
	}

	if err := s.audit.Record(ctx, &organizationID, user.ID, "auth.sso_login", "user", user.ID, nil); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	tokens, err := s.issueTokens(ctx, user, org.ID, membership.RoleCode)
	if err != nil {
		return Result{}, fmt.Errorf("issue sso tokens: %w", err)
	}

	return Result{User: toPublic(user), Organization: toOrganizationPublicPtr(org), Tokens: tokens}, nil
}

// Refresh rotates tokens for a valid refresh session.
func (s *Service) Refresh(ctx context.Context, sessionID, refreshToken string) (TokenPair, error) {
	userID, orgID, storedHash, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return TokenPair{}, ErrSessionInvalid
	}

	if subtle.ConstantTimeCompare([]byte(security.HashToken(refreshToken)), []byte(storedHash)) != 1 {
		if delErr := s.sessions.Delete(ctx, sessionID); delErr != nil {
			return TokenPair{}, fmt.Errorf("%w: revoke session: %w", ErrSessionInvalid, delErr)
		}

		return TokenPair{}, ErrSessionInvalid
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return TokenPair{}, ErrSessionInvalid
	}

	var roleCode string

	if orgID == "" {
		if s.platformStaff == nil {
			return TokenPair{}, ErrSessionInvalid
		}

		roleCode, err = s.platformStaff.GetByUserID(ctx, userID)
		if err != nil {
			return TokenPair{}, ErrSessionInvalid
		}
	} else {
		membership, membershipErr := s.orgs.Membership(ctx, orgID, userID)
		if membershipErr != nil {
			return TokenPair{}, ErrSessionInvalid
		}

		org, orgErr := s.orgs.Get(ctx, orgID)
		if orgErr != nil || !organizationAllowsLogin(org.Status) {
			return TokenPair{}, ErrSessionInvalid
		}

		roleCode = membership.RoleCode
	}

	if err := s.sessions.Delete(ctx, sessionID); err != nil {
		return TokenPair{}, fmt.Errorf("delete old session: %w", err)
	}

	return s.issueTokens(ctx, user, orgID, roleCode)
}

func organizationAllowsLogin(status string) bool {
	return status == organizations.StatusActive() || status == organizations.StatusTrial()
}

// Logout revokes the current session.
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if err := s.sessions.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("logout: %w", err)
	}

	return nil
}

// ListOrganizations returns the caller's active tenant memberships with
// display metadata for the workspace switcher.
func (s *Service) ListOrganizations(ctx context.Context, userID string) ([]OrganizationChoice, error) {
	memberships, err := s.orgs.ListMembershipsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	out := make([]OrganizationChoice, 0, len(memberships))
	for _, membership := range memberships {
		org, getErr := s.orgs.Get(ctx, membership.OrganizationID)
		if getErr != nil || !organizationAllowsLogin(org.Status) {
			continue
		}
		out = append(out, OrganizationChoice{
			Organization: toOrganizationPublic(org),
			RoleCode:     membership.RoleCode,
		})
	}
	return out, nil
}

// SwitchOrganization replaces the caller's current session with one scoped
// to another active membership. It never accepts a role from the client.
func (s *Service) SwitchOrganization(
	ctx context.Context,
	principal security.Principal,
	organizationID string,
) (Result, error) {
	organizationID = strings.TrimSpace(organizationID)
	if principal.UserID == "" || principal.SessionID == "" || organizationID == "" || principal.Impersonator {
		return Result{}, ErrInvalidInput
	}
	membership, err := s.orgs.Membership(ctx, organizationID, principal.UserID)
	if err != nil {
		return Result{}, ErrInvalidCredentials
	}
	org, err := s.orgs.Get(ctx, organizationID)
	if err != nil || !organizationAllowsLogin(org.Status) {
		return Result{}, ErrInvalidCredentials
	}
	user, err := s.users.GetByID(ctx, principal.UserID)
	if err != nil {
		return Result{}, ErrInvalidCredentials
	}
	if err := s.audit.Record(ctx, &organizationID, user.ID, "auth.organization_switched", "organization", organizationID, map[string]any{
		"fromOrganizationId": principal.OrganizationID,
	}); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}
	if err := s.sessions.Delete(ctx, principal.SessionID); err != nil {
		return Result{}, fmt.Errorf("revoke previous session: %w", err)
	}
	tokens, err := s.issueTokens(ctx, user, org.ID, membership.RoleCode)
	if err != nil {
		return Result{}, fmt.Errorf("issue switched session: %w", err)
	}
	return Result{User: toPublic(user), Organization: toOrganizationPublicPtr(org), Tokens: tokens}, nil
}

// Me returns the authenticated profile.
func (s *Service) Me(ctx context.Context, principal security.Principal) (map[string]any, error) {
	user, err := s.users.GetByID(ctx, principal.UserID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}

	if principal.OrganizationID == "" {
		return map[string]any{
			"user":         toPublic(user),
			"organization": nil,
			"roleCode":     principal.RoleCode,
			fieldSessionID: principal.SessionID,
			"mfaEnabled":   user.MFAEnabled,
		}, nil
	}

	org, err := s.orgs.Get(ctx, principal.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("load organization: %w", err)
	}

	profile := map[string]any{
		"user":         toPublic(user),
		"organization": toOrganizationPublic(org),
		"roleCode":     principal.RoleCode,
		fieldSessionID: principal.SessionID,
		"mfaEnabled":   user.MFAEnabled,
		"permissions":  s.resolvePermissions(ctx, principal),
	}

	// Requests running under a platform support impersonation token expose the
	// session context so the tenant portal can banner the support access
	// (PRD 5.2.2). The key is absent on ordinary sessions.
	if principal.Impersonator {
		profile["impersonation"] = map[string]any{
			fieldSessionID: principal.ImpersonationSessionID,
			"agentUserId":  principal.UserID,
		}
	}

	return profile, nil
}

// GetUserByEmail loads a user by email for bootstrap and admin flows.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (User, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return User{}, fmt.Errorf("get user by email: %w", err)
	}

	return user, nil
}

// CreateUserAccount creates an active user account with email/password.
func (s *Service) CreateUserAccount(ctx context.Context, email, displayName, password string) (User, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))

	normalizedName := strings.TrimSpace(displayName)
	if normalizedEmail == "" || normalizedName == "" || !strings.Contains(normalizedEmail, "@") {
		return User{}, ErrInvalidInput
	}

	if len(password) < s.cfg.PasswordMinLen {
		return User{}, ErrWeakPassword
	}

	return s.createUser(ctx, normalizedEmail, normalizedName, password)
}

func (s *Service) validateRegistration(in RegisterInput) (string, string, string, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	displayName := strings.TrimSpace(in.DisplayName)
	organizationName := strings.TrimSpace(in.OrganizationName)

	if email == "" || displayName == "" || organizationName == "" || !strings.Contains(email, "@") {
		return "", "", "", ErrInvalidInput
	}

	if len(in.Password) < s.cfg.PasswordMinLen {
		return "", "", "", ErrWeakPassword
	}

	return email, displayName, organizationName, nil
}

func (s *Service) createUser(ctx context.Context, email, displayName, password string) (User, error) {
	user, err := s.buildUser(email, displayName, password)
	if err != nil {
		return User{}, err
	}

	if err := s.users.Create(ctx, user); err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

// buildUser constructs (but does not persist) an active user account.
func (s *Service) buildUser(email, displayName, password string) (User, error) {
	hash, err := security.HashPassword(password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()

	return User{
		ID:           uuid.NewString(),
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: hash,
		Status:       userStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (s *Service) createOrganization(
	ctx context.Context,
	in RegisterInput,
	organizationName, ownerID string,
) (organizations.Organization, error) {
	slug := strings.TrimSpace(in.OrganizationSlug)
	if slug == "" {
		slug = organizations.Slugify(organizationName)
	}

	org, _, err := s.orgs.CreateWithOwner(ctx, organizations.CreateInput{
		Name:     organizationName,
		Slug:     slug,
		Timezone: in.Timezone,
		OwnerID:  ownerID,
	})
	if err != nil {
		return organizations.Organization{}, fmt.Errorf("create organization: %w", err)
	}

	return org, nil
}

func (s *Service) recordRegistration(ctx context.Context, user User, org organizations.Organization) error {
	orgID := org.ID
	if err := s.audit.Record(ctx, &orgID, user.ID, "auth.register", "organization", org.ID, map[string]any{
		fieldEmail: user.Email,
		"slug":     org.Slug,
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	return nil
}

func (s *Service) resolveLoginMembership(
	ctx context.Context,
	userID, organizationID string,
) (string, string, error) {
	if organizationID == "" {
		memberships, err := s.orgs.ListMembershipsForUser(ctx, userID)
		if err != nil {
			return "", "", fmt.Errorf("list memberships: %w", err)
		}

		if len(memberships) == 0 {
			return "", "", ErrInvalidCredentials
		}

		// Pick deterministically when the user belongs to several
		// organizations; membership list order is storage-dependent.
		sort.Slice(memberships, func(i, j int) bool {
			return memberships[i].OrganizationID < memberships[j].OrganizationID
		})

		return memberships[0].OrganizationID, memberships[0].RoleCode, nil
	}

	membership, err := s.orgs.Membership(ctx, organizationID, userID)
	if err != nil {
		return "", "", ErrInvalidCredentials
	}

	return organizationID, membership.RoleCode, nil
}

func (s *Service) loginAsPlatformStaff(ctx context.Context, user User) (Result, error) {
	if s.platformStaff == nil {
		return Result{}, ErrInvalidCredentials
	}

	roleCode, err := s.platformStaff.GetByUserID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, ErrPlatformStaffNotFound) {
			return Result{}, ErrInvalidCredentials
		}

		return Result{}, fmt.Errorf("load platform staff: %w", err)
	}

	// Platform staff are not hard-locked when MFA is not enrolled (they get a
	// setup prompt in settings instead); an ENABLED enrollment always
	// challenges.
	mfaRequired, err := s.mfaRequiredFor(ctx, "", user.ID)
	if err != nil {
		return Result{}, err
	}

	if mfaRequired {
		return s.challengeMFA(ctx, user, "", roleCode)
	}

	if err := s.audit.Record(ctx, nil, user.ID, "auth.login", "user", user.ID, nil); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	tokens, err := s.issueTokens(ctx, user, "", roleCode)
	if err != nil {
		return Result{}, fmt.Errorf("issue platform login tokens: %w", err)
	}

	return Result{User: toPublic(user), Organization: nil, Tokens: tokens}, nil
}

// recordLoginFailure audits a rejected login attempt so credential-guessing
// shows up in the audit log. The actor is the matched account when known (""
// when the email is not registered). The write is best-effort: a broken audit
// store must never change the generic invalid-credentials response.
func (s *Service) recordLoginFailure(ctx context.Context, userID, email string) {
	err := s.audit.RecordResult(
		ctx, nil, userID, "auth.login", "user", userID,
		audit.ResultFailure, "invalid_credentials",
		map[string]any{fieldEmail: email},
	)
	if err != nil {
		slog.WarnContext(ctx, "audit failed login attempt", "error", err)
	}
}

func (s *Service) issueTokens(ctx context.Context, user User, orgID, roleCode string) (TokenPair, error) {
	sessionID := uuid.NewString()

	refresh, err := security.NewRefreshToken()
	if err != nil {
		return TokenPair{}, fmt.Errorf("create refresh token: %w", err)
	}

	if err := s.sessions.Save(ctx, sessionID, user.ID, orgID, security.HashToken(refresh)); err != nil {
		return TokenPair{}, fmt.Errorf("save refresh session: %w", err)
	}

	access, err := security.IssueAccessToken(s.cfg.JWTSecret, s.cfg.AccessTTL, security.Principal{
		UserID:         user.ID,
		Email:          user.Email,
		OrganizationID: orgID,
		RoleCode:       roleCode,
		SessionID:      sessionID,
	})
	if err != nil {
		return TokenPair{}, fmt.Errorf("issue access token: %w", err)
	}

	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh + "." + sessionID,
		TokenType:    tokenTypeBearer,
		ExpiresIn:    int64(s.cfg.AccessTTL.Seconds()),
	}, nil
}

// ParseRefreshToken splits the combined refresh token payload.
func ParseRefreshToken(combined string) (string, string, error) {
	parts := strings.Split(combined, ".")
	if len(parts) != refreshTokenPartsExpected || parts[0] == "" || parts[1] == "" {
		return "", "", ErrSessionInvalid
	}

	return parts[0], parts[1], nil
}

// resolvePermissions returns the caller's effective permission list, sorted
// for a stable response. Resolution failures degrade to an empty list rather
// than failing the profile read. Support impersonation principals bypass the
// resolver and hold exactly the fixed read-only set, matching the
// authorization middleware.
func (s *Service) resolvePermissions(ctx context.Context, principal security.Principal) []string {
	if principal.Impersonator {
		set := security.ImpersonatorPermissions()
		permissions := make([]string, 0, len(set))
		for permission := range set {
			permissions = append(permissions, permission)
		}

		sort.Strings(permissions)

		return permissions
	}

	if s.permissions == nil {
		return []string{}
	}

	set, err := s.permissions.ResolvePermissions(ctx, principal.OrganizationID, principal.RoleCode)
	if err != nil {
		slog.ErrorContext(ctx, "resolve permissions for me", "error", err)

		return []string{}
	}

	permissions := make([]string, 0, len(set))
	for permission := range set {
		permissions = append(permissions, permission)
	}

	sort.Strings(permissions)

	return permissions
}
