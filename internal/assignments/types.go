// Package assignments manages journey assignments and step progress.
package assignments

import (
	"errors"
	"time"
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
)

const (
	statusScheduled  = "scheduled"
	statusInProgress = "in_progress"
	statusCompleted  = "completed"

	stepPending          = "pending"
	stepInProgress       = "in_progress"
	stepAwaitingApproval = "awaiting_approval"
	stepCompleted        = "completed"

	approvalPending  = "pending"
	approvalApproved = "approved"
	approvalRejected = "rejected"
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
	ID                  string         `bson:"_id"                   json:"id"`
	OrganizationID      string         `bson:"organizationId"        json:"organizationId"`
	JourneyAssignmentID string         `bson:"journeyAssignmentId"   json:"journeyAssignmentId"`
	JourneyStepID       string         `bson:"journeyStepId"         json:"journeyStepId"`
	EmployeeID          string         `bson:"employeeId"            json:"employeeId"`
	StepType            string         `bson:"stepType"              json:"stepType"`
	Title               string         `bson:"title"                 json:"title"`
	Instructions        string         `bson:"instructions"          json:"instructions"`
	Position            int            `bson:"position"              json:"position"`
	Status              string         `bson:"status"                json:"status"`
	DueAt               *time.Time     `bson:"dueAt,omitempty"       json:"dueAt,omitempty"`
	Submission          map[string]any `bson:"submission,omitempty"  json:"submission,omitempty"`
	Score               *float64       `bson:"score,omitempty"       json:"score,omitempty"`
	CompletedAt         *time.Time     `bson:"completedAt,omitempty" json:"completedAt,omitempty"`
	CreatedAt           time.Time      `bson:"createdAt"             json:"createdAt"`
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

// CompleteStepInput submits and/or completes a step.
type CompleteStepInput struct {
	Submission map[string]any
	Score      *float64
}

// DecideApprovalInput records an approval decision.
type DecideApprovalInput struct {
	Approve bool
	Note    string
}
