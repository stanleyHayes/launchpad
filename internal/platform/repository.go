package platform

import "context"

// Repository persists platform staff records.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	GetByUserID(ctx context.Context, userID string) (Staff, error)
	GetByID(ctx context.Context, staffID string) (Staff, error)
	List(ctx context.Context) ([]Staff, error)
	Create(ctx context.Context, staff Staff) error
	Update(ctx context.Context, staff Staff) error
}
