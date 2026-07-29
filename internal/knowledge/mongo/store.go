// Package mongo is the MongoDB persistence adapter for the knowledge domain.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/knowledge"
)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldStatus         = "status"
	fieldCreatedAt      = "createdAt"
)

var _ knowledge.Repository = (*Store)(nil)

// Store is the MongoDB knowledge document repository.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("knowledge_documents")}
}

// EnsureIndexes creates knowledge document indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldStatus, Value: 1}}},
		{Keys: bson.D{{Key: "reviewDate", Value: 1}, {Key: "staleNotifiedAt", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure knowledge document indexes: %w", err)
	}

	return nil
}

// ListStale returns documents whose explicit review date or retention window
// has elapsed. Archived and already-notified documents are excluded.
func (s *Store) ListStale(ctx context.Context, now time.Time) ([]knowledge.Document, error) {
	filter := bson.M{
		fieldStatus:       bson.M{"$ne": knowledge.StatusArchived},
		"staleNotifiedAt": bson.M{"$exists": false},
		"$or": bson.A{
			bson.M{"reviewDate": bson.M{"$lte": now}},
			bson.M{
				"retentionDays": bson.M{"$gt": 0},
				"$expr": bson.M{"$lte": bson.A{
					bson.M{"$ifNull": bson.A{"$lastSyncedAt", "$updatedAt"}},
					bson.M{"$dateSubtract": bson.M{
						"startDate": now, "unit": "day", "amount": "$retentionDays",
					}},
				}},
			},
		},
	}
	cursor, err := s.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("find stale knowledge documents: %w", err)
	}
	defer cursor.Close(ctx)
	var items []knowledge.Document
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode stale knowledge documents: %w", err)
	}
	return items, nil
}

// Create inserts a knowledge document.
func (s *Store) Create(ctx context.Context, doc knowledge.Document) error {
	_, err := s.col.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("insert knowledge document: %w", err)
	}

	return nil
}

// GetByID loads a document scoped to a tenant.
func (s *Store) GetByID(
	ctx context.Context,
	organizationID, documentID string,
) (knowledge.Document, error) {
	var doc knowledge.Document

	err := s.col.FindOne(ctx, bson.M{
		fieldID:             documentID,
		fieldOrganizationID: organizationID,
	}).Decode(&doc)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return knowledge.Document{}, knowledge.ErrNotFound
	}

	if err != nil {
		return knowledge.Document{}, fmt.Errorf("find knowledge document: %w", err)
	}

	return doc, nil
}

// List returns a tenant's documents newest first.
func (s *Store) List(ctx context.Context, organizationID string) ([]knowledge.Document, error) {
	cursor, err := s.col.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find knowledge documents: %w", err)
	}

	items := make([]knowledge.Document, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode knowledge documents: %w", decodeErr),
			fmt.Errorf("close knowledge documents cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode knowledge documents: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close knowledge documents cursor: %w", closeErr)
	}

	return items, nil
}

// Update replaces a document scoped to its tenant.
func (s *Store) Update(ctx context.Context, doc knowledge.Document) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{
		fieldID:             doc.ID,
		fieldOrganizationID: doc.OrganizationID,
	}, doc)
	if err != nil {
		return fmt.Errorf("replace knowledge document: %w", err)
	}

	if res.MatchedCount == 0 {
		return knowledge.ErrNotFound
	}

	return nil
}

// DeleteForOrganization removes every knowledge document of the organization
// and returns the number deleted. It serves only the platform GDPR tenant
// purge (PRD 7.4).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete knowledge documents for organization: %w", err)
	}

	return res.DeletedCount, nil
}
