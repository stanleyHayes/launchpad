// Package mongo is the MongoDB persistence adapter for SSO configuration.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/sso"
	"launchpad/pkg/security"
)

const fieldID = "_id"

var _ sso.ConfigStore = (*Store)(nil)

// Store persists per-organization OIDC configuration, keyed by organization id.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("sso_configs")}
}

// EnsureIndexes is a no-op: the collection is keyed by _id (organization id).
func (s *Store) EnsureIndexes(context.Context) error { return nil }

// GetByOrganization loads a tenant's OIDC config.
func (s *Store) GetByOrganization(ctx context.Context, organizationID string) (sso.Config, error) {
	var config sso.Config

	err := s.col.FindOne(ctx, bson.M{fieldID: organizationID}).Decode(&config)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return sso.Config{}, sso.ErrNotConfigured
	}

	if err != nil {
		return sso.Config{}, fmt.Errorf("find sso config: %w", err)
	}

	config.ClientSecret, err = security.DecryptSecret(config.ClientSecret)
	if err != nil {
		return sso.Config{}, fmt.Errorf("decrypt sso client secret: %w", err)
	}

	return config, nil
}

// SetConfig upserts a tenant's OIDC config. The client secret is encrypted at
// rest when ENCRYPTION_KEY is configured.
func (s *Store) SetConfig(ctx context.Context, config sso.Config) error {
	var err error

	config.ClientSecret, err = security.EncryptSecret(config.ClientSecret)
	if err != nil {
		return fmt.Errorf("encrypt sso client secret: %w", err)
	}

	_, err = s.col.ReplaceOne(
		ctx,
		bson.M{fieldID: config.OrganizationID},
		config,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("upsert sso config: %w", err)
	}

	return nil
}

// DeleteForOrganization removes the organization's OIDC config (keyed by
// organization id) and returns the number of documents deleted. It serves
// only the platform GDPR tenant purge (PRD 7.4).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete sso config for organization: %w", err)
	}

	return res.DeletedCount, nil
}
