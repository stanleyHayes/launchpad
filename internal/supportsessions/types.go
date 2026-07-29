// Package supportsessions implements the PRD 5.2.2 audited support
// impersonation mode: platform staff open a time-boxed, reason-bound support
// session against a tenant organization and receive a short-lived read-only
// impersonation token. Every session start/end is audited, organization
// owners are notified in-app, and any request made with the token records
// the impersonation context on its audit events.
package supportsessions

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates the support session does not exist.
	ErrNotFound = errors.New("support session not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid support session input")
	// ErrSessionEnded indicates the support session was already ended.
	ErrSessionEnded = errors.New("support session already ended")
)

const (
	// MinReasonLength is the minimum length of the mandatory session reason.
	MinReasonLength = 10
	// MaxDurationMinutes caps a support session at two hours.
	MaxDurationMinutes = 120
	// TokenTTL is the lifetime of the impersonation access token. It is much
	// shorter than the session so a leaked token expires quickly; the session
	// itself bounds how long new tokens may be issued for it.
	TokenTTL = 15 * time.Minute

	// EndReasonEndedByAgent is recorded when platform staff end a session early.
	EndReasonEndedByAgent = "ended_by_agent"
	// EndReasonExpired marks sessions whose expiry lapsed without an explicit end.
	EndReasonExpired = "expired"

	maxEndReasonLength = 200
)

// Session is one audited support impersonation session against a tenant.
type Session struct {
	ID             string     `bson:"_id"                 json:"id"`
	OrganizationID string     `bson:"organizationId"      json:"organizationId"`
	AgentUserID    string     `bson:"agentUserId"         json:"agentUserId"`
	Reason         string     `bson:"reason"              json:"reason"`
	CreatedAt      time.Time  `bson:"createdAt"           json:"createdAt"`
	ExpiresAt      time.Time  `bson:"expiresAt"           json:"expiresAt"`
	EndedAt        *time.Time `bson:"endedAt,omitempty"   json:"endedAt,omitempty"`
	EndReason      string     `bson:"endReason,omitempty" json:"endReason,omitempty"`
}

// CreateInput starts a support session. DurationMinutes of zero selects the
// default (the two-hour cap); anything above the cap is rejected.
type CreateInput struct {
	OrganizationID  string
	AgentUserID     string
	AgentEmail      string
	Reason          string
	DurationMinutes int
}
