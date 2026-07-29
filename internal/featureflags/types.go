package featureflags

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates the flag or override does not exist.
	ErrNotFound = errors.New("feature flag not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid feature flag input")
	// ErrKeyTaken indicates the flag key is already registered.
	ErrKeyTaken = errors.New("feature flag key already taken")
)

const (
	flagKeyAIAssistant = "ai_assistant"
	flagKeySlack       = "integrations_slack"
	flagKeySSO         = "sso"
	planCodeEnterprise = "enterprise"
)

// Flag is a global feature toggle with optional plan restrictions.
type Flag struct {
	Key               string     `bson:"_id"                     json:"key"`
	Description       string     `bson:"description"             json:"description"`
	Enabled           bool       `bson:"enabled"                 json:"enabled"`
	PlanCodes         []string   `bson:"planCodes"               json:"planCodes"`
	RolloutPercentage int        `bson:"rolloutPercentage"       json:"rolloutPercentage"`
	CohortUserIDs     []string   `bson:"cohortUserIds,omitempty" json:"cohortUserIds"`
	ExpiresAt         *time.Time `bson:"expiresAt,omitempty"     json:"expiresAt,omitempty"`
	CreatedAt         time.Time  `bson:"createdAt"               json:"createdAt"`
	UpdatedAt         time.Time  `bson:"updatedAt"               json:"updatedAt"`
}

// Override is a tenant-specific feature flag override.
type Override struct {
	ID             string    `bson:"_id"            json:"id"`
	OrganizationID string    `bson:"organizationId" json:"organizationId"`
	Key            string    `bson:"key"            json:"key"`
	Enabled        bool      `bson:"enabled"        json:"enabled"`
	UpdatedAt      time.Time `bson:"updatedAt"      json:"updatedAt"`
	UpdatedBy      string    `bson:"updatedBy"      json:"updatedBy"`
}

// CreateFlagInput creates a global flag.
type CreateFlagInput struct {
	Key               string
	Description       string
	Enabled           bool
	PlanCodes         []string
	RolloutPercentage int
	CohortUserIDs     []string
	ExpiresAt         *time.Time
	ActorUserID       string
}

// UpdateFlagInput patches a global flag.
type UpdateFlagInput struct {
	Description       *string
	Enabled           *bool
	PlanCodes         *[]string
	RolloutPercentage *int
	CohortUserIDs     *[]string
	ExpiresAt         **time.Time
	UpdatedBy         string
}

// History records each platform mutation for rollout review. Snapshot is the
// resulting flag state; tenant override changes also carry the target.
type History struct {
	ID             string    `bson:"_id"                      json:"id"`
	Key            string    `bson:"key"                      json:"key"`
	Action         string    `bson:"action"                   json:"action"`
	ActorUserID    string    `bson:"actorUserId"              json:"actorUserId"`
	OrganizationID string    `bson:"organizationId,omitempty" json:"organizationId,omitempty"`
	Snapshot       Flag      `bson:"snapshot"                 json:"snapshot"`
	CreatedAt      time.Time `bson:"createdAt"                json:"createdAt"`
}

// SetOverrideInput sets a tenant override.
type SetOverrideInput struct {
	OrganizationID string
	Key            string
	Enabled        bool
	UpdatedBy      string
}
