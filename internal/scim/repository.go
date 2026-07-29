package scim

import (
	"context"
	"time"
)

// Store persists SCIM user records, tenant-scoped by organizationID.
type Store interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, record Record) error
	GetByID(ctx context.Context, organizationID, id string) (Record, error)
	GetByUserName(ctx context.Context, organizationID, userName string) (Record, error)
	// List returns a page of tenant records (sorted by userName) plus the total
	// number of matching records. limit <= 0 returns no records, only the total.
	List(ctx context.Context, organizationID, userNameFilter string, offset, limit int) ([]Record, int, error)
	Update(ctx context.Context, record Record) error
	Delete(ctx context.Context, organizationID, id string) error
	// DeleteForOrganization removes every SCIM user record of the
	// organization and returns the number deleted. Called only by the
	// platform GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// GroupStore persists SCIM group records, tenant-scoped by organizationID.
type GroupStore interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, group GroupRecord) error
	GetByID(ctx context.Context, organizationID, id string) (GroupRecord, error)
	GetByDisplayName(ctx context.Context, organizationID, displayName string) (GroupRecord, error)
	// List returns a page of tenant groups (sorted by displayName) plus the total
	// number of matching groups. limit <= 0 returns no groups, only the total.
	List(ctx context.Context, organizationID, displayNameFilter string, offset, limit int) ([]GroupRecord, int, error)
	Update(ctx context.Context, group GroupRecord) error
	Delete(ctx context.Context, organizationID, id string) error
	// DeleteForOrganization removes every SCIM group record of the
	// organization and returns the number deleted. Called only by the
	// platform GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// TokenStore persists per-organization SCIM provisioning tokens (hashed).
type TokenStore interface {
	EnsureIndexes(ctx context.Context) error
	// StoreToken replaces any existing token for the organization with tokenHash.
	StoreToken(ctx context.Context, organizationID, tokenHash string, issuedAt, expiresAt time.Time) error
	// ResolveOrganization returns the organization a token hash belongs to,
	// plus the token's expiry (zero for tokens issued before expiry existed).
	ResolveOrganization(ctx context.Context, tokenHash string) (string, time.Time, error)
	// DeleteForOrganization removes every provisioning token of the
	// organization and returns the number deleted. Called only by the
	// platform GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// Provisioner exposes the primitive account/membership operations SCIM needs.
// The service composes them; the cross-tenant safety decision lives in the
// service (CreateUser), not in the adapter, so it can be tested.
type Provisioner interface {
	// CreateAccount provisions a brand-new user account. It returns
	// ErrAccountExists if an account with the email already exists (in which
	// case the caller must NOT assume ownership of that account).
	CreateAccount(ctx context.Context, email, displayName string) (string, error)
	// FindOrgMember returns the user id when the email already belongs to a
	// member of the organization (any status), else ErrNotOrgMember.
	FindOrgMember(ctx context.Context, organizationID, email string) (string, error)
	// AddMember adds a freshly created user to the organization.
	AddMember(ctx context.Context, organizationID, userID string) error
	// SetActive activates or deactivates the user's membership in the org.
	SetActive(ctx context.Context, organizationID, userID string, active bool) error
}

// AuditRecorder writes audit events for privileged SCIM actions. *audit.Service
// satisfies it; the port keeps the SCIM service decoupled and testable.
type AuditRecorder interface {
	Record(
		ctx context.Context,
		organizationID *string,
		actorUserID, action, resourceType, resourceID string,
		metadata map[string]any,
	) error
}
