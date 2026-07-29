package billing

import "time"

// PlanResponse is the API representation of a Plan. It decouples the public
// contract from the persistence layout.
type PlanResponse struct {
	Code              string    `json:"code"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	PriceMonthlyCents int       `json:"priceMonthlyCents"`
	Currency          string    `json:"currency"`
	Features          []string  `json:"features"`
	Active            bool      `json:"active"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// ToResponse maps the persistence document to its API representation.
func (p Plan) ToResponse() PlanResponse {
	return PlanResponse(p)
}

func toPlanResponses(items []Plan) []PlanResponse {
	responses := make([]PlanResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}

// SubscriptionResponse is the API representation of a Subscription.
type SubscriptionResponse struct {
	ID               string     `json:"id"`
	OrganizationID   string     `json:"organizationId"`
	PlanCode         string     `json:"planCode"`
	Status           string     `json:"status"`
	CurrentPeriodEnd *time.Time `json:"currentPeriodEnd,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// ToResponse maps the persistence document to its API representation.
func (s Subscription) ToResponse() SubscriptionResponse {
	return SubscriptionResponse(s)
}

func toSubscriptionResponses(items []Subscription) []SubscriptionResponse {
	responses := make([]SubscriptionResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}

// InvoiceResponse is the API representation of an Invoice. It decouples the
// public contract from the persistence layout.
type InvoiceResponse struct {
	ID                string     `json:"id"`
	OrganizationID    string     `json:"organizationId"`
	Number            string     `json:"number"`
	SubscriptionID    string     `json:"subscriptionId"`
	PlanCode          string     `json:"planCode"`
	AmountCents       int        `json:"amountCents"`
	SubtotalCents     int        `json:"subtotalCents"`
	TaxCents          int        `json:"taxCents"`
	DiscountCents     int        `json:"discountCents"`
	CouponCode        string     `json:"couponCode,omitempty"`
	Currency          string     `json:"currency"`
	Status            string     `json:"status"`
	PeriodStart       time.Time  `json:"periodStart"`
	PeriodEnd         time.Time  `json:"periodEnd"`
	DueAt             time.Time  `json:"dueAt"`
	PaidAt            *time.Time `json:"paidAt,omitempty"`
	PaymentRef        string     `json:"paymentRef,omitempty"`
	DunningAttempts   int        `json:"dunningAttempts"`
	LastDunningAt     *time.Time `json:"lastDunningAt,omitempty"`
	RefundedAt        *time.Time `json:"refundedAt,omitempty"`
	RefundAmountCents int        `json:"refundAmountCents"`
	RefundReason      string     `json:"refundReason,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
}

// ToResponse maps the persistence document to its API representation.
func (i Invoice) ToResponse() InvoiceResponse {
	return InvoiceResponse{
		ID: i.ID, OrganizationID: i.OrganizationID, Number: i.Number,
		SubscriptionID: i.SubscriptionID, PlanCode: i.PlanCode,
		AmountCents: i.AmountCents, SubtotalCents: i.SubtotalCents,
		TaxCents: i.TaxCents, DiscountCents: i.DiscountCents, CouponCode: i.CouponCode,
		Currency: i.Currency, Status: i.Status, PeriodStart: i.PeriodStart,
		PeriodEnd: i.PeriodEnd, DueAt: i.DueAt, PaidAt: i.PaidAt,
		PaymentRef: i.PaymentRef, DunningAttempts: i.DunningAttempts,
		LastDunningAt: i.LastDunningAt, RefundedAt: i.RefundedAt,
		RefundAmountCents: i.RefundAmountCents, RefundReason: i.RefundReason,
		CreatedAt: i.CreatedAt,
	}
}

func toInvoiceResponses(items []Invoice) []InvoiceResponse {
	responses := make([]InvoiceResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}

// CheckoutSessionResponse is the hosted-checkout redirect target returned by
// the invoice pay endpoint.
type CheckoutSessionResponse struct {
	CheckoutURL string `json:"checkoutUrl"`
	Reference   string `json:"reference"`
}
