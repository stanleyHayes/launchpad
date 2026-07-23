package featureflags

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository persists feature flags and overrides.
type Repository interface {
	UpsertFlag(ctx context.Context, flag Flag) error
	GetFlag(ctx context.Context, key string) (Flag, error)
	ListFlags(ctx context.Context) ([]Flag, error)
	CreateFlag(ctx context.Context, flag Flag) error
	UpdateFlag(ctx context.Context, flag Flag) error
	UpsertOverride(ctx context.Context, override Override) error
	GetOverride(ctx context.Context, organizationID, key string) (Override, error)
	ListOverridesByOrganization(ctx context.Context, organizationID string) ([]Override, error)
}

// Store is the MongoDB feature flag repository.
type Store struct {
	flags     *mongo.Collection
	overrides *mongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *mongo.Database) *Store {
	return &Store{
		flags:     db.Collection("feature_flags"),
		overrides: db.Collection("feature_flag_overrides"),
	}
}

// EnsureIndexes creates feature flag indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.overrides.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldKey, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure feature flag override indexes: %w", err)
	}

	return nil
}

// UpsertFlag inserts or replaces a flag by key.
func (s *Store) UpsertFlag(ctx context.Context, flag Flag) error {
	opts := options.Replace().SetUpsert(true)

	_, err := s.flags.ReplaceOne(ctx, bson.M{fieldID: flag.Key}, flag, opts)
	if err != nil {
		return fmt.Errorf("upsert feature flag: %w", err)
	}

	return nil
}

// CreateFlag inserts a new flag.
func (s *Store) CreateFlag(ctx context.Context, flag Flag) error {
	_, err := s.flags.InsertOne(ctx, flag)
	if mongo.IsDuplicateKeyError(err) {
		return ErrKeyTaken
	}

	if err != nil {
		return fmt.Errorf("insert feature flag: %w", err)
	}

	return nil
}

// GetFlag loads a flag by key.
func (s *Store) GetFlag(ctx context.Context, key string) (Flag, error) {
	var flag Flag

	err := s.flags.FindOne(ctx, bson.M{fieldID: key}).Decode(&flag)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Flag{}, ErrNotFound
	}

	if err != nil {
		return Flag{}, fmt.Errorf("find feature flag: %w", err)
	}

	return flag, nil
}

// ListFlags returns all global flags ordered by key.
func (s *Store) ListFlags(ctx context.Context) ([]Flag, error) {
	cursor, err := s.flags.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: fieldID, Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("find feature flags: %w", err)
	}

	items := make([]Flag, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode feature flags: %w", decodeErr),
			fmt.Errorf("close feature flags cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode feature flags: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close feature flags cursor: %w", closeErr)
	}

	return items, nil
}

// UpdateFlag replaces an existing flag.
func (s *Store) UpdateFlag(ctx context.Context, flag Flag) error {
	res, err := s.flags.ReplaceOne(ctx, bson.M{fieldID: flag.Key}, flag)
	if err != nil {
		return fmt.Errorf("replace feature flag: %w", err)
	}

	if res.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// UpsertOverride inserts or replaces a tenant override.
func (s *Store) UpsertOverride(ctx context.Context, override Override) error {
	filter := bson.M{
		fieldOrganizationID: override.OrganizationID,
		fieldKey:            override.Key,
	}

	var existing Override

	err := s.overrides.FindOne(ctx, filter).Decode(&existing)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return fmt.Errorf("find feature flag override: %w", err)
	}

	if errors.Is(err, mongo.ErrNoDocuments) {
		_, insertErr := s.overrides.InsertOne(ctx, override)
		if insertErr != nil {
			return fmt.Errorf("insert feature flag override: %w", insertErr)
		}

		return nil
	}

	override.ID = existing.ID

	res, replaceErr := s.overrides.ReplaceOne(ctx, bson.M{fieldID: existing.ID}, override)
	if replaceErr != nil {
		return fmt.Errorf("replace feature flag override: %w", replaceErr)
	}

	if res.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// GetOverride loads a tenant override.
func (s *Store) GetOverride(ctx context.Context, organizationID, key string) (Override, error) {
	var override Override

	err := s.overrides.FindOne(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldKey:            key,
	}).Decode(&override)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Override{}, ErrNotFound
	}

	if err != nil {
		return Override{}, fmt.Errorf("find feature flag override: %w", err)
	}

	return override, nil
}

// ListOverridesByOrganization returns overrides for a tenant.
func (s *Store) ListOverridesByOrganization(ctx context.Context, organizationID string) ([]Override, error) {
	cursor, err := s.overrides.Find(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return nil, fmt.Errorf("find feature flag overrides: %w", err)
	}

	items := make([]Override, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode feature flag overrides: %w", decodeErr),
			fmt.Errorf("close feature flag overrides cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode feature flag overrides: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close feature flag overrides cursor: %w", closeErr)
	}

	return items, nil
}
