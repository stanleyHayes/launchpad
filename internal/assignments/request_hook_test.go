package assignments_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"launchpad/internal/assignments"
)

type requestCall struct {
	organizationID string
	employeeID     string
	kind           string
	item           string
	details        string
}

// stubRequestCreator captures CreateFromStep calls.
type stubRequestCreator struct {
	calls []requestCall
	err   error
}

func (s *stubRequestCreator) CreateFromStep(
	_ context.Context,
	organizationID, employeeID, kind, item, details string,
) error {
	if s.err != nil {
		return s.err
	}

	s.calls = append(s.calls, requestCall{
		organizationID: organizationID,
		employeeID:     employeeID,
		kind:           kind,
		item:           item,
		details:        details,
	})

	return nil
}

func TestCompleteEquipmentRequestStepAutoCreatesRequest(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, testStatusInProgress, "equipment_request")
	repo.steps[testStepID] = withInstructions(repo.steps[testStepID], "Standard build")

	creator := &stubRequestCreator{}
	svc := newStepSvc(repo, nil)
	svc.SetRequestCreator(creator)

	step, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.CompleteStepInput{Submission: map[string]any{"item": "laptop"}},
	)
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if step.Status != "completed" {
		t.Fatalf("status = %q, want completed", step.Status)
	}

	if len(creator.calls) != 1 {
		t.Fatalf("calls = %+v, want one request", creator.calls)
	}

	call := creator.calls[0]
	if call.kind != "equipment" {
		t.Errorf("kind = %q, want equipment", call.kind)
	}

	if call.item != "laptop" {
		t.Errorf("item = %q, want laptop from submission", call.item)
	}

	if call.organizationID != testOrgID || call.employeeID != testEmployeeID {
		t.Errorf("scope = %q/%q, want %q/%q", call.organizationID, call.employeeID, testOrgID, testEmployeeID)
	}

	if !strings.Contains(call.details, "Read the handbook") ||
		!strings.Contains(call.details, "Standard build") {
		t.Errorf("details should carry step title and instructions, got %q", call.details)
	}
}

func TestCompleteAccessRequestStepAutoCreatesAccessRequest(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, testStatusInProgress, "access_request")

	creator := &stubRequestCreator{}
	svc := newStepSvc(repo, nil)
	svc.SetRequestCreator(creator)

	if _, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.CompleteStepInput{},
	); err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if len(creator.calls) != 1 || creator.calls[0].kind != "access" {
		t.Fatalf("calls = %+v, want one access request", creator.calls)
	}

	if creator.calls[0].item != "" {
		t.Errorf("item = %q, want empty (no submission)", creator.calls[0].item)
	}
}

func TestCompleteRequestStepFailsWhenRequestCreationFails(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, testStatusInProgress, "equipment_request")

	creator := &stubRequestCreator{err: errors.New("requests store down")}
	svc := newStepSvc(repo, nil)
	svc.SetRequestCreator(creator)

	_, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.CompleteStepInput{},
	)
	if err == nil {
		t.Fatal("expected CompleteStep to fail when request creation fails")
	}

	if repo.steps[testStepID].Status != testStatusInProgress {
		t.Fatalf("step status persisted as %q, want still in_progress", repo.steps[testStepID].Status)
	}
}

func TestCompleteRequestStepWithoutCreatorStillCompletes(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, testStatusInProgress, "equipment_request")
	svc := newStepSvc(repo, nil)

	step, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.CompleteStepInput{},
	)
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if step.Status != "completed" {
		t.Fatalf("status = %q, want completed", step.Status)
	}
}

func TestCompleteTaskStepDoesNotCreateRequest(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, testStatusInProgress, "task")

	creator := &stubRequestCreator{}
	svc := newStepSvc(repo, nil)
	svc.SetRequestCreator(creator)

	if _, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.CompleteStepInput{},
	); err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if len(creator.calls) != 0 {
		t.Fatalf("calls = %+v, want none for a task step", creator.calls)
	}
}

func withInstructions(step assignments.StepAssignment, instructions string) assignments.StepAssignment {
	step.Instructions = instructions

	return step
}
