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
}
