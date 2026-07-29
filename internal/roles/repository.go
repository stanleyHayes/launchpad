package roles

import (
	"context"
)

// Repository persists custom roles. Every query is scoped by organization.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, role Role) error
	GetByID(ctx context.Context, organizationID, id string) (Role, error)
	GetByName(ctx context.Context, organizationID, name string) (Role, error)
	List(ctx context.Context, organizationID string) ([]Role, error)
	Update(ctx context.Context, role Role) error
	Delete(ctx context.Context, organizationID, id string) error
	// DeleteForOrganization removes every custom role of the organization
	// and returns the number deleted. Called only by the platform GDPR
	// tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}
