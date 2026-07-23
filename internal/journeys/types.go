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
)

const (
	statusDraft     = "draft"
	statusPublished = "published"
	statusArchived  = "archived"

	stepTypeDocument = "document"
	stepTypeQuiz     = "quiz"
	stepTypeTask     = "task"
	stepTypeApproval = "approval"

	fieldOrganizationID = "organizationId"
	fieldCreatedAt      = "createdAt"
	fieldTemplateID     = "journeyTemplateId"
	fieldVersion        = "version"
	fieldPosition       = "position"
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
	ID                string         `bson:"_id"               json:"id"`
	OrganizationID    string         `bson:"organizationId"    json:"organizationId"`
	JourneyTemplateID string         `bson:"journeyTemplateId" json:"journeyTemplateId"`
	Version           int            `bson:"version"           json:"version"`
	StepType          string         `bson:"stepType"          json:"stepType"`
	Title             string         `bson:"title"             json:"title"`
	Instructions      string         `bson:"instructions"      json:"instructions"`
	Position          int            `bson:"position"          json:"position"`
	DueOffsetDays     int            `bson:"dueOffsetDays"     json:"dueOffsetDays"`
	Config            map[string]any `bson:"config"            json:"config"`
	CreatedAt         time.Time      `bson:"createdAt"         json:"createdAt"`
}

// CreateTemplateInput creates a draft journey.
type CreateTemplateInput struct {
	Name        string
	Description string
	CreatedBy   string
}

// AddStepInput adds a step to the current draft version.
type AddStepInput struct {
	StepType      string
	Title         string
	Instructions  string
	DueOffsetDays int
	Config        map[string]any
}

func isValidStepType(stepType string) bool {
	switch stepType {
	case stepTypeDocument, stepTypeQuiz, stepTypeTask, stepTypeApproval:
		return true
	default:
		return false
	}
}
