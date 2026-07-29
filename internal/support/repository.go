package support

import "context"

// Repository persists support tickets.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, ticket Ticket) error
	GetByID(ctx context.Context, id string) (Ticket, error)
	GetByIDForOrganization(ctx context.Context, organizationID, id string) (Ticket, error)
	ListByOrganization(ctx context.Context, organizationID string) ([]Ticket, error)
	ListAll(ctx context.Context) ([]Ticket, error)
	Update(ctx context.Context, ticket Ticket) error
	CountOpen(ctx context.Context) (int64, error)
	CreateBlocker(ctx context.Context, blocker Blocker) error
	ListBlockers(ctx context.Context, organizationID string) ([]Blocker, error)
	// DeleteForOrganization removes every support ticket and blocker of the
	// organization and returns the number of documents deleted. Called only
	// by the platform GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}
