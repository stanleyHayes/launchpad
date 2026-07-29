package featureflags_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/featureflags"
	"launchpad/pkg/security"
)

// memoryRepo is an in-memory featureflags.Repository for tests.
type memoryRepo struct {
	flags     map[string]featureflags.Flag
	overrides map[string]featureflags.Override
	history   []featureflags.History
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		flags:     map[string]featureflags.Flag{},
		overrides: map[string]featureflags.Override{},
		history:   []featureflags.History{},
	}
}

func (m *memoryRepo) AppendHistory(_ context.Context, history featureflags.History) error {
	m.history = append(m.history, history)
	return nil
}

func (m *memoryRepo) ListHistory(
	_ context.Context,
	key string,
	limit int64,
) ([]featureflags.History, error) {
	items := make([]featureflags.History, 0)
	for i := len(m.history) - 1; i >= 0 && int64(len(items)) < limit; i-- {
		if m.history[i].Key == key {
			items = append(items, m.history[i])
		}
	}
	return items, nil
}

func overrideKey(organizationID, key string) string { return organizationID + "|" + key }

func (m *memoryRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryRepo) UpsertFlag(_ context.Context, flag featureflags.Flag) error {
	m.flags[flag.Key] = flag

	return nil
}

func (m *memoryRepo) GetFlag(_ context.Context, key string) (featureflags.Flag, error) {
	flag, ok := m.flags[key]
	if !ok {
		return featureflags.Flag{}, featureflags.ErrNotFound
	}

	return flag, nil
}

func (m *memoryRepo) ListFlags(context.Context) ([]featureflags.Flag, error) {
	items := make([]featureflags.Flag, 0, len(m.flags))
	for _, flag := range m.flags {
		items = append(items, flag)
	}

	return items, nil
}

func (m *memoryRepo) CreateFlag(_ context.Context, flag featureflags.Flag) error {
	if _, ok := m.flags[flag.Key]; ok {
		return featureflags.ErrKeyTaken
	}

	m.flags[flag.Key] = flag

	return nil
}

func (m *memoryRepo) UpdateFlag(_ context.Context, flag featureflags.Flag) error {
	if _, ok := m.flags[flag.Key]; !ok {
		return featureflags.ErrNotFound
	}

	m.flags[flag.Key] = flag

	return nil
}

func (m *memoryRepo) UpsertOverride(_ context.Context, override featureflags.Override) error {
	m.overrides[overrideKey(override.OrganizationID, override.Key)] = override

	return nil
}

func (m *memoryRepo) GetOverride(
	_ context.Context,
	organizationID, key string,
) (featureflags.Override, error) {
	override, ok := m.overrides[overrideKey(organizationID, key)]
	if !ok {
		return featureflags.Override{}, featureflags.ErrNotFound
	}

	return override, nil
}

func (m *memoryRepo) ListOverridesByOrganization(
	_ context.Context,
	organizationID string,
) ([]featureflags.Override, error) {
	items := make([]featureflags.Override, 0)

	for _, override := range m.overrides {
		if override.OrganizationID == organizationID {
			items = append(items, override)
		}
	}

	return items, nil
}

// stubOrgReader satisfies featureflags.OrganizationReader.
type stubOrgReader struct{}

func (stubOrgReader) PlanCode(context.Context, string) (string, error) { return "growth", nil }

// recordingAuditRepo captures written audit events.
type recordingAuditRepo struct {
	events []audit.Event
}

func (r *recordingAuditRepo) EnsureIndexes(context.Context) error { return nil }

func (r *recordingAuditRepo) Write(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)

	return nil
}

func (r *recordingAuditRepo) ListByOrganization(
	context.Context,
	string,
	int64,
) ([]audit.Event, error) {
	return r.events, nil
}

func (r *recordingAuditRepo) ListAll(context.Context, int64) ([]audit.Event, error) {
	return r.events, nil
}

func TestPlatformSetOverrideRecordsTenantScopedAudit(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	if err := repo.CreateFlag(context.Background(), featureflags.Flag{Key: "beta-dashboard", Enabled: false}); err != nil {
		t.Fatalf("seed flag: %v", err)
	}

	svc := featureflags.NewService(repo, stubOrgReader{})
	auditRepo := &recordingAuditRepo{}
	handler := featureflags.NewHandler(svc, audit.NewService(auditRepo))

	req := newAuthedRequest(
		http.MethodPut,
		"/platform/organizations/org-7/feature-flags/beta-dashboard",
		`{"enabled":true}`,
		security.Principal{UserID: "staff-9", RoleCode: "platform_admin"},
		map[string]string{"organizationID": "org-7", "key": "beta-dashboard"},
	)
	rec := httptest.NewRecorder()

	handler.HandlePlatformSetOverride(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	if len(auditRepo.events) != 1 {
		t.Fatalf("recorded %d audit events, want 1", len(auditRepo.events))
	}

	event := auditRepo.events[0]
	if event.Action != "feature_flag_override.set" {
		t.Errorf("action = %q, want feature_flag_override.set", event.Action)
	}

	if event.OrganizationID == nil || *event.OrganizationID != "org-7" {
		t.Errorf("organizationId = %v, want org-7", event.OrganizationID)
	}

	if event.ActorUserID != "staff-9" {
		t.Errorf("actor = %q, want staff-9", event.ActorUserID)
	}
}

func TestPlatformCreateFlagRecordsGlobalAudit(t *testing.T) {
	t.Parallel()

	svc := featureflags.NewService(newMemoryRepo(), stubOrgReader{})
	auditRepo := &recordingAuditRepo{}
	handler := featureflags.NewHandler(svc, audit.NewService(auditRepo))

	req := newAuthedRequest(
		http.MethodPost,
		"/platform/feature-flags",
		`{"key":"new-onboarding","enabled":true}`,
		security.Principal{UserID: "staff-2", RoleCode: "platform_owner"},
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandlePlatformCreate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	if len(auditRepo.events) != 1 {
		t.Fatalf("recorded %d audit events, want 1", len(auditRepo.events))
	}

	event := auditRepo.events[0]
	if event.Action != "feature_flag.created" {
		t.Errorf("action = %q, want feature_flag.created", event.Action)
	}

	if event.OrganizationID != nil {
		t.Errorf("organizationId = %v, want nil for global flag", *event.OrganizationID)
	}
}

func newAuthedRequest(
	method, target, body string,
	principal security.Principal,
	params map[string]string,
) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}

	ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
	ctx = security.WithPrincipal(ctx, principal)

	return httptest.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
}

func TestSeedDefaultsDoesNotOverwriteExistingFlags(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := featureflags.NewService(repo, stubOrgReader{})
	ctx := context.Background()

	if err := svc.SeedDefaults(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// An admin enables and re-describes the flag.
	enabled := true
	description := "enabled by admin"

	if _, err := svc.UpdateFlag(ctx, "ai_assistant", featureflags.UpdateFlagInput{
		Enabled:     &enabled,
		Description: &description,
	}); err != nil {
		t.Fatalf("update flag: %v", err)
	}

	if err := svc.SeedDefaults(ctx); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	flag, err := repo.GetFlag(ctx, "ai_assistant")
	if err != nil {
		t.Fatalf("get flag: %v", err)
	}

	if !flag.Enabled || flag.Description != description {
		t.Fatalf("re-seed overwrote admin changes: %+v", flag)
	}
}

func TestResolveHonorsExpiryCohortAndStablePercentage(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := featureflags.NewService(repo, stubOrgReader{})
	expired := time.Now().Add(-time.Minute)
	for _, flag := range []featureflags.Flag{
		{Key: "expired", Enabled: true, RolloutPercentage: 100, ExpiresAt: &expired},
		{Key: "cohort", Enabled: true, RolloutPercentage: 1, CohortUserIDs: []string{"user-test"}},
		{Key: "gradual", Enabled: true, RolloutPercentage: 50},
	} {
		if err := repo.CreateFlag(context.Background(), flag); err != nil {
			t.Fatalf("seed %s: %v", flag.Key, err)
		}
	}

	first, err := svc.Resolve(context.Background(), "org-7", "growth", "user-test")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	second, err := svc.Resolve(context.Background(), "org-7", "growth", "user-test")
	if err != nil {
		t.Fatalf("resolve again: %v", err)
	}
	if first["expired"] {
		t.Fatal("expired flag resolved enabled")
	}
	if !first["cohort"] {
		t.Fatal("explicit test cohort did not bypass gradual rollout")
	}
	if first["gradual"] != second["gradual"] {
		t.Fatal("percentage rollout was not stable for the same tenant")
	}
}

func TestFlagMutationsAppendHistory(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := featureflags.NewService(repo, stubOrgReader{})
	flag, err := svc.CreateFlag(context.Background(), featureflags.CreateFlagInput{
		Key: "new-ui", Enabled: true, RolloutPercentage: 25, ActorUserID: "staff-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	percentage := 75
	if _, err := svc.UpdateFlag(context.Background(), flag.Key, featureflags.UpdateFlagInput{
		RolloutPercentage: &percentage, UpdatedBy: "staff-2",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	items, err := svc.ListHistory(context.Background(), flag.Key, 50)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(items) != 2 || items[0].Action != "updated" ||
		items[0].Snapshot.RolloutPercentage != 75 || items[1].Action != "created" {
		t.Fatalf("history = %+v", items)
	}
}

// failingAuditRepo always fails writes.
type failingAuditRepo struct{}

func (failingAuditRepo) EnsureIndexes(context.Context) error { return nil }

func (failingAuditRepo) Write(context.Context, audit.Event) error {
	return errors.New("audit store down")
}

func (failingAuditRepo) ListByOrganization(context.Context, string, int64) ([]audit.Event, error) {
	return nil, nil
}

func (failingAuditRepo) ListAll(context.Context, int64) ([]audit.Event, error) {
	return nil, nil
}

func (failingAuditRepo) CountByOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (failingAuditRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func TestPlatformCreateFlagSucceedsWhenAuditFails(t *testing.T) {
	t.Parallel()

	svc := featureflags.NewService(newMemoryRepo(), stubOrgReader{})
	handler := featureflags.NewHandler(svc, audit.NewService(failingAuditRepo{}))

	req := newAuthedRequest(
		http.MethodPost,
		"/platform/feature-flags",
		`{"key":"new-flag"}`,
		security.Principal{UserID: "staff-1", RoleCode: "platform_owner"},
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandlePlatformCreate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 even when audit fails (body: %s)", rec.Code, rec.Body.String())
	}
}

func (m *memoryRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (r *recordingAuditRepo) CountByOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (r *recordingAuditRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
