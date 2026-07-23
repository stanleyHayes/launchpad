package assignments_test

import (
	"context"
	"testing"
	"time"

	"launchpad/internal/assignments"
	"launchpad/internal/employees"
	"launchpad/internal/journeys"
	"launchpad/internal/notifications"
)

const (
	testOrgID            = "org-1"
	testEmployeeID       = "emp-1"
	testEmployeeUser     = "user-emp"
	testManagerUser      = "mgr-1"
	testAssignmentID     = "asg-1"
	testStepID           = "step-1"
	testApprovalID       = "appr-1"
	testStatusInProgress = "in_progress"
)

type memoryRepo struct {
	assignments map[string]assignments.JourneyAssignment
	steps       map[string]assignments.StepAssignment
	approvals   map[string]assignments.Approval
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		assignments: map[string]assignments.JourneyAssignment{},
		steps:       map[string]assignments.StepAssignment{},
		approvals:   map[string]assignments.Approval{},
	}
}

func (m *memoryRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryRepo) CreateAssignment(_ context.Context, assignment assignments.JourneyAssignment) error {
	m.assignments[assignment.ID] = assignment

	return nil
}

func (m *memoryRepo) CreateStepAssignments(_ context.Context, steps []assignments.StepAssignment) error {
	for _, step := range steps {
		m.steps[step.ID] = step
	}

	return nil
}

func (m *memoryRepo) CreateApproval(_ context.Context, approval assignments.Approval) error {
	m.approvals[approval.ID] = approval

	return nil
}

func (m *memoryRepo) FindActiveAssignment(
	context.Context,
	string,
	string,
	string,
) (assignments.JourneyAssignment, error) {
	return assignments.JourneyAssignment{}, assignments.ErrNotFound
}

func (m *memoryRepo) GetAssignment(
	_ context.Context,
	_,
	assignmentID string,
) (assignments.JourneyAssignment, error) {
	item, ok := m.assignments[assignmentID]
	if !ok {
		return assignments.JourneyAssignment{}, assignments.ErrNotFound
	}

	return item, nil
}

func (m *memoryRepo) ListAssignments(context.Context, string) ([]assignments.JourneyAssignment, error) {
	return nil, nil
}

func (m *memoryRepo) ListAssignmentsForEmployee(
	context.Context,
	string,
	string,
) ([]assignments.JourneyAssignment, error) {
	return nil, nil
}

func (m *memoryRepo) UpdateAssignment(_ context.Context, assignment assignments.JourneyAssignment) error {
	m.assignments[assignment.ID] = assignment

	return nil
}

func (m *memoryRepo) ListSteps(
	_ context.Context,
	_,
	journeyAssignmentID string,
) ([]assignments.StepAssignment, error) {
	items := make([]assignments.StepAssignment, 0)

	for _, step := range m.steps {
		if step.JourneyAssignmentID == journeyAssignmentID {
			items = append(items, step)
		}
	}

	return items, nil
}

func (m *memoryRepo) GetStep(_ context.Context, _, stepAssignmentID string) (assignments.StepAssignment, error) {
	item, ok := m.steps[stepAssignmentID]
	if !ok {
		return assignments.StepAssignment{}, assignments.ErrStepNotFound
	}

	return item, nil
}

func (m *memoryRepo) UpdateStep(_ context.Context, step assignments.StepAssignment) error {
	m.steps[step.ID] = step

	return nil
}

func (m *memoryRepo) GetApproval(_ context.Context, _, approvalID string) (assignments.Approval, error) {
	item, ok := m.approvals[approvalID]
	if !ok {
		return assignments.Approval{}, assignments.ErrNotFound
	}

	return item, nil
}

func (m *memoryRepo) ListApprovals(context.Context, string) ([]assignments.Approval, error) {
	items := make([]assignments.Approval, 0, len(m.approvals))
	for _, approval := range m.approvals {
		items = append(items, approval)
	}

	return items, nil
}

func (m *memoryRepo) GetApprovalByStep(
	_ context.Context,
	_,
	stepAssignmentID string,
) (assignments.Approval, error) {
	for _, approval := range m.approvals {
		if approval.StepAssignmentID == stepAssignmentID {
			return approval, nil
		}
	}

	return assignments.Approval{}, assignments.ErrNotFound
}

func (m *memoryRepo) UpdateApproval(_ context.Context, approval assignments.Approval) error {
	m.approvals[approval.ID] = approval

	return nil
}

type stubJourneys struct{}

func (stubJourneys) RequirePublished(context.Context, string, string) (journeys.Template, error) {
	return journeys.Template{}, nil
}

func (stubJourneys) ListStepsForVersion(context.Context, string, string, int) ([]journeys.Step, error) {
	return nil, nil
}

type stubEmployees struct {
	byID     map[string]employees.Employee
	byUserID map[string]employees.Employee
}

func (s stubEmployees) Get(_ context.Context, _, employeeID string) (employees.Employee, error) {
	item, ok := s.byID[employeeID]
	if !ok {
		return employees.Employee{}, employees.ErrNotFound
	}

	return item, nil
}

func (s stubEmployees) GetByUserID(_ context.Context, _, userID string) (employees.Employee, error) {
	item, ok := s.byUserID[userID]
	if !ok {
		return employees.Employee{}, employees.ErrNotFound
	}

	return item, nil
}

type stubNotify struct {
	calls []notifications.CreateInput
}

func (n *stubNotify) Create(
	_ context.Context,
	_ string,
	in notifications.CreateInput,
) (notifications.Notification, error) {
	n.calls = append(n.calls, in)

	return notifications.Notification{
		ID:     "n1",
		Title:  in.Title,
		Body:   in.Body,
		UserID: in.UserID,
	}, nil
}

func TestCompleteApprovalStepReopensRejectedApprovalAndNotifies(t *testing.T) {
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
			byUserID: map[string]employees.Employee{
				testEmployeeUser: {ID: testEmployeeID, UserID: testEmployeeUser},
			},
		},
		notify,
	)

	now := time.Now().UTC()
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
		StepType:            "approval",
		Title:               "Laptop checklist",
		Status:              testStatusInProgress,
		CreatedAt:           now,
	}

	decided := now.Add(-time.Hour)
	repo.approvals[testApprovalID] = assignments.Approval{
		ID:               testApprovalID,
		OrganizationID:   testOrgID,
		StepAssignmentID: testStepID,
		ApproverUserID:   testManagerUser,
		Status:           "rejected",
		Note:             "Fix serial",
		DecidedAt:        &decided,
	}

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

	if step.Status != "awaiting_approval" {
		t.Fatalf("step status = %s, want awaiting_approval", step.Status)
	}

	approval := repo.approvals[testApprovalID]
	if approval.Status != "pending" || approval.DecidedAt != nil || approval.Note != "" {
		t.Fatalf("approval not reopened: %+v", approval)
	}

	if len(notify.calls) != 1 || notify.calls[0].UserID != testManagerUser {
		t.Fatalf("expected manager notification, got %+v", notify.calls)
	}
}

func TestDecideApprovalNotifiesEmployee(t *testing.T) {
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

	now := time.Now().UTC()
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
		StepType:            "approval",
		Title:               "Laptop checklist",
		Status:              "awaiting_approval",
		CreatedAt:           now,
	}
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
		assignments.DecideApprovalInput{Approve: true, Note: "Looks good"},
	)
	if err != nil {
		t.Fatalf("DecideApproval: %v", err)
	}

	if len(notify.calls) != 1 || notify.calls[0].UserID != testEmployeeUser {
		t.Fatalf("expected employee notification, got %+v", notify.calls)
	}

	if notify.calls[0].Title != "Step approved" {
		t.Fatalf("title = %q, want Step approved", notify.calls[0].Title)
	}
}
