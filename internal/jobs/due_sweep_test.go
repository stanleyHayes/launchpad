package jobs_test

import (
	"context"
	"testing"
	"time"

	"launchpad/internal/assignments"
	"launchpad/internal/employees"
	"launchpad/internal/jobs"
	"launchpad/internal/notifications"
)

type fakeStepStore struct {
	steps map[string]assignments.StepAssignment
}

func newFakeStepStore(steps ...assignments.StepAssignment) *fakeStepStore {
	store := &fakeStepStore{steps: map[string]assignments.StepAssignment{}}
	for _, step := range steps {
		store.steps[step.ID] = step
	}

	return store
}

func (f *fakeStepStore) ListDueSoonSteps(
	_ context.Context,
	from, to time.Time,
) ([]assignments.StepAssignment, error) {
	items := make([]assignments.StepAssignment, 0)

	for _, step := range f.steps {
		if step.Status == "completed" || step.DueAt == nil || step.DueSoonNotifiedAt != nil {
			continue
		}

		if step.DueAt.After(from) && !step.DueAt.After(to) {
			items = append(items, step)
		}
	}

	return items, nil
}

func (f *fakeStepStore) ListOverdueSteps(_ context.Context, now time.Time) ([]assignments.StepAssignment, error) {
	items := make([]assignments.StepAssignment, 0)

	for _, step := range f.steps {
		if step.Status == "completed" || step.DueAt == nil || step.OverdueNotifiedAt != nil {
			continue
		}

		if step.DueAt.Before(now) {
			items = append(items, step)
		}
	}

	return items, nil
}

func (f *fakeStepStore) UpdateStep(_ context.Context, step assignments.StepAssignment) error {
	f.steps[step.ID] = step

	return nil
}

type fakeEmployeeReader struct {
	byID map[string]employees.Employee
}

func (f fakeEmployeeReader) Get(_ context.Context, _, employeeID string) (employees.Employee, error) {
	employee, ok := f.byID[employeeID]
	if !ok {
		return employees.Employee{}, employees.ErrNotFound
	}

	return employee, nil
}

type fakeNotifier struct {
	created []notifications.CreateInput
}

func (f *fakeNotifier) Create(
	_ context.Context,
	_ string,
	in notifications.CreateInput,
) (notifications.Notification, error) {
	f.created = append(f.created, in)

	return notifications.Notification{ID: "n", Title: in.Title, Body: in.Body, UserID: in.UserID}, nil
}

func stepFixture(id string, dueAt time.Time) assignments.StepAssignment {
	return assignments.StepAssignment{
		ID:                  id,
		OrganizationID:      "org-1",
		JourneyAssignmentID: "asg-1",
		EmployeeID:          "emp-1",
		Title:               "Read the handbook",
		Status:              "in_progress",
		DueAt:               &dueAt,
	}
}

func newSweepDeps(steps ...assignments.StepAssignment) (*fakeStepStore, *fakeNotifier, jobs.SweepFunc) {
	store := newFakeStepStore(steps...)
	notifier := &fakeNotifier{}
	reader := fakeEmployeeReader{byID: map[string]employees.Employee{
		"emp-1": {ID: "emp-1", UserID: "user-1"},
	}}
	sweep := jobs.NewDueNotificationSweep(store, reader, notifier, jobs.DefaultDueSoonHorizon)

	return store, notifier, sweep
}

func titles(in []notifications.CreateInput) []string {
	out := make([]string, 0, len(in))
	for _, call := range in {
		out = append(out, call.Title)
	}

	return out
}

func TestDueSoonNotifiesOnceAcrossTicks(t *testing.T) {
	t.Parallel()

	store, notifier, sweep := newSweepDeps(stepFixture("step-1", time.Now().Add(6*time.Hour)))

	for range 2 {
		if err := sweep(context.Background()); err != nil {
			t.Fatalf("sweep tick: %v", err)
		}
	}

	if len(notifier.created) != 1 {
		t.Fatalf("expected one due-soon notification across two ticks, got %v", titles(notifier.created))
	}

	call := notifier.created[0]
	if call.Title != "Step due soon" || call.UserID != "user-1" {
		t.Fatalf("unexpected notification: %+v", call)
	}

	if call.Type != notifications.TypeDueSoon {
		t.Fatalf("type = %q, want due_soon", call.Type)
	}

	if call.Link != "/assignments/asg-1" {
		t.Fatalf("link = %q, want /assignments/asg-1", call.Link)
	}

	if store.steps["step-1"].DueSoonNotifiedAt == nil {
		t.Fatal("expected step marked dueSoonNotifiedAt")
	}
}

func TestOverdueNotifiesOnceAcrossTicks(t *testing.T) {
	t.Parallel()

	store, notifier, sweep := newSweepDeps(stepFixture("step-1", time.Now().Add(-2*time.Hour)))

	for range 2 {
		if err := sweep(context.Background()); err != nil {
			t.Fatalf("sweep tick: %v", err)
		}
	}

	if len(notifier.created) != 1 {
		t.Fatalf("expected one overdue notification across two ticks, got %v", titles(notifier.created))
	}

	if notifier.created[0].Title != "Step overdue" {
		t.Fatalf("unexpected notification: %+v", notifier.created[0])
	}

	if notifier.created[0].Type != notifications.TypeOverdue {
		t.Fatalf("type = %q, want overdue", notifier.created[0].Type)
	}

	if notifier.created[0].Link != "/assignments/asg-1" {
		t.Fatalf("link = %q, want /assignments/asg-1", notifier.created[0].Link)
	}

	step := store.steps["step-1"]
	if step.OverdueNotifiedAt == nil {
		t.Fatal("expected step marked overdueNotifiedAt")
	}

	if step.Status != "in_progress" {
		t.Fatalf("sweep must not change step status, got %q", step.Status)
	}
}

func TestSweepSkipsCompletedSteps(t *testing.T) {
	t.Parallel()

	dueSoon := stepFixture("step-due", time.Now().Add(6*time.Hour))
	dueSoon.Status = "completed"
	overdue := stepFixture("step-over", time.Now().Add(-2*time.Hour))
	overdue.Status = "completed"

	_, notifier, sweep := newSweepDeps(dueSoon, overdue)

	if err := sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if len(notifier.created) != 0 {
		t.Fatalf("completed steps must not be notified, got %v", titles(notifier.created))
	}
}

func TestSweepSkipsEmployeesWithoutAccount(t *testing.T) {
	t.Parallel()

	store := newFakeStepStore(stepFixture("step-1", time.Now().Add(6*time.Hour)))
	notifier := &fakeNotifier{}
	reader := fakeEmployeeReader{byID: map[string]employees.Employee{
		"emp-1": {ID: "emp-1", UserID: ""},
	}}
	sweep := jobs.NewDueNotificationSweep(store, reader, notifier, jobs.DefaultDueSoonHorizon)

	if err := sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if len(notifier.created) != 0 {
		t.Fatalf("no account means no notification, got %v", titles(notifier.created))
	}
}
