package assignments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/employees"
	"launchpad/internal/journeys"
	"launchpad/internal/notifications"
	"launchpad/internal/support"
)

const (
	stepTypeApproval = "approval"
	stepTypeQuiz     = "quiz"
	// equipment_request/access_request steps auto-create an equipment/access
	// request for the employee when they complete (PRD §5.3.8).
	stepTypeEquipmentRequest = "equipment_request"
	stepTypeAccessRequest    = "access_request"
	// assessment steps complete when the employee's latest attempt on the
	// linked assessment is a graded pass (PRD §5.3.6).
	stepTypeAssessment = "assessment"
	// meeting steps complete when the employee schedules the meeting through
	// the step's schedule form (PRD §5.3.7).
	stepTypeMeeting = "meeting"
	percentScale    = 100.0
	// quizPassingScore is the minimum score (percent correct) to pass a quiz.
	quizPassingScore = 70.0
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
	List(ctx context.Context, organizationID string, offset, limit int64) ([]employees.Employee, error)
}

// Notifier creates notifications.
type Notifier interface {
	Create(ctx context.Context, organizationID string, in notifications.CreateInput) (notifications.Notification, error)
}

// BlockerStore records employee blockers and their backing support tickets.
// Implemented by internal/support's service.
type BlockerStore interface {
	ReportBlocker(ctx context.Context, in support.ReportBlockerInput) (support.Blocker, error)
	ListBlockers(ctx context.Context, organizationID string) ([]support.Blocker, error)
}

// RequestCreator auto-creates an equipment/access request when a request step
// completes. Implemented by internal/requests's service; kind is "equipment"
// or "access", item is one of the PRD §5.3.8 item codes (empty allowed), and
// details describes the request.
type RequestCreator interface {
	CreateFromStep(ctx context.Context, organizationID, employeeID, kind, item, details string) error
}

// AssessmentVerifier checks whether an employee's latest attempt on an
// assessment passed. Implemented by internal/assessments's service.
type AssessmentVerifier interface {
	LatestAttemptPassed(ctx context.Context, organizationID, assessmentID, employeeID string) (bool, error)
}

// MeetingScheduler schedules the meeting backing a meeting step when the
// employee submits the step's schedule form. Implemented by
// internal/meetings's service; meetingType is one of the PRD §5.3.7 type
// codes (empty defaults to manager_intro), startsAt must be an RFC3339
// timestamp, and location is free text (a URL or a room).
type MeetingScheduler interface {
	CreateFromStep(
		ctx context.Context,
		organizationID, employeeID, meetingType, title, startsAt string,
		durationMin int,
		location string,
	) error
}

// Service implements assignment use cases.
type Service struct {
	repo        Repository
	journeys    JourneyReader
	employees   EmployeeReader
	notify      Notifier
	blockers    BlockerStore
	requests    RequestCreator
	assessments AssessmentVerifier
	meetings    MeetingScheduler
}

// NewService constructs a Service. The blocker store is optional so existing
// wiring keeps working; blocker endpoints report ErrInvalidState without it.
func NewService(
	repo Repository,
	journeyReader JourneyReader,
	employeeReader EmployeeReader,
	notifier Notifier,
	blockerStores ...BlockerStore,
) *Service {
	var blockers BlockerStore
	if len(blockerStores) > 0 {
		blockers = blockerStores[0]
	}

	return &Service{
		repo:      repo,
		journeys:  journeyReader,
		employees: employeeReader,
		notify:    notifier,
		blockers:  blockers,
		requests:  nil,
	}
}

// SetRequestCreator wires the optional request creator port. Nil (the
// default) disables auto-creation: request steps then complete without
// raising a request.
func (s *Service) SetRequestCreator(requestCreator RequestCreator) {
	s.requests = requestCreator
}

// SetAssessmentVerifier wires the optional assessment verifier port. Nil
// (the default) leaves assessment steps uncompletable: completing one then
// reports ErrInvalidState.
func (s *Service) SetAssessmentVerifier(verifier AssessmentVerifier) {
	s.assessments = verifier
}

// SetMeetingScheduler wires the optional meeting scheduler port. Nil (the
// default) leaves meeting steps uncompletable: completing one then reports
// ErrInvalidState.
func (s *Service) SetMeetingScheduler(scheduler MeetingScheduler) {
	s.meetings = scheduler
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

	employee, err := s.employees.Get(ctx, organizationID, in.EmployeeID)
	if err != nil {
		return AssignResult{}, fmt.Errorf("load employee: %w", err)
	}

	template, err := s.journeys.RequirePublished(ctx, organizationID, in.JourneyTemplateID)
	if err != nil {
		return AssignResult{}, fmt.Errorf("load journey: %w", err)
	}

	return s.assignToEmployee(ctx, organizationID, actorUserID, employee, template, in.StartsAt)
}

// AssignToDepartment assigns a published journey to every employee in a
// department. Employees who already have an active assignment for the
// template are skipped, so the operation is safe to re-run.
func (s *Service) AssignToDepartment(
	ctx context.Context,
	organizationID, actorUserID string,
	in AssignDepartmentInput,
) (AssignDepartmentResult, error) {
	if organizationID == "" || in.DepartmentID == "" || in.JourneyTemplateID == "" {
		return AssignDepartmentResult{}, ErrInvalidInput
	}

	template, err := s.journeys.RequirePublished(ctx, organizationID, in.JourneyTemplateID)
	if err != nil {
		return AssignDepartmentResult{}, fmt.Errorf("load journey: %w", err)
	}

	total := AssignDepartmentResult{Employees: 0, Assigned: 0, Skipped: 0}

	const pageSize = 100

	for offset := int64(0); ; offset += pageSize {
		page, done, err := s.assignToDepartmentPage(ctx, organizationID, actorUserID, in, template, offset, pageSize)
		if err != nil {
			return AssignDepartmentResult{}, err
		}

		total.Employees += page.Employees
		total.Assigned += page.Assigned
		total.Skipped += page.Skipped

		if done {
			break
		}
	}

	return total, nil
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

// GetForActor returns one assignment when the actor manages the organization
// or owns the assignment.
func (s *Service) GetForActor(
	ctx context.Context,
	organizationID, actorUserID string,
	isManager bool,
	assignmentID string,
) (JourneyAssignment, error) {
	assignment, err := s.repo.GetAssignment(ctx, organizationID, assignmentID)
	if err != nil {
		return JourneyAssignment{}, fmt.Errorf("get assignment: %w", err)
	}

	if err := s.authorizeAssignmentAccess(ctx, organizationID, actorUserID, isManager, assignment); err != nil {
		return JourneyAssignment{}, err
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

// ListStepsForActor lists steps for an assignment when the actor manages the
// organization or owns the assignment.
func (s *Service) ListStepsForActor(
	ctx context.Context,
	organizationID, actorUserID string,
	isManager bool,
	assignmentID string,
	locale ...string,
) ([]StepAssignment, error) {
	assignment, err := s.repo.GetAssignment(ctx, organizationID, assignmentID)
	if err != nil {
		return nil, fmt.Errorf("get assignment: %w", err)
	}

	if err := s.authorizeAssignmentAccess(ctx, organizationID, actorUserID, isManager, assignment); err != nil {
		return nil, err
	}

	items, err := s.repo.ListSteps(ctx, organizationID, assignmentID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}

	if len(locale) > 0 {
		applyStepTranslations(items, locale[0])
	}

	return items, nil
}

func applyStepTranslations(items []StepAssignment, locale string) {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return
	}
	for index := range items {
		rawTranslations, ok := items[index].Config["translations"]
		if !ok {
			continue
		}
		encoded, err := json.Marshal(rawTranslations)
		if err != nil {
			continue
		}
		var translations map[string]struct {
			Title        string `json:"title"`
			Instructions string `json:"instructions"`
		}
		if json.Unmarshal(encoded, &translations) != nil {
			continue
		}
		translation, ok := translations[locale]
		if !ok {
			if separator := strings.IndexByte(locale, '-'); separator > 0 {
				translation, ok = translations[locale[:separator]]
			}
		}
		if !ok {
			continue
		}
		if translation.Title != "" {
			items[index].Title = translation.Title
		}
		if translation.Instructions != "" {
			items[index].Instructions = translation.Instructions
		}
		items[index].Locale = locale
	}
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

	if err := s.applyCompletion(ctx, organizationID, &step, in); err != nil {
		return StepAssignment{}, err
	}
	step.AttemptCount++
	if step.Status == stepInProgress && step.MaxAttempts > 0 && step.AttemptCount >= step.MaxAttempts {
		now := time.Now().UTC()
		step.Status = stepBlocked
		step.EscalatedAt = &now
	}

	// A request step completes when its request exists, so create the request
	// before persisting the completion (a no-op for other step types).
	if err := s.createRequestForStep(ctx, organizationID, step); err != nil {
		return StepAssignment{}, err
	}

	if err := s.repo.UpdateStep(ctx, step); err != nil {
		return StepAssignment{}, err
	}

	if step.Status == stepAwaitingApproval {
		if err := s.reopenApprovalForStep(ctx, organizationID, step); err != nil {
			return StepAssignment{}, err
		}
	}

	if step.Status == stepCompleted {
		if err := s.recomputeProgress(ctx, organizationID, step.JourneyAssignmentID); err != nil {
			return StepAssignment{}, err
		}
	}

	return step, nil
}

// OverrideStep lets an authorized manager resolve or reopen an exceptional
// workflow step. The reason and actor are retained on the assignment step so
// the intervention is visible independently of the audit log.
func (s *Service) OverrideStep(
	ctx context.Context,
	organizationID, actorUserID, stepAssignmentID string,
	in OverrideStepInput,
) (StepAssignment, error) {
	in.Action = strings.ToLower(strings.TrimSpace(in.Action))
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Reason == "" {
		return StepAssignment{}, fmt.Errorf("%w: override reason is required", ErrInvalidInput)
	}
	if in.Action != "complete" && in.Action != "skip" && in.Action != "reopen" {
		return StepAssignment{}, fmt.Errorf("%w: override action must be complete, skip, or reopen", ErrInvalidInput)
	}

	step, err := s.repo.GetStep(ctx, organizationID, stepAssignmentID)
	if err != nil {
		return StepAssignment{}, err
	}

	now := time.Now().UTC()
	step.OverrideAction = in.Action
	step.OverrideReason = in.Reason
	step.OverrideByUserID = actorUserID
	step.OverriddenAt = &now

	switch in.Action {
	case "complete":
		markStepCompleted(&step)
	case "skip":
		step.Status = stepSkipped
		step.CompletedAt = &now
	case "reopen":
		step.Status = stepInProgress
		step.CompletedAt = nil
		step.EscalatedAt = nil
	}

	if err := s.repo.UpdateStep(ctx, step); err != nil {
		return StepAssignment{}, err
	}
	if in.Action == "complete" || in.Action == "skip" {
		if err := s.recomputeProgress(ctx, organizationID, step.JourneyAssignmentID); err != nil {
			return StepAssignment{}, err
		}
	}

	return step, nil
}

// StartStep marks a step started by its owner: a pending step moves to
// in_progress and StartedAt is recorded once. Starting an already-started
// step is a no-op; completed or approval-pending steps cannot be started.
func (s *Service) StartStep(
	ctx context.Context,
	organizationID, userID, stepAssignmentID string,
) (StepAssignment, error) {
	step, err := s.resolveOwnedStep(ctx, organizationID, userID, stepAssignmentID)
	if err != nil {
		return StepAssignment{}, err
	}

	if step.Status == stepAwaitingApproval || step.Status == stepCompleted {
		return StepAssignment{}, ErrInvalidState
	}

	if step.StartedAt == nil {
		now := time.Now().UTC()
		step.StartedAt = &now
	}

	if step.Status == stepPending {
		step.Status = stepInProgress
	}

	if err := s.repo.UpdateStep(ctx, step); err != nil {
		return StepAssignment{}, err
	}

	return step, nil
}

// SubmitStep stores partial progress on a step without completing it
// (document/task steps save drafts here; quiz and assessment steps are only
// ever submitted through CompleteStep so grading stays server-side).
// Submitting implies the step is started.
func (s *Service) SubmitStep(
	ctx context.Context,
	organizationID, userID, stepAssignmentID string,
	in SubmitStepInput,
) (StepAssignment, error) {
	if in.Submission == nil {
		return StepAssignment{}, fmt.Errorf("%w: submission is required", ErrInvalidInput)
	}

	step, err := s.resolveOwnedStep(ctx, organizationID, userID, stepAssignmentID)
	if err != nil {
		return StepAssignment{}, err
	}

	if step.StepType == stepTypeQuiz || step.StepType == stepTypeAssessment || step.Status != stepInProgress {
		return StepAssignment{}, ErrInvalidState
	}

	step.Submission = in.Submission

	if step.StartedAt == nil {
		now := time.Now().UTC()
		step.StartedAt = &now
	}

	if err := s.repo.UpdateStep(ctx, step); err != nil {
		return StepAssignment{}, err
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

func finalizeStepCompletion(step *StepAssignment) {
	if step.StepType == stepTypeApproval {
		step.Status = stepAwaitingApproval

		return
	}

	markStepCompleted(step)
}

// quizAnswers extracts {questionId: optionIndex} answers from a quiz
// submission. Answers for unknown question ids are ignored during grading.
func quizAnswers(submission map[string]any) (map[string]int, error) {
	raw, ok := submission["answers"]
	if !ok {
		return nil, fmt.Errorf("%w: quiz submission requires answers", ErrInvalidInput)
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: quiz submission requires answers", ErrInvalidInput)
	}

	var answers map[string]int
	if err := json.Unmarshal(encoded, &answers); err != nil || len(answers) == 0 {
		return nil, fmt.Errorf("%w: quiz submission requires answers", ErrInvalidInput)
	}

	return answers, nil
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

	if err := s.notifyApprovalDecision(ctx, organizationID, step, in.Approve); err != nil {
		return Approval{}, err
	}

	return approval, nil
}

// createRequestForStep raises the equipment/access request backing a request
// step. It is a no-op for non-request step types, for steps that did not
// complete, and when no request creator is wired. The request carries the
// step title (and instructions) as its details; the employee may pick a
// specific item via the submission's "item" field.
func (s *Service) createRequestForStep(ctx context.Context, organizationID string, step StepAssignment) error {
	if s.requests == nil || step.Status != stepCompleted {
		return nil
	}

	var kind string

	switch step.StepType {
	case stepTypeEquipmentRequest:
		kind = "equipment"
	case stepTypeAccessRequest:
		kind = "access"
	default:
		return nil
	}

	item, _ := step.Submission["item"].(string)

	details := step.Title
	if step.Instructions != "" {
		details += "\n\n" + step.Instructions
	}

	if err := s.requests.CreateFromStep(ctx, organizationID, step.EmployeeID, kind, item, details); err != nil {
		return fmt.Errorf("create %s request for step: %w", kind, err)
	}

	return nil
}

// resolveOwnedStep loads a step assignment and checks the caller owns it.
func (s *Service) resolveOwnedStep(
	ctx context.Context,
	organizationID, userID, stepAssignmentID string,
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

	return step, nil
}

// applyCompletion applies the submission and moves the step into its next
// state: server-graded completion for quizzes, awaiting approval for
// approval steps, and direct completion otherwise.
func (s *Service) applyCompletion(
	ctx context.Context,
	organizationID string,
	step *StepAssignment,
	in CompleteStepInput,
) error {
	if step.StepType == stepTypeQuiz {
		return s.gradeQuizStep(ctx, organizationID, step, in)
	}

	if step.StepType == stepTypeAssessment {
		return s.completeAssessmentStep(ctx, organizationID, step, in)
	}

	if step.StepType == stepTypeMeeting {
		return s.completeMeetingStep(ctx, organizationID, step, in)
	}

	applyCompleteStepInput(step, in)
	finalizeStepCompletion(step)

	return nil
}

// gradeQuizStep scores a quiz submission server-side against the answer key
// stored on the journey step definition. Client-supplied scores are never
// trusted (H-1): the score is always recomputed from the submitted answers.
// A passing score completes the step; otherwise the step stays in progress
// with the score recorded so the employee can retry.
func (s *Service) gradeQuizStep(
	ctx context.Context,
	organizationID string,
	step *StepAssignment,
	in CompleteStepInput,
) error {
	answers, err := quizAnswers(in.Submission)
	if err != nil {
		return err
	}

	quiz, err := s.loadQuizConfig(ctx, organizationID, step)
	if err != nil {
		return err
	}

	correct := 0

	for _, question := range quiz.Questions {
		if answer, ok := answers[question.ID]; ok && answer == question.CorrectOption {
			correct++
		}
	}

	score := float64(correct) / float64(len(quiz.Questions)) * percentScale
	step.Submission = in.Submission
	step.Score = &score

	if score >= quizPassingScore {
		markStepCompleted(step)
	}

	return nil
}

// completeAssessmentStep completes an assessment step when the employee's
// latest attempt on the linked assessment is a graded pass (PRD §5.3.6).
// Grading itself happens in the assessments module when the attempt is
// submitted; this step only verifies the outcome, so without a passing
// attempt the step stays in progress.
func (s *Service) completeAssessmentStep(
	ctx context.Context,
	organizationID string,
	step *StepAssignment,
	in CompleteStepInput,
) error {
	if s.assessments == nil {
		return fmt.Errorf("%w: assessments are not available", ErrInvalidState)
	}

	assessmentID := step.AssessmentID
	if assessmentID == "" {
		var err error

		assessmentID, err = s.loadStepAssessmentID(ctx, organizationID, step)
		if err != nil {
			return err
		}
	}

	passed, err := s.assessments.LatestAttemptPassed(ctx, organizationID, assessmentID, step.EmployeeID)
	if err != nil {
		return fmt.Errorf("verify assessment attempt: %w", err)
	}

	if !passed {
		return fmt.Errorf("%w: assessment step requires a passing attempt", ErrInvalidState)
	}

	applyCompleteStepInput(step, in)
	markStepCompleted(step)

	return nil
}

// completeMeetingStep schedules the meeting backing a meeting step from the
// schedule form's submission and completes the step (PRD §5.3.7). The step
// completes only once the meeting exists and is scheduled: a missing or
// invalid start time (or a scheduling failure) leaves the step in progress.
func (s *Service) completeMeetingStep(
	ctx context.Context,
	organizationID string,
	step *StepAssignment,
	in CompleteStepInput,
) error {
	if s.meetings == nil {
		return fmt.Errorf("%w: meetings are not available", ErrInvalidState)
	}

	meetingType, _ := in.Submission["meetingType"].(string)
	startsAt, _ := in.Submission["startsAt"].(string)
	location, _ := in.Submission["location"].(string)

	durationMin := 0

	switch raw := in.Submission["durationMin"].(type) {
	case float64:
		durationMin = int(raw)
	case int:
		durationMin = raw
	}

	if err := s.meetings.CreateFromStep(
		ctx,
		organizationID,
		step.EmployeeID,
		meetingType,
		step.Title,
		startsAt,
		durationMin,
		location,
	); err != nil {
		return fmt.Errorf("schedule meeting for step: %w", err)
	}

	applyCompleteStepInput(step, in)
	markStepCompleted(step)

	return nil
}

// loadStepAssessmentID loads the linked assessment id from the journey step
// frozen at the assignment's template version. Only a fallback for steps
// assigned before the id was snapshotted onto the step assignment.
func (s *Service) loadStepAssessmentID(
	ctx context.Context,
	organizationID string,
	step *StepAssignment,
) (string, error) {
	assignment, err := s.repo.GetAssignment(ctx, organizationID, step.JourneyAssignmentID)
	if err != nil {
		return "", err
	}

	steps, err := s.journeys.ListStepsForVersion(
		ctx,
		organizationID,
		assignment.JourneyTemplateID,
		assignment.TemplateVersion,
	)
	if err != nil {
		return "", fmt.Errorf("list journey steps: %w", err)
	}

	for _, journeyStep := range steps {
		if journeyStep.ID != step.JourneyStepID {
			continue
		}

		if assessmentID := journeys.AssessmentIDFromConfig(journeyStep.Config); assessmentID != "" {
			return assessmentID, nil
		}

		return "", fmt.Errorf("%w: assessment step has no assessmentId", ErrInvalidInput)
	}

	return "", fmt.Errorf("%w: assessment step definition not found", ErrInvalidInput)
}

// loadQuizConfig loads the quiz definition for a step assignment from the
// journey step frozen at the assignment's template version.
func (s *Service) loadQuizConfig(
	ctx context.Context,
	organizationID string,
	step *StepAssignment,
) (journeys.QuizConfig, error) {
	assignment, err := s.repo.GetAssignment(ctx, organizationID, step.JourneyAssignmentID)
	if err != nil {
		return journeys.QuizConfig{}, err
	}

	steps, err := s.journeys.ListStepsForVersion(
		ctx,
		organizationID,
		assignment.JourneyTemplateID,
		assignment.TemplateVersion,
	)
	if err != nil {
		return journeys.QuizConfig{}, fmt.Errorf("list journey steps: %w", err)
	}

	for _, journeyStep := range steps {
		if journeyStep.ID != step.JourneyStepID {
			continue
		}

		quiz, err := journeys.ParseQuizConfig(journeyStep.Config)
		if err != nil {
			return journeys.QuizConfig{}, fmt.Errorf("parse quiz config: %w", err)
		}

		if len(quiz.Questions) == 0 {
			return journeys.QuizConfig{}, fmt.Errorf("%w: quiz step has no questions", ErrInvalidInput)
		}

		return quiz, nil
	}

	return journeys.QuizConfig{}, fmt.Errorf("%w: quiz step definition not found", ErrInvalidInput)
}

// assignToDepartmentPage assigns the template to one page of employees in the
// department, returning per-page counts and whether this was the last page.
func (s *Service) assignToDepartmentPage(
	ctx context.Context,
	organizationID, actorUserID string,
	in AssignDepartmentInput,
	template journeys.Template,
	offset, limit int64,
) (AssignDepartmentResult, bool, error) {
	page, err := s.employees.List(ctx, organizationID, offset, limit)
	if err != nil {
		return AssignDepartmentResult{}, false, fmt.Errorf("list employees: %w", err)
	}

	result := AssignDepartmentResult{Employees: 0, Assigned: 0, Skipped: 0}

	for _, employee := range page {
		if employee.DepartmentID != in.DepartmentID || employee.Status != statusActive {
			continue
		}

		result.Employees++

		if _, err := s.assignToEmployee(ctx, organizationID, actorUserID, employee, template, in.StartsAt); err != nil {
			if errors.Is(err, ErrAlreadyAssigned) {
				result.Skipped++

				continue
			}

			return AssignDepartmentResult{}, false, err
		}

		result.Assigned++
	}

	return result, len(page) < int(limit), nil
}

// assignToEmployee performs the shared assign core for a resolved employee
// and published template.
func (s *Service) assignToEmployee(
	ctx context.Context,
	organizationID, actorUserID string,
	employee employees.Employee,
	template journeys.Template,
	startsAt time.Time,
) (AssignResult, error) {
	startsAt = startsAt.UTC()
	if startsAt.IsZero() {
		startsAt = time.Now().UTC()
	}

	if err := s.ensureNoActiveAssignment(ctx, organizationID, employee.ID, template.ID); err != nil {
		return AssignResult{}, err
	}

	steps, err := s.journeys.ListStepsForVersion(ctx, organizationID, template.ID, template.CurrentVersion)
	if err != nil {
		return AssignResult{}, fmt.Errorf("list journey steps: %w", err)
	}

	if len(steps) == 0 {
		return AssignResult{}, ErrInvalidInput
	}
	steps, err = s.expandSubflows(ctx, organizationID, steps, 0)
	if err != nil {
		return AssignResult{}, err
	}

	now := time.Now().UTC()
	assignment := newJourneyAssignment(organizationID, employee.ID, template, startsAt, now)
	stepAssignments, approvals := buildStepAssignments(
		organizationID,
		employee.ID,
		assignment,
		steps,
		startsAt,
		now,
		s.resolveApprover(ctx, organizationID, employee, actorUserID),
	)

	if err := s.persistAssignment(ctx, assignment, stepAssignments, approvals); err != nil {
		return AssignResult{}, err
	}

	// The assignment is already persisted; a notification failure must not
	// turn the committed change into a client-visible error (a retry would
	// hit ErrAlreadyAssigned). Log and return success.
	if err := s.notifyAssignment(ctx, organizationID, employee, template, assignment.ID); err != nil {
		slog.ErrorContext(ctx, "assignment notification failed", "assignmentId", assignment.ID, "error", err)
	}

	return AssignResult{Assignment: assignment, Steps: stepAssignments}, nil
}

func (s *Service) expandSubflows(
	ctx context.Context, organizationID string, steps []journeys.Step, depth int,
) ([]journeys.Step, error) {
	if depth > 3 {
		return nil, fmt.Errorf("%w: subflow nesting exceeds three levels", ErrInvalidInput)
	}
	expanded := make([]journeys.Step, 0, len(steps))
	for _, container := range steps {
		rawID, _ := container.Config["subflowTemplateId"].(string)
		templateID := strings.TrimSpace(rawID)
		if templateID == "" {
			expanded = append(expanded, container)
			continue
		}
		template, err := s.journeys.RequirePublished(ctx, organizationID, templateID)
		if err != nil {
			return nil, fmt.Errorf("load reusable subflow %q: %w", templateID, err)
		}
		substeps, err := s.journeys.ListStepsForVersion(ctx, organizationID, template.ID, template.CurrentVersion)
		if err != nil {
			return nil, fmt.Errorf("list reusable subflow %q: %w", templateID, err)
		}
		substeps, err = s.expandSubflows(ctx, organizationID, substeps, depth+1)
		if err != nil {
			return nil, err
		}
		idMap := make(map[string]string, len(substeps))
		for _, substep := range substeps {
			idMap[substep.ID] = container.ID + ":" + substep.ID
		}
		for _, substep := range substeps {
			substep.ID = idMap[substep.ID]
			for index, prerequisite := range substep.PrerequisiteStepIDs {
				if mapped := idMap[prerequisite]; mapped != "" {
					substep.PrerequisiteStepIDs[index] = mapped
				}
			}
			if rawCondition, ok := substep.Config["condition"]; ok {
				encoded, _ := json.Marshal(rawCondition)
				var condition map[string]any
				if json.Unmarshal(encoded, &condition) == nil {
					if sourceID, _ := condition["stepId"].(string); idMap[sourceID] != "" {
						condition["stepId"] = idMap[sourceID]
						substep.Config = cloneConfig(substep.Config)
						substep.Config["condition"] = condition
					}
				}
			}
			if substep.Stage == "" {
				substep.Stage = container.Stage
			}
			if substep.ParallelGroup == "" {
				substep.ParallelGroup = container.ParallelGroup
			}
			if substep.Locale == "" {
				substep.Locale = container.Locale
			}
			expanded = append(expanded, substep)
		}
	}
	for index := range expanded {
		expanded[index].Position = index + 1
	}
	return expanded, nil
}

func (s *Service) authorizeAssignmentAccess(
	ctx context.Context,
	organizationID, actorUserID string,
	isManager bool,
	assignment JourneyAssignment,
) error {
	if isManager {
		return nil
	}

	employee, err := s.employees.GetByUserID(ctx, organizationID, actorUserID)
	if err != nil {
		return fmt.Errorf("resolve employee: %w", err)
	}

	if employee.ID != assignment.EmployeeID {
		return ErrForbidden
	}

	return nil
}

// resolveApprover picks the employee's manager as the approval approver,
// falling back to the assigning actor when no manager is linked.
func (s *Service) resolveApprover(
	ctx context.Context,
	organizationID string,
	employee employees.Employee,
	fallbackUserID string,
) string {
	if employee.ManagerEmployeeID == "" {
		return fallbackUserID
	}

	manager, err := s.employees.Get(ctx, organizationID, employee.ManagerEmployeeID)
	if err != nil || manager.UserID == "" {
		return fallbackUserID
	}

	return manager.UserID
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
	assignmentID string,
) error {
	if employee.UserID == "" || s.notify == nil {
		return nil
	}

	if _, notifyErr := s.notify.Create(ctx, organizationID, notifications.CreateInput{
		UserID: employee.UserID,
		Type:   notifications.TypeAssignment,
		Title:  "New onboarding journey",
		Body:   "You have been assigned: " + template.Name,
		Link:   "/assignments/" + assignmentID,
	}); notifyErr != nil {
		return fmt.Errorf("notify employee: %w", notifyErr)
	}

	return nil
}

func (s *Service) reopenApprovalForStep(
	ctx context.Context,
	organizationID string,
	step StepAssignment,
) error {
	approval, err := s.repo.GetApprovalByStep(ctx, organizationID, step.ID)
	if err != nil {
		return fmt.Errorf("load approval for step: %w", err)
	}

	approval.Status = approvalPending
	approval.Note = ""
	approval.DecidedAt = nil

	if err := s.repo.UpdateApproval(ctx, approval); err != nil {
		return fmt.Errorf("reopen approval: %w", err)
	}

	if s.notify == nil || approval.ApproverUserID == "" {
		return nil
	}

	if _, notifyErr := s.notify.Create(ctx, organizationID, notifications.CreateInput{
		UserID: approval.ApproverUserID,
		Type:   notifications.TypeApproval,
		Title:  "Approval needed",
		Body:   "An employee submitted \"" + step.Title + "\" for your review.",
		Link:   "/assignments/" + step.JourneyAssignmentID,
	}); notifyErr != nil {
		return fmt.Errorf("notify approver: %w", notifyErr)
	}

	return nil
}

func (s *Service) notifyApprovalDecision(
	ctx context.Context,
	organizationID string,
	step StepAssignment,
	approved bool,
) error {
	if s.notify == nil {
		return nil
	}

	employee, err := s.employees.Get(ctx, organizationID, step.EmployeeID)
	if err != nil {
		return fmt.Errorf("load employee for approval notice: %w", err)
	}

	if employee.UserID == "" {
		return nil
	}

	title := "Step rejected"
	body := "\"" + step.Title + "\" was sent back. Update it and resubmit when ready."

	if approved {
		title = "Step approved"
		body = "\"" + step.Title + "\" was approved. Your journey can continue."
	}

	if _, notifyErr := s.notify.Create(ctx, organizationID, notifications.CreateInput{
		UserID: employee.UserID,
		Type:   notifications.TypeApproval,
		Title:  title,
		Body:   body,
		Link:   "/assignments/" + step.JourneyAssignmentID,
	}); notifyErr != nil {
		return fmt.Errorf("notify employee of approval decision: %w", notifyErr)
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
	approverUserID string,
) ([]StepAssignment, []Approval) {
	stepAssignments := make([]StepAssignment, 0, len(steps))
	approvals := make([]Approval, 0)

	for index, step := range steps {
		status := stepPending
		firstStage := ""
		if len(steps) > 0 {
			firstStage = steps[0].Stage
		}
		initialParallel := index == 0 ||
			(firstStage != "" && step.Stage == firstStage) ||
			(index > 0 && steps[0].ParallelGroup != "" && step.ParallelGroup == steps[0].ParallelGroup)
		if initialParallel && len(step.PrerequisiteStepIDs) == 0 && assignment.Status == statusInProgress {
			status = stepInProgress
		}

		var dueAt *time.Time

		if step.DueOffsetDays > 0 {
			due := addDueDays(startsAt, step.DueOffsetDays, step.BusinessDays)
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
			Stage:               step.Stage,
			ParallelGroup:       step.ParallelGroup,
			PrerequisiteStepIDs: append([]string(nil), step.PrerequisiteStepIDs...),
			Locale:              step.Locale,
			Config:              cloneConfig(step.Config),
			Status:              status,
			DueAt:               dueAt,
			Submission:          nil,
			Score:               nil,
			MaxAttempts:         configInt(step.Config, "maxAttempts"),
			QuizQuestions:       nil,
			CompletedAt:         nil,
			CreatedAt:           now,
		}

		if step.StepType == stepTypeQuiz {
			stepAssignment.QuizQuestions = journeys.PublicQuizQuestions(step.Config)
		}

		if step.StepType == stepTypeAssessment {
			stepAssignment.AssessmentID = journeys.AssessmentIDFromConfig(step.Config)
		}

		stepAssignments = append(stepAssignments, stepAssignment)

		if step.StepType == stepTypeApproval {
			approvals = append(approvals, Approval{
				ID:               uuid.NewString(),
				OrganizationID:   organizationID,
				StepAssignmentID: stepAssignment.ID,
				ApproverUserID:   approverUserID,
				Status:           approvalPending,
				Note:             "",
				DecidedAt:        nil,
				CreatedAt:        now,
			})
		}
	}

	return stepAssignments, approvals
}

func cloneConfig(config map[string]any) map[string]any {
	if config == nil {
		return map[string]any{}
	}
	encoded, _ := json.Marshal(config)
	var cloned map[string]any
	_ = json.Unmarshal(encoded, &cloned)
	return cloned
}

func configInt(config map[string]any, key string) int {
	value, ok := config[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func markStepCompleted(step *StepAssignment) {
	now := time.Now().UTC()
	step.Status = stepCompleted
	step.CompletedAt = &now
}

// countCompletedSteps tallies steps in the completed status.
func countCompletedSteps(steps []StepAssignment) int {
	completed := 0

	for _, step := range steps {
		if step.Status == stepCompleted || step.Status == stepSkipped {
			completed++
		}
	}

	return completed
}

// advanceNextStep moves the first pending step to in_progress.
func (s *Service) advanceNextStep(ctx context.Context, steps []StepAssignment) error {
	completed := make(map[string]bool, len(steps))
	for _, step := range steps {
		if step.Status == stepCompleted || step.Status == stepSkipped {
			completed[step.JourneyStepID] = true
		}
	}
	started := false
	targetStage := ""
	targetGroup := ""
	for _, step := range steps {
		if step.Status != stepPending {
			continue
		}

		ready := len(step.PrerequisiteStepIDs) > 0
		for _, prerequisite := range step.PrerequisiteStepIDs {
			if !completed[prerequisite] {
				ready = false
				break
			}
		}
		if len(step.PrerequisiteStepIDs) == 0 {
			ready = !started ||
				(targetStage != "" && step.Stage == targetStage) ||
				(targetGroup != "" && step.ParallelGroup == targetGroup)
		}
		if !ready {
			continue
		}
		if !conditionMatches(step, steps) {
			step.Status = stepSkipped
			if err := s.repo.UpdateStep(ctx, step); err != nil {
				return fmt.Errorf("skip conditional assignment step: %w", err)
			}
			completed[step.JourneyStepID] = true
			continue
		}
		step.Status = stepInProgress
		if err := s.repo.UpdateStep(ctx, step); err != nil {
			return fmt.Errorf("start next assignment step: %w", err)
		}

		started = true
		if targetStage == "" && targetGroup == "" {
			targetStage, targetGroup = step.Stage, step.ParallelGroup
		}
		if step.ParallelGroup == "" && step.Stage == "" {
			break
		}
	}

	return nil
}

func conditionMatches(step StepAssignment, steps []StepAssignment) bool {
	raw, ok := step.Config["condition"]
	if !ok {
		return true
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return false
	}
	var condition struct {
		StepID string `json:"stepId"`
		Field  string `json:"field"`
		Equals any    `json:"equals"`
	}
	if json.Unmarshal(encoded, &condition) != nil || condition.StepID == "" || condition.Field == "" {
		return false
	}
	for _, candidate := range steps {
		if candidate.JourneyStepID != condition.StepID {
			continue
		}
		actual, exists := candidate.Submission[condition.Field]
		if !exists {
			return false
		}
		actualJSON, _ := json.Marshal(actual)
		expectedJSON, _ := json.Marshal(condition.Equals)
		return string(actualJSON) == string(expectedJSON)
	}
	return false
}

func addDueDays(start time.Time, days int, businessDays bool) time.Time {
	if !businessDays {
		return start.AddDate(0, 0, days)
	}
	result := start
	for remaining := days; remaining > 0; {
		result = result.AddDate(0, 0, 1)
		if result.Weekday() != time.Saturday && result.Weekday() != time.Sunday {
			remaining--
		}
	}
	return result
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

	completed := countCompletedSteps(steps)

	assignment.ProgressPercent = float64(completed) / float64(len(steps)) * percentScale
	// completingNow guards the journey-completed notification: CompletedAt is
	// set exactly once, so a later recompute of an already-completed
	// assignment can never double-notify.
	completingNow := completed == len(steps) && assignment.CompletedAt == nil

	switch {
	case completingNow:
		now := time.Now().UTC()

		assignment.Status = statusCompleted
		assignment.CompletedAt = &now
	case assignment.Status == statusInProgress:
		if err := s.advanceNextStep(ctx, steps); err != nil {
			return err
		}
		refreshed, err := s.repo.ListSteps(ctx, organizationID, journeyAssignmentID)
		if err != nil {
			return err
		}
		completed = countCompletedSteps(refreshed)
		assignment.ProgressPercent = float64(completed) / float64(len(refreshed)) * percentScale
		if completed == len(refreshed) && assignment.CompletedAt == nil {
			now := time.Now().UTC()
			assignment.Status, assignment.CompletedAt, completingNow = statusCompleted, &now, true
		}
	}

	if err := s.repo.UpdateAssignment(ctx, assignment); err != nil {
		return err
	}

	if completingNow {
		// The completion is already persisted; a notification failure must not
		// fail the step completion (same trade-off as notifyAssignment).
		if err := s.notifyJourneyCompleted(ctx, organizationID, assignment); err != nil {
			slog.ErrorContext(ctx, "journey completed notification failed", "assignmentId", assignment.ID, "error", err)
		}
	}

	return nil
}

func (s *Service) notifyJourneyCompleted(
	ctx context.Context,
	organizationID string,
	assignment JourneyAssignment,
) error {
	if s.notify == nil {
		return nil
	}

	employee, err := s.employees.Get(ctx, organizationID, assignment.EmployeeID)
	if err != nil {
		return fmt.Errorf("load employee for journey-completed notice: %w", err)
	}

	if employee.UserID == "" {
		return nil
	}

	if _, notifyErr := s.notify.Create(ctx, organizationID, notifications.CreateInput{
		UserID: employee.UserID,
		Type:   notifications.TypeJourneyCompleted,
		Title:  "Journey completed",
		Body:   "Congratulations — you completed every step of your journey.",
		Link:   "/assignments/" + assignment.ID,
	}); notifyErr != nil {
		return fmt.Errorf("notify employee of journey completion: %w", notifyErr)
	}

	return nil
}
