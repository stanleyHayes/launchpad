// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/audit"
)

const (
	defaultListLimit int64 = 50
	maxListLimit     int64 = 100
	fieldCreatedAt         = "createdAt"
)

var _ audit.Repository = (*Store)(nil)

// Store persists audit events.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs an audit Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("audit_events")}
}

// EnsureIndexes creates audit indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: "organizationId", Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: "actorUserId", Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure audit indexes: %w", err)
	}

	return nil
}

// Write inserts an audit event.
func (s *Store) Write(ctx context.Context, event audit.Event) error {
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
func (s *Store) ListByOrganization(ctx context.Context, organizationID string, limit int64) ([]audit.Event, error) {
	if limit <= 0 || limit > maxListLimit {
		limit = defaultListLimit
	}

	opts := options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}).SetLimit(limit)

	cursor, err := s.col.Find(ctx, bson.M{"organizationId": organizationID}, opts)
	if err != nil {
		return nil, fmt.Errorf("find audit events: %w", err)
	}

	events := make([]audit.Event, 0)
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
