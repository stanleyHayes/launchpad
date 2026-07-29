// Package notifications manages tenant-scoped in-app notifications.
package notifications

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates a notification was not found.
	ErrNotFound = errors.New("notification not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid notification input")
)

// Notification types classify a notification so clients can badge it and
// route the click-through. TypeSystem is the fallback for notifications
// created without a type (and for documents that predate typing).
const (
	TypeAssignment       = "assignment"
	TypeApproval         = "approval"
	TypeDueSoon          = "due_soon"
	TypeOverdue          = "overdue"
	TypeJourneyCompleted = "journey_completed"
	TypeBlocker          = "blocker"
	TypeMeetingReminder  = "meeting_reminder"
	TypeSystem           = "system"
)

// Notification is an in-app notification.
type Notification struct {
	ID             string `bson:"_id"            json:"id"`
	OrganizationID string `bson:"organizationId" json:"organizationId"`
	UserID         string `bson:"userId"         json:"userId"`
	Type           string `bson:"type"           json:"type"`
	Title          string `bson:"title"          json:"title"`
	Body           string `bson:"body"           json:"body"`
	// Link is an app-relative deep path (e.g. /assignments/{id}) the client
	// navigates to when the notification is opened. Empty means no target.
	Link      string     `bson:"link,omitempty"   json:"link,omitempty"`
	ReadAt    *time.Time `bson:"readAt,omitempty" json:"readAt,omitempty"`
	CreatedAt time.Time  `bson:"createdAt"        json:"createdAt"`
}

// CreateInput creates a notification. Type defaults to TypeSystem when empty;
// Link must be an app-relative path (leading slash) when set.
type CreateInput struct {
	UserID string
	Type   string
	Title  string
	Body   string
	Link   string
}

// ChannelConfig holds an organization's outbound chat webhook destinations.
// A notification created for the tenant is also delivered to any configured
// channel (best-effort). Empty fields mean the channel is not configured.
type ChannelConfig struct {
	OrganizationID  string    `bson:"_id"                       json:"organizationId"`
	SlackWebhookURL string    `bson:"slackWebhookUrl,omitempty" json:"-"`
	TeamsWebhookURL string    `bson:"teamsWebhookUrl,omitempty" json:"-"`
	UpdatedAt       time.Time `bson:"updatedAt"                 json:"updatedAt"`
}

// ChannelStatus is the API-safe view of a tenant's chat channels: it reports
// whether each channel is configured without ever exposing the secret webhook
// URL (which is a bearer credential), mirroring how SSO/HRIS secrets are masked.
type ChannelStatus struct {
	OrganizationID  string    `json:"organizationId"`
	SlackConfigured bool      `json:"slackConfigured"`
	TeamsConfigured bool      `json:"teamsConfigured"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// ToChannelStatus builds the masked API view of a channel configuration.
func ToChannelStatus(config ChannelConfig) ChannelStatus {
	return ChannelStatus{
		OrganizationID:  config.OrganizationID,
		SlackConfigured: config.SlackWebhookURL != "",
		TeamsConfigured: config.TeamsWebhookURL != "",
		UpdatedAt:       config.UpdatedAt,
	}
}

// SetChannelsInput updates channel webhooks. A non-nil empty string clears a
// channel; a nil pointer leaves it unchanged.
type SetChannelsInput struct {
	SlackWebhookURL *string
	TeamsWebhookURL *string
}

const (
	DeliveryPending    = "pending"
	DeliveryDelivered  = "delivered"
	DeliveryRetrying   = "retrying"
	DeliveryDeadLetter = "dead_letter"
)

// Delivery records one outbound channel attempt without storing channel secrets.
type Delivery struct {
	ID             string     `bson:"_id" json:"id"`
	OrganizationID string     `bson:"organizationId" json:"organizationId"`
	NotificationID string     `bson:"notificationId" json:"notificationId"`
	UserID         string     `bson:"userId" json:"userId"`
	Channel        string     `bson:"channel" json:"channel"`
	Status         string     `bson:"status" json:"status"`
	Attempts       int        `bson:"attempts" json:"attempts"`
	LastError      string     `bson:"lastError,omitempty" json:"lastError,omitempty"`
	NextAttemptAt  *time.Time `bson:"nextAttemptAt,omitempty" json:"nextAttemptAt,omitempty"`
	CreatedAt      time.Time  `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time  `bson:"updatedAt" json:"updatedAt"`
}
