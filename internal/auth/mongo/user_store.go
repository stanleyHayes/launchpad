// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/auth"
)

var _ auth.UserRepository = (*UserStore)(nil)

const userEmailField = "email"

// UserStore is the MongoDB user repository.
type UserStore struct {
	col *drivermongo.Collection
}

// NewUserStore constructs a UserStore.
func NewUserStore(db *drivermongo.Database) *UserStore {
	return &UserStore{col: db.Collection("users")}
}

// EnsureIndexes creates required user indexes.
func (s *UserStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: userEmailField, Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return fmt.Errorf("ensure user indexes: %w", err)
	}

	return nil
}

// Create inserts a user document.
func (s *UserStore) Create(ctx context.Context, user auth.User) error {
	_, err := s.col.InsertOne(ctx, user)
	if drivermongo.IsDuplicateKeyError(err) {
		return auth.ErrEmailTaken
	}

	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

// GetByEmail loads a user by email.
func (s *UserStore) GetByEmail(ctx context.Context, email string) (auth.User, error) {
	var user auth.User

	err := s.col.FindOne(ctx, bson.M{userEmailField: strings.ToLower(email)}).Decode(&user)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return auth.User{}, auth.ErrInvalidCredentials
	}

	if err != nil {
		return auth.User{}, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}

// GetByID loads a user by identifier.
func (s *UserStore) GetByID(ctx context.Context, id string) (auth.User, error) {
	var user auth.User

	err := s.col.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return auth.User{}, auth.ErrInvalidCredentials
	}

	if err != nil {
		return auth.User{}, fmt.Errorf("find user by id: %w", err)
	}

	return user, nil
}
