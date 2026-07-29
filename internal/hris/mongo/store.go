// Package mongo is the MongoDB persistence adapter for the HRIS domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/hris"
	"launchpad/pkg/security"
)

const (
	fieldID      = "_id"
	fieldLastRun = "lastRun"
)

var _ hris.ConfigStore = (*ConfigStore)(nil)

// ConfigStore persists per-organization HRIS configuration.
type ConfigStore struct {
	col *drivermongo.Collection
}

// NewConfigStore constructs a ConfigStore.
func NewConfigStore(db *drivermongo.Database) *ConfigStore {
	return &ConfigStore{col: db.Collection("hris_configs")}
}

// EnsureIndexes is a no-op: the collection is keyed by _id (organization id).
func (s *ConfigStore) EnsureIndexes(context.Context) error { return nil }

// GetByOrganization loads a tenant's HRIS config.
func (s *ConfigStore) GetByOrganization(ctx context.Context, organizationID string) (hris.Config, error) {
	var config hris.Config

	err := s.col.FindOne(ctx, bson.M{fieldID: organizationID}).Decode(&config)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return hris.Config{}, hris.ErrNotConfigured
	}

	if err != nil {
		return hris.Config{}, fmt.Errorf("find hris config: %w", err)
	}

	config.APIToken, err = security.DecryptSecret(config.APIToken)
	if err != nil {
		return hris.Config{}, fmt.Errorf("decrypt hris api token: %w", err)
	}

	return config, nil
}

// SetConfig upserts a tenant's HRIS config. The API token is encrypted at
// rest when ENCRYPTION_KEY is configured.
func (s *ConfigStore) SetConfig(ctx context.Context, config hris.Config) error {
	var err error

	config.APIToken, err = security.EncryptSecret(config.APIToken)
	if err != nil {
		return fmt.Errorf("encrypt hris api token: %w", err)
	}

	_, err = s.col.ReplaceOne(
		ctx,
		bson.M{fieldID: config.OrganizationID},
		config,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("upsert hris config: %w", err)
	}

	return nil
}

var _ hris.Store = (*Store)(nil)

// Store persists the latest synced directory and run per organization.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("hris_state")}
}

// EnsureIndexes is a no-op: the collection is keyed by _id (organization id).
func (s *Store) EnsureIndexes(context.Context) error { return nil }

type stateDoc struct {
	OrganizationID string                `bson:"_id"`
	Entries        []hris.DirectoryEntry `bson:"entries"`
	LastRun        hris.SyncRun          `bson:"lastRun"`
}

// SaveResult replaces the tenant's directory snapshot and last run.
func (s *Store) SaveResult(
	ctx context.Context,
	organizationID string,
	entries []hris.DirectoryEntry,
	run hris.SyncRun,
) error {
	doc := stateDoc{OrganizationID: organizationID, Entries: entries, LastRun: run}

	_, err := s.col.ReplaceOne(ctx, bson.M{fieldID: organizationID}, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("save hris result: %w", err)
	}

	return nil
}

// SaveRun records a run without touching the stored directory snapshot.
func (s *Store) SaveRun(ctx context.Context, organizationID string, run hris.SyncRun) error {
	_, err := s.col.UpdateOne(
		ctx,
		bson.M{fieldID: organizationID},
		bson.M{"$set": bson.M{fieldLastRun: run}},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("save hris run: %w", err)
	}

	return nil
}

// GetState returns the tenant's directory and last run.
func (s *Store) GetState(ctx context.Context, organizationID string) (hris.State, error) {
	var doc stateDoc

	err := s.col.FindOne(ctx, bson.M{fieldID: organizationID}).Decode(&doc)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return hris.State{}, hris.ErrNotConfigured
	}

	if err != nil {
		return hris.State{}, fmt.Errorf("find hris state: %w", err)
	}

	entries := doc.Entries
	if entries == nil {
		entries = []hris.DirectoryEntry{}
	}

	return hris.State{Entries: entries, LastRun: doc.LastRun}, nil
}

// DeleteForOrganization removes the organization's HRIS config (keyed by
// organization id) and returns the number of documents deleted. It serves
// only the platform GDPR tenant purge (PRD 7.4).
func (s *ConfigStore) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete hris config for organization: %w", err)
	}

	return res.DeletedCount, nil
}

// DeleteForOrganization removes the organization's synced directory state
// (keyed by organization id) and returns the number of documents deleted. It
// serves only the platform GDPR tenant purge (PRD 7.4).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete hris state for organization: %w", err)
	}

	return res.DeletedCount, nil
}
