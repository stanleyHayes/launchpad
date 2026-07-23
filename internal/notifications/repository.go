package notifications

import "context"

// Repository persists notifications.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, notification Notification) error
	ListForUser(ctx context.Context, organizationID, userID string) ([]Notification, error)
	Get(ctx context.Context, organizationID, userID, notificationID string) (Notification, error)
	Update(ctx context.Context, notification Notification) error
}
