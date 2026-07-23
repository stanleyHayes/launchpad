// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/platform"
)

const (
	fieldUserID       = "userId"
	staffStatusActive = "active"
)

var _ platform.Repository = (*Store)(nil)

// Store is the MongoDB platform staff repository.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("platform_staff")}
}

// EnsureIndexes creates platform staff indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldUserID, Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return fmt.Errorf("ensure platform staff indexes: %w", err)
	}

	return nil
}

// GetByUserID loads an active staff record by user id.
func (s *Store) GetByUserID(ctx context.Context, userID string) (platform.Staff, error) {
	var staff platform.Staff

	err := s.col.FindOne(ctx, bson.M{fieldUserID: userID, "status": staffStatusActive}).Decode(&staff)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return platform.Staff{}, platform.ErrNotFound
	}

	if err != nil {
		return platform.Staff{}, fmt.Errorf("find platform staff: %w", err)
	}

	return staff, nil
}

// Create inserts a staff record.
func (s *Store) Create(ctx context.Context, staff platform.Staff) error {
	_, err := s.col.InsertOne(ctx, staff)
	if err != nil {
		return fmt.Errorf("insert platform staff: %w", err)
	}

	return nil
}
