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

func (m *memoryRepo) ListAssignments(
	_ context.Context,
	organizationID string,
) ([]assignments.JourneyAssignment, error) {
	items := make([]assignments.JourneyAssignment, 0)

	for _, assignment := range m.assignments {
		if assignment.OrganizationID == organizationID {
			items = append(items, assignment)
		}
	}

	return items, nil
}

func (m *memoryRepo) ListAssignmentsForEmployee(
	_ context.Context,
	organizationID,
	employeeID string,
) ([]assignments.JourneyAssignment, error) {
	items := make([]assignments.JourneyAssignment, 0)

	for _, assignment := range m.assignments {
		if assignment.OrganizationID == organizationID && assignment.EmployeeID == employeeID {
			items = append(items, assignment)
		}
	}

	return items, nil
}

func (m *memoryRepo) UpdateAssignment(_ context.Context, assignment assignments.JourneyAssignment) error {
	m.assignments[assignment.ID] = assignment

	return nil
}

func (m *memoryRepo) ListSteps(
	_ context.Context,
	organizationID,
	journeyAssignmentID string,
) ([]assignments.StepAssignment, error) {
	items := make([]assignments.StepAssignment, 0)

	for _, step := range m.steps {
		if step.OrganizationID == organizationID && step.JourneyAssignmentID == journeyAssignmentID {
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

func (m *memoryRepo) ListDueSoonSteps(
	_ context.Context,
	from, to time.Time,
) ([]assignments.StepAssignment, error) {
	items := make([]assignments.StepAssignment, 0)

	for _, step := range m.steps {
		if step.Status == "completed" || step.DueAt == nil || step.DueSoonNotifiedAt != nil {
			continue
		}

		if step.DueAt.After(from) && !step.DueAt.After(to) {
			items = append(items, step)
		}
	}

	return items, nil
}

func (m *memoryRepo) ListOverdueSteps(_ context.Context, now time.Time) ([]assignments.StepAssignment, error) {
	items := make([]assignments.StepAssignment, 0)

	for _, step := range m.steps {
		if step.Status == "completed" || step.DueAt == nil || step.OverdueNotifiedAt != nil {
			continue
		}

		if step.DueAt.Before(now) {
			items = append(items, step)
		}
	}

	return items, nil
}

func (m *memoryRepo) GetApproval(_ context.Context, _, approvalID string) (assignments.Approval, error) {
	item, ok := m.approvals[approvalID]
	if !ok {
		return assignments.Approval{}, assignments.ErrNotFound
	}

	return item, nil
}

func (m *memoryRepo) ListApprovals(_ context.Context, organizationID string) ([]assignments.Approval, error) {
	items := make([]assignments.Approval, 0, len(m.approvals))
	for _, approval := range m.approvals {
		if approval.OrganizationID == organizationID {
			items = append(items, approval)
		}
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

type stubJourneys struct {
	template journeys.Template
	steps    []journeys.Step
}

func (s stubJourneys) RequirePublished(context.Context, string, string) (journeys.Template, error) {
	return s.template, nil
}

func (s stubJourneys) ListStepsForVersion(context.Context, string, string, int) ([]journeys.Step, error) {
	return s.steps, nil
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

func (s stubEmployees) List(_ context.Context, _ string, offset, limit int64) ([]employees.Employee, error) {
	all := make([]employees.Employee, 0, len(s.byID))
	for _, employee := range s.byID {
		all = append(all, employee)
	}

	if offset >= int64(len(all)) {
		return nil, nil
	}

	end := min(offset+limit, int64(len(all)))

	return all[offset:end], nil
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

	// Approving the final step completes the journey, so the employee gets
	// both the journey-completed and the step-approved notifications.
	var approved *notifications.CreateInput

	for i := range notify.calls {
		if notify.calls[i].Title == "Step approved" {
			approved = &notify.calls[i]
		}
	}

	if approved == nil || approved.UserID != testEmployeeUser {
		t.Fatalf("expected employee step-approved notification, got %+v", notify.calls)
	}
}

func (m *memoryRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func TestOverrideStepRequiresReasonAndCompletesAssignment(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := assignments.NewService(repo, stubJourneys{}, stubEmployees{}, &stubNotify{})
	repo.assignments[testAssignmentID] = assignments.JourneyAssignment{
		ID: testAssignmentID, OrganizationID: testOrgID, EmployeeID: testEmployeeID, Status: testStatusInProgress,
	}
	repo.steps[testStepID] = assignments.StepAssignment{
		ID: testStepID, OrganizationID: testOrgID, JourneyAssignmentID: testAssignmentID,
		EmployeeID: testEmployeeID, Status: "blocked",
	}

	if _, err := svc.OverrideStep(context.Background(), testOrgID, testManagerUser, testStepID, assignments.OverrideStepInput{
		Action: "complete",
	}); err == nil {
		t.Fatal("expected an override without a reason to fail")
	}

	step, err := svc.OverrideStep(context.Background(), testOrgID, testManagerUser, testStepID, assignments.OverrideStepInput{
		Action: "complete", Reason: "Verified outside the system",
	})
	if err != nil {
		t.Fatalf("OverrideStep: %v", err)
	}
	if step.Status != "completed" || step.OverrideByUserID != testManagerUser || step.OverriddenAt == nil {
		t.Fatalf("override metadata/status = %+v", step)
	}
	if repo.assignments[testAssignmentID].Status != "completed" {
		t.Fatalf("assignment status = %s, want completed", repo.assignments[testAssignmentID].Status)
	}
}

func TestListStepsForActorAppliesLocaleTranslation(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := assignments.NewService(repo, stubJourneys{}, stubEmployees{}, &stubNotify{})
	repo.assignments[testAssignmentID] = assignments.JourneyAssignment{
		ID: testAssignmentID, OrganizationID: testOrgID, EmployeeID: testEmployeeID,
	}
	repo.steps[testStepID] = assignments.StepAssignment{
		ID: testStepID, OrganizationID: testOrgID, JourneyAssignmentID: testAssignmentID,
		Title: "Welcome", Instructions: "Read this", Config: map[string]any{
			"translations": map[string]any{
				"fr": map[string]any{"title": "Bienvenue", "instructions": "Lisez ceci"},
			},
		},
	}

	steps, err := svc.ListStepsForActor(
		context.Background(), testOrgID, testManagerUser, true, testAssignmentID, "fr-CA",
	)
	if err != nil {
		t.Fatalf("ListStepsForActor: %v", err)
	}
	if len(steps) != 1 || steps[0].Title != "Bienvenue" || steps[0].Instructions != "Lisez ceci" {
		t.Fatalf("translated steps = %+v", steps)
	}
}
