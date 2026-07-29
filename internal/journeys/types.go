// Package journeys manages onboarding journey templates and steps.
package journeys

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates a journey template was not found.
	ErrNotFound = errors.New("journey not found")
	// ErrStepNotFound indicates a journey step was not found.
	ErrStepNotFound = errors.New("journey step not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid journey input")
	// ErrNotDraft indicates the journey is not in draft status.
	ErrNotDraft = errors.New("journey is not a draft")
	// ErrNotPublished indicates the journey is not published.
	ErrNotPublished = errors.New("journey is not published")
	// ErrNoSteps indicates publish was attempted without steps.
	ErrNoSteps = errors.New("journey has no steps")
	// ErrStepPositionTaken indicates a step already exists at the position.
	ErrStepPositionTaken = errors.New("journey step position already taken")
)

const (
	statusDraft     = "draft"
	statusPublished = "published"
	statusArchived  = "archived"

	stepTypeDocument = "document"
	stepTypeQuiz     = "quiz"
	stepTypeTask     = "task"
	stepTypeApproval = "approval"
	// equipment_request/access_request steps auto-create an equipment/access
	// request for the employee when completed (PRD §5.3.8).
	stepTypeEquipmentRequest = "equipment_request"
	stepTypeAccessRequest    = "access_request"
	// assessment steps link a published assessment; completion requires a
	// passing attempt (PRD §5.3.6).
	stepTypeAssessment = "assessment"
	// meeting steps complete when the employee schedules the meeting through
	// the step's schedule form (PRD §5.3.7).
	stepTypeMeeting               = "meeting"
	stepTypeInformation           = "information"
	stepTypePolicyAcknowledgement = "policy_acknowledgement"
	stepTypeVideo                 = "video"
	stepTypeExternalCourse        = "external_course"
	stepTypeSurvey                = "survey"
	stepTypeFileSubmission        = "file_submission"
	stepTypeTextSubmission        = "text_submission"
	stepTypeCodingExercise        = "coding_exercise"
	stepTypeShadowingSession      = "shadowing_session"
	stepTypeChecklist             = "checklist"
	stepTypeIntegrationAction     = "integration_action"
	stepTypeManagerFeedback       = "manager_feedback"
	stepTypeEmployeeReflection    = "employee_reflection"
	stepTypeCertification         = "certification"
)

// Template is a versioned onboarding journey definition.
type Template struct {
	ID             string    `bson:"_id"            json:"id"`
	OrganizationID string    `bson:"organizationId" json:"organizationId"`
	Name           string    `bson:"name"           json:"name"`
	Description    string    `bson:"description"    json:"description"`
	Status         string    `bson:"status"         json:"status"`
	CurrentVersion int       `bson:"currentVersion" json:"currentVersion"`
	CreatedBy      string    `bson:"createdBy"      json:"createdBy"`
	CreatedAt      time.Time `bson:"createdAt"      json:"createdAt"`
	UpdatedAt      time.Time `bson:"updatedAt"      json:"updatedAt"`
}

// Step is a single step inside a journey template version.
type Step struct {
	ID                  string         `bson:"_id"               json:"id"`
	OrganizationID      string         `bson:"organizationId"    json:"organizationId"`
	JourneyTemplateID   string         `bson:"journeyTemplateId" json:"journeyTemplateId"`
	Version             int            `bson:"version"           json:"version"`
	StepType            string         `bson:"stepType"          json:"stepType"`
	Title               string         `bson:"title"             json:"title"`
	Instructions        string         `bson:"instructions"      json:"instructions"`
	Position            int            `bson:"position"          json:"position"`
	DueOffsetDays       int            `bson:"dueOffsetDays"     json:"dueOffsetDays"`
	BusinessDays        bool           `bson:"businessDays"      json:"businessDays"`
	Stage               string         `bson:"stage,omitempty"   json:"stage,omitempty"`
	ParallelGroup       string         `bson:"parallelGroup,omitempty" json:"parallelGroup,omitempty"`
	PrerequisiteStepIDs []string       `bson:"prerequisiteStepIds,omitempty" json:"prerequisiteStepIds,omitempty"`
	Locale              string         `bson:"locale,omitempty"  json:"locale,omitempty"`
	Config              map[string]any `bson:"config"            json:"config"`
	CreatedAt           time.Time      `bson:"createdAt"         json:"createdAt"`
}

// CreateTemplateInput creates a draft journey.
type CreateTemplateInput struct {
	Name        string
	Description string
	CreatedBy   string
}

// VersionSummary describes one version of a journey template.
type VersionSummary struct {
	Version   int    `json:"version"`
	Status    string `json:"status"`
	StepCount int64  `json:"stepCount"`
}

// AddStepInput adds a step to the current draft version.
type AddStepInput struct {
	StepType            string
	Title               string
	Instructions        string
	DueOffsetDays       int
	BusinessDays        bool
	Stage               string
	ParallelGroup       string
	PrerequisiteStepIDs []string
	Locale              string
	Config              map[string]any
}

// ImportStep is a portable journey step used by curated template installers.
type ImportStep struct {
	StepType            string
	Title               string
	Instructions        string
	DueOffsetDays       int
	BusinessDays        bool
	Stage               string
	ParallelGroup       string
	PrerequisiteStepIDs []string
	Locale              string
	Config              map[string]any
}

func isValidStepType(stepType string) bool {
	switch stepType {
	case stepTypeDocument, stepTypeQuiz, stepTypeTask, stepTypeApproval,
		stepTypeEquipmentRequest, stepTypeAccessRequest, stepTypeAssessment,
		stepTypeMeeting, stepTypeInformation, stepTypePolicyAcknowledgement,
		stepTypeVideo, stepTypeExternalCourse, stepTypeSurvey, stepTypeFileSubmission,
		stepTypeTextSubmission, stepTypeCodingExercise, stepTypeShadowingSession,
		stepTypeChecklist, stepTypeIntegrationAction, stepTypeManagerFeedback,
		stepTypeEmployeeReflection, stepTypeCertification:
		return true
	default:
		return false
	}
}
