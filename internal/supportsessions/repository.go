package supportsessions

import "context"

// Repository persists support sessions.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, session Session) error
	GetByID(ctx context.Context, id string) (Session, error)
	Update(ctx context.Context, session Session) error
	ListByOrganization(ctx context.Context, organizationID string) ([]Session, error)
}
