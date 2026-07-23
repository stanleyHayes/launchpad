package analytics_test

import (
	"context"
	"testing"
	"time"

	"launchpad/internal/analytics"
	"launchpad/internal/assignments"
	"launchpad/internal/employees"
)

type stubAssignments struct {
	items     []assignments.JourneyAssignment
	approvals []assignments.Approval
}

func (s stubAssignments) List(context.Context, string) ([]assignments.JourneyAssignment, error) {
	return s.items, nil
}

func (s stubAssignments) ListApprovals(context.Context, string) ([]assignments.Approval, error) {
	return s.approvals, nil
}

type stubEmployees struct {
	items []employees.Employee
}

func (s stubEmployees) List(context.Context, string, int64) ([]employees.Employee, error) {
	return s.items, nil
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
		stubEmployees{items: []employees.Employee{{}, {}}},
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
