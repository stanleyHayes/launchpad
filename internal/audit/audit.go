// Package audit records and retrieves immutable organization audit events.
package audit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	defaultListLimit int64 = 50
	maxListLimit     int64 = 100
	fieldCreatedAt         = "createdAt"
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

// Store persists audit events.
type Store struct {
	col *mongo.Collection
}

// NewStore constructs an audit Store.
func NewStore(db *mongo.Database) *Store {
	return &Store{col: db.Collection("audit_events")}
}

// EnsureIndexes creates audit indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "organizationId", Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: "actorUserId", Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure audit indexes: %w", err)
	}

	return nil
}

// Write inserts an audit event.
func (s *Store) Write(ctx context.Context, event Event) error {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}

	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	_, err := s.col.InsertOne(ctx, event)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}

	return nil
}

// ListByOrganization returns recent audit events for a tenant.
func (s *Store) ListByOrganization(ctx context.Context, organizationID string, limit int64) ([]Event, error) {
	if limit <= 0 || limit > maxListLimit {
		limit = defaultListLimit
	}

	opts := options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}).SetLimit(limit)

	cursor, err := s.col.Find(ctx, bson.M{"organizationId": organizationID}, opts)
	if err != nil {
		return nil, fmt.Errorf("find audit events: %w", err)
	}

	events := make([]Event, 0)
	decodeErr := cursor.All(ctx, &events)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode audit events: %w", decodeErr),
			fmt.Errorf("close audit cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode audit events: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close audit cursor: %w", closeErr)
	}

	return events, nil
}

// Service exposes audit use cases.
type Service struct {
	store *Store
}

// NewService constructs an audit Service.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// Record writes an audit event.
func (s *Service) Record(
	ctx context.Context,
	organizationID *string,
	actorUserID, action, resourceType, resourceID string,
	metadata map[string]any,
) error {
	return s.store.Write(ctx, Event{
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
	return s.store.ListByOrganization(ctx, organizationID, limit)
}
