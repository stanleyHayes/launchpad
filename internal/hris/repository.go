package hris

import "context"

// ConfigStore persists per-organization HRIS configuration.
type ConfigStore interface {
	EnsureIndexes(ctx context.Context) error
	// GetByOrganization returns the tenant's config, or ErrNotConfigured if none.
	GetByOrganization(ctx context.Context, organizationID string) (Config, error)
	// SetConfig upserts the tenant's config.
	SetConfig(ctx context.Context, config Config) error
	// DeleteForOrganization removes the organization's HRIS config and
	// returns the number of documents deleted. Called only by the platform
	// GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// Store persists the latest synced directory and run per organization.
type Store interface {
	EnsureIndexes(ctx context.Context) error
	// SaveResult replaces the tenant's directory snapshot and last run.
	SaveResult(ctx context.Context, organizationID string, entries []DirectoryEntry, run SyncRun) error
	// SaveRun records a run without touching the stored directory (used when a
	// sync fails, so the previous snapshot is preserved).
	SaveRun(ctx context.Context, organizationID string, run SyncRun) error
	// GetState returns the tenant's directory + last run, or ErrNotConfigured if
	// no sync has run.
	GetState(ctx context.Context, organizationID string) (State, error)
	// DeleteForOrganization removes the organization's directory snapshot and
	// run state, returning the number of documents deleted. Called only by
	// the platform GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// Provider fetches an employee directory from an HRIS.
type Provider interface {
	FetchDirectory(ctx context.Context, config Config) ([]DirectoryEntry, error)
}

// Providers resolves a provider implementation by name.
type Providers interface {
	// For returns the provider for a config's Provider name, or
	// ErrUnsupportedProvider.
	For(providerName string) (Provider, error)
}

// DirectoryApplier maps a synced directory snapshot into the employees domain.
// The concrete adapter (over the employees and departments services) lives in
// the app layer; HRIS depends only on this port so it stays decoupled and
// testable.
type DirectoryApplier interface {
	Apply(ctx context.Context, organizationID string, entries []DirectoryEntry) (ApplyResult, error)
}

// AuditRecorder writes audit events for privileged HRIS actions. *audit.Service
// satisfies it; the port keeps the HRIS service decoupled and testable.
type AuditRecorder interface {
	Record(
		ctx context.Context,
		organizationID *string,
		actorUserID, action, resourceType, resourceID string,
		metadata map[string]any,
	) error
}
