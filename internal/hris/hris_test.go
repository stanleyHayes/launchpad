package hris_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/hris"
)

const (
	orgID    = "org-1"
	provider = "bamboohr"
)

type fakeConfigStore struct {
	config *hris.Config
	saved  *hris.Config
	err    error
}

func (f *fakeConfigStore) EnsureIndexes(context.Context) error { return nil }

func (f *fakeConfigStore) GetByOrganization(context.Context, string) (hris.Config, error) {
	if f.err != nil {
		return hris.Config{}, f.err
	}

	if f.config == nil {
		return hris.Config{}, hris.ErrNotConfigured
	}

	return *f.config, nil
}

func (f *fakeConfigStore) SetConfig(_ context.Context, config hris.Config) error {
	stored := config
	f.saved = &stored

	return nil
}

type fakeStore struct {
	savedEntries []hris.DirectoryEntry
	savedResult  bool
	savedRun     *hris.SyncRun
	state        hris.State
	stateErr     error
}

func (f *fakeStore) EnsureIndexes(context.Context) error { return nil }

func (f *fakeStore) SaveResult(_ context.Context, _ string, entries []hris.DirectoryEntry, run hris.SyncRun) error {
	f.savedResult = true
	f.savedEntries = entries
	stored := run
	f.savedRun = &stored

	return nil
}

func (f *fakeStore) SaveRun(_ context.Context, _ string, run hris.SyncRun) error {
	stored := run
	f.savedRun = &stored

	return nil
}

func (f *fakeStore) GetState(context.Context, string) (hris.State, error) {
	if f.stateErr != nil {
		return hris.State{}, f.stateErr
	}

	return f.state, nil
}

type fakeProvider struct {
	entries []hris.DirectoryEntry
	err     error
}

func (f *fakeProvider) FetchDirectory(context.Context, hris.Config) ([]hris.DirectoryEntry, error) {
	return f.entries, f.err
}

type fakeProviders struct {
	provider *fakeProvider
}

func (f fakeProviders) For(name string) (hris.Provider, error) {
	if name == provider {
		return f.provider, nil
	}

	return nil, hris.ErrUnsupportedProvider
}

// fakeApplier records the entries it was asked to apply and returns a canned
// result/error.
type fakeApplier struct {
	result     hris.ApplyResult
	err        error
	calls      int
	gotEntries []hris.DirectoryEntry
}

func (f *fakeApplier) Apply(_ context.Context, _ string, entries []hris.DirectoryEntry) (hris.ApplyResult, error) {
	f.calls++
	f.gotEntries = entries

	return f.result, f.err
}

// recordingAudit captures audit calls and can be told to fail.
type recordingAudit struct {
	calls      int
	lastAction string
	lastMeta   map[string]any
	err        error
}

func (a *recordingAudit) Record(
	_ context.Context,
	_ *string,
	_, action, _, _ string,
	metadata map[string]any,
) error {
	a.calls++
	a.lastAction = action
	a.lastMeta = metadata

	return a.err
}

// noopAudit satisfies hris.AuditRecorder without recording anything.
type noopAudit struct{}

func (noopAudit) Record(context.Context, *string, string, string, string, string, map[string]any) error {
	return nil
}

func enabledConfig() *hris.Config {
	return &hris.Config{
		OrganizationID: orgID,
		Provider:       provider,
		Subdomain:      "acme",
		APIToken:       "token",
		Enabled:        true,
	}
}

func newService(cfg *fakeConfigStore, store *fakeStore, prov *fakeProvider) *hris.Service {
	return hris.NewService(cfg, store, fakeProviders{provider: prov}, &fakeApplier{}, noopAudit{})
}

func TestSyncSuccessStoresDirectory(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	prov := &fakeProvider{entries: []hris.DirectoryEntry{
		{ExternalID: "1", Email: "a@co.com", Active: true},
		{ExternalID: "2", Email: "b@co.com", Active: true},
	}}
	svc := newService(&fakeConfigStore{config: enabledConfig()}, store, prov)

	run, err := svc.Sync(context.Background(), orgID)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	if run.Status != hris.StatusSuccess || run.EntryCount != 2 {
		t.Fatalf("unexpected run: %+v", run)
	}

	if !store.savedResult || len(store.savedEntries) != 2 {
		t.Fatalf("expected directory saved, got %+v", store.savedEntries)
	}
}

func TestSyncFailurePreservesDirectory(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	prov := &fakeProvider{err: errors.New("upstream 500")}
	svc := newService(&fakeConfigStore{config: enabledConfig()}, store, prov)

	run, err := svc.Sync(context.Background(), orgID)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	if run.Status != hris.StatusFailed || run.Error == "" {
		t.Fatalf("expected failed run with error, got %+v", run)
	}

	if store.savedResult {
		t.Fatalf("failed sync must NOT overwrite the directory snapshot")
	}

	if store.savedRun == nil || store.savedRun.Status != hris.StatusFailed {
		t.Fatalf("expected the failed run to be recorded")
	}
}

func TestSyncNotConfigured(t *testing.T) {
	t.Parallel()

	svc := newService(&fakeConfigStore{}, &fakeStore{}, &fakeProvider{})

	if _, err := svc.Sync(context.Background(), orgID); !errors.Is(err, hris.ErrNotConfigured) {
		t.Fatalf("got %v, want ErrNotConfigured", err)
	}
}

func TestSyncSurfacesInfrastructureErrors(t *testing.T) {
	t.Parallel()

	svc := newService(&fakeConfigStore{err: errors.New("mongo unreachable")}, &fakeStore{}, &fakeProvider{})

	_, err := svc.Sync(context.Background(), orgID)
	if err == nil || errors.Is(err, hris.ErrNotConfigured) {
		t.Fatalf("got %v, want wrapped infrastructure error, not ErrNotConfigured", err)
	}
}

func TestSyncDisabledConfigIsNotConfigured(t *testing.T) {
	t.Parallel()

	disabled := enabledConfig()
	disabled.Enabled = false
	svc := newService(&fakeConfigStore{config: disabled}, &fakeStore{}, &fakeProvider{})

	if _, err := svc.Sync(context.Background(), orgID); !errors.Is(err, hris.ErrNotConfigured) {
		t.Fatalf("got %v, want ErrNotConfigured for disabled config", err)
	}
}

func TestSetConfigValidation(t *testing.T) {
	t.Parallel()

	svc := newService(&fakeConfigStore{}, &fakeStore{}, &fakeProvider{})
	ctx := context.Background()

	valid := hris.SetConfigInput{Provider: provider, Subdomain: "acme", APIToken: "t", Enabled: true}
	if _, err := svc.SetConfig(ctx, orgID, valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	missingToken := valid

	missingToken.APIToken = ""
	if _, err := svc.SetConfig(ctx, orgID, missingToken); !errors.Is(err, hris.ErrInvalidInput) {
		t.Fatalf("expected missing token rejected, got %v", err)
	}

	unknownProvider := valid

	unknownProvider.Provider = "workday"
	if _, err := svc.SetConfig(ctx, orgID, unknownProvider); !errors.Is(err, hris.ErrInvalidInput) {
		t.Fatalf("expected unknown provider rejected, got %v", err)
	}

	insecureBase := valid

	insecureBase.BaseURL = "http://internal.local"
	if _, err := svc.SetConfig(ctx, orgID, insecureBase); !errors.Is(err, hris.ErrInvalidInput) {
		t.Fatalf("expected non-https base URL rejected, got %v", err)
	}
}

func applyService(store *fakeStore, applier hris.DirectoryApplier, audit hris.AuditRecorder) *hris.Service {
	cfg := &fakeConfigStore{config: enabledConfig()}
	providers := fakeProviders{provider: &fakeProvider{}}

	return hris.NewService(cfg, store, providers, applier, audit)
}

func TestApplyRunsApplierAndAudits(t *testing.T) {
	t.Parallel()

	entries := []hris.DirectoryEntry{
		{ExternalID: "1", Email: "a@co.com", Active: true},
		{ExternalID: "2", Email: "b@co.com", Active: true},
	}
	store := &fakeStore{state: hris.State{Entries: entries}}
	applier := &fakeApplier{result: hris.ApplyResult{Total: 2, Created: 1, Skipped: 1}}
	auditRec := &recordingAudit{}
	svc := applyService(store, applier, auditRec)

	result, err := svc.Apply(context.Background(), orgID, "user-1")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if applier.calls != 1 || len(applier.gotEntries) != 2 {
		t.Fatalf("expected applier called with 2 entries, got calls=%d entries=%d", applier.calls, len(applier.gotEntries))
	}

	if result.Created != 1 || result.Skipped != 1 || result.Total != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}

	if auditRec.calls != 1 || auditRec.lastAction != "hris.directory.applied" {
		t.Fatalf("expected one applied audit event, got calls=%d action=%q", auditRec.calls, auditRec.lastAction)
	}

	if got, ok := auditRec.lastMeta["created"].(int); !ok || got != 1 {
		t.Fatalf("expected audit metadata created=1, got %v", auditRec.lastMeta["created"])
	}
}

func TestApplyNoDirectoryWhenEmpty(t *testing.T) {
	t.Parallel()

	applier := &fakeApplier{}
	svc := applyService(&fakeStore{state: hris.State{Entries: []hris.DirectoryEntry{}}}, applier, &recordingAudit{})

	if _, err := svc.Apply(context.Background(), orgID, "user-1"); !errors.Is(err, hris.ErrNoDirectory) {
		t.Fatalf("got %v, want ErrNoDirectory", err)
	}

	if applier.calls != 0 {
		t.Fatalf("applier must not run when there is no directory")
	}
}

func TestApplyNoDirectoryWhenNeverSynced(t *testing.T) {
	t.Parallel()

	store := &fakeStore{stateErr: hris.ErrNotConfigured}
	svc := applyService(store, &fakeApplier{}, &recordingAudit{})

	if _, err := svc.Apply(context.Background(), orgID, "user-1"); !errors.Is(err, hris.ErrNoDirectory) {
		t.Fatalf("got %v, want ErrNoDirectory", err)
	}
}

func TestApplyAuditFailurePropagates(t *testing.T) {
	t.Parallel()

	store := &fakeStore{state: hris.State{Entries: []hris.DirectoryEntry{{ExternalID: "1", Email: "a@co.com"}}}}
	applier := &fakeApplier{result: hris.ApplyResult{Total: 1, Created: 1}}
	auditRec := &recordingAudit{err: errors.New("audit backend down")}
	svc := applyService(store, applier, auditRec)

	if _, err := svc.Apply(context.Background(), orgID, "user-1"); err == nil {
		t.Fatal("expected apply to fail when the audit write fails")
	}
}

func TestApplyRejectsEmptyOrganization(t *testing.T) {
	t.Parallel()

	svc := applyService(&fakeStore{}, &fakeApplier{}, &recordingAudit{})

	if _, err := svc.Apply(context.Background(), "", "user-1"); !errors.Is(err, hris.ErrInvalidInput) {
		t.Fatalf("got %v, want ErrInvalidInput", err)
	}
}

func (f *fakeConfigStore) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (f *fakeStore) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
