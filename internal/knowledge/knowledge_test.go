package knowledge_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/knowledge"
	"launchpad/internal/notifications"
	"launchpad/internal/organizations"
	"launchpad/pkg/security"
)

const (
	orgID     = "org-1"
	testTitle = "Doc"
)

// memoryRepo is an in-memory, tenant-scoped knowledge.Repository.
type memoryRepo struct {
	docs map[string]knowledge.Document
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{docs: map[string]knowledge.Document{}}
}

func (m *memoryRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryRepo) Create(_ context.Context, doc knowledge.Document) error {
	m.docs[doc.ID] = doc

	return nil
}

func (m *memoryRepo) GetByID(
	_ context.Context,
	organizationID, documentID string,
) (knowledge.Document, error) {
	doc, ok := m.docs[documentID]
	if !ok || doc.OrganizationID != organizationID {
		return knowledge.Document{}, knowledge.ErrNotFound
	}

	return doc, nil
}

func (m *memoryRepo) List(_ context.Context, organizationID string) ([]knowledge.Document, error) {
	items := make([]knowledge.Document, 0)

	for _, doc := range m.docs {
		if doc.OrganizationID == organizationID {
			items = append(items, doc)
		}
	}

	return items, nil
}

func (m *memoryRepo) ListStale(_ context.Context, now time.Time) ([]knowledge.Document, error) {
	items := make([]knowledge.Document, 0)
	for _, doc := range m.docs {
		reference := doc.UpdatedAt
		if doc.LastSyncedAt != nil {
			reference = *doc.LastSyncedAt
		}
		reviewDue := doc.ReviewDate != nil && !doc.ReviewDate.After(now)
		retentionDue := doc.RetentionDays > 0 && !reference.AddDate(0, 0, doc.RetentionDays).After(now)
		if doc.Status != knowledge.StatusArchived && doc.StaleNotifiedAt == nil && (reviewDue || retentionDue) {
			items = append(items, doc)
		}
	}
	return items, nil
}

func (m *memoryRepo) Update(_ context.Context, doc knowledge.Document) error {
	existing, ok := m.docs[doc.ID]
	if !ok || existing.OrganizationID != doc.OrganizationID {
		return knowledge.ErrNotFound
	}

	m.docs[doc.ID] = doc

	return nil
}

// spyIndexer records Index/Remove calls.
type spyIndexer struct {
	indexed []string
	removed []string
}

type staticConnector struct{ body string }

func (c staticConnector) Fetch(context.Context, string, string) (string, error) { return c.body, nil }

type notificationSpy struct{ calls int }

func (n *notificationSpy) Create(
	_ context.Context, organizationID string, in notifications.CreateInput,
) (notifications.Notification, error) {
	n.calls++
	return notifications.Notification{OrganizationID: organizationID, UserID: in.UserID}, nil
}

func (s *spyIndexer) Index(_ context.Context, doc knowledge.Document) error {
	s.indexed = append(s.indexed, doc.ID)

	return nil
}

func (s *spyIndexer) Remove(_ context.Context, _, documentID string) error {
	s.removed = append(s.removed, documentID)

	return nil
}

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

func TestLifecycleCreateApproveIndexArchive(t *testing.T) {
	t.Parallel()

	indexer := &spyIndexer{}
	svc := knowledge.NewService(newMemoryRepo(), indexer)

	doc, err := svc.Create(context.Background(), orgID, "user-1", knowledge.CreateInput{
		Title: "Security Policy",
		Body:  "Do not share credentials.",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if doc.Status != knowledge.StatusDraft || doc.Version != 1 {
		t.Fatalf("expected draft v1, got status=%s version=%d", doc.Status, doc.Version)
	}

	approved, err := svc.Approve(context.Background(), orgID, doc.ID, "manager-1")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}

	if approved.Status != knowledge.StatusApproved || approved.ApprovedByUserID != "manager-1" {
		t.Fatalf("expected approved by manager-1, got %+v", approved)
	}

	indexed, err := svc.Index(context.Background(), orgID, doc.ID)
	if err != nil {
		t.Fatalf("index: %v", err)
	}

	if indexed.Status != knowledge.StatusIndexed || indexed.IndexedAt == nil {
		t.Fatalf("expected indexed with timestamp, got %+v", indexed)
	}

	if len(indexer.indexed) != 1 || indexer.indexed[0] != doc.ID {
		t.Fatalf("indexer.Index not called for %s, got %v", doc.ID, indexer.indexed)
	}

	archived, err := svc.Archive(context.Background(), orgID, doc.ID)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}

	if archived.Status != knowledge.StatusArchived {
		t.Fatalf("expected archived, got %s", archived.Status)
	}

	if len(indexer.removed) != 1 || indexer.removed[0] != doc.ID {
		t.Fatalf("indexer.Remove not called for %s, got %v", doc.ID, indexer.removed)
	}
}

func TestIndexRequiresApproval(t *testing.T) {
	t.Parallel()

	svc := knowledge.NewService(newMemoryRepo(), knowledge.NoopIndexer{})

	doc, err := svc.Create(context.Background(), orgID, "user-1", knowledge.CreateInput{Title: testTitle})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.Index(context.Background(), orgID, doc.ID); err == nil {
		t.Fatalf("expected index-before-approve to fail")
	}
}

func TestRestrictedDocumentsHiddenFromNonManagers(t *testing.T) {
	t.Parallel()

	svc := knowledge.NewService(newMemoryRepo(), knowledge.NoopIndexer{})

	open, err := svc.Create(context.Background(), orgID, "user-1", knowledge.CreateInput{
		Title:       "Handbook",
		AccessScope: knowledge.ScopeOrganization,
	})
	if err != nil {
		t.Fatalf("create open: %v", err)
	}

	restricted, err := svc.Create(context.Background(), orgID, "user-1", knowledge.CreateInput{
		Title:       "Comp Bands",
		AccessScope: knowledge.ScopeRestricted,
	})
	if err != nil {
		t.Fatalf("create restricted: %v", err)
	}

	// Non-manager list excludes restricted documents.
	visible, err := svc.List(context.Background(), orgID, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(visible) != 1 || visible[0].ID != open.ID {
		t.Fatalf("non-manager should see only the open document, got %+v", visible)
	}

	// Non-manager get of a restricted document returns not-found (no existence disclosure).
	if _, err := svc.Get(context.Background(), orgID, restricted.ID, false); err == nil {
		t.Fatalf("expected restricted get to be hidden from non-manager")
	}

	// Manager sees both.
	all, err := svc.List(context.Background(), orgID, true)
	if err != nil {
		t.Fatalf("manager list: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("manager should see both documents, got %d", len(all))
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	svc := knowledge.NewService(newMemoryRepo(), knowledge.NoopIndexer{})

	if _, err := svc.Create(context.Background(), orgID, "user-1", knowledge.CreateInput{Title: "  "}); err == nil {
		t.Fatalf("expected empty title to be rejected")
	}

	if _, err := svc.Create(context.Background(), orgID, "user-1", knowledge.CreateInput{
		Title:  "Doc",
		Source: "carrier-pigeon",
	}); err == nil {
		t.Fatalf("expected invalid source to be rejected")
	}
}

func TestTenantIsolationOnGet(t *testing.T) {
	t.Parallel()

	svc := knowledge.NewService(newMemoryRepo(), knowledge.NoopIndexer{})

	doc, err := svc.Create(context.Background(), orgID, "user-1", knowledge.CreateInput{Title: testTitle})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.Get(context.Background(), "org-2", doc.ID, true); err == nil {
		t.Fatalf("expected cross-tenant get to fail")
	}
}

func TestHandleApproveRecordsTenantScopedAudit(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := knowledge.NewService(repo, knowledge.NoopIndexer{})

	doc, err := svc.Create(context.Background(), orgID, "author-1", knowledge.CreateInput{Title: testTitle})
	if err != nil {
		t.Fatalf("seed doc: %v", err)
	}

	auditRepo := &recordingAuditRepo{}
	handler := knowledge.NewHandler(svc, audit.NewService(auditRepo))

	req := newAuthedRequest(
		http.MethodPost,
		"/knowledge/documents/"+doc.ID+"/approve",
		"",
		security.Principal{UserID: "mgr-1", OrganizationID: orgID, RoleCode: organizations.RoleHRAdmin()},
		map[string]string{"documentID": doc.ID},
	)
	rec := httptest.NewRecorder()

	handler.HandleApprove(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	if len(auditRepo.events) != 1 {
		t.Fatalf("recorded %d audit events, want 1", len(auditRepo.events))
	}

	event := auditRepo.events[0]
	if event.Action != "knowledge_document.approved" {
		t.Errorf("action = %q, want knowledge_document.approved", event.Action)
	}

	if event.OrganizationID == nil || *event.OrganizationID != orgID {
		t.Errorf("organizationId = %v, want org-1", event.OrganizationID)
	}

	if event.ActorUserID != "mgr-1" || event.ResourceID != doc.ID {
		t.Errorf("actor/resource mismatch: %+v", event)
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

func TestConnectorSyncCreatesReviewableVersion(t *testing.T) {
	t.Parallel()
	repo := newMemoryRepo()
	indexer := &spyIndexer{}
	svc := knowledge.NewService(repo, indexer).WithConnector(staticConnector{body: "updated policy"})
	doc, err := svc.Create(context.Background(), orgID, "owner-1", knowledge.CreateInput{
		Title: "Policy", Source: "url", URI: "https://example.com/policy", Body: "old policy",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if doc, err = svc.Approve(context.Background(), orgID, doc.ID, "owner-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if _, err = svc.Index(context.Background(), orgID, doc.ID); err != nil {
		t.Fatalf("Index: %v", err)
	}
	synced, err := svc.Sync(context.Background(), orgID, doc.ID)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if synced.Status != knowledge.StatusDraft || synced.Version != 2 || synced.LastSyncedAt == nil {
		t.Fatalf("synced document = %+v", synced)
	}
	if len(indexer.removed) != 1 {
		t.Fatalf("index removals = %v", indexer.removed)
	}
}

func TestNotifyStaleDeduplicatesOwnerAlert(t *testing.T) {
	t.Parallel()
	repo := newMemoryRepo()
	notifier := &notificationSpy{}
	svc := knowledge.NewService(repo, knowledge.NoopIndexer{}).WithNotifier(notifier)
	reviewDate := time.Now().UTC().Add(-time.Hour)
	doc, err := svc.Create(context.Background(), orgID, "owner-1", knowledge.CreateInput{
		Title: "Review me", OwnerUserID: "owner-1", ReviewDate: &reviewDate,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.NotifyStale(context.Background()); err != nil {
		t.Fatalf("NotifyStale: %v", err)
	}
	if err := svc.NotifyStale(context.Background()); err != nil {
		t.Fatalf("NotifyStale repeat: %v", err)
	}
	if notifier.calls != 1 || repo.docs[doc.ID].StaleNotifiedAt == nil {
		t.Fatalf("calls = %d doc = %+v", notifier.calls, repo.docs[doc.ID])
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
