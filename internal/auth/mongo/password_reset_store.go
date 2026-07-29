package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/auth"
)

var _ auth.PasswordResetStore = (*PasswordResetStore)(nil)

const (
	passwordResetTokenHashField = "tokenHash"
	passwordResetExpiresField   = "expiresAt"
)

// PasswordResetStore persists single-use, expiring password-reset tokens.
type PasswordResetStore struct {
	col *drivermongo.Collection
}

// NewPasswordResetStore constructs a PasswordResetStore.
func NewPasswordResetStore(db *drivermongo.Database) *PasswordResetStore {
	return &PasswordResetStore{col: db.Collection("auth_password_resets")}
}

// EnsureIndexes creates a unique index on the token hash and a TTL index that
// purges expired resets.
func (s *PasswordResetStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: passwordResetTokenHashField, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: passwordResetExpiresField, Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure password reset indexes: %w", err)
	}

	return nil
}

// Save inserts a password reset.
func (s *PasswordResetStore) Save(ctx context.Context, reset auth.PasswordReset) error {
	if _, err := s.col.InsertOne(ctx, reset); err != nil {
		return fmt.Errorf("insert password reset: %w", err)
	}

	return nil
}

// Consume atomically finds and deletes a non-expired reset by token hash, so
// concurrent redemptions of the same token cannot both succeed. The expiry
// filter guards the window before the TTL reaper removes an expired document.
func (s *PasswordResetStore) Consume(ctx context.Context, tokenHash string) (auth.PasswordReset, error) {
	var reset auth.PasswordReset

	err := s.col.FindOneAndDelete(ctx, bson.M{
		passwordResetTokenHashField: tokenHash,
		passwordResetExpiresField:   bson.M{"$gt": time.Now().UTC()},
	}).Decode(&reset)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return auth.PasswordReset{}, auth.ErrPasswordResetInvalid
	}

	if err != nil {
		return auth.PasswordReset{}, fmt.Errorf("consume password reset: %w", err)
	}

	return reset, nil
}

// DeleteForUsers removes every password reset of the given users and returns
// the number deleted. Reset tokens carry no organization scope, so the
// platform GDPR tenant purge (PRD 7.4) deletes them by the user ids
// collected from the organization's memberships.
func (s *PasswordResetStore) DeleteForUsers(ctx context.Context, userIDs []string) (int64, error) {
	if len(userIDs) == 0 {
		return 0, nil
	}

	res, err := s.col.DeleteMany(ctx, bson.M{"userId": bson.M{"$in": userIDs}})
	if err != nil {
		return 0, fmt.Errorf("delete password resets for users: %w", err)
	}

	return res.DeletedCount, nil
}
