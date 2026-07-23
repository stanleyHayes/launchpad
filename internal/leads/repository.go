package leads

import "context"

// Repository persists leads.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, lead Lead) error
	List(ctx context.Context) ([]Lead, error)
	Count(ctx context.Context) (int64, error)
}
