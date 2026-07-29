package employees_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"launchpad/internal/employees"
)

type recordingRuleApplier struct {
	calls []employees.Employee
	err   error
}

func (r *recordingRuleApplier) ApplyAssignmentRules(_ context.Context, employee employees.Employee) error {
	r.calls = append(r.calls, employee)

	return r.err
}

func validCreateInput() employees.CreateInput {
	return employees.CreateInput{
		FirstName: "Ada",
		LastName:  "Lovelace",
		WorkEmail: "ada@example.com",
		StartDate: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestCreateAppliesAssignmentRules(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	applier := &recordingRuleApplier{}
	svc := employees.NewService(repo, noopReferences{})
	svc.SetRuleApplier(applier)

	created, err := svc.Create(context.Background(), "org-1", validCreateInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(applier.calls) != 1 || applier.calls[0].ID != created.ID {
		t.Fatalf("applier calls = %+v, want one call with the created employee", applier.calls)
	}
}

func TestCreateWithoutRuleApplierIsNoOp(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := employees.NewService(repo, noopReferences{})

	created, err := svc.Create(context.Background(), "org-1", validCreateInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created.ID == "" || repo.items[created.ID].ID != created.ID {
		t.Fatalf("employee not persisted: %+v", created)
	}
}

func TestCreateSucceedsWhenRuleApplicationFails(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	applier := &recordingRuleApplier{err: errors.New("boom")}
	svc := employees.NewService(repo, noopReferences{})
	svc.SetRuleApplier(applier)

	created, err := svc.Create(context.Background(), "org-1", validCreateInput())
	if err != nil {
		t.Fatalf("Create must not fail on rule application errors: %v", err)
	}

	if len(applier.calls) != 1 || repo.items[created.ID].ID != created.ID {
		t.Fatalf("employee not persisted despite rule failure: %+v", created)
	}
}
