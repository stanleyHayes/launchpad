// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/leads"
)

var _ leads.Repository = (*Store)(nil)

const fieldCreatedAt = "createdAt"

// Store is the MongoDB leads repository.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("leads")}
}

// EnsureIndexes creates lead indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: "email", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure lead indexes: %w", err)
	}

	return nil
}

// Create inserts a lead.
func (s *Store) Create(ctx context.Context, lead leads.Lead) error {
	_, err := s.col.InsertOne(ctx, lead)
	if err != nil {
		return fmt.Errorf("insert lead: %w", err)
	}

	return nil
}

// List returns leads ordered by newest first.
func (s *Store) List(ctx context.Context) ([]leads.Lead, error) {
	cursor, err := s.col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("find leads: %w", err)
	}

	items := make([]leads.Lead, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode leads: %w", decodeErr),
			fmt.Errorf("close leads cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode leads: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close leads cursor: %w", closeErr)
	}

	return items, nil
}

// Count returns the total number of leads.
func (s *Store) Count(ctx context.Context) (int64, error) {
	count, err := s.col.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("count leads: %w", err)
	}

	return count, nil
}
