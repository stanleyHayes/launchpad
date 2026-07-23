package platform

import "context"

// Repository persists platform staff records.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	GetByUserID(ctx context.Context, userID string) (Staff, error)
	Create(ctx context.Context, staff Staff) error
}
