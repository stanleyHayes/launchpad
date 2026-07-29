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
	defaultListLimit    int64 = 50
	maxListLimit        int64 = 100
	fieldCreatedAt            = "createdAt"
	fieldOrganizationID       = "organizationId"
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
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: "actorUserId", Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		// Backs the platform-wide listing, which sorts by createdAt unfiltered.
		{Keys: bson.D{{Key: fieldCreatedAt, Value: -1}}},
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
	return s.list(ctx, bson.M{fieldOrganizationID: organizationID}, limit)
}

// ListAll returns recent audit events across all tenants, including
// platform-level events that carry no organization.
func (s *Store) ListAll(ctx context.Context, limit int64) ([]audit.Event, error) {
	return s.list(ctx, bson.M{}, limit)
}

// CountByOrganization returns the number of audit events of a tenant. Used
// by the GDPR data export summary (PRD 7.4).
func (s *Store) CountByOrganization(ctx context.Context, organizationID string) (int64, error) {
	count, err := s.col.CountDocuments(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("count audit events: %w", err)
	}

	return count, nil
}

// DeleteForOrganization removes every audit event of the organization and
// returns the number deleted. It serves only the platform GDPR tenant purge
// (PRD 7.4); the caller writes a platform-level tombstone event afterwards
// so the purge itself stays audited.
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete audit events for organization: %w", err)
	}

	return res.DeletedCount, nil
}

func (s *Store) list(ctx context.Context, filter bson.M, limit int64) ([]audit.Event, error) {
	switch {
	case limit <= 0:
		limit = defaultListLimit
	case limit > maxListLimit:
		limit = maxListLimit
	}

	opts := options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}).SetLimit(limit)

	cursor, err := s.col.Find(ctx, filter, opts)
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
