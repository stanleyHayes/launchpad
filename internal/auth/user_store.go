package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const userEmailField = "email"

// UserRepository persists users.
type UserRepository interface {
	Create(ctx context.Context, user User) error
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
}

// UserStore is the MongoDB user repository.
type UserStore struct {
	col *mongo.Collection
}

// NewUserStore constructs a UserStore.
func NewUserStore(db *mongo.Database) *UserStore {
	return &UserStore{col: db.Collection("users")}
}

// EnsureIndexes creates required user indexes.
func (s *UserStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: userEmailField, Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return fmt.Errorf("ensure user indexes: %w", err)
	}

	return nil
}

// Create inserts a user document.
func (s *UserStore) Create(ctx context.Context, user User) error {
	_, err := s.col.InsertOne(ctx, user)
	if mongo.IsDuplicateKeyError(err) {
		return ErrEmailTaken
	}

	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

// GetByEmail loads a user by email.
func (s *UserStore) GetByEmail(ctx context.Context, email string) (User, error) {
	var user User

	err := s.col.FindOne(ctx, bson.M{userEmailField: strings.ToLower(email)}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, ErrInvalidCredentials
	}

	if err != nil {
		return User{}, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}

// GetByID loads a user by identifier.
func (s *UserStore) GetByID(ctx context.Context, id string) (User, error) {
	var user User

	err := s.col.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, ErrInvalidCredentials
	}

	if err != nil {
		return User{}, fmt.Errorf("find user by id: %w", err)
	}

	return user, nil
}
