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

// Notification is an in-app notification.
type Notification struct {
	ID             string     `bson:"_id"              json:"id"`
	OrganizationID string     `bson:"organizationId"   json:"organizationId"`
	UserID         string     `bson:"userId"           json:"userId"`
	Title          string     `bson:"title"            json:"title"`
	Body           string     `bson:"body"             json:"body"`
	ReadAt         *time.Time `bson:"readAt,omitempty" json:"readAt,omitempty"`
	CreatedAt      time.Time  `bson:"createdAt"        json:"createdAt"`
}

// CreateInput creates a notification.
type CreateInput struct {
	UserID string
	Title  string
	Body   string
}
