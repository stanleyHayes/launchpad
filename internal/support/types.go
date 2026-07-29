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

// Blocker categories reported by employees (PRD §5.4.4).
const (
	categoryHR      = "hr"
	categoryIT      = "it"
	categoryManager = "manager"
	categoryOther   = "other"
)

// Ticket is a customer support request. Category classifies the ticket using
// the blocker taxonomy (hr/it/manager/other); it may be empty.
type Ticket struct {
	ID              string          `bson:"_id"                      json:"id"`
	OrganizationID  string          `bson:"organizationId"           json:"organizationId"`
	CreatedByUserID string          `bson:"createdByUserId"          json:"createdByUserId"`
	Subject         string          `bson:"subject"                  json:"subject"`
	Body            string          `bson:"body"                     json:"body"`
	Priority        string          `bson:"priority"                 json:"priority"`
	Category        string          `bson:"category,omitempty"       json:"category,omitempty"`
	Status          string          `bson:"status"                   json:"status"`
	AssigneeUserID  string          `bson:"assigneeUserId,omitempty" json:"assigneeUserId,omitempty"`
	SLADueAt        time.Time       `bson:"slaDueAt" json:"slaDueAt"`
	FirstResponseAt *time.Time      `bson:"firstResponseAt,omitempty" json:"firstResponseAt,omitempty"`
	ResolvedAt      *time.Time      `bson:"resolvedAt,omitempty" json:"resolvedAt,omitempty"`
	EscalationCount int             `bson:"escalationCount,omitempty" json:"escalationCount"`
	Tags            []string        `bson:"tags,omitempty" json:"tags,omitempty"`
	Messages        []TicketMessage `bson:"messages,omitempty" json:"messages,omitempty"`
	CreatedAt       time.Time       `bson:"createdAt"                json:"createdAt"`
	UpdatedAt       time.Time       `bson:"updatedAt"                json:"updatedAt"`
}

type TicketMessage struct {
	ID           string    `bson:"id" json:"id"`
	AuthorUserID string    `bson:"authorUserId" json:"authorUserId"`
	Body         string    `bson:"body" json:"body"`
	Internal     bool      `bson:"internal" json:"internal"`
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
}

type SupportSummary struct {
	Open                        int     `json:"open"`
	Overdue                     int     `json:"overdue"`
	Urgent                      int     `json:"urgent"`
	Unassigned                  int     `json:"unassigned"`
	AverageFirstResponseMinutes float64 `json:"averageFirstResponseMinutes"`
}

// CreateTicketInput creates a support ticket.
type CreateTicketInput struct {
	OrganizationID  string
	CreatedByUserID string
	Subject         string
	Body            string
	Priority        string
	Category        string
}

// Blocker is an employee-reported blocker, backed by a support ticket so it
// also appears in the organization's support queue.
type Blocker struct {
	ID               string    `bson:"_id"                        json:"id"`
	OrganizationID   string    `bson:"organizationId"             json:"organizationId"`
	EmployeeID       string    `bson:"employeeId"                 json:"employeeId"`
	ReportedByUserID string    `bson:"reportedByUserId"           json:"reportedByUserId"`
	StepAssignmentID string    `bson:"stepAssignmentId,omitempty" json:"stepAssignmentId,omitempty"`
	Category         string    `bson:"category"                   json:"category"`
	Message          string    `bson:"message"                    json:"message"`
	TicketID         string    `bson:"ticketId"                   json:"ticketId"`
	CreatedAt        time.Time `bson:"createdAt"                  json:"createdAt"`
}

// ReportBlockerInput reports a blocker for an employee. EmployeeName and
// StepTitle only shape the backing ticket's subject/body.
type ReportBlockerInput struct {
	OrganizationID   string
	EmployeeID       string
	ReportedByUserID string
	EmployeeName     string
	StepAssignmentID string
	StepTitle        string
	Category         string
	Message          string
}

// UpdateTicketStatusInput updates ticket workflow state.
type UpdateTicketStatusInput struct {
	TicketID       string
	Status         string
	AssigneeUserID *string
}
