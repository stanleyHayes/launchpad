// Package audit records and retrieves immutable organization audit events.
package audit

import (
	"context"
	"time"
)

// Event is an immutable audit record.
type Event struct {
	ID             string         `bson:"_id"                      json:"id"`
	OrganizationID *string        `bson:"organizationId,omitempty" json:"organizationId,omitempty"`
	ActorUserID    string         `bson:"actorUserId"              json:"actorUserId"`
	Action         string         `bson:"action"                   json:"action"`
	ResourceType   string         `bson:"resourceType"             json:"resourceType"`
	ResourceID     string         `bson:"resourceId"               json:"resourceId"`
	Metadata       map[string]any `bson:"metadata,omitempty"       json:"metadata,omitempty"`
	CreatedAt      time.Time      `bson:"createdAt"                json:"createdAt"`
}

// Repository persists audit events.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Write(ctx context.Context, event Event) error
	ListByOrganization(ctx context.Context, organizationID string, limit int64) ([]Event, error)
}

// Service exposes audit use cases.
type Service struct {
	repo Repository
}

// NewService constructs an audit Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Record writes an audit event.
func (s *Service) Record(
	ctx context.Context,
	organizationID *string,
	actorUserID, action, resourceType, resourceID string,
	metadata map[string]any,
) error {
	return s.repo.Write(ctx, Event{
		OrganizationID: organizationID,
		ActorUserID:    actorUserID,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		Metadata:       metadata,
	})
}

// List returns organization audit events.
func (s *Service) List(ctx context.Context, organizationID string, limit int64) ([]Event, error) {
	return s.repo.ListByOrganization(ctx, organizationID, limit)
}
