package billing

import "context"

// Repository persists billing plans and subscriptions.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	UpsertPlan(ctx context.Context, plan Plan) error
	GetPlan(ctx context.Context, code string) (Plan, error)
	ListPlans(ctx context.Context, activeOnly bool) ([]Plan, error)
	CreatePlan(ctx context.Context, plan Plan) error
	UpdatePlan(ctx context.Context, plan Plan) error
	GetSubscriptionByOrganization(ctx context.Context, organizationID string) (Subscription, error)
	CreateSubscription(ctx context.Context, subscription Subscription) error
	UpdateSubscription(ctx context.Context, subscription Subscription) error
	ListSubscriptions(ctx context.Context) ([]Subscription, error)
	// NextInvoiceSequence returns the next per-organization invoice sequence
	// number, starting at 1.
	NextInvoiceSequence(ctx context.Context, organizationID string) (int64, error)
	CreateInvoice(ctx context.Context, invoice Invoice) error
	GetInvoice(ctx context.Context, organizationID, invoiceID string) (Invoice, error)
	GetInvoiceByPaymentRef(ctx context.Context, reference string) (Invoice, error)
	ListInvoicesByOrganization(ctx context.Context, organizationID string) ([]Invoice, error)
	ListInvoices(ctx context.Context) ([]Invoice, error)
	CreateCoupon(ctx context.Context, coupon Coupon) error
	GetCoupon(ctx context.Context, code string) (Coupon, error)
	ListCoupons(ctx context.Context) ([]Coupon, error)
	UpdateCoupon(ctx context.Context, coupon Coupon) error
	UpdateInvoice(ctx context.Context, invoice Invoice) error
	// DeleteForOrganization removes every billing subscription, invoice, and
	// invoice sequence of the organization and returns the number deleted.
	// Plans are platform-wide and are kept. Called only by the platform GDPR
	// tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}
