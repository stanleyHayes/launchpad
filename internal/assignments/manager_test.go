package assignments_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/assignments"
	"launchpad/internal/audit"
	"launchpad/internal/employees"
	"launchpad/internal/notifications"
	"launchpad/internal/support"
	"launchpad/pkg/security"
)

// stubBlockers is an in-memory assignments.BlockerStore honoring the
// organization filter, so tests can prove tenant isolation.
type stubBlockers struct {
	blockers []support.Blocker
	lastIn   support.ReportBlockerInput
}

func (s *stubBlockers) ReportBlocker(
	_ context.Context,
	in support.ReportBlockerInput,
) (support.Blocker, error) {
	s.lastIn = in

	blocker := support.Blocker{
		ID:               "blk-1",
		OrganizationID:   in.OrganizationID,
		EmployeeID:       in.EmployeeID,
		ReportedByUserID: in.ReportedByUserID,
		StepAssignmentID: in.StepAssignmentID,
		Category:         in.Category,
		Message:          in.Message,
		TicketID:         "tkt-1",
		CreatedAt:        time.Now().UTC(),
	}
	s.blockers = append(s.blockers, blocker)

	return blocker, nil
}

func (s *stubBlockers) ListBlockers(_ context.Context, organizationID string) ([]support.Blocker, error) {
	items := make([]support.Blocker, 0)

	for _, blocker := range s.blockers {
		if blocker.OrganizationID == organizationID {
			items = append(items, blocker)
		}
	}

	return items, nil
}

// managerFixture seeds a manager (emp-m/mgr-u) with two reports (emp-a,
// emp-b), an unrelated employee (emp-c) reporting to someone else, and
// assignments/steps/approvals exercising every rollup counter.
type managerFixture struct {
	repo      *memoryRepo
	employees stubEmployees
	notify    *stubNotify
}

func newManagerFixture() *managerFixture {
	repo := newMemoryRepo()
	now := time.Now().UTC()
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	repo.assignments["asg-a1"] = assignments.JourneyAssignment{
		ID: "asg-a1", OrganizationID: testOrgID, EmployeeID: "emp-a", Status: "in_progress",
	}
	repo.assignments["asg-a2"] = assignments.JourneyAssignment{
		ID: "asg-a2", OrganizationID: testOrgID, EmployeeID: "emp-a", Status: "completed",
	}
	repo.assignments["asg-b1"] = assignments.JourneyAssignment{
		ID: "asg-b1", OrganizationID: testOrgID, EmployeeID: "emp-b", Status: "scheduled",
	}
	repo.assignments["asg-c1"] = assignments.JourneyAssignment{
		ID: "asg-c1", OrganizationID: testOrgID, EmployeeID: "emp-c", Status: "in_progress",
	}
	// Same employee, other tenant: must never leak into org-1 rollups.
	repo.assignments["asg-x"] = assignments.JourneyAssignment{
		ID: "asg-x", OrganizationID: "org-2", EmployeeID: "emp-a", Status: "in_progress",
	}

	repo.steps["s1"] = assignments.StepAssignment{
		ID: "s1", OrganizationID: testOrgID, JourneyAssignmentID: "asg-a1",
		EmployeeID: "emp-a", Title: "Laptop setup", Status: "in_progress", DueAt: &past,
	}
	repo.steps["s2"] = assignments.StepAssignment{
		ID: "s2", OrganizationID: testOrgID, JourneyAssignmentID: "asg-a1",
		EmployeeID: "emp-a", Status: "completed", DueAt: &past,
	}
	repo.steps["s3"] = assignments.StepAssignment{
		ID: "s3", OrganizationID: testOrgID, JourneyAssignmentID: "asg-a1",
		EmployeeID: "emp-a", Status: "in_progress", DueAt: &future,
	}
	repo.steps["s4"] = assignments.StepAssignment{
		ID: "s4", OrganizationID: testOrgID, JourneyAssignmentID: "asg-a1",
		EmployeeID: "emp-a", Status: "pending",
	}
	repo.steps["s-c1"] = assignments.StepAssignment{
		ID: "s-c1", OrganizationID: testOrgID, JourneyAssignmentID: "asg-c1",
		EmployeeID: "emp-c", Status: "in_progress", DueAt: &past,
	}
	repo.steps["s-x1"] = assignments.StepAssignment{
		ID: "s-x1", OrganizationID: "org-2", JourneyAssignmentID: "asg-x",
		EmployeeID: "emp-a", Status: "in_progress", DueAt: &past,
	}

	repo.approvals["appr-1"] = assignments.Approval{
		ID: "appr-1", OrganizationID: testOrgID, StepAssignmentID: "s1",
		ApproverUserID: testManagerUser, Status: "pending",
	}
	repo.approvals["appr-2"] = assignments.Approval{
		ID: "appr-2", OrganizationID: testOrgID, StepAssignmentID: "s-c1",
		ApproverUserID: testManagerUser, Status: "pending",
	}
	repo.approvals["appr-3"] = assignments.Approval{
		ID: "appr-3", OrganizationID: testOrgID, StepAssignmentID: "s1",
		ApproverUserID: testManagerUser, Status: "rejected",
	}

	directory := map[string]employees.Employee{
		"emp-m": {ID: "emp-m", UserID: testManagerUser, FirstName: "Mara", LastName: "Boss"},
		"emp-a": {
			ID: "emp-a", UserID: "user-a", FirstName: "Ada", LastName: "Lovelace",
			ManagerEmployeeID: "emp-m",
		},
		"emp-b": {
			ID: "emp-b", UserID: "user-b", FirstName: "Bob", LastName: "Builder",
			ManagerEmployeeID: "emp-m",
		},
		"emp-c": {ID: "emp-c", UserID: "user-c", FirstName: "Cid", LastName: "Other", ManagerEmployeeID: "emp-x"},
	}

	return &managerFixture{
		repo: repo,
		employees: stubEmployees{
			byID: directory,
			byUserID: map[string]employees.Employee{
				testManagerUser: directory["emp-m"],
				"user-a":        directory["emp-a"],
				"user-b":        directory["emp-b"],
			},
		},
		notify: &stubNotify{},
	}
}

func summaryByEmployee(
	t *testing.T,
	summaries []assignments.TeamReportSummary,
	employeeID string,
) assignments.TeamReportSummary {
	t.Helper()

	for _, summary := range summaries {
		if summary.EmployeeID == employeeID {
			return summary
		}
	}

	t.Fatalf("no rollup summary for %s in %+v", employeeID, summaries)

	return assignments.TeamReportSummary{}
}

func TestTeamRollupCountsAssignmentsOverdueAndApprovals(t *testing.T) {
	t.Parallel()

	fixture := newManagerFixture()
	svc := assignments.NewService(fixture.repo, stubJourneys{}, fixture.employees, fixture.notify)

	summaries, err := svc.TeamRollup(context.Background(), testOrgID, testManagerUser)
	if err != nil {
		t.Fatalf("TeamRollup: %v", err)
	}

	if len(summaries) != 2 {
		t.Fatalf("rollup size = %d, want 2 direct reports: %+v", len(summaries), summaries)
	}

	ada := summaryByEmployee(t, summaries, "emp-a")
	if ada.Name != "Ada Lovelace" {
		t.Errorf("name = %q, want Ada Lovelace", ada.Name)
	}

	if ada.ActiveAssignments != 1 || ada.CompletedAssignments != 1 {
		t.Errorf("assignment counts = %d active/%d completed, want 1/1",
			ada.ActiveAssignments, ada.CompletedAssignments)
	}

	// s1 is the only overdue step: s2 is completed, s3 is future-dated, s4
	// has no due date.
	if ada.OverdueSteps != 1 {
		t.Errorf("overdue = %d, want 1", ada.OverdueSteps)
	}

	// appr-1 is the only pending approval on Ada's steps for this manager.
	if ada.PendingApprovals != 1 {
		t.Errorf("pendingApprovals = %d, want 1", ada.PendingApprovals)
	}

	bob := summaryByEmployee(t, summaries, "emp-b")
	if bob.ActiveAssignments != 1 || bob.OverdueSteps != 0 || bob.PendingApprovals != 0 {
		t.Errorf("bob summary = %+v, want 1 active and nothing else", bob)
	}
}

func TestTeamRollupScopesToDirectReportsAndTenant(t *testing.T) {
	t.Parallel()

	fixture := newManagerFixture()
	svc := assignments.NewService(fixture.repo, stubJourneys{}, fixture.employees, fixture.notify)

	// emp-c reports to someone else: invisible to this manager even though a
	// pending approval for the manager exists on emp-c's step.
	summaries, err := svc.TeamRollup(context.Background(), testOrgID, testManagerUser)
	if err != nil {
		t.Fatalf("TeamRollup: %v", err)
	}

	for _, summary := range summaries {
		if summary.EmployeeID == "emp-c" {
			t.Fatalf("manager must not see emp-c: %+v", summaries)
		}
	}

	// Cross-tenant: in org-2 only asg-x exists for emp-a, so the org-1
	// counts must not bleed through.
	otherOrg, err := svc.TeamRollup(context.Background(), "org-2", testManagerUser)
	if err != nil {
		t.Fatalf("TeamRollup org-2: %v", err)
	}

	ada := summaryByEmployee(t, otherOrg, "emp-a")
	if ada.ActiveAssignments != 1 || ada.CompletedAssignments != 0 ||
		ada.OverdueSteps != 1 || ada.PendingApprovals != 0 {
		t.Errorf("org-2 rollup = %+v, want only asg-x counts", ada)
	}
}

func TestReportBlockerCreatesTicketAndNotifiesManager(t *testing.T) {
	t.Parallel()

	fixture := newManagerFixture()
	blockers := &stubBlockers{}
	svc := assignments.NewService(fixture.repo, stubJourneys{}, fixture.employees, fixture.notify, blockers)

	blocker, err := svc.ReportBlocker(context.Background(), testOrgID, "user-a", assignments.ReportBlockerInput{
		StepAssignmentID: "s1",
		Category:         "it",
		Message:          "VPN access not provisioned",
	})
	if err != nil {
		t.Fatalf("ReportBlocker: %v", err)
	}

	if blocker.TicketID == "" {
		t.Error("blocker must reference its backing support ticket")
	}

	in := blockers.lastIn
	if in.EmployeeID != "emp-a" || in.ReportedByUserID != "user-a" ||
		in.StepAssignmentID != "s1" || in.StepTitle != "Laptop setup" ||
		in.Category != "it" || in.EmployeeName != "Ada Lovelace" {
		t.Errorf("blocker input = %+v", in)
	}

	if len(fixture.notify.calls) != 1 || fixture.notify.calls[0].UserID != testManagerUser {
		t.Fatalf("expected one manager notification, got %+v", fixture.notify.calls)
	}

	call := fixture.notify.calls[0]
	if call.Type != notifications.TypeBlocker {
		t.Fatalf("type = %q, want blocker", call.Type)
	}

	if call.Link != "/assignments/asg-a1" {
		t.Fatalf("link = %q, want /assignments/asg-a1", call.Link)
	}
}

func TestReportBlockerRejectsOtherEmployeesStep(t *testing.T) {
	t.Parallel()

	fixture := newManagerFixture()
	svc := assignments.NewService(fixture.repo, stubJourneys{}, fixture.employees, fixture.notify, &stubBlockers{})

	_, err := svc.ReportBlocker(context.Background(), testOrgID, "user-b", assignments.ReportBlockerInput{
		StepAssignmentID: "s1",
		Category:         "it",
		Message:          "Not my step",
	})
	if !errors.Is(err, assignments.ErrForbidden) {
		t.Fatalf("got %v, want %v", err, assignments.ErrForbidden)
	}
}

func TestListTeamBlockersScopesToReportsAndTenant(t *testing.T) {
	t.Parallel()

	fixture := newManagerFixture()
	blockers := &stubBlockers{blockers: []support.Blocker{
		{ID: "b1", OrganizationID: testOrgID, EmployeeID: "emp-a", Category: "hr", Message: "No laptop"},
		{ID: "b2", OrganizationID: testOrgID, EmployeeID: "emp-c", Category: "it", Message: "Not a report"},
		{ID: "b3", OrganizationID: "org-2", EmployeeID: "emp-a", Category: "it", Message: "Other tenant"},
	}}
	svc := assignments.NewService(fixture.repo, stubJourneys{}, fixture.employees, fixture.notify, blockers)

	items, err := svc.ListTeamBlockers(context.Background(), testOrgID, testManagerUser)
	if err != nil {
		t.Fatalf("ListTeamBlockers: %v", err)
	}

	if len(items) != 1 || items[0].ID != "b1" {
		t.Fatalf("blockers = %+v, want only b1", items)
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

func TestHandleReportBlockerRecordsAudit(t *testing.T) {
	t.Parallel()

	fixture := newManagerFixture()
	auditRepo := &recordingAuditRepo{}
	svc := assignments.NewService(fixture.repo, stubJourneys{}, fixture.employees, fixture.notify, &stubBlockers{})
	handler := assignments.NewHandler(svc, audit.NewService(auditRepo))

	rctx := chi.NewRouteContext()
	ctx := security.WithPrincipal(
		context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
		security.Principal{UserID: "user-a", OrganizationID: testOrgID},
	)
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"/me/blockers",
		strings.NewReader(`{"stepAssignmentId":"s1","category":"it","message":"VPN access not provisioned"}`),
	)
	rec := httptest.NewRecorder()

	handler.HandleReportBlocker(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	if len(auditRepo.events) != 1 {
		t.Fatalf("recorded %d audit events, want 1", len(auditRepo.events))
	}

	event := auditRepo.events[0]
	if event.Action != "blocker.reported" || event.ResourceType != "blocker" {
		t.Errorf("audit event = %s %s, want blocker.reported/blocker", event.Action, event.ResourceType)
	}

	if event.ActorUserID != "user-a" {
		t.Errorf("actor = %q, want user-a", event.ActorUserID)
	}
}

func (r *recordingAuditRepo) CountByOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (r *recordingAuditRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
