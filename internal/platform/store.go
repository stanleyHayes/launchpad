package platform

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository persists platform staff records.
type Repository interface {
	GetByUserID(ctx context.Context, userID string) (Staff, error)
	Create(ctx context.Context, staff Staff) error
}

// Store is the MongoDB platform staff repository.
type Store struct {
	col *mongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *mongo.Database) *Store {
	return &Store{col: db.Collection("platform_staff")}
}

// EnsureIndexes creates platform staff indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: fieldUserID, Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return fmt.Errorf("ensure platform staff indexes: %w", err)
	}

	return nil
}

// GetByUserID loads an active staff record by user id.
func (s *Store) GetByUserID(ctx context.Context, userID string) (Staff, error) {
	var staff Staff

	err := s.col.FindOne(ctx, bson.M{fieldUserID: userID, "status": staffStatusActive}).Decode(&staff)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Staff{}, ErrNotFound
	}

	if err != nil {
		return Staff{}, fmt.Errorf("find platform staff: %w", err)
	}

	return staff, nil
}

// Create inserts a staff record.
func (s *Store) Create(ctx context.Context, staff Staff) error {
	_, err := s.col.InsertOne(ctx, staff)
	if err != nil {
		return fmt.Errorf("insert platform staff: %w", err)
	}

	return nil
}
