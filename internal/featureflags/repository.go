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
	AppendHistory(ctx context.Context, history History) error
	ListHistory(ctx context.Context, key string, limit int64) ([]History, error)
	// DeleteForOrganization removes every feature-flag override of the
	// organization and returns the number deleted. Flag definitions are
	// platform-wide and are kept. Called only by the platform GDPR tenant
	// purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}
