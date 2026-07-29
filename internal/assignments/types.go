// Package assignments manages journey assignments and step progress.
package assignments

import (
	"errors"
	"time"

	"launchpad/internal/journeys"
)

var (
	// ErrNotFound indicates an assignment was not found.
	ErrNotFound = errors.New("assignment not found")
	// ErrStepNotFound indicates a step assignment was not found.
	ErrStepNotFound = errors.New("step assignment not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid assignment input")
	// ErrAlreadyAssigned indicates the employee already has this journey.
	ErrAlreadyAssigned = errors.New("journey already assigned to employee")
	// ErrInvalidState indicates an illegal status transition.
	ErrInvalidState = errors.New("invalid assignment state")
	// ErrApprovalRequired indicates completion requires approval.
	ErrApprovalRequired = errors.New("approval required before completion")
	// ErrForbidden indicates the actor may not access the assignment.
	ErrForbidden = errors.New("assignment access denied")
)

const (
	statusScheduled  = "scheduled"
	statusInProgress = "in_progress"
	statusCompleted  = "completed"

	stepPending          = "pending"
	stepInProgress       = "in_progress"
	stepAwaitingApproval = "awaiting_approval"
	stepSkipped          = "skipped"
	stepBlocked          = "blocked"
	stepCompleted        = "completed"

	approvalPending  = "pending"
	approvalApproved = "approved"
	approvalRejected = "rejected"

	// statusActive is the employee status eligible for rule/bulk assignment.
	statusActive = "active"
)

// JourneyAssignment is a frozen journey assigned to an employee.
type JourneyAssignment struct {
	ID                string     `bson:"_id"                   json:"id"`
	OrganizationID    string     `bson:"organizationId"        json:"organizationId"`
	EmployeeID        string     `bson:"employeeId"            json:"employeeId"`
	JourneyTemplateID string     `bson:"journeyTemplateId"     json:"journeyTemplateId"`
	TemplateVersion   int        `bson:"templateVersion"       json:"templateVersion"`
	Status            string     `bson:"status"                json:"status"`
	StartsAt          time.Time  `bson:"startsAt"              json:"startsAt"`
	DueAt             *time.Time `bson:"dueAt,omitempty"       json:"dueAt,omitempty"`
	ProgressPercent   float64    `bson:"progressPercent"       json:"progressPercent"`
	CompletedAt       *time.Time `bson:"completedAt,omitempty" json:"completedAt,omitempty"`
	CreatedAt         time.Time  `bson:"createdAt"             json:"createdAt"`
}

// StepAssignment tracks one step for an assigned journey.
type StepAssignment struct {
	ID                  string         `bson:"_id"                  json:"id"`
	OrganizationID      string         `bson:"organizationId"       json:"organizationId"`
	JourneyAssignmentID string         `bson:"journeyAssignmentId"  json:"journeyAssignmentId"`
	JourneyStepID       string         `bson:"journeyStepId"        json:"journeyStepId"`
	EmployeeID          string         `bson:"employeeId"           json:"employeeId"`
	StepType            string         `bson:"stepType"             json:"stepType"`
	Title               string         `bson:"title"                json:"title"`
	Instructions        string         `bson:"instructions"         json:"instructions"`
	Position            int            `bson:"position"             json:"position"`
	Stage               string         `bson:"stage,omitempty"      json:"stage,omitempty"`
	ParallelGroup       string         `bson:"parallelGroup,omitempty" json:"parallelGroup,omitempty"`
	PrerequisiteStepIDs []string       `bson:"prerequisiteStepIds,omitempty" json:"prerequisiteStepIds,omitempty"`
	Locale              string         `bson:"locale,omitempty" json:"locale,omitempty"`
	Config              map[string]any `bson:"config,omitempty" json:"-"`
	Status              string         `bson:"status"               json:"status"`
	DueAt               *time.Time     `bson:"dueAt,omitempty"      json:"dueAt,omitempty"`
	StartedAt           *time.Time     `bson:"startedAt,omitempty"  json:"startedAt,omitempty"`
	Submission          map[string]any `bson:"submission,omitempty" json:"submission,omitempty"`
	Score               *float64       `bson:"score,omitempty"      json:"score,omitempty"`
	AttemptCount        int            `bson:"attemptCount,omitempty" json:"attemptCount"`
	MaxAttempts         int            `bson:"maxAttempts,omitempty" json:"maxAttempts"`
	EscalatedAt         *time.Time     `bson:"escalatedAt,omitempty" json:"escalatedAt,omitempty"`
	OverrideAction      string         `bson:"overrideAction,omitempty" json:"overrideAction,omitempty"`
	OverrideReason      string         `bson:"overrideReason,omitempty" json:"overrideReason,omitempty"`
	OverrideByUserID    string         `bson:"overrideByUserId,omitempty" json:"overrideByUserId,omitempty"`
	OverriddenAt        *time.Time     `bson:"overriddenAt,omitempty" json:"overriddenAt,omitempty"`
	// QuizQuestions is the answer-key-free snapshot of the quiz questions,
	// frozen at assign time so employees can render the quiz. Empty for
	// non-quiz steps.
	QuizQuestions []journeys.QuizQuestionView `bson:"quizQuestions,omitempty" json:"quizQuestions,omitempty"`
	// AssessmentID links an assessment step to its published assessment,
	// frozen at assign time from the step config. Empty for non-assessment
	// steps.
	AssessmentID string     `bson:"assessmentId,omitempty" json:"assessmentId,omitempty"`
	CompletedAt  *time.Time `bson:"completedAt,omitempty"   json:"completedAt,omitempty"`
	// DueSoonNotifiedAt and OverdueNotifiedAt dedupe scheduler notifications:
	// once set, the due sweep never sends that kind again. Internal
	// bookkeeping, not part of the API response.
	DueSoonNotifiedAt *time.Time `bson:"dueSoonNotifiedAt,omitempty" json:"-"`
	OverdueNotifiedAt *time.Time `bson:"overdueNotifiedAt,omitempty" json:"-"`
	CreatedAt         time.Time  `bson:"createdAt"                   json:"createdAt"`
}

// Approval is a manager decision on an approval step.
type Approval struct {
	ID               string     `bson:"_id"                 json:"id"`
	OrganizationID   string     `bson:"organizationId"      json:"organizationId"`
	StepAssignmentID string     `bson:"stepAssignmentId"    json:"stepAssignmentId"`
	ApproverUserID   string     `bson:"approverUserId"      json:"approverUserId"`
	Status           string     `bson:"status"              json:"status"`
	Note             string     `bson:"note"                json:"note"`
	DecidedAt        *time.Time `bson:"decidedAt,omitempty" json:"decidedAt,omitempty"`
	CreatedAt        time.Time  `bson:"createdAt"           json:"createdAt"`
}

// AssignInput assigns a published journey to an employee.
type AssignInput struct {
	EmployeeID        string
	JourneyTemplateID string
	StartsAt          time.Time
}

// AssignDepartmentInput assigns a published journey to every employee in a
// department.
type AssignDepartmentInput struct {
	DepartmentID      string
	JourneyTemplateID string
	StartsAt          time.Time
}

// AssignDepartmentResult summarizes a department-wide assignment run:
// employees in the department, newly assigned, and skipped because they
// already had an active assignment for the template.
type AssignDepartmentResult struct {
	Employees int `json:"employees"`
	Assigned  int `json:"assigned"`
	Skipped   int `json:"skipped"`
}

// CompleteStepInput submits and/or completes a step.
type CompleteStepInput struct {
	Submission map[string]any
	Score      *float64
}

// SubmitStepInput stores partial progress on a step without completing it.
type SubmitStepInput struct {
	Submission map[string]any
}

// DecideApprovalInput records an approval decision.
type DecideApprovalInput struct {
	Approve bool
	Note    string
}

type OverrideStepInput struct {
	Action string
	Reason string
}

// TeamReportSummary is one direct report's rollup for the manager dashboard
// (PRD §5.3.9).
type TeamReportSummary struct {
	EmployeeID           string `json:"employeeId"`
	Name                 string `json:"name"`
	ActiveAssignments    int    `json:"activeAssignments"`
	CompletedAssignments int    `json:"completedAssignments"`
	OverdueSteps         int    `json:"overdueSteps"`
	PendingApprovals     int    `json:"pendingApprovals"`
}

// ReportBlockerInput reports a blocker on the caller's onboarding.
// StepAssignmentID is optional; Category is one of hr/it/manager/other.
type ReportBlockerInput struct {
	StepAssignmentID string
	Category         string
	Message          string
}
