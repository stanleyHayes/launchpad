package featureflags

import "context"

// Repository persists feature flags and overrides.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	UpsertFlag(ctx context.Context, flag Flag) error
	GetFlag(ctx context.Context, key string) (Flag, error)
	ListFlags(ctx context.Context) ([]Flag, error)
	CreateFlag(ctx context.Context, flag Flag) error
	UpdateFlag(ctx context.Context, flag Flag) error
	UpsertOverride(ctx context.Context, override Override) error
	GetOverride(ctx context.Context, organizationID, key string) (Override, error)
	ListOverridesByOrganization(ctx context.Context, organizationID string) ([]Override, error)
}
