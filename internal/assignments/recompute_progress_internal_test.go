package assignments

import (
	"context"
	"testing"
	"time"

	"launchpad/internal/employees"
	"launchpad/internal/notifications"
)

type recomputeStubRepo struct {
	Repository

	assignment JourneyAssignment
	steps      []StepAssignment
}

func (r *recomputeStubRepo) GetAssignment(context.Context, string, string) (JourneyAssignment, error) {
	return r.assignment, nil
}

func (r *recomputeStubRepo) ListSteps(context.Context, string, string) ([]StepAssignment, error) {
	return r.steps, nil
}

func (r *recomputeStubRepo) UpdateAssignment(_ context.Context, assignment JourneyAssignment) error {
	r.assignment = assignment

	return nil
}

type recomputeStubEmployees struct {
	EmployeeReader

	employee employees.Employee
}

func (s recomputeStubEmployees) Get(context.Context, string, string) (employees.Employee, error) {
	return s.employee, nil
}

type recomputeStubNotifier struct {
	Notifier

	calls []notifications.CreateInput
}

func (n *recomputeStubNotifier) Create(
	_ context.Context,
	_ string,
	in notifications.CreateInput,
) (notifications.Notification, error) {
	n.calls = append(n.calls, in)

	return notifications.Notification{}, nil
}

// A recompute of an already-completed assignment must not re-notify: the
// CompletedAt guard fires exactly once.
func TestRecomputeProgressNotifiesJourneyCompletedOnce(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &recomputeStubRepo{
		assignment: JourneyAssignment{
			ID:             "asg-1",
			OrganizationID: "org-1",
			EmployeeID:     "emp-1",
			Status:         statusInProgress,
		},
		steps: []StepAssignment{
			{ID: "s1", Status: stepCompleted, CompletedAt: &now},
			{ID: "s2", Status: stepCompleted, CompletedAt: &now},
		},
	}
	notifier := &recomputeStubNotifier{}
	svc := NewService(
		repo,
		nil,
		recomputeStubEmployees{employee: employees.Employee{ID: "emp-1", UserID: "user-1"}},
		notifier,
	)

	for range 2 {
		if err := svc.recomputeProgress(context.Background(), "org-1", "asg-1"); err != nil {
			t.Fatalf("recomputeProgress: %v", err)
		}
	}

	if len(notifier.calls) != 1 {
		t.Fatalf("expected exactly one journey-completed notification across recomputes, got %d", len(notifier.calls))
	}
}
