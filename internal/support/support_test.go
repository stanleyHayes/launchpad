package support_test

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
	"launchpad/internal/support"
	"launchpad/pkg/security"
)

// memoryRepo is an in-memory support.Repository for tests.
type memoryRepo struct {
	tickets  map[string]support.Ticket
	blockers map[string]support.Blocker
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		tickets:  map[string]support.Ticket{},
		blockers: map[string]support.Blocker{},
	}
}

func (m *memoryRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryRepo) Create(_ context.Context, ticket support.Ticket) error {
	m.tickets[ticket.ID] = ticket

	return nil
}

func (m *memoryRepo) GetByID(_ context.Context, id string) (support.Ticket, error) {
	ticket, ok := m.tickets[id]
	if !ok {
		return support.Ticket{}, support.ErrNotFound
	}

	return ticket, nil
}

func (m *memoryRepo) GetByIDForOrganization(
	_ context.Context,
	organizationID, id string,
) (support.Ticket, error) {
	ticket, ok := m.tickets[id]
	if !ok || ticket.OrganizationID != organizationID {
		return support.Ticket{}, support.ErrNotFound
	}

	return ticket, nil
}

func (m *memoryRepo) ListByOrganization(_ context.Context, organizationID string) ([]support.Ticket, error) {
	items := make([]support.Ticket, 0)

	for _, ticket := range m.tickets {
		if ticket.OrganizationID == organizationID {
			items = append(items, ticket)
		}
	}

	return items, nil
}

func (m *memoryRepo) ListAll(context.Context) ([]support.Ticket, error) {
	items := make([]support.Ticket, 0, len(m.tickets))
	for _, ticket := range m.tickets {
		items = append(items, ticket)
	}

	return items, nil
}

func (m *memoryRepo) Update(_ context.Context, ticket support.Ticket) error {
	if _, ok := m.tickets[ticket.ID]; !ok {
		return support.ErrNotFound
	}

	m.tickets[ticket.ID] = ticket

	return nil
}

func (m *memoryRepo) CountOpen(context.Context) (int64, error) { return 0, nil }

func (m *memoryRepo) CreateBlocker(_ context.Context, blocker support.Blocker) error {
	m.blockers[blocker.ID] = blocker

	return nil
}

func (m *memoryRepo) ListBlockers(_ context.Context, organizationID string) ([]support.Blocker, error) {
	items := make([]support.Blocker, 0)

	for _, blocker := range m.blockers {
		if blocker.OrganizationID == organizationID {
			items = append(items, blocker)
		}
	}

	return items, nil
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

func TestPlatformUpdateStatusRecordsAudit(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := support.NewService(repo)

	ticket, err := svc.Create(context.Background(), support.CreateTicketInput{
		OrganizationID:  "org-1",
		CreatedByUserID: "user-1",
		Subject:         "Cannot log in",
		Body:            "Login page returns an error.",
	})
	if err != nil {
		t.Fatalf("seed ticket: %v", err)
	}

	auditRepo := &recordingAuditRepo{}
	handler := support.NewHandler(svc, audit.NewService(auditRepo))

	req := newAuthedRequest(
		http.MethodPost,
		"/platform/support/tickets/"+ticket.ID+"/status",
		`{"status":"in_progress"}`,
		security.Principal{UserID: "staff-1", RoleCode: "platform_admin"},
		map[string]string{"ticketID": ticket.ID},
	)
	rec := httptest.NewRecorder()

	handler.HandlePlatformUpdateStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	if len(auditRepo.events) != 1 {
		t.Fatalf("recorded %d audit events, want 1", len(auditRepo.events))
	}

	event := auditRepo.events[0]
	if event.Action != "support_ticket.status_updated" {
		t.Errorf("action = %q, want support_ticket.status_updated", event.Action)
	}

	if event.ActorUserID != "staff-1" {
		t.Errorf("actor = %q, want staff-1", event.ActorUserID)
	}

	if event.OrganizationID == nil || *event.OrganizationID != "org-1" {
		t.Errorf("organizationId = %v, want org-1", event.OrganizationID)
	}

	if event.ResourceID != ticket.ID {
		t.Errorf("resourceId = %q, want %q", event.ResourceID, ticket.ID)
	}
}

func TestReportBlockerCreatesCategorizedHighPriorityTicket(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := support.NewService(repo)

	blocker, err := svc.ReportBlocker(context.Background(), support.ReportBlockerInput{
		OrganizationID:   "org-1",
		EmployeeID:       "emp-1",
		ReportedByUserID: "user-1",
		EmployeeName:     "Ada Lovelace",
		StepAssignmentID: "step-1",
		StepTitle:        "Laptop setup",
		Category:         "it",
		Message:          "VPN access not provisioned",
	})
	if err != nil {
		t.Fatalf("ReportBlocker: %v", err)
	}

	ticket, err := repo.GetByID(context.Background(), blocker.TicketID)
	if err != nil {
		t.Fatalf("backing ticket: %v", err)
	}

	if ticket.Category != "it" || ticket.Priority != "high" {
		t.Errorf("ticket category/priority = %q/%q, want it/high", ticket.Category, ticket.Priority)
	}

	if !strings.Contains(ticket.Body, "Laptop setup") {
		t.Errorf("ticket body should reference the step, got %q", ticket.Body)
	}

	items, err := svc.ListBlockers(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("ListBlockers: %v", err)
	}

	if len(items) != 1 || items[0].ID != blocker.ID {
		t.Fatalf("blockers = %+v, want the reported blocker", items)
	}
}

func TestReportBlockerRejectsInvalidCategory(t *testing.T) {
	t.Parallel()

	svc := support.NewService(newMemoryRepo())

	_, err := svc.ReportBlocker(context.Background(), support.ReportBlockerInput{
		OrganizationID:   "org-1",
		EmployeeID:       "emp-1",
		ReportedByUserID: "user-1",
		Category:         "facilities",
		Message:          "Desk broken",
	})
	if !errors.Is(err, support.ErrInvalidInput) {
		t.Fatalf("got %v, want %v", err, support.ErrInvalidInput)
	}
}

func TestSupportConversationTracksFirstResponseAndSLA(t *testing.T) {
	t.Parallel()
	repo := newMemoryRepo()
	svc := support.NewService(repo)
	ticket, err := svc.Create(t.Context(), support.CreateTicketInput{
		OrganizationID: "org-1", CreatedByUserID: "customer",
		Subject: "Outage", Body: "Cannot continue", Priority: "urgent",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if ticket.SLADueAt.Sub(ticket.CreatedAt) != 4*time.Hour {
		t.Fatalf("urgent SLA = %s, want 4h", ticket.SLADueAt.Sub(ticket.CreatedAt))
	}
	updated, err := svc.AddMessage(t.Context(), ticket.ID, "agent", "We are investigating.", false)
	if err != nil {
		t.Fatalf("add message: %v", err)
	}
	if updated.FirstResponseAt == nil || len(updated.Messages) != 1 {
		t.Fatalf("first response not tracked: %+v", updated)
	}
}

func TestEscalationAndSupportSummary(t *testing.T) {
	t.Parallel()
	repo := newMemoryRepo()
	svc := support.NewService(repo)
	ticket, _ := svc.Create(t.Context(), support.CreateTicketInput{
		OrganizationID: "org-1", CreatedByUserID: "customer",
		Subject: "Issue", Body: "Help",
	})
	ticket.SLADueAt = time.Now().UTC().Add(-time.Minute)
	repo.tickets[ticket.ID] = ticket
	escalated, err := svc.Escalate(t.Context(), ticket.ID, "agent-2")
	if err != nil {
		t.Fatalf("escalate: %v", err)
	}
	if escalated.Priority != "urgent" || escalated.EscalationCount != 1 || escalated.AssigneeUserID != "agent-2" {
		t.Fatalf("unexpected escalation: %+v", escalated)
	}
	// Force the escalated SLA overdue to validate the queue metric.
	escalated.SLADueAt = time.Now().UTC().Add(-time.Minute)
	repo.tickets[ticket.ID] = escalated
	summary, err := svc.Summary(t.Context())
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.Open != 1 || summary.Overdue != 1 || summary.Urgent != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestOrgCreateAcceptsOptionalCategory(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	handler := support.NewHandler(support.NewService(repo), audit.NewService(&recordingAuditRepo{}))

	req := newAuthedRequest(
		http.MethodPost,
		"/support/tickets",
		`{"subject":"Need a laptop","body":"No device yet.","priority":"high","category":"it"}`,
		security.Principal{UserID: "user-1", OrganizationID: "org-1", RoleCode: "employee"},
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleOrgCreate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	items, err := repo.ListByOrganization(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}

	if len(items) != 1 || items[0].Category != "it" {
		t.Fatalf("tickets = %+v, want one ticket categorized it", items)
	}
}

func TestOrgCreateRejectsUnknownCategory(t *testing.T) {
	t.Parallel()

	handler := support.NewHandler(support.NewService(newMemoryRepo()), audit.NewService(&recordingAuditRepo{}))

	req := newAuthedRequest(
		http.MethodPost,
		"/support/tickets",
		`{"subject":"Need a laptop","body":"No device yet.","category":"facilities"}`,
		security.Principal{UserID: "user-1", OrganizationID: "org-1", RoleCode: "employee"},
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleOrgCreate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
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

func (m *memoryRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (r *recordingAuditRepo) CountByOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (r *recordingAuditRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
