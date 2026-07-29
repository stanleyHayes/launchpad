package analytics_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"launchpad/internal/analytics"
	"launchpad/internal/assignments"
	"launchpad/internal/assistant"
	"launchpad/internal/departments"
	"launchpad/internal/employees"
)

type stubAssignments struct {
	items     []assignments.JourneyAssignment
	approvals []assignments.Approval
	steps     map[string][]assignments.StepAssignment
}

func (s stubAssignments) List(context.Context, string) ([]assignments.JourneyAssignment, error) {
	return s.items, nil
}

func (s stubAssignments) ListApprovals(context.Context, string) ([]assignments.Approval, error) {
	return s.approvals, nil
}

func (s stubAssignments) ListSteps(
	_ context.Context,
	_ string,
	journeyAssignmentID string,
) ([]assignments.StepAssignment, error) {
	return s.steps[journeyAssignmentID], nil
}

type stubEmployees struct {
	count int64
	items []employees.Employee
}

func (s stubEmployees) Count(context.Context, string) (int64, error) {
	return s.count, nil
}

func (s stubEmployees) List(
	_ context.Context,
	_ string,
	offset, limit int64,
) ([]employees.Employee, error) {
	if offset >= int64(len(s.items)) {
		return nil, nil
	}

	end := min(offset+limit, int64(len(s.items)))

	return s.items[offset:end], nil
}

type stubDirectory struct {
	departments []departments.Department
	jobRoles    []departments.JobRole
}

func (s stubDirectory) ListDepartments(context.Context, string) ([]departments.Department, error) {
	return s.departments, nil
}

func (s stubDirectory) ListJobRoles(context.Context, string) ([]departments.JobRole, error) {
	return s.jobRoles, nil
}

type stubAssistant struct {
	interactions []assistant.Interaction
}

func (s stubAssistant) ListInteractions(
	_ context.Context,
	organizationID string,
) ([]assistant.Interaction, error) {
	interactions := make([]assistant.Interaction, 0, len(s.interactions))
	for _, interaction := range s.interactions {
		if interaction.OrganizationID == organizationID {
			interactions = append(interactions, interaction)
		}
	}

	return interactions, nil
}

func TestOnboardingSummaryComputesRates(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	startsAt := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)

	svc := analytics.NewService(
		stubAssignments{
			items: []assignments.JourneyAssignment{
				{Status: "scheduled"},
				{Status: "in_progress"},
				{Status: "completed", StartsAt: startsAt, CompletedAt: &completedAt},
			},
			approvals: []assignments.Approval{
				{Status: "pending"},
				{Status: "approved"},
			},
		},
		stubEmployees{count: 2},
	)

	summary, err := svc.OnboardingSummary(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("OnboardingSummary: %v", err)
	}

	if summary.EmployeeCount != 2 {
		t.Fatalf("employee count = %d, want 2", summary.EmployeeCount)
	}

	if summary.ScheduledAssignmentCount != 1 ||
		summary.ActiveAssignmentCount != 1 ||
		summary.CompletedAssignmentCount != 1 {
		t.Fatalf("unexpected assignment counts: %+v", summary)
	}

	if summary.PendingApprovalCount != 1 {
		t.Fatalf("pending approvals = %d, want 1", summary.PendingApprovalCount)
	}

	if summary.CompletionRate != 0.33 {
		t.Fatalf("completion rate = %v, want 0.33", summary.CompletionRate)
	}

	if summary.AverageDaysToComplete != 9 {
		t.Fatalf("average days = %v, want 9", summary.AverageDaysToComplete)
	}
}

func TestOnboardingSummaryRejectsEmptyOrg(t *testing.T) {
	t.Parallel()

	svc := analytics.NewService(stubAssignments{}, stubEmployees{})

	_, err := svc.OnboardingSummary(context.Background(), "")
	if err == nil {
		t.Fatal("expected invalid input")
	}
}

func TestOnboardingSummaryComputesOverdueRate(t *testing.T) {
	t.Parallel()

	past := time.Now().UTC().Add(-48 * time.Hour)
	future := time.Now().UTC().Add(48 * time.Hour)

	svc := analytics.NewService(
		stubAssignments{
			items: []assignments.JourneyAssignment{
				{ID: "a-1", Status: "in_progress"},
				{ID: "a-2", Status: "completed"},
			},
			steps: map[string][]assignments.StepAssignment{
				"a-1": {
					{Status: "pending", DueAt: &past},
					{Status: "in_progress", DueAt: &past},
					{Status: "pending", DueAt: &future},
					{Status: "pending"},
					{Status: "completed", DueAt: &past},
				},
				"a-2": {
					{Status: "completed", DueAt: &past},
				},
			},
		},
		stubEmployees{count: 1},
	)

	summary, err := svc.OnboardingSummary(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("OnboardingSummary: %v", err)
	}

	if summary.IncompleteStepCount != 4 {
		t.Fatalf("incomplete steps = %d, want 4", summary.IncompleteStepCount)
	}

	if summary.OverdueStepCount != 2 {
		t.Fatalf("overdue steps = %d, want 2", summary.OverdueStepCount)
	}

	if summary.OverdueRate != 0.5 {
		t.Fatalf("overdue rate = %v, want 0.5", summary.OverdueRate)
	}
}

func breakdownService() *analytics.Service {
	return analytics.NewService(
		stubAssignments{
			items: []assignments.JourneyAssignment{
				{EmployeeID: "e-1", Status: "completed"},
				{EmployeeID: "e-1", Status: "in_progress"},
				{EmployeeID: "e-2", Status: "completed"},
				{EmployeeID: "e-3", Status: "scheduled"},
				{EmployeeID: "e-gone", Status: "completed"},
			},
		},
		stubEmployees{
			count: 3,
			items: []employees.Employee{
				{ID: "e-1", DepartmentID: "d-eng", JobRoleID: "r-eng"},
				{ID: "e-2", DepartmentID: "d-eng", JobRoleID: "r-ops"},
				{ID: "e-3"},
			},
		},
	).WithSources(
		stubDirectory{
			departments: []departments.Department{{ID: "d-eng", Name: "Engineering"}},
			jobRoles: []departments.JobRole{
				{ID: "r-eng", Name: "Engineer"},
				{ID: "r-ops", Name: "Operator"},
			},
		},
		stubAssistant{},
	)
}

func TestOnboardingBreakdownByDepartment(t *testing.T) {
	t.Parallel()

	breakdown, err := breakdownService().OnboardingBreakdown(
		context.Background(),
		"org-1",
		analytics.BreakdownByDepartment,
	)
	if err != nil {
		t.Fatalf("OnboardingBreakdown: %v", err)
	}

	if breakdown.By != analytics.BreakdownByDepartment {
		t.Fatalf("by = %q, want %q", breakdown.By, analytics.BreakdownByDepartment)
	}

	if len(breakdown.Rows) != 2 {
		t.Fatalf("rows = %d, want 2: %+v", len(breakdown.Rows), breakdown.Rows)
	}

	engineering := breakdown.Rows[0]
	if engineering.ID != "d-eng" || engineering.Name != "Engineering" {
		t.Fatalf("unexpected first row: %+v", engineering)
	}

	if engineering.EmployeeCount != 2 || engineering.AssignmentCount != 3 ||
		engineering.CompletedAssignmentCount != 2 {
		t.Fatalf("unexpected engineering counts: %+v", engineering)
	}

	if engineering.CompletionRate != 0.67 {
		t.Fatalf("engineering completion rate = %v, want 0.67", engineering.CompletionRate)
	}

	unassigned := breakdown.Rows[1]
	if unassigned.ID != "" || unassigned.Name != "Unassigned" {
		t.Fatalf("unexpected unassigned row: %+v", unassigned)
	}

	if unassigned.EmployeeCount != 1 || unassigned.AssignmentCount != 1 ||
		unassigned.CompletedAssignmentCount != 0 || unassigned.CompletionRate != 0 {
		t.Fatalf("unexpected unassigned counts: %+v", unassigned)
	}
}

func TestOnboardingBreakdownByJobRole(t *testing.T) {
	t.Parallel()

	breakdown, err := breakdownService().OnboardingBreakdown(
		context.Background(),
		"org-1",
		analytics.BreakdownByJobRole,
	)
	if err != nil {
		t.Fatalf("OnboardingBreakdown: %v", err)
	}

	if len(breakdown.Rows) != 3 {
		t.Fatalf("rows = %d, want 3: %+v", len(breakdown.Rows), breakdown.Rows)
	}

	byID := map[string]analytics.BreakdownRow{}
	for _, row := range breakdown.Rows {
		byID[row.ID] = row
	}

	engineer := byID["r-eng"]
	if engineer.Name != "Engineer" || engineer.AssignmentCount != 2 ||
		engineer.CompletedAssignmentCount != 1 || engineer.CompletionRate != 0.5 {
		t.Fatalf("unexpected engineer row: %+v", engineer)
	}

	operator := byID["r-ops"]
	if operator.Name != "Operator" || operator.AssignmentCount != 1 ||
		operator.CompletedAssignmentCount != 1 || operator.CompletionRate != 1 {
		t.Fatalf("unexpected operator row: %+v", operator)
	}

	if byID[""].Name != "Unassigned" || byID[""].EmployeeCount != 1 {
		t.Fatalf("unexpected unassigned row: %+v", byID[""])
	}
}

func TestOnboardingBreakdownRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	svc := breakdownService()

	if _, err := svc.OnboardingBreakdown(context.Background(), "org-1", "location"); err == nil {
		t.Fatal("expected invalid group-by error")
	}

	if _, err := svc.OnboardingBreakdown(context.Background(), "", analytics.BreakdownByDepartment); err == nil {
		t.Fatal("expected empty organization error")
	}
}

func TestOnboardingBreakdownRequiresDirectory(t *testing.T) {
	t.Parallel()

	svc := analytics.NewService(stubAssignments{}, stubEmployees{})

	if _, err := svc.OnboardingBreakdown(
		context.Background(),
		"org-1",
		analytics.BreakdownByDepartment,
	); err == nil {
		t.Fatal("expected source-not-configured error")
	}
}

func TestBreakdownIsolatesTenants(t *testing.T) {
	t.Parallel()

	svc := analytics.NewService(
		stubAssignments{
			items: []assignments.JourneyAssignment{
				{OrganizationID: "org-1", EmployeeID: "e-1", Status: "completed"},
			},
		},
		stubEmployees{
			count: 1,
			items: []employees.Employee{
				{ID: "e-1", OrganizationID: "org-1", DepartmentID: "d-eng"},
			},
		},
	).WithSources(
		stubDirectory{departments: []departments.Department{{ID: "d-eng", Name: "Engineering"}}},
		stubAssistant{
			interactions: []assistant.Interaction{
				{OrganizationID: "org-1", Question: "org one question", Refused: true},
				{OrganizationID: "org-2", Question: "org two question", Refused: true},
			},
		},
	)

	report, err := svc.AssistantReport(context.Background(), "org-2")
	if err != nil {
		t.Fatalf("AssistantReport: %v", err)
	}

	if report.TotalQuestions != 1 {
		t.Fatalf("total questions = %d, want 1 (org-2 only)", report.TotalQuestions)
	}

	for _, stat := range report.TopRefusedQuestions {
		if stat.Question == "org one question" {
			t.Fatalf("cross-tenant question leaked into report: %+v", report.TopRefusedQuestions)
		}
	}
}

func TestAssistantReportAggregates(t *testing.T) {
	t.Parallel()

	interactions := []assistant.Interaction{
		{OrganizationID: "org-1", Question: "How do I enroll in benefits?", Refused: true},
		{OrganizationID: "org-1", Question: "How do I enroll in benefits?", Refused: true},
		{OrganizationID: "org-1", Question: "How do I enroll in benefits?", Refused: true},
		{OrganizationID: "org-1", Question: "What is the wifi password?", Refused: true, Helpful: new(false)},
		{OrganizationID: "org-1", Question: "Where is the office?", Refused: false, Helpful: new(true)},
		{OrganizationID: "org-1", Question: "Who is my buddy?", Refused: false, Helpful: new(true)},
		{OrganizationID: "org-1", Question: "When do I start?", Refused: false},
	}

	svc := analytics.NewService(stubAssignments{}, stubEmployees{}).
		WithSources(stubDirectory{}, stubAssistant{interactions: interactions})

	report, err := svc.AssistantReport(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("AssistantReport: %v", err)
	}

	if report.TotalQuestions != 7 {
		t.Fatalf("total questions = %d, want 7", report.TotalQuestions)
	}

	if report.RefusalCount != 4 || report.RefusalRate != 0.57 {
		t.Fatalf("refusals = %d rate = %v, want 4 / 0.57", report.RefusalCount, report.RefusalRate)
	}

	if report.FeedbackCount != 3 || report.HelpfulCount != 2 || report.HelpfulRate != 0.67 {
		t.Fatalf(
			"feedback = %d helpful = %d rate = %v, want 3 / 2 / 0.67",
			report.FeedbackCount, report.HelpfulCount, report.HelpfulRate,
		)
	}

	if len(report.TopRefusedQuestions) != 2 {
		t.Fatalf("top refused = %d, want 2: %+v", len(report.TopRefusedQuestions), report.TopRefusedQuestions)
	}

	top := report.TopRefusedQuestions[0]
	if top.Question != "How do I enroll in benefits?" || top.Count != 3 {
		t.Fatalf("unexpected top refused question: %+v", top)
	}
}

func TestAssistantReportCapsTopRefusedAtTen(t *testing.T) {
	t.Parallel()

	interactions := make([]assistant.Interaction, 0, 12)
	for index := range 12 {
		interactions = append(interactions, assistant.Interaction{
			OrganizationID: "org-1",
			Question:       fmt.Sprintf("question %02d", index),
			Refused:        true,
		})
	}

	svc := analytics.NewService(stubAssignments{}, stubEmployees{}).
		WithSources(stubDirectory{}, stubAssistant{interactions: interactions})

	report, err := svc.AssistantReport(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("AssistantReport: %v", err)
	}

	if len(report.TopRefusedQuestions) != 10 {
		t.Fatalf("top refused = %d, want capped 10", len(report.TopRefusedQuestions))
	}
}

func TestAssistantReportRequiresSource(t *testing.T) {
	t.Parallel()

	svc := analytics.NewService(stubAssignments{}, stubEmployees{})

	if _, err := svc.AssistantReport(context.Background(), "org-1"); err == nil {
		t.Fatal("expected source-not-configured error")
	}

	if _, err := svc.WithSources(stubDirectory{}, stubAssistant{}).
		AssistantReport(context.Background(), ""); err == nil {
		t.Fatal("expected empty organization error")
	}
}
