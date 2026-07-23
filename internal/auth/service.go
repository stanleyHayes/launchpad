package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/audit"
	"launchpad/internal/organizations"
	"launchpad/pkg/security"
)

const fieldEmail = "email"

// Config holds auth service settings.
type Config struct {
	JWTSecret      string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	PasswordMinLen int
}

// PlatformStaffReader loads platform staff roles for authentication.
type PlatformStaffReader interface {
	GetByUserID(ctx context.Context, userID string) (roleCode string, err error)
}

// Service implements authentication use cases.
type Service struct {
	users         UserRepository
	orgs          *organizations.Service
	audit         *audit.Service
	sessions      SessionRepository
	platformStaff PlatformStaffReader
	cfg           Config
}

// Result is returned after successful registration or login.
type Result struct {
	User         UserPublic                  `json:"user"`
	Organization *organizations.Organization `json:"organization"`
	Tokens       TokenPair                   `json:"tokens"`
}

// NewService constructs an auth Service.
func NewService(
	users UserRepository,
	orgs *organizations.Service,
	auditSvc *audit.Service,
	sessions SessionRepository,
	cfg Config,
	platformStaff PlatformStaffReader,
) *Service {
	return &Service{
		users:         users,
		orgs:          orgs,
		audit:         auditSvc,
		sessions:      sessions,
		platformStaff: platformStaff,
		cfg:           cfg,
	}
}

// Register creates a user, organization, and session.
func (s *Service) Register(ctx context.Context, in RegisterInput) (Result, error) {
	email, displayName, organizationName, err := s.validateRegistration(in)
	if err != nil {
		return Result{}, err
	}

	user, err := s.createUser(ctx, email, displayName, in.Password)
	if err != nil {
		return Result{}, err
	}

	org, err := s.createOrganization(ctx, in, organizationName, user.ID)
	if err != nil {
		return Result{}, err
	}

	if err := s.recordRegistration(ctx, user, org); err != nil {
		return Result{}, err
	}

	tokens, err := s.issueTokens(ctx, user, org.ID, roleOrganizationOwner)
	if err != nil {
		return Result{}, fmt.Errorf("issue registration tokens: %w", err)
	}

	return Result{User: toPublic(user), Organization: &org, Tokens: tokens}, nil
}

// Login authenticates a user and returns tokens.
func (s *Service) Login(ctx context.Context, in LoginInput) (Result, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return Result{}, ErrInvalidCredentials
	}

	if !security.CheckPassword(user.PasswordHash, in.Password) {
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
	if err != nil {
		return Result{}, ErrInvalidCredentials
	}

	if err := s.audit.Record(ctx, &orgID, user.ID, "auth.login", "user", user.ID, nil); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	tokens, err := s.issueTokens(ctx, user, org.ID, roleCode)
	if err != nil {
		return Result{}, fmt.Errorf("issue login tokens: %w", err)
	}

	return Result{User: toPublic(user), Organization: &org, Tokens: tokens}, nil
}

// Refresh rotates tokens for a valid refresh session.
func (s *Service) Refresh(ctx context.Context, sessionID, refreshToken string) (TokenPair, error) {
	userID, orgID, storedHash, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return TokenPair{}, ErrSessionInvalid
	}

	if security.HashToken(refreshToken) != storedHash {
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

		roleCode = membership.RoleCode
	}

	if err := s.sessions.Delete(ctx, sessionID); err != nil {
		return TokenPair{}, fmt.Errorf("delete old session: %w", err)
	}

	return s.issueTokens(ctx, user, orgID, roleCode)
}

// Logout revokes the current session.
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if err := s.sessions.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("logout: %w", err)
	}

	return nil
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
			"sessionId":    principal.SessionID,
		}, nil
	}

	org, err := s.orgs.Get(ctx, principal.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("load organization: %w", err)
	}

	return map[string]any{
		"user":         toPublic(user),
		"organization": org,
		"roleCode":     principal.RoleCode,
		"sessionId":    principal.SessionID,
	}, nil
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
	hash, err := security.HashPassword(password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	user := User{
		ID:           uuid.NewString(),
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: hash,
		Status:       userStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
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

	if err := s.audit.Record(ctx, nil, user.ID, "auth.login", "user", user.ID, nil); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	tokens, err := s.issueTokens(ctx, user, "", roleCode)
	if err != nil {
		return Result{}, fmt.Errorf("issue platform login tokens: %w", err)
	}

	return Result{User: toPublic(user), Organization: nil, Tokens: tokens}, nil
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
