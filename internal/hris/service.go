package hris

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Service implements HRIS use cases.
type Service struct {
	configs   ConfigStore
	store     Store
	providers Providers
	applier   DirectoryApplier
	audit     AuditRecorder
}

// NewService constructs a Service.
func NewService(
	configs ConfigStore,
	store Store,
	providers Providers,
	applier DirectoryApplier,
	audit AuditRecorder,
) *Service {
	return &Service{
		configs:   configs,
		store:     store,
		providers: providers,
		applier:   applier,
		audit:     audit,
	}
}

// GetConfig returns the tenant's HRIS configuration (token omitted), or an empty
// disabled config when none is set.
func (s *Service) GetConfig(ctx context.Context, organizationID string) (Config, error) {
	if organizationID == "" {
		return Config{}, ErrInvalidInput
	}

	config, err := s.configs.GetByOrganization(ctx, organizationID)
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			return Config{OrganizationID: organizationID}, nil
		}

		return Config{}, fmt.Errorf("get hris config: %w", err)
	}

	return config, nil
}

// SetConfig validates and stores the tenant's HRIS configuration.
func (s *Service) SetConfig(ctx context.Context, organizationID string, in SetConfigInput) (Config, error) {
	if organizationID == "" {
		return Config{}, ErrInvalidInput
	}

	config := Config{
		OrganizationID: organizationID,
		Provider:       strings.TrimSpace(in.Provider),
		Subdomain:      strings.TrimSpace(in.Subdomain),
		APIToken:       in.APIToken,
		BaseURL:        strings.TrimSpace(in.BaseURL),
		Enabled:        in.Enabled,
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.validateConfig(config); err != nil {
		return Config{}, err
	}

	if err := s.configs.SetConfig(ctx, config); err != nil {
		return Config{}, fmt.Errorf("set hris config: %w", err)
	}

	return config, nil
}

// Sync fetches the tenant's HRIS directory and records the result. Provider
// failures are captured in the returned run (Status = failed) without discarding
// the previously synced directory; only infrastructure errors are returned.
func (s *Service) Sync(ctx context.Context, organizationID string) (SyncRun, error) {
	config, err := s.configs.GetByOrganization(ctx, organizationID)
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			return SyncRun{}, ErrNotConfigured
		}

		return SyncRun{}, fmt.Errorf("get hris config: %w", err)
	}

	if !config.Enabled {
		return SyncRun{}, ErrNotConfigured
	}

	provider, err := s.providers.For(config.Provider)
	if err != nil {
		return SyncRun{}, ErrUnsupportedProvider
	}

	startedAt := time.Now().UTC()

	entries, fetchErr := provider.FetchDirectory(ctx, config)
	if fetchErr != nil {
		run := newRun(config.Provider, StatusFailed, 0, fetchErr.Error(), startedAt)
		if err := s.store.SaveRun(ctx, organizationID, run); err != nil {
			return SyncRun{}, fmt.Errorf("save failed sync run: %w", err)
		}

		return run, nil
	}

	run := newRun(config.Provider, StatusSuccess, len(entries), "", startedAt)
	if err := s.store.SaveResult(ctx, organizationID, entries, run); err != nil {
		return SyncRun{}, fmt.Errorf("save sync result: %w", err)
	}

	return run, nil
}

// GetState returns the tenant's latest synced directory and last run.
func (s *Service) GetState(ctx context.Context, organizationID string) (State, error) {
	if organizationID == "" {
		return State{}, ErrInvalidInput
	}

	state, err := s.store.GetState(ctx, organizationID)
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			var lastRun SyncRun

			return State{Entries: []DirectoryEntry{}, LastRun: lastRun}, nil
		}

		return State{}, fmt.Errorf("get hris state: %w", err)
	}

	return state, nil
}

// Apply maps the tenant's latest synced directory snapshot into the employees
// domain via the configured DirectoryApplier, records an audit event, and
// returns a summary. It operates on the stored snapshot (not a live fetch);
// ErrNoDirectory is returned when no directory has been synced yet.
func (s *Service) Apply(ctx context.Context, organizationID, actorUserID string) (ApplyResult, error) {
	if organizationID == "" {
		return ApplyResult{}, ErrInvalidInput
	}

	state, err := s.store.GetState(ctx, organizationID)
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			return ApplyResult{}, ErrNoDirectory
		}

		return ApplyResult{}, fmt.Errorf("get hris state for apply: %w", err)
	}

	if len(state.Entries) == 0 {
		return ApplyResult{}, ErrNoDirectory
	}

	result, err := s.applier.Apply(ctx, organizationID, state.Entries)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("apply hris directory: %w", err)
	}

	orgID := organizationID
	if err := s.audit.Record(ctx, &orgID, actorUserID, "hris.directory.applied", "hris", organizationID, map[string]any{
		"total":   result.Total,
		"created": result.Created,
		"skipped": result.Skipped,
		"failed":  result.Failed,
	}); err != nil {
		return ApplyResult{}, fmt.Errorf("record hris apply audit: %w", err)
	}

	return result, nil
}

func (s *Service) validateConfig(config Config) error {
	if config.BaseURL != "" && !isHTTPSURL(config.BaseURL) {
		return ErrInvalidInput
	}

	if !config.Enabled {
		return nil
	}

	if config.Provider == "" || config.Subdomain == "" || config.APIToken == "" {
		return ErrInvalidInput
	}

	if _, err := s.providers.For(config.Provider); err != nil {
		return ErrInvalidInput
	}

	return nil
}

func newRun(provider, status string, entryCount int, errMessage string, startedAt time.Time) SyncRun {
	return SyncRun{
		Provider:   provider,
		Status:     status,
		EntryCount: entryCount,
		Error:      errMessage,
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
	}
}

func isHTTPSURL(raw string) bool {
	parsed, err := url.Parse(raw)

	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}
