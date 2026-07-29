// Package audit records and retrieves immutable organization audit events.
package audit

import (
	"context"
	"time"

	"launchpad/pkg/security"
)

// Event is an immutable audit record.
type Event struct {
	ID             string  `bson:"_id"                      json:"id"`
	OrganizationID *string `bson:"organizationId,omitempty" json:"organizationId,omitempty"`
	ActorUserID    string  `bson:"actorUserId"              json:"actorUserId"`
	ActorType      string  `bson:"actorType"                json:"actorType"`
	Action         string  `bson:"action"                   json:"action"`
	ResourceType   string  `bson:"resourceType"             json:"resourceType"`
	ResourceID     string  `bson:"resourceId"               json:"resourceId"`
	IP             string  `bson:"ip,omitempty"             json:"ip,omitempty"`
	UserAgent      string  `bson:"userAgent,omitempty"      json:"userAgent,omitempty"`
	RequestID      string  `bson:"requestId,omitempty"      json:"requestId,omitempty"`
	Result         string  `bson:"result,omitempty"         json:"result,omitempty"`
	FailureReason  string  `bson:"failureReason,omitempty"  json:"failureReason,omitempty"`
	Before         any     `bson:"before,omitempty"          json:"before,omitempty"`
	After          any     `bson:"after,omitempty"           json:"after,omitempty"`
	// ImpersonatorUserID and ImpersonationSessionID are set only when the
	// request ran under a platform support impersonation token (PRD 5.2.2):
	// ActorUserID stays the support agent, and these fields link the event to
	// the support session that authorized the access.
	ImpersonatorUserID     string         `bson:"impersonatorUserId,omitempty"     json:"impersonatorUserId,omitempty"`
	ImpersonationSessionID string         `bson:"impersonationSessionId,omitempty" json:"impersonationSessionId,omitempty"`
	Metadata               map[string]any `bson:"metadata,omitempty"               json:"metadata,omitempty"`
	CreatedAt              time.Time      `bson:"createdAt"                        json:"createdAt"`
}

// Audit result values recorded on every event.
const (
	ResultSuccess = "success"
	ResultFailure = "failure"
)

// Repository persists audit events.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Write(ctx context.Context, event Event) error
	ListByOrganization(ctx context.Context, organizationID string, limit int64) ([]Event, error)
	ListAll(ctx context.Context, limit int64) ([]Event, error)
	// CountByOrganization returns the number of audit events of an
	// organization. Used by the GDPR data export summary (PRD 7.4).
	CountByOrganization(ctx context.Context, organizationID string) (int64, error)
	// DeleteForOrganization removes every audit event of the organization
	// and returns the number deleted. Called only by the platform GDPR
	// tenant purge; the caller writes a platform-level tombstone event
	// afterwards so the purge itself stays audited.
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// Service exposes audit use cases.
type Service struct {
	repo Repository
}

// NewService constructs an audit Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Record writes a successful audit event, capturing the request metadata
// (IP, user-agent, request id) that Middleware stored on the context.
func (s *Service) Record(
	ctx context.Context,
	organizationID *string,
	actorUserID, action, resourceType, resourceID string,
	metadata map[string]any,
) error {
	return s.RecordResult(
		ctx, organizationID, actorUserID, action, resourceType, resourceID, ResultSuccess, "", metadata,
	)
}

// RecordResult writes an audit event with an explicit result; an empty result
// defaults to ResultSuccess. The pattern for failure paths: where a handler
// or service already logs an error for an audited action, call RecordResult
// with ResultFailure and a short machine-readable failureReason before
// writing the error response, treating the audit write as best-effort so a
// broken audit store never changes the client-facing error. See
// auth.Service.Login and platform.Handler.HandleSuspendOrganization for
// working examples.
func (s *Service) RecordResult(
	ctx context.Context,
	organizationID *string,
	actorUserID, action, resourceType, resourceID, result, failureReason string,
	metadata map[string]any,
) error {
	if result == "" {
		result = ResultSuccess
	}

	rc := requestContextFrom(ctx)
	event := Event{
		OrganizationID: organizationID,
		ActorUserID:    actorUserID,
		ActorType:      actorType(ctx, organizationID, actorUserID),
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		IP:             rc.IP,
		UserAgent:      rc.UserAgent,
		RequestID:      rc.RequestID,
		Result:         result,
		FailureReason:  failureReason,
		Metadata:       metadata,
	}
	if metadata != nil {
		before, hasBefore := metadata["before"]
		after, hasAfter := metadata["after"]
		event.Before = before
		if hasAfter {
			event.After = after
		} else if !hasBefore {
			// Existing mutation call sites historically supplied the resulting
			// fields directly as metadata. Preserve that API while exposing the
			// same structured payload through the first-class After field.
			event.After = metadata
		}
	}

	// Requests made with a platform support impersonation token carry the
	// support session on the principal; record it so every action taken
	// during the session is traceable back to it (PRD 5.2.2).
	if principal, ok := security.PrincipalFromContext(ctx); ok && principal.Impersonator {
		event.ImpersonatorUserID = principal.UserID
		event.ImpersonationSessionID = principal.ImpersonationSessionID
	}

	return s.repo.Write(ctx, event)
}

func actorType(ctx context.Context, organizationID *string, actorUserID string) string {
	if actorUserID == "" {
		return "system"
	}
	if principal, ok := security.PrincipalFromContext(ctx); ok {
		if principal.Impersonator {
			return "impersonator"
		}
		if principal.OrganizationID == "" {
			return "platform_staff"
		}
	}
	if organizationID == nil {
		return "platform_staff"
	}
	return "organization_user"
}

// List returns organization audit events.
func (s *Service) List(ctx context.Context, organizationID string, limit int64) ([]Event, error) {
	return s.repo.ListByOrganization(ctx, organizationID, limit)
}

// ListAll returns recent audit events across all organizations, including
// platform-level events that carry no organization. Platform staff only.
func (s *Service) ListAll(ctx context.Context, limit int64) ([]Event, error) {
	return s.repo.ListAll(ctx, limit)
}
