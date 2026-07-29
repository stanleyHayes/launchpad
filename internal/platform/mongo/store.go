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

// GetByID loads a staff record by id, regardless of status, so platform
// administrators can review and reactivate deactivated accounts.
func (s *Store) GetByID(ctx context.Context, staffID string) (platform.Staff, error) {
	var staff platform.Staff

	err := s.col.FindOne(ctx, bson.M{"_id": staffID}).Decode(&staff)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return platform.Staff{}, platform.ErrNotFound
	}

	if err != nil {
		return platform.Staff{}, fmt.Errorf("find platform staff: %w", err)
	}

	return staff, nil
}

// List returns all staff records ordered by creation time.
func (s *Store) List(ctx context.Context) ([]platform.Staff, error) {
	cursor, err := s.col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list platform staff: %w", err)
	}

	defer func() { _ = cursor.Close(ctx) }()

	items := []platform.Staff{}
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode platform staff: %w", err)
	}

	return items, nil
}

// Create inserts a staff record.
func (s *Store) Create(ctx context.Context, staff platform.Staff) error {
	_, err := s.col.InsertOne(ctx, staff)
	if err != nil {
		return fmt.Errorf("insert platform staff: %w", err)
	}

	return nil
}

// Update replaces a staff record (role, status).
func (s *Store) Update(ctx context.Context, staff platform.Staff) error {
	result, err := s.col.ReplaceOne(ctx, bson.M{"_id": staff.ID}, staff)
	if err != nil {
		return fmt.Errorf("replace platform staff: %w", err)
	}

	if result.MatchedCount == 0 {
		return platform.ErrNotFound
	}

	return nil
}
