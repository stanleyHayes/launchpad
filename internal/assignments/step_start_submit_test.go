package assignments_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"launchpad/internal/assignments"
	"launchpad/internal/employees"
	"launchpad/internal/notifications"
)

// newStepSvc builds a service with one employee (testEmployeeID, owned by
// testEmployeeUser) and the given steps in the repo.
func newStepSvc(repo *memoryRepo, notify assignments.Notifier) *assignments.Service {
	return assignments.NewService(
		repo,
		stubJourneys{},
		stubEmployees{
			byID: map[string]employees.Employee{
				testEmployeeID: {ID: testEmployeeID, UserID: testEmployeeUser},
			},
			byUserID: map[string]employees.Employee{
				testEmployeeUser: {ID: testEmployeeID, UserID: testEmployeeUser},
			},
		},
		notify,
	)
}

func seedStep(repo *memoryRepo, status, stepType string) {
	repo.assignments[testAssignmentID] = assignments.JourneyAssignment{
		ID:             testAssignmentID,
		OrganizationID: testOrgID,
		EmployeeID:     testEmployeeID,
		Status:         testStatusInProgress,
	}
	repo.steps[testStepID] = assignments.StepAssignment{
		ID:                  testStepID,
		OrganizationID:      testOrgID,
		JourneyAssignmentID: testAssignmentID,
		EmployeeID:          testEmployeeID,
		StepType:            stepType,
		Title:               "Read the handbook",
		Status:              status,
		CreatedAt:           time.Now().UTC(),
	}
}

func TestStartStepMovesPendingToInProgress(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, "pending", "document")
	svc := newStepSvc(repo, nil)

	step, err := svc.StartStep(context.Background(), testOrgID, testEmployeeUser, testStepID)
	if err != nil {
		t.Fatalf("StartStep: %v", err)
	}

	if step.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", step.Status)
	}

	if step.StartedAt == nil {
		t.Fatal("expected startedAt set")
	}

	if repo.steps[testStepID].StartedAt == nil {
		t.Fatal("expected startedAt persisted")
	}
}

func TestStartStepIsIdempotentAndKeepsFirstStartedAt(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, "pending", "task")
	svc := newStepSvc(repo, nil)

	first, err := svc.StartStep(context.Background(), testOrgID, testEmployeeUser, testStepID)
	if err != nil {
		t.Fatalf("first StartStep: %v", err)
	}

	second, err := svc.StartStep(context.Background(), testOrgID, testEmployeeUser, testStepID)
	if err != nil {
		t.Fatalf("second StartStep: %v", err)
	}

	if !second.StartedAt.Equal(*first.StartedAt) {
		t.Fatalf("startedAt changed on restart: %v vs %v", second.StartedAt, first.StartedAt)
	}
}

func TestStartStepRejectsCompletedAndNonOwner(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, "completed", "document")
	svc := newStepSvc(repo, nil)

	if _, err := svc.StartStep(context.Background(), testOrgID, testEmployeeUser, testStepID); !errors.Is(
		err,
		assignments.ErrInvalidState,
	) {
		t.Fatalf("completed step: got %v, want ErrInvalidState", err)
	}

	repo.steps[testStepID] = assignments.StepAssignment{
		ID:             testStepID,
		OrganizationID: testOrgID,
		EmployeeID:     "emp-other",
		Status:         "pending",
	}

	if _, err := svc.StartStep(context.Background(), testOrgID, testEmployeeUser, testStepID); !errors.Is(
		err,
		assignments.ErrInvalidState,
	) {
		t.Fatalf("non-owner: got %v, want ErrInvalidState", err)
	}
}

func TestSubmitStepStoresSubmissionWithoutCompleting(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, testStatusInProgress, "document")
	svc := newStepSvc(repo, nil)

	step, err := svc.SubmitStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.SubmitStepInput{Submission: map[string]any{"draft": "half done"}},
	)
	if err != nil {
		t.Fatalf("SubmitStep: %v", err)
	}

	if step.Status != testStatusInProgress {
		t.Fatalf("status = %q, want still in_progress", step.Status)
	}

	if step.CompletedAt != nil {
		t.Fatal("submit must not complete the step")
	}

	if step.Submission["draft"] != "half done" {
		t.Fatalf("submission not stored: %+v", step.Submission)
	}

	if step.StartedAt == nil {
		t.Fatal("submit implies started; expected startedAt set")
	}

	if repo.steps[testStepID].Submission["draft"] != "half done" {
		t.Fatal("expected submission persisted")
	}
}

func TestSubmitStepRejectsQuizPendingAndEmpty(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, testStatusInProgress, "quiz")
	svc := newStepSvc(repo, nil)

	_, err := svc.SubmitStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.SubmitStepInput{Submission: map[string]any{"answers": map[string]int{"q1": 0}}},
	)
	if !errors.Is(err, assignments.ErrInvalidState) {
		t.Fatalf("quiz submit: got %v, want ErrInvalidState", err)
	}

	seedStep(repo, "pending", "document")

	_, err = svc.SubmitStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.SubmitStepInput{Submission: map[string]any{"draft": "x"}},
	)
	if !errors.Is(err, assignments.ErrInvalidState) {
		t.Fatalf("pending step submit: got %v, want ErrInvalidState", err)
	}

	_, err = svc.SubmitStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.SubmitStepInput{},
	)
	if !errors.Is(err, assignments.ErrInvalidInput) {
		t.Fatalf("empty submission: got %v, want ErrInvalidInput", err)
	}
}

func TestApprovalNotificationsCarryTypeAndLink(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	notify := &stubNotify{}
	svc := assignments.NewService(
		repo,
		stubJourneys{},
		stubEmployees{
			byID: map[string]employees.Employee{
				testEmployeeID: {ID: testEmployeeID, UserID: testEmployeeUser},
			},
			byUserID: map[string]employees.Employee{},
		},
		notify,
	)

	seedStep(repo, "awaiting_approval", "approval")
	repo.approvals[testApprovalID] = assignments.Approval{
		ID:               testApprovalID,
		OrganizationID:   testOrgID,
		StepAssignmentID: testStepID,
		ApproverUserID:   testManagerUser,
		Status:           "pending",
	}

	_, err := svc.DecideApproval(
		context.Background(),
		testOrgID,
		testManagerUser,
		testApprovalID,
		assignments.DecideApprovalInput{Approve: false, Note: "Fix serial"},
	)
	if err != nil {
		t.Fatalf("DecideApproval: %v", err)
	}

	var decision *notifications.CreateInput

	for i := range notify.calls {
		if notify.calls[i].Title == "Step rejected" {
			decision = &notify.calls[i]
		}
	}

	if decision == nil {
		t.Fatalf("expected step-rejected notification, got %+v", notify.calls)
	}

	if decision.Type != notifications.TypeApproval {
		t.Fatalf("type = %q, want approval", decision.Type)
	}

	if decision.Link != "/assignments/"+testAssignmentID {
		t.Fatalf("link = %q, want /assignments/%s", decision.Link, testAssignmentID)
	}
}

func TestJourneyCompletedNotificationCarriesTypeAndLink(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	notify := &stubNotify{}
	svc := newStepSvc(repo, notify)
	seedStep(repo, testStatusInProgress, "document")

	_, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.CompleteStepInput{},
	)
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if len(notify.calls) != 1 {
		t.Fatalf("expected one journey-completed notification, got %+v", notify.calls)
	}

	call := notify.calls[0]
	if call.Type != notifications.TypeJourneyCompleted {
		t.Fatalf("type = %q, want journey_completed", call.Type)
	}

	if call.Link != "/assignments/"+testAssignmentID {
		t.Fatalf("link = %q, want /assignments/%s", call.Link, testAssignmentID)
	}
}
