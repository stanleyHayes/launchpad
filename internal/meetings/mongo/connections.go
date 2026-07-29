package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/meetings"
	"launchpad/pkg/security"
)

const fieldProvider = "provider"

var _ meetings.ConnectionRepository = (*ConnectionStore)(nil)

// ConnectionStore persists per-organization calendar connections.
type ConnectionStore struct {
	col *drivermongo.Collection
}

// NewConnectionStore constructs a ConnectionStore.
func NewConnectionStore(db *drivermongo.Database) *ConnectionStore {
	return &ConnectionStore{col: db.Collection("calendar_connections")}
}

// EnsureIndexes creates the unique (organization, provider) index that
// enforces one calendar connection per provider per tenant.
func (s *ConnectionStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldProvider, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure calendar connection indexes: %w", err)
	}

	return nil
}

// Upsert creates or replaces a tenant's connection for a provider. The token
// is encrypted at rest when ENCRYPTION_KEY is configured.
func (s *ConnectionStore) Upsert(ctx context.Context, conn meetings.CalendarConnection) error {
	var err error

	conn.Token, err = security.EncryptSecret(conn.Token)
	if err != nil {
		return fmt.Errorf("encrypt calendar token: %w", err)
	}
	if conn.RefreshToken != "" {
		conn.RefreshToken, err = security.EncryptSecret(conn.RefreshToken)
		if err != nil {
			return fmt.Errorf("encrypt calendar refresh token: %w", err)
		}
	}

	_, err = s.col.ReplaceOne(
		ctx,
		bson.M{fieldOrganizationID: conn.OrganizationID, fieldProvider: conn.Provider},
		conn,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("upsert calendar connection: %w", err)
	}

	return nil
}

// Get loads a tenant's connection for a provider, decrypting the token.
func (s *ConnectionStore) Get(
	ctx context.Context,
	organizationID, provider string,
) (meetings.CalendarConnection, error) {
	var conn meetings.CalendarConnection

	err := s.col.FindOne(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldProvider:       provider,
	}).Decode(&conn)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return meetings.CalendarConnection{}, meetings.ErrNotFound
	}

	if err != nil {
		return meetings.CalendarConnection{}, fmt.Errorf("find calendar connection: %w", err)
	}

	token, err := security.DecryptSecret(conn.Token)
	if err != nil {
		return meetings.CalendarConnection{}, fmt.Errorf("decrypt calendar token: %w", err)
	}

	conn.Token = token
	if conn.RefreshToken != "" {
		refreshToken, refreshErr := security.DecryptSecret(conn.RefreshToken)
		if refreshErr != nil {
			return meetings.CalendarConnection{}, fmt.Errorf("decrypt calendar refresh token: %w", refreshErr)
		}
		conn.RefreshToken = refreshToken
	}

	return conn, nil
}

// Delete removes a tenant's connection for a provider.
func (s *ConnectionStore) Delete(ctx context.Context, organizationID, provider string) error {
	result, err := s.col.DeleteOne(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldProvider:       provider,
	})
	if err != nil {
		return fmt.Errorf("delete calendar connection: %w", err)
	}

	if result.DeletedCount == 0 {
		return meetings.ErrNotFound
	}

	return nil
}

// DeleteForOrganization removes every calendar connection of the organization
// and returns the number deleted. It serves only the platform GDPR tenant
// purge (PRD 7.4).
func (s *ConnectionStore) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete calendar connections for organization: %w", err)
	}

	return res.DeletedCount, nil
}
