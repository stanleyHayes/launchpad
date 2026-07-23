package assignments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/employees"
	"launchpad/internal/journeys"
	"launchpad/internal/notifications"
)

const (
	stepTypeApproval = "approval"
	stepTypeQuiz     = "quiz"
	passingScore     = 70.0
	percentScale     = 100.0
)

// JourneyReader loads published journeys and steps.
type JourneyReader interface {
	RequirePublished(ctx context.Context, organizationID, templateID string) (journeys.Template, error)
	ListStepsForVersion(ctx context.Context, organizationID, templateID string, version int) ([]journeys.Step, error)
}

// EmployeeReader loads employees.
type EmployeeReader interface {
	Get(ctx context.Context, organizationID, employeeID string) (employees.Employee, error)
	GetByUserID(ctx context.Context, organizationID, userID string) (employees.Employee, error)
}

// Notifier creates notifications.
type Notifier interface {
	Create(ctx context.Context, organizationID string, in notifications.CreateInput) (notifications.Notification, error)
}

// Service implements assignment use cases.
type Service struct {
	repo      Repository
	journeys  JourneyReader
	employees EmployeeReader
	notify    Notifier
}

// NewService constructs a Service.
func NewService(
	repo Repository,
	journeyReader JourneyReader,
	employeeReader EmployeeReader,
	notifier Notifier,
) *Service {
	return &Service{
		repo:      repo,
		journeys:  journeyReader,
		employees: employeeReader,
		notify:    notifier,
	}
}

// AssignResult is returned after assigning a journey.
type AssignResult struct {
	Assignment JourneyAssignment `json:"assignment"`
	Steps      []StepAssignment  `json:"steps"`
}

// Assign assigns a published journey to an employee.
func (s *Service) Assign(
	ctx context.Context,
	organizationID, actorUserID string,
	in AssignInput,
) (AssignResult, error) {
	if organizationID == "" || in.EmployeeID == "" || in.JourneyTemplateID == "" {
		return AssignResult{}, ErrInvalidInput
	}

	startsAt := in.StartsAt.UTC()
	if startsAt.IsZero() {
		startsAt = time.Now().UTC()
	}

	employee, err := s.employees.Get(ctx, organizationID, in.EmployeeID)
	if err != nil {
		return AssignResult{}, fmt.Errorf("load employee: %w", err)
	}

	template, err := s.journeys.RequirePublished(ctx, organizationID, in.JourneyTemplateID)
	if err != nil {
		return AssignResult{}, fmt.Errorf("load journey: %w", err)
	}

	if err := s.ensureNoActiveAssignment(ctx, organizationID, in.EmployeeID, template.ID); err != nil {
		return AssignResult{}, err
	}

	steps, err := s.journeys.ListStepsForVersion(ctx, organizationID, template.ID, template.CurrentVersion)
	if err != nil {
		return AssignResult{}, fmt.Errorf("list journey steps: %w", err)
	}

	if len(steps) == 0 {
		return AssignResult{}, ErrInvalidInput
	}

	now := time.Now().UTC()
	assignment := newJourneyAssignment(organizationID, in.EmployeeID, template, startsAt, now)
	stepAssignments, approvals := buildStepAssignments(
		organizationID,
		in.EmployeeID,
		assignment,
		steps,
		startsAt,
		now,
		actorUserID,
	)

	if err := s.persistAssignment(ctx, assignment, stepAssignments, approvals); err != nil {
		return AssignResult{}, err
	}

	if err := s.notifyAssignment(ctx, organizationID, employee, template); err != nil {
		return AssignResult{}, err
	}

	return AssignResult{Assignment: assignment, Steps: stepAssignments}, nil
}

// List lists organization assignments.
func (s *Service) List(ctx context.Context, organizationID string) ([]JourneyAssignment, error) {
	items, err := s.repo.ListAssignments(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}

	return items, nil
}

// ListMine lists assignments for the caller's employee record.
func (s *Service) ListMine(ctx context.Context, organizationID, userID string) ([]JourneyAssignment, error) {
	employee, err := s.employees.GetByUserID(ctx, organizationID, userID)
	if err != nil {
		return nil, fmt.Errorf("resolve employee: %w", err)
	}

	items, err := s.repo.ListAssignmentsForEmployee(ctx, organizationID, employee.ID)
	if err != nil {
		return nil, fmt.Errorf("list my assignments: %w", err)
	}

	return items, nil
}

// Get returns one assignment.
func (s *Service) Get(ctx context.Context, organizationID, assignmentID string) (JourneyAssignment, error) {
	assignment, err := s.repo.GetAssignment(ctx, organizationID, assignmentID)
	if err != nil {
		return JourneyAssignment{}, fmt.Errorf("get assignment: %w", err)
	}

	return assignment, nil
}

// ListSteps lists steps for an assignment.
func (s *Service) ListSteps(ctx context.Context, organizationID, assignmentID string) ([]StepAssignment, error) {
	if _, err := s.repo.GetAssignment(ctx, organizationID, assignmentID); err != nil {
		return nil, fmt.Errorf("get assignment: %w", err)
	}

	items, err := s.repo.ListSteps(ctx, organizationID, assignmentID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}

	return items, nil
}

// CompleteStep submits/completes a step for an employee.
func (s *Service) CompleteStep(
	ctx context.Context,
	organizationID, userID, stepAssignmentID string,
	in CompleteStepInput,
) (StepAssignment, error) {
	step, err := s.repo.GetStep(ctx, organizationID, stepAssignmentID)
	if err != nil {
		return StepAssignment{}, err
	}

	employee, err := s.employees.GetByUserID(ctx, organizationID, userID)
	if err != nil {
		return StepAssignment{}, fmt.Errorf("resolve employee: %w", err)
	}

	if step.EmployeeID != employee.ID {
		return StepAssignment{}, ErrInvalidState
	}

	if step.Status != stepInProgress {
		return StepAssignment{}, ErrInvalidState
	}

	applyCompleteStepInput(&step, in)

	if err := finalizeStepCompletion(&step); err != nil {
		return StepAssignment{}, err
	}

	if err := s.repo.UpdateStep(ctx, step); err != nil {
		return StepAssignment{}, err
	}

	if step.Status == stepCompleted {
		if err := s.recomputeProgress(ctx, organizationID, step.JourneyAssignmentID); err != nil {
			return StepAssignment{}, err
		}
	}

	return step, nil
}

func applyCompleteStepInput(step *StepAssignment, in CompleteStepInput) {
	if in.Submission != nil {
		step.Submission = in.Submission
	}

	if in.Score != nil {
		step.Score = in.Score
	}
}

func finalizeStepCompletion(step *StepAssignment) error {
	switch step.StepType {
	case stepTypeApproval:
		step.Status = stepAwaitingApproval
	case stepTypeQuiz:
		if step.Score == nil {
			score, ok := scoreFromSubmission(step.Submission)
			if ok {
				step.Score = &score
			}
		}

		if step.Score == nil || *step.Score < passingScore {
			return ErrInvalidInput
		}

		markStepCompleted(step)
	default:
		markStepCompleted(step)
	}

	return nil
}

func scoreFromSubmission(submission map[string]any) (float64, bool) {
	if submission == nil {
		return 0, false
	}

	raw, ok := submission["score"]
	if !ok {
		return 0, false
	}

	switch score := raw.(type) {
	case float64:
		return score, true
	case float32:
		return float64(score), true
	case int:
		return float64(score), true
	case int64:
		return float64(score), true
	default:
		return 0, false
	}
}

// ListApprovals lists approvals.
func (s *Service) ListApprovals(ctx context.Context, organizationID string) ([]Approval, error) {
	items, err := s.repo.ListApprovals(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}

	return items, nil
}

// DecideApproval approves or rejects a pending approval.
func (s *Service) DecideApproval(
	ctx context.Context,
	organizationID, approverUserID, approvalID string,
	in DecideApprovalInput,
) (Approval, error) {
	approval, err := s.repo.GetApproval(ctx, organizationID, approvalID)
	if err != nil {
		return Approval{}, err
	}

	if approval.Status != approvalPending {
		return Approval{}, ErrInvalidState
	}

	if approval.ApproverUserID != approverUserID {
		return Approval{}, ErrInvalidState
	}

	step, err := s.repo.GetStep(ctx, organizationID, approval.StepAssignmentID)
	if err != nil {
		return Approval{}, err
	}

	now := time.Now().UTC()
	approval.DecidedAt = &now
	approval.Note = in.Note

	if in.Approve {
		approval.Status = approvalApproved

		markStepCompleted(&step)
	} else {
		approval.Status = approvalRejected
		step.Status = stepInProgress
	}

	if err := s.repo.UpdateApproval(ctx, approval); err != nil {
		return Approval{}, err
	}

	if err := s.repo.UpdateStep(ctx, step); err != nil {
		return Approval{}, err
	}

	if step.Status == stepCompleted {
		if err := s.recomputeProgress(ctx, organizationID, step.JourneyAssignmentID); err != nil {
			return Approval{}, err
		}
	}

	return approval, nil
}

func (s *Service) ensureNoActiveAssignment(
	ctx context.Context,
	organizationID, employeeID, templateID string,
) error {
	if _, err := s.repo.FindActiveAssignment(ctx, organizationID, employeeID, templateID); err == nil {
		return ErrAlreadyAssigned
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}

	return nil
}

func (s *Service) persistAssignment(
	ctx context.Context,
	assignment JourneyAssignment,
	stepAssignments []StepAssignment,
	approvals []Approval,
) error {
	if err := s.repo.CreateAssignment(ctx, assignment); err != nil {
		return err
	}

	if err := s.repo.CreateStepAssignments(ctx, stepAssignments); err != nil {
		return err
	}

	for _, approval := range approvals {
		if err := s.repo.CreateApproval(ctx, approval); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) notifyAssignment(
	ctx context.Context,
	organizationID string,
	employee employees.Employee,
	template journeys.Template,
) error {
	if employee.UserID == "" || s.notify == nil {
		return nil
	}

	if _, notifyErr := s.notify.Create(ctx, organizationID, notifications.CreateInput{
		UserID: employee.UserID,
		Title:  "New onboarding journey",
		Body:   "You have been assigned: " + template.Name,
	}); notifyErr != nil {
		return fmt.Errorf("notify employee: %w", notifyErr)
	}

	return nil
}

func newJourneyAssignment(
	organizationID, employeeID string,
	template journeys.Template,
	startsAt, now time.Time,
) JourneyAssignment {
	status := statusInProgress
	if startsAt.After(now) {
		status = statusScheduled
	}

	return JourneyAssignment{
		ID:                uuid.NewString(),
		OrganizationID:    organizationID,
		EmployeeID:        employeeID,
		JourneyTemplateID: template.ID,
		TemplateVersion:   template.CurrentVersion,
		Status:            status,
		StartsAt:          startsAt,
		DueAt:             nil,
		ProgressPercent:   0,
		CompletedAt:       nil,
		CreatedAt:         now,
	}
}

func buildStepAssignments(
	organizationID, employeeID string,
	assignment JourneyAssignment,
	steps []journeys.Step,
	startsAt, now time.Time,
	actorUserID string,
) ([]StepAssignment, []Approval) {
	stepAssignments := make([]StepAssignment, 0, len(steps))
	approvals := make([]Approval, 0)

	for index, step := range steps {
		status := stepPending
		if index == 0 && assignment.Status == statusInProgress {
			status = stepInProgress
		}

		var dueAt *time.Time

		if step.DueOffsetDays > 0 {
			due := startsAt.AddDate(0, 0, step.DueOffsetDays)
			dueAt = &due
		}

		stepAssignment := StepAssignment{
			ID:                  uuid.NewString(),
			OrganizationID:      organizationID,
			JourneyAssignmentID: assignment.ID,
			JourneyStepID:       step.ID,
			EmployeeID:          employeeID,
			StepType:            step.StepType,
			Title:               step.Title,
			Instructions:        step.Instructions,
			Position:            step.Position,
			Status:              status,
			DueAt:               dueAt,
			Submission:          nil,
			Score:               nil,
			CompletedAt:         nil,
			CreatedAt:           now,
		}
		stepAssignments = append(stepAssignments, stepAssignment)

		if step.StepType == stepTypeApproval {
			approvals = append(approvals, Approval{
				ID:               uuid.NewString(),
				OrganizationID:   organizationID,
				StepAssignmentID: stepAssignment.ID,
				ApproverUserID:   actorUserID,
				Status:           approvalPending,
				Note:             "",
				DecidedAt:        nil,
				CreatedAt:        now,
			})
		}
	}

	return stepAssignments, approvals
}

func markStepCompleted(step *StepAssignment) {
	now := time.Now().UTC()
	step.Status = stepCompleted
	step.CompletedAt = &now
}

func (s *Service) recomputeProgress(ctx context.Context, organizationID, journeyAssignmentID string) error {
	assignment, err := s.repo.GetAssignment(ctx, organizationID, journeyAssignmentID)
	if err != nil {
		return err
	}

	steps, err := s.repo.ListSteps(ctx, organizationID, journeyAssignmentID)
	if err != nil {
		return err
	}

	if len(steps) == 0 {
		return nil
	}

	completed := 0

	for _, step := range steps {
		if step.Status == stepCompleted {
			completed++
		}
	}

	assignment.ProgressPercent = float64(completed) / float64(len(steps)) * percentScale
	if completed == len(steps) {
		now := time.Now().UTC()
		assignment.Status = statusCompleted
		assignment.CompletedAt = &now
	} else if assignment.Status == statusInProgress {
		for _, step := range steps {
			if step.Status != stepPending {
				continue
			}

			step.Status = stepInProgress
			if err := s.repo.UpdateStep(ctx, step); err != nil {
				return fmt.Errorf("start next assignment step: %w", err)
			}

			break
		}
	}

	return s.repo.UpdateAssignment(ctx, assignment)
}
