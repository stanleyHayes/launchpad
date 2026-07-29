package assignments

import (
	"time"

	"launchpad/internal/journeys"
)

// JourneyAssignmentResponse is the API representation of a JourneyAssignment.
// It decouples the public contract from the persistence layout.
type JourneyAssignmentResponse struct {
	ID                string     `json:"id"`
	OrganizationID    string     `json:"organizationId"`
	EmployeeID        string     `json:"employeeId"`
	JourneyTemplateID string     `json:"journeyTemplateId"`
	TemplateVersion   int        `json:"templateVersion"`
	Status            string     `json:"status"`
	StartsAt          time.Time  `json:"startsAt"`
	DueAt             *time.Time `json:"dueAt,omitempty"`
	ProgressPercent   float64    `json:"progressPercent"`
	CompletedAt       *time.Time `json:"completedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
}

// ToResponse maps the persistence document to its API representation.
func (a JourneyAssignment) ToResponse() JourneyAssignmentResponse {
	return JourneyAssignmentResponse(a)
}

func toJourneyAssignmentResponses(items []JourneyAssignment) []JourneyAssignmentResponse {
	responses := make([]JourneyAssignmentResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}

// StepAssignmentResponse is the API representation of a StepAssignment.
type StepAssignmentResponse struct {
	ID                  string                      `json:"id"`
	OrganizationID      string                      `json:"organizationId"`
	JourneyAssignmentID string                      `json:"journeyAssignmentId"`
	JourneyStepID       string                      `json:"journeyStepId"`
	EmployeeID          string                      `json:"employeeId"`
	StepType            string                      `json:"stepType"`
	Title               string                      `json:"title"`
	Instructions        string                      `json:"instructions"`
	Position            int                         `json:"position"`
	Stage               string                      `json:"stage,omitempty"`
	ParallelGroup       string                      `json:"parallelGroup,omitempty"`
	PrerequisiteStepIDs []string                    `json:"prerequisiteStepIds,omitempty"`
	Locale              string                      `json:"locale,omitempty"`
	Status              string                      `json:"status"`
	DueAt               *time.Time                  `json:"dueAt,omitempty"`
	StartedAt           *time.Time                  `json:"startedAt,omitempty"`
	Submission          map[string]any              `json:"submission,omitempty"`
	Score               *float64                    `json:"score,omitempty"`
	AttemptCount        int                         `json:"attemptCount"`
	MaxAttempts         int                         `json:"maxAttempts"`
	EscalatedAt         *time.Time                  `json:"escalatedAt,omitempty"`
	OverrideAction      string                      `json:"overrideAction,omitempty"`
	OverrideReason      string                      `json:"overrideReason,omitempty"`
	OverrideByUserID    string                      `json:"overrideByUserId,omitempty"`
	OverriddenAt        *time.Time                  `json:"overriddenAt,omitempty"`
	QuizQuestions       []journeys.QuizQuestionView `json:"quizQuestions,omitempty"`
	CompletedAt         *time.Time                  `json:"completedAt,omitempty"`
	CreatedAt           time.Time                   `json:"createdAt"`
}

// ToResponse maps the persistence document to its API representation. The
// notification dedupe timestamps stay internal and are not exposed.
func (s StepAssignment) ToResponse() StepAssignmentResponse {
	return StepAssignmentResponse{
		ID:                  s.ID,
		OrganizationID:      s.OrganizationID,
		JourneyAssignmentID: s.JourneyAssignmentID,
		JourneyStepID:       s.JourneyStepID,
		EmployeeID:          s.EmployeeID,
		StepType:            s.StepType,
		Title:               s.Title,
		Instructions:        s.Instructions,
		Position:            s.Position,
		Stage:               s.Stage,
		ParallelGroup:       s.ParallelGroup,
		PrerequisiteStepIDs: s.PrerequisiteStepIDs,
		Locale:              s.Locale,
		Status:              s.Status,
		DueAt:               s.DueAt,
		StartedAt:           s.StartedAt,
		Submission:          s.Submission,
		Score:               s.Score,
		AttemptCount:        s.AttemptCount,
		MaxAttempts:         s.MaxAttempts,
		EscalatedAt:         s.EscalatedAt,
		OverrideAction:      s.OverrideAction,
		OverrideReason:      s.OverrideReason,
		OverrideByUserID:    s.OverrideByUserID,
		OverriddenAt:        s.OverriddenAt,
		QuizQuestions:       s.QuizQuestions,
		CompletedAt:         s.CompletedAt,
		CreatedAt:           s.CreatedAt,
	}
}

func toStepAssignmentResponses(items []StepAssignment) []StepAssignmentResponse {
	responses := make([]StepAssignmentResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}

// ApprovalResponse is the API representation of an Approval.
type ApprovalResponse struct {
	ID               string     `json:"id"`
	OrganizationID   string     `json:"organizationId"`
	StepAssignmentID string     `json:"stepAssignmentId"`
	ApproverUserID   string     `json:"approverUserId"`
	Status           string     `json:"status"`
	Note             string     `json:"note"`
	DecidedAt        *time.Time `json:"decidedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// ToResponse maps the persistence document to its API representation.
func (a Approval) ToResponse() ApprovalResponse {
	return ApprovalResponse(a)
}

func toApprovalResponses(items []Approval) []ApprovalResponse {
	responses := make([]ApprovalResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}

// AssignResultResponse is the API representation of an AssignResult.
type AssignResultResponse struct {
	Assignment JourneyAssignmentResponse `json:"assignment"`
	Steps      []StepAssignmentResponse  `json:"steps"`
}

// ToResponse maps the service result to its API representation.
func (r AssignResult) ToResponse() AssignResultResponse {
	return AssignResultResponse{
		Assignment: r.Assignment.ToResponse(),
		Steps:      toStepAssignmentResponses(r.Steps),
	}
}
