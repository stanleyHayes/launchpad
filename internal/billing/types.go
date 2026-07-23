package billing

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates the plan or subscription does not exist.
	ErrNotFound = errors.New("billing record not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid billing input")
	// ErrCodeTaken indicates the plan code is already registered.
	ErrCodeTaken = errors.New("plan code already taken")
)

const (
	planStarter    = "starter"
	planGrowth     = "growth"
	planEnterprise = "enterprise"

	featureCoreOnboarding = "core_onboarding"
	featureAnalytics      = "analytics"
	featureSSO            = "sso"
	featureSupportSLA     = "support_sla"

	growthPriceMonthlyCents = 9900

	currencyUSD = "USD"

	statusTrialing = "trialing"
	statusActive   = "active"
	statusPastDue  = "past_due"
	statusCanceled = "canceled"
)

// Plan is a sellable subscription tier.
type Plan struct {
	Code              string    `bson:"_id"               json:"code"`
	Name              string    `bson:"name"              json:"name"`
	Description       string    `bson:"description"       json:"description"`
	PriceMonthlyCents int       `bson:"priceMonthlyCents" json:"priceMonthlyCents"`
	Currency          string    `bson:"currency"          json:"currency"`
	Features          []string  `bson:"features"          json:"features"`
	Active            bool      `bson:"active"            json:"active"`
	CreatedAt         time.Time `bson:"createdAt"         json:"createdAt"`
	UpdatedAt         time.Time `bson:"updatedAt"         json:"updatedAt"`
}

// Subscription tracks an organization's billing subscription.
type Subscription struct {
	ID               string     `bson:"_id"                        json:"id"`
	OrganizationID   string     `bson:"organizationId"             json:"organizationId"`
	PlanCode         string     `bson:"planCode"                   json:"planCode"`
	Status           string     `bson:"status"                     json:"status"`
	CurrentPeriodEnd *time.Time `bson:"currentPeriodEnd,omitempty" json:"currentPeriodEnd,omitempty"`
	CreatedAt        time.Time  `bson:"createdAt"                  json:"createdAt"`
	UpdatedAt        time.Time  `bson:"updatedAt"                  json:"updatedAt"`
}

// CreatePlanInput creates a billing plan.
type CreatePlanInput struct {
	Code              string
	Name              string
	Description       string
	PriceMonthlyCents int
	Currency          string
	Features          []string
	Active            bool
}

// UpdatePlanInput patches a billing plan.
type UpdatePlanInput struct {
	Name              *string
	Description       *string
	PriceMonthlyCents *int
	Currency          *string
	Features          *[]string
	Active            *bool
}

// SetOrganizationPlanInput assigns a plan to an organization.
type SetOrganizationPlanInput struct {
	OrganizationID string
	PlanCode       string
	Status         string
}
