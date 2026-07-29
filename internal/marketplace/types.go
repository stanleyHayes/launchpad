// Package marketplace manages the moderated global onboarding-template catalogue.
package marketplace

import (
	"errors"
	"time"
)

var (
	ErrNotFound        = errors.New("marketplace template not found")
	ErrInvalidInput    = errors.New("invalid marketplace input")
	ErrInvalidState    = errors.New("invalid marketplace state")
	ErrPaymentRequired = errors.New("marketplace template purchase required")
)

const (
	StatusDraft     = "draft"
	StatusSubmitted = "submitted"
	StatusPublished = "published"
	StatusRemoved   = "removed"
)

type Step struct {
	StepType      string         `bson:"stepType" json:"stepType"`
	Title         string         `bson:"title" json:"title"`
	Instructions  string         `bson:"instructions" json:"instructions"`
	DueOffsetDays int            `bson:"dueOffsetDays" json:"dueOffsetDays"`
	Config        map[string]any `bson:"config" json:"config"`
}

type Template struct {
	ID                        string    `bson:"_id" json:"id"`
	Name                      string    `bson:"name" json:"name"`
	Slug                      string    `bson:"slug" json:"slug"`
	Description               string    `bson:"description" json:"description"`
	Category                  string    `bson:"category" json:"category"`
	Status                    string    `bson:"status" json:"status"`
	Official                  bool      `bson:"official" json:"official"`
	Featured                  bool      `bson:"featured" json:"featured"`
	Version                   int       `bson:"version" json:"version"`
	SubmittedByOrganizationID string    `bson:"submittedByOrganizationId,omitempty" json:"submittedByOrganizationId,omitempty"`
	Steps                     []Step    `bson:"steps" json:"steps"`
	InstallationCount         int64     `bson:"installationCount" json:"installationCount"`
	RatingAverage             float64   `bson:"ratingAverage" json:"ratingAverage"`
	RatingCount               int64     `bson:"ratingCount" json:"ratingCount"`
	PriceCents                int       `bson:"priceCents" json:"priceCents"`
	Currency                  string    `bson:"currency" json:"currency"`
	CreatedBy                 string    `bson:"createdBy" json:"createdBy"`
	CreatedAt                 time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt                 time.Time `bson:"updatedAt" json:"updatedAt"`
}

const (
	PurchasePending = "pending"
	PurchasePaid    = "paid"
)

type Purchase struct {
	ID                   string     `bson:"_id" json:"id"`
	TemplateID           string     `bson:"templateId" json:"templateId"`
	OrganizationID       string     `bson:"organizationId" json:"organizationId"`
	BuyerUserID          string     `bson:"buyerUserId" json:"buyerUserId"`
	SellerOrganizationID string     `bson:"sellerOrganizationId" json:"sellerOrganizationId"`
	AmountCents          int        `bson:"amountCents" json:"amountCents"`
	Currency             string     `bson:"currency" json:"currency"`
	PlatformFeeCents     int        `bson:"platformFeeCents" json:"platformFeeCents"`
	SellerEarningsCents  int        `bson:"sellerEarningsCents" json:"sellerEarningsCents"`
	Reference            string     `bson:"reference" json:"reference"`
	Status               string     `bson:"status" json:"status"`
	InstallationID       string     `bson:"installationId,omitempty" json:"installationId,omitempty"`
	JourneyTemplateID    string     `bson:"journeyTemplateId,omitempty" json:"journeyTemplateId,omitempty"`
	PaidAt               *time.Time `bson:"paidAt,omitempty" json:"paidAt,omitempty"`
	CreatedAt            time.Time  `bson:"createdAt" json:"createdAt"`
	UpdatedAt            time.Time  `bson:"updatedAt" json:"updatedAt"`
}

type Checkout struct {
	AuthorizationURL string   `json:"authorizationUrl"`
	Reference        string   `json:"reference"`
	Purchase         Purchase `json:"purchase"`
}

type Installation struct {
	ID                string    `bson:"_id" json:"id"`
	TemplateID        string    `bson:"templateId" json:"templateId"`
	TemplateVersion   int       `bson:"templateVersion" json:"templateVersion"`
	OrganizationID    string    `bson:"organizationId" json:"organizationId"`
	JourneyTemplateID string    `bson:"journeyTemplateId" json:"journeyTemplateId"`
	InstalledBy       string    `bson:"installedBy" json:"installedBy"`
	InstalledAt       time.Time `bson:"installedAt" json:"installedAt"`
}

type Rating struct {
	ID             string    `bson:"_id" json:"id"`
	TemplateID     string    `bson:"templateId" json:"templateId"`
	OrganizationID string    `bson:"organizationId" json:"organizationId"`
	UserID         string    `bson:"userId" json:"userId"`
	Score          int       `bson:"score" json:"score"`
	CreatedAt      time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time `bson:"updatedAt" json:"updatedAt"`
}

type CreateInput struct {
	Name                      string
	Description               string
	Category                  string
	Official                  bool
	SubmittedByOrganizationID string
	Steps                     []Step
	CreatedBy                 string
	PriceCents                int
	Currency                  string
}
