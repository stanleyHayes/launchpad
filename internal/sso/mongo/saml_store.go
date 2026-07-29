package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/sso"
)

var _ sso.SAMLConfigStore = (*SAMLStore)(nil)

type SAMLStore struct {
	col *drivermongo.Collection
}

func NewSAMLStore(db *drivermongo.Database) *SAMLStore {
	return &SAMLStore{col: db.Collection("saml_configs")}
}

// EnsureIndexes is a no-op because _id is the organization id.
func (s *SAMLStore) EnsureIndexes(context.Context) error { return nil }

func (s *SAMLStore) GetSAMLByOrganization(ctx context.Context, organizationID string) (sso.SAMLConfig, error) {
	var config sso.SAMLConfig
	err := s.col.FindOne(ctx, bson.M{fieldID: organizationID}).Decode(&config)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return sso.SAMLConfig{}, sso.ErrNotConfigured
	}
	if err != nil {
		return sso.SAMLConfig{}, fmt.Errorf("find saml config: %w", err)
	}

	return config, nil
}

func (s *SAMLStore) SetSAMLConfig(ctx context.Context, config sso.SAMLConfig) error {
	_, err := s.col.ReplaceOne(ctx, bson.M{fieldID: config.OrganizationID}, config, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("upsert saml config: %w", err)
	}

	return nil
}

func (s *SAMLStore) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	result, err := s.col.DeleteMany(ctx, bson.M{fieldID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete saml config: %w", err)
	}

	return result.DeletedCount, nil
}
