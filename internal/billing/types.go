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
	// ErrNotConfigured indicates the payment provider is not configured.
	ErrNotConfigured = errors.New("payment provider not configured")
	// ErrInvalidSignature indicates webhook signature verification failed.
	ErrInvalidSignature = errors.New("invalid webhook signature")
	// ErrInvoiceNotPayable indicates the invoice is not in a payable state.
	ErrInvoiceNotPayable = errors.New("invoice is not payable")
	// ErrPaymentMismatch indicates the verified payment does not match the invoice.
	ErrPaymentMismatch = errors.New("verified payment does not match invoice")
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

	invoiceStatusOpen          = "open"
	invoiceStatusPaid          = "paid"
	invoiceStatusRefunded      = "refunded"
	invoiceStatusUncollectible = "uncollectible"

	invoiceDueWindowDays = 7
	monthsPerYear        = 12
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

// Invoice is an org-scoped bill for one subscription period. Status is one of
// draft | open | paid | void | uncollectible; this implementation issues open
// invoices and settles them to paid, the other states are reserved for the
// collections lifecycle (PRD 5.2.3 failed-payment recovery).
type Invoice struct {
	ID                string     `bson:"_id"                  json:"id"`
	OrganizationID    string     `bson:"organizationId"       json:"organizationId"`
	Number            string     `bson:"number"               json:"number"`
	SubscriptionID    string     `bson:"subscriptionId"       json:"subscriptionId"`
	PlanCode          string     `bson:"planCode"             json:"planCode"`
	AmountCents       int        `bson:"amountCents"          json:"amountCents"`
	SubtotalCents     int        `bson:"subtotalCents"        json:"subtotalCents"`
	TaxCents          int        `bson:"taxCents,omitempty"   json:"taxCents,omitempty"`
	DiscountCents     int        `bson:"discountCents,omitempty" json:"discountCents,omitempty"`
	CouponCode        string     `bson:"couponCode,omitempty" json:"couponCode,omitempty"`
	Currency          string     `bson:"currency"             json:"currency"`
	Status            string     `bson:"status"               json:"status"`
	PeriodStart       time.Time  `bson:"periodStart"          json:"periodStart"`
	PeriodEnd         time.Time  `bson:"periodEnd"            json:"periodEnd"`
	DueAt             time.Time  `bson:"dueAt"                json:"dueAt"`
	PaidAt            *time.Time `bson:"paidAt,omitempty"     json:"paidAt,omitempty"`
	PaymentRef        string     `bson:"paymentRef,omitempty" json:"paymentRef,omitempty"`
	DunningAttempts   int        `bson:"dunningAttempts,omitempty" json:"dunningAttempts,omitempty"`
	LastDunningAt     *time.Time `bson:"lastDunningAt,omitempty" json:"lastDunningAt,omitempty"`
	RefundedAt        *time.Time `bson:"refundedAt,omitempty" json:"refundedAt,omitempty"`
	RefundAmountCents int        `bson:"refundAmountCents,omitempty" json:"refundAmountCents,omitempty"`
	RefundReason      string     `bson:"refundReason,omitempty" json:"refundReason,omitempty"`
	CreatedAt         time.Time  `bson:"createdAt"            json:"createdAt"`
}

type AdjustInvoiceInput struct {
	TaxRateBasisPoints int
	CouponCode         string
}

type Coupon struct {
	Code                  string     `bson:"_id" json:"code"`
	PercentOffBasisPoints int        `bson:"percentOffBasisPoints,omitempty" json:"percentOffBasisPoints"`
	AmountOffCents        int        `bson:"amountOffCents,omitempty" json:"amountOffCents"`
	Currency              string     `bson:"currency,omitempty" json:"currency,omitempty"`
	MaxRedemptions        int        `bson:"maxRedemptions,omitempty" json:"maxRedemptions"`
	RedemptionCount       int        `bson:"redemptionCount,omitempty" json:"redemptionCount"`
	ExpiresAt             *time.Time `bson:"expiresAt,omitempty" json:"expiresAt,omitempty"`
	Active                bool       `bson:"active" json:"active"`
	CreatedAt             time.Time  `bson:"createdAt" json:"createdAt"`
}

type CreateCouponInput struct {
	Code                  string
	PercentOffBasisPoints int
	AmountOffCents        int
	Currency              string
	MaxRedemptions        int
	ExpiresAt             *time.Time
}

// PlanRevenue is the MRR contribution of one plan.
type PlanRevenue struct {
	PlanCode            string `json:"planCode"`
	Currency            string `json:"currency"`
	ActiveSubscriptions int    `json:"activeSubscriptions"`
	MRRCents            int64  `json:"mrrCents"`
}

// RevenueSummary reports MRR/ARR computed from active subscriptions. Plans
// carry a monthly price only, so ARR is MRR x 12; an annual-priced plan would
// be normalized by dividing its price by 12 before accumulation.
type RevenueSummary struct {
	MRRTotalCents       int64         `json:"mrrTotalCents"`
	ARRTotalCents       int64         `json:"arrTotalCents"`
	ActiveSubscriptions int           `json:"activeSubscriptions"`
	Plans               []PlanRevenue `json:"plans"`
}

// WebhookOutcome describes what a payment webhook call changed.
type WebhookOutcome struct {
	Settled        bool
	InvoiceID      string
	OrganizationID string
}
