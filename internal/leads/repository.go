package leads

import (
	"context"
	"time"
)

// Repository persists leads.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, lead Lead) error
	// List returns up to limit leads newest-first. When before is non-zero,
	// only leads created before that instant are returned (keyset pagination).
	List(ctx context.Context, limit int64, before time.Time) ([]Lead, error)
	Count(ctx context.Context) (int64, error)
}
