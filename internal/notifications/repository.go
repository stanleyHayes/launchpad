package notifications

import (
	"context"
	"time"
)

// Repository persists notifications.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, notification Notification) error
	ListForUser(ctx context.Context, organizationID, userID string) ([]Notification, error)
	Get(ctx context.Context, organizationID, userID, notificationID string) (Notification, error)
	Update(ctx context.Context, notification Notification) error
	// DeleteForOrganization removes every notification of the organization
	// and returns the number deleted. Called only by the platform GDPR
	// tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// ChannelStore persists per-organization outbound channel configuration.
type ChannelStore interface {
	EnsureIndexes(ctx context.Context) error
	// GetChannels returns the tenant's channel config, or ErrNotFound if unset.
	GetChannels(ctx context.Context, organizationID string) (ChannelConfig, error)
	// SetChannels upserts the tenant's channel config.
	SetChannels(ctx context.Context, config ChannelConfig) error
	// DeleteForOrganization removes the organization's channel config and
	// returns the number of documents deleted. Called only by the platform
	// GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// Dispatcher delivers a notification to an organization's configured chat
// channels. Implementations must treat delivery as best-effort and must not
// mutate the notification.
type Dispatcher interface {
	Dispatch(ctx context.Context, config ChannelConfig, notification Notification) error
}

// EmailDispatcher delivers a notification to its user's email address.
type EmailDispatcher interface {
	DispatchEmail(ctx context.Context, notification Notification) error
}

// SMSDispatcher resolves the recipient and submits a compact text message to
// the configured SMS provider. An unconfigured provider should be omitted.
type SMSDispatcher interface {
	DispatchSMS(ctx context.Context, notification Notification) error
}

type DeliveryStore interface {
	EnsureIndexes(ctx context.Context) error
	CreateDelivery(ctx context.Context, delivery Delivery) error
	UpdateDelivery(ctx context.Context, delivery Delivery) error
	GetDelivery(ctx context.Context, id string) (Delivery, error)
	ListDeliveries(ctx context.Context) ([]Delivery, error)
	ListDueDeliveries(ctx context.Context, now time.Time) ([]Delivery, error)
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}
