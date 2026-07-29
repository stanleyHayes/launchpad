// Package mongo is the MongoDB persistence adapter for the integrations domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/integrations"
	"launchpad/pkg/security"
)

const (
	fieldOrganizationID = "organizationId"
	fieldProvider       = "provider"
)

var _ integrations.Repository = (*Store)(nil)

// Store persists per-organization provider connections.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("integration_connections")}
}

// EnsureIndexes creates the unique (organization, provider) index that
// enforces one connection per provider per tenant.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldProvider, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure integration indexes: %w", err)
	}

	return nil
}

// Upsert creates or replaces a tenant's connection for a provider. The token
// is encrypted at rest when ENCRYPTION_KEY is configured.
func (s *Store) Upsert(ctx context.Context, conn integrations.Connection) error {
	var err error

	conn.Token, err = security.EncryptSecret(conn.Token)
	if err != nil {
		return fmt.Errorf("encrypt integration token: %w", err)
	}

	_, err = s.col.ReplaceOne(
		ctx,
		bson.M{fieldOrganizationID: conn.OrganizationID, fieldProvider: conn.Provider},
		conn,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("upsert integration connection: %w", err)
	}

	return nil
}

// Get loads a tenant's connection for a provider, decrypting the token.
func (s *Store) Get(ctx context.Context, organizationID, provider string) (integrations.Connection, error) {
	var conn integrations.Connection

	err := s.col.FindOne(ctx, bson.M{fieldOrganizationID: organizationID, fieldProvider: provider}).Decode(&conn)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return integrations.Connection{}, integrations.ErrNotFound
	}

	if err != nil {
		return integrations.Connection{}, fmt.Errorf("find integration connection: %w", err)
	}

	if err := decryptToken(&conn); err != nil {
		return integrations.Connection{}, err
	}

	return conn, nil
}

// List loads all of a tenant's connections, decrypting each token.
func (s *Store) List(ctx context.Context, organizationID string) ([]integrations.Connection, error) {
	cursor, err := s.col.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldProvider, Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find integration connections: %w", err)
	}

	defer func() { _ = cursor.Close(ctx) }()

	var docs []integrations.Connection
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode integration connections: %w", err)
	}

	connections := make([]integrations.Connection, 0, len(docs))

	for _, doc := range docs {
		if err := decryptToken(&doc); err != nil {
			return nil, err
		}

		connections = append(connections, doc)
	}

	return connections, nil
}

// Delete removes a tenant's connection for a provider.
func (s *Store) Delete(ctx context.Context, organizationID, provider string) error {
	result, err := s.col.DeleteOne(ctx, bson.M{fieldOrganizationID: organizationID, fieldProvider: provider})
	if err != nil {
		return fmt.Errorf("delete integration connection: %w", err)
	}

	if result.DeletedCount == 0 {
		return integrations.ErrNotFound
	}

	return nil
}

func decryptToken(conn *integrations.Connection) error {
	token, err := security.DecryptSecret(conn.Token)
	if err != nil {
		return fmt.Errorf("decrypt integration token: %w", err)
	}

	conn.Token = token

	return nil
}

// DeleteForOrganization removes every integration connection of the
// organization and returns the number deleted. It serves only the platform
// GDPR tenant purge (PRD 7.4).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete integration connections for organization: %w", err)
	}

	return res.DeletedCount, nil
}
