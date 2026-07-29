package cms_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/cms"
	"launchpad/pkg/security"
)

type memoryRepo struct {
	pages map[string]cms.Page
}

func (m *memoryRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryRepo) Create(_ context.Context, page cms.Page) error {
	if m.pages == nil {
		m.pages = map[string]cms.Page{}
	}

	for _, existing := range m.pages {
		if existing.Slug == page.Slug {
			return cms.ErrSlugTaken
		}
	}

	m.pages[page.ID] = page

	return nil
}

func (m *memoryRepo) GetByID(_ context.Context, id string) (cms.Page, error) {
	page, ok := m.pages[id]
	if !ok {
		return cms.Page{}, cms.ErrNotFound
	}

	return page, nil
}

func (m *memoryRepo) GetBySlug(_ context.Context, slug string) (cms.Page, error) {
	for _, page := range m.pages {
		if page.Slug == slug {
			return page, nil
		}
	}

	return cms.Page{}, cms.ErrNotFound
}

func (m *memoryRepo) List(context.Context) ([]cms.Page, error) {
	items := make([]cms.Page, 0, len(m.pages))
	for _, page := range m.pages {
		items = append(items, page)
	}

	return items, nil
}

func (m *memoryRepo) Update(_ context.Context, page cms.Page) error {
	if _, ok := m.pages[page.ID]; !ok {
		return cms.ErrNotFound
	}

	m.pages[page.ID] = page

	return nil
}

func TestCreateAndPublish(t *testing.T) {
	t.Parallel()

	svc := cms.NewService(&memoryRepo{})

	page, err := svc.Create(context.Background(), cms.CreateInput{
		Slug:  "pricing",
		Title: "Pricing",
		Body:  "Starter and growth plans.",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if page.Status != "draft" {
		t.Fatalf("status = %s, want draft", page.Status)
	}

	published, err := svc.Publish(context.Background(), page.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if published.Status != "published" || published.PublishedAt == nil {
		t.Fatalf("expected published page, got %+v", published)
	}

	publicPage, err := svc.GetPublishedBySlug(context.Background(), "pricing")
	if err != nil {
		t.Fatalf("get published: %v", err)
	}

	if publicPage.ID != page.ID {
		t.Fatalf("public page id mismatch")
	}
}

func TestCreateRejectsInvalidSlug(t *testing.T) {
	t.Parallel()

	svc := cms.NewService(&memoryRepo{})

	_, err := svc.Create(context.Background(), cms.CreateInput{
		Slug:  "Bad Slug",
		Title: "Pricing",
		Body:  "Body",
	})
	if !errors.Is(err, cms.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, cms.ErrInvalidInput)
	}
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

func TestPlatformCreateAndPublishRecordAudit(t *testing.T) {
	t.Parallel()

	svc := cms.NewService(&memoryRepo{})
	auditRepo := &recordingAuditRepo{}
	handler := cms.NewHandler(svc, audit.NewService(auditRepo))
	staff := security.Principal{UserID: "staff-1", RoleCode: "platform_admin"}

	createReq := newAuthedRequest(
		http.MethodPost,
		"/platform/cms/pages",
		`{"slug":"pricing","title":"Pricing","body":"Plans and pricing."}`,
		staff,
		nil,
	)
	createRec := httptest.NewRecorder()

	handler.HandlePlatformCreate(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", createRec.Code, createRec.Body.String())
	}

	if len(auditRepo.events) != 1 || auditRepo.events[0].Action != "cms_page.created" {
		t.Fatalf("expected one cms_page.created event, got %+v", auditRepo.events)
	}

	// CMS pages are global marketing content, so audit events carry no organization.
	if auditRepo.events[0].OrganizationID != nil {
		t.Errorf("create organizationId = %v, want nil", *auditRepo.events[0].OrganizationID)
	}

	pageID := auditRepo.events[0].ResourceID

	publishReq := newAuthedRequest(
		http.MethodPost,
		"/platform/cms/pages/"+pageID+"/publish",
		"",
		staff,
		map[string]string{"pageID": pageID},
	)
	publishRec := httptest.NewRecorder()

	handler.HandlePlatformPublish(publishRec, publishReq)

	if publishRec.Code != http.StatusOK {
		t.Fatalf("publish status = %d, want 200 (body: %s)", publishRec.Code, publishRec.Body.String())
	}

	if len(auditRepo.events) != 2 || auditRepo.events[1].Action != "cms_page.published" {
		t.Fatalf("expected a cms_page.published event, got %+v", auditRepo.events)
	}

	if auditRepo.events[1].ActorUserID != "staff-1" {
		t.Errorf("publish actor = %q, want staff-1", auditRepo.events[1].ActorUserID)
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

func (r *recordingAuditRepo) CountByOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (r *recordingAuditRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
