package support

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates the ticket does not exist.
	ErrNotFound = errors.New("support ticket not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid support ticket input")
)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldStatus         = "status"
	fieldCreatedAt      = "createdAt"

	priorityLow    = "low"
	priorityNormal = "normal"
	priorityHigh   = "high"
	priorityUrgent = "urgent"

	statusOpen       = "open"
	statusInProgress = "in_progress"
	statusWaiting    = "waiting"
	statusResolved   = "resolved"
	statusClosed     = "closed"
)

// Ticket is a customer support request.
type Ticket struct {
	ID              string    `bson:"_id"                      json:"id"`
	OrganizationID  string    `bson:"organizationId"           json:"organizationId"`
	CreatedByUserID string    `bson:"createdByUserId"          json:"createdByUserId"`
	Subject         string    `bson:"subject"                  json:"subject"`
	Body            string    `bson:"body"                     json:"body"`
	Priority        string    `bson:"priority"                 json:"priority"`
	Status          string    `bson:"status"                   json:"status"`
	AssigneeUserID  string    `bson:"assigneeUserId,omitempty" json:"assigneeUserId,omitempty"`
	CreatedAt       time.Time `bson:"createdAt"                json:"createdAt"`
	UpdatedAt       time.Time `bson:"updatedAt"                json:"updatedAt"`
}

// CreateTicketInput creates a support ticket.
type CreateTicketInput struct {
	OrganizationID  string
	CreatedByUserID string
	Subject         string
	Body            string
	Priority        string
}

// UpdateTicketStatusInput updates ticket workflow state.
type UpdateTicketStatusInput struct {
	TicketID       string
	Status         string
	AssigneeUserID *string
}
