package integrations

import "context"

// Repository persists integration connections, always organization-scoped.
// One connection per (organization, provider), enforced by a unique index.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	// Upsert creates or replaces the organization's connection for conn.Provider.
	Upsert(ctx context.Context, conn Connection) error
	// Get returns the organization's connection for provider, or ErrNotFound.
	Get(ctx context.Context, organizationID, provider string) (Connection, error)
	// List returns all of the organization's connections.
	List(ctx context.Context, organizationID string) ([]Connection, error)
	// Delete removes the organization's connection for provider, or ErrNotFound.
	Delete(ctx context.Context, organizationID, provider string) error
	// DeleteForOrganization removes every integration connection of the
	// organization and returns the number deleted. Called only by the
	// platform GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// AuditRecorder writes audit events for privileged integration actions.
// *audit.Service satisfies it; the port keeps this package decoupled and
// testable.
type AuditRecorder interface {
	Record(
		ctx context.Context,
		organizationID *string,
		actorUserID, action, resourceType, resourceID string,
		metadata map[string]any,
	) error
}
