package scim

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/pkg/security"
)

const (
	tokenPrefix     = "scim_"
	scimActor       = "scim"
	resourceUser    = "scim_user"
	resourceOrg     = "organization"
	actionToken     = "scim.token.generated" //nolint:gosec // audit action name, not a credential
	actionTokenUse  = "scim.token.used"      //nolint:gosec // audit action name, not a credential
	actionProvision = "scim.user.provisioned"
	actionUpdate    = "scim.user.updated"
	actionDelete    = "scim.user.deprovisioned"

	// TokenTTL bounds how long a provisioning token stays valid. Rotating
	// (regenerating) a token issues a fresh 90-day window; tokens issued
	// before expiry existed carry a zero expiry and stay valid until rotated.
	TokenTTL = 90 * 24 * time.Hour
)

// Service implements SCIM provisioning use cases.
type Service struct {
	store       Store
	tokens      TokenStore
	provisioner Provisioner
	groups      GroupStore
	audit       AuditRecorder
}

// NewService constructs a Service.
func NewService(
	store Store,
	tokens TokenStore,
	provisioner Provisioner,
	groups GroupStore,
	audit AuditRecorder,
) *Service {
	return &Service{
		store:       store,
		tokens:      tokens,
		provisioner: provisioner,
		groups:      groups,
		audit:       audit,
	}
}

// GenerateToken issues a new provisioning token for an organization, replacing
// any previous one, and returns the plaintext token (shown to the caller once)
// plus its expiry.
func (s *Service) GenerateToken(ctx context.Context, organizationID, actorUserID string) (string, time.Time, error) {
	if organizationID == "" {
		return "", time.Time{}, ErrInvalid
	}

	random, err := security.NewRefreshToken()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate scim token: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(TokenTTL)

	token := tokenPrefix + random
	if err := s.tokens.StoreToken(ctx, organizationID, security.HashToken(token), now, expiresAt); err != nil {
		return "", time.Time{}, fmt.Errorf("store scim token: %w", err)
	}

	if err := s.recordAudit(ctx, organizationID, actorUserID, actionToken, resourceOrg, organizationID, map[string]any{
		"expiresAt": expiresAt,
	}); err != nil {
		return "", time.Time{}, err
	}

	return token, expiresAt, nil
}

// ResolveOrganization authenticates a provisioning token and returns its
// tenant. Expired tokens are rejected with the same error as unknown tokens
// (no expiry oracle), and every successful use is audit-logged.
func (s *Service) ResolveOrganization(ctx context.Context, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ErrUnauthorized
	}

	organizationID, expiresAt, err := s.tokens.ResolveOrganization(ctx, security.HashToken(token))
	if err != nil {
		return "", ErrUnauthorized
	}

	if !expiresAt.IsZero() && time.Now().UTC().After(expiresAt) {
		return "", ErrUnauthorized
	}

	if err := s.recordAudit(ctx, organizationID, scimActor, actionTokenUse, resourceOrg, organizationID, nil); err != nil {
		return "", err
	}

	return organizationID, nil
}

// CreateUser provisions a new user into the tenant.
func (s *Service) CreateUser(ctx context.Context, organizationID string, in CreateUserInput) (Record, error) {
	userName := strings.TrimSpace(in.UserName)
	email := strings.ToLower(strings.TrimSpace(in.Email))

	if userName == "" || email == "" || !strings.Contains(email, "@") {
		return Record{}, ErrInvalid
	}

	if _, err := s.store.GetByUserName(ctx, organizationID, userName); err == nil {
		return Record{}, ErrConflict
	} else if !errors.Is(err, ErrNotFound) {
		return Record{}, fmt.Errorf("check existing scim user: %w", err)
	}

	userID, err := s.resolveProvisionedUser(ctx, organizationID, email, displayName(in))
	if err != nil {
		return Record{}, err
	}

	if !in.Active {
		if err := s.provisioner.SetActive(ctx, organizationID, userID, false); err != nil {
			return Record{}, fmt.Errorf("deactivate provisioned user: %w", err)
		}
	}

	now := time.Now().UTC()
	record := Record{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		UserID:         userID,
		ExternalID:     strings.TrimSpace(in.ExternalID),
		UserName:       userName,
		GivenName:      strings.TrimSpace(in.GivenName),
		FamilyName:     strings.TrimSpace(in.FamilyName),
		Active:         in.Active,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.store.Create(ctx, record); err != nil {
		// Compensate: deactivate the membership so the failure cannot leave an
		// active member with no SCIM record. A retry reuses the account and
		// reactivates the membership (see resolveProvisionedUser).
		if activeErr := s.provisioner.SetActive(ctx, organizationID, userID, false); activeErr != nil {
			slog.ErrorContext(ctx, "compensating scim membership deactivation failed",
				"organizationId", organizationID, "userId", userID, "error", activeErr)
		}

		return Record{}, fmt.Errorf("store scim user: %w", err)
	}

	if err := s.recordAudit(ctx, organizationID, scimActor, actionProvision, resourceUser, record.ID, map[string]any{
		"userName": record.UserName,
		"active":   record.Active,
	}); err != nil {
		return Record{}, err
	}

	return record, nil
}

// GetUser returns a SCIM user record by resource id within the tenant.
func (s *Service) GetUser(ctx context.Context, organizationID, id string) (Record, error) {
	record, err := s.store.GetByID(ctx, organizationID, id)
	if err != nil {
		return Record{}, wrapLookup(err)
	}

	return record, nil
}

// ListUsers returns a page of tenant users (optionally filtered by an exact
// userName) plus the total number of matching users.
func (s *Service) ListUsers(
	ctx context.Context,
	organizationID, userNameFilter string,
	offset, limit int,
) ([]Record, int, error) {
	records, total, err := s.store.List(ctx, organizationID, strings.TrimSpace(userNameFilter), offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list scim users: %w", err)
	}

	return records, total, nil
}

// ReplaceUser applies a full-resource update (SCIM PUT).
func (s *Service) ReplaceUser(ctx context.Context, organizationID, id string, in CreateUserInput) (Record, error) {
	record, err := s.store.GetByID(ctx, organizationID, id)
	if err != nil {
		return Record{}, wrapLookup(err)
	}

	record.ExternalID = strings.TrimSpace(in.ExternalID)
	record.GivenName = strings.TrimSpace(in.GivenName)
	record.FamilyName = strings.TrimSpace(in.FamilyName)

	return s.applyActiveAndSave(ctx, record, in.Active)
}

// SetActive applies an active/inactive change (SCIM PATCH), (de)provisioning access.
func (s *Service) SetActive(ctx context.Context, organizationID, id string, active bool) (Record, error) {
	record, err := s.store.GetByID(ctx, organizationID, id)
	if err != nil {
		return Record{}, wrapLookup(err)
	}

	return s.applyActiveAndSave(ctx, record, active)
}

// DeleteUser deprovisions a user and removes the SCIM record.
func (s *Service) DeleteUser(ctx context.Context, organizationID, id string) error {
	record, err := s.store.GetByID(ctx, organizationID, id)
	if err != nil {
		return wrapLookup(err)
	}

	if err := s.provisioner.SetActive(ctx, organizationID, record.UserID, false); err != nil {
		return fmt.Errorf("deprovision user: %w", err)
	}

	if err := s.store.Delete(ctx, organizationID, id); err != nil {
		return fmt.Errorf("delete scim user: %w", err)
	}

	return s.recordAudit(ctx, organizationID, scimActor, actionDelete, resourceUser, id, map[string]any{
		"userName": record.UserName,
	})
}

// resolveProvisionedUser returns the user id to associate with a SCIM record.
// It only creates a brand-new account, or re-manages an account that is ALREADY
// a member of THIS organization (the deprovision→reprovision cycle). If the
// email belongs to an account that is not a member of this organization, it
// returns ErrConflict rather than grafting a foreign-tenant account into the
// caller's organization.
func (s *Service) resolveProvisionedUser(
	ctx context.Context,
	organizationID, email, displayName string,
) (string, error) {
	userID, err := s.provisioner.CreateAccount(ctx, email, displayName)
	if err == nil {
		if addErr := s.provisioner.AddMember(ctx, organizationID, userID); addErr != nil {
			return "", fmt.Errorf("add membership: %w", addErr)
		}

		return userID, nil
	}

	if !errors.Is(err, ErrAccountExists) {
		return "", fmt.Errorf("create account: %w", err)
	}

	userID, err = s.provisioner.FindOrgMember(ctx, organizationID, email)
	if err != nil {
		if errors.Is(err, ErrNotOrgMember) {
			return "", ErrConflict
		}

		return "", fmt.Errorf("find organization member: %w", err)
	}

	if err := s.provisioner.SetActive(ctx, organizationID, userID, true); err != nil {
		return "", fmt.Errorf("reactivate membership: %w", err)
	}

	return userID, nil
}

func (s *Service) applyActiveAndSave(ctx context.Context, record Record, active bool) (Record, error) {
	if err := s.provisioner.SetActive(ctx, record.OrganizationID, record.UserID, active); err != nil {
		return Record{}, fmt.Errorf("set user active: %w", err)
	}

	record.Active = active
	record.UpdatedAt = time.Now().UTC()

	if err := s.store.Update(ctx, record); err != nil {
		return Record{}, fmt.Errorf("update scim user: %w", err)
	}

	if err := s.recordAudit(ctx, record.OrganizationID, scimActor, actionUpdate, resourceUser, record.ID, map[string]any{
		"active": record.Active,
	}); err != nil {
		return Record{}, err
	}

	return record, nil
}

func (s *Service) recordAudit(
	ctx context.Context,
	organizationID, actorUserID, action, resourceType, resourceID string,
	metadata map[string]any,
) error {
	orgID := organizationID
	if err := s.audit.Record(ctx, &orgID, actorUserID, action, resourceType, resourceID, metadata); err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}

	return nil
}

func wrapLookup(err error) error {
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}

	return fmt.Errorf("load scim user: %w", err)
}

func displayName(in CreateUserInput) string {
	if name := strings.TrimSpace(in.DisplayName); name != "" {
		return name
	}

	full := strings.TrimSpace(strings.TrimSpace(in.GivenName) + " " + strings.TrimSpace(in.FamilyName))
	if full != "" {
		return full
	}

	return strings.TrimSpace(in.UserName)
}
