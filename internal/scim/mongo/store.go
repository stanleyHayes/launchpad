// Package mongo is the MongoDB persistence adapter for the SCIM domain.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/scim"
)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldUserName       = "userName"
	fieldTokenHash      = "tokenHash"
	fieldIssuedAt       = "issuedAt"
	fieldExpiresAt      = "expiresAt"

	// listLimit bounds a single List response so a tenant with many users
	// cannot force an unbounded result set (matches the advertised maxResults).
	listLimit = 200
)

var _ scim.Store = (*Store)(nil)

// Store persists SCIM user records.
type Store struct {
	records *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{records: db.Collection("scim_users")}
}

// EnsureIndexes creates SCIM user indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.records.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldUserName, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure scim user indexes: %w", err)
	}

	return nil
}

// Create inserts a SCIM user record.
func (s *Store) Create(ctx context.Context, record scim.Record) error {
	_, err := s.records.InsertOne(ctx, record)
	if drivermongo.IsDuplicateKeyError(err) {
		return scim.ErrConflict
	}

	if err != nil {
		return fmt.Errorf("insert scim user: %w", err)
	}

	return nil
}

// GetByID loads a record by resource id within a tenant.
func (s *Store) GetByID(ctx context.Context, organizationID, id string) (scim.Record, error) {
	return s.findOne(ctx, bson.M{fieldID: id, fieldOrganizationID: organizationID})
}

// GetByUserName loads a record by userName within a tenant.
func (s *Store) GetByUserName(ctx context.Context, organizationID, userName string) (scim.Record, error) {
	return s.findOne(ctx, bson.M{fieldOrganizationID: organizationID, fieldUserName: userName})
}

// List returns a page of tenant records (sorted by userName) plus the total
// matching count. limit <= 0 returns no records, only the total.
func (s *Store) List(
	ctx context.Context,
	organizationID, userNameFilter string,
	offset, limit int,
) ([]scim.Record, int, error) {
	filter := bson.M{fieldOrganizationID: organizationID}
	if userNameFilter != "" {
		filter[fieldUserName] = userNameFilter
	}

	total, err := s.records.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count scim users: %w", err)
	}

	if limit <= 0 {
		return []scim.Record{}, int(total), nil
	}

	cursor, err := s.records.Find(ctx, filter, pageOptions(fieldUserName, offset, limit))
	if err != nil {
		return nil, 0, fmt.Errorf("find scim users: %w", err)
	}

	records, err := collectCursor[scim.Record](ctx, cursor, "scim users")
	if err != nil {
		return nil, 0, err
	}

	return records, int(total), nil
}

// pageOptions builds a sorted, skip/limit Find option for a SCIM list page,
// clamping the limit to listLimit and the offset to a non-negative value.
func pageOptions(sortField string, offset, limit int) *options.FindOptionsBuilder {
	if limit > listLimit {
		limit = listLimit
	}

	if offset < 0 {
		offset = 0
	}

	return options.Find().
		SetSort(bson.D{{Key: sortField, Value: 1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
}

// collectCursor drains a cursor into a slice, joining any decode and close
// errors. `resource` names the collection for error messages (e.g. "scim users").
func collectCursor[T any](ctx context.Context, cursor *drivermongo.Cursor, resource string) ([]T, error) {
	items := make([]T, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode %s: %w", resource, decodeErr),
			fmt.Errorf("close %s cursor: %w", resource, closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode %s: %w", resource, decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close %s cursor: %w", resource, closeErr)
	}

	return items, nil
}

// Update replaces a record.
func (s *Store) Update(ctx context.Context, record scim.Record) error {
	res, err := s.records.ReplaceOne(
		ctx,
		bson.M{fieldID: record.ID, fieldOrganizationID: record.OrganizationID},
		record,
	)
	if err != nil {
		return fmt.Errorf("replace scim user: %w", err)
	}

	if res.MatchedCount == 0 {
		return scim.ErrNotFound
	}

	return nil
}

// Delete removes a record within a tenant.
func (s *Store) Delete(ctx context.Context, organizationID, id string) error {
	res, err := s.records.DeleteOne(ctx, bson.M{fieldID: id, fieldOrganizationID: organizationID})
	if err != nil {
		return fmt.Errorf("delete scim user: %w", err)
	}

	if res.DeletedCount == 0 {
		return scim.ErrNotFound
	}

	return nil
}

// DeleteForOrganization removes every SCIM user record of the organization
// and returns the number deleted. It serves only the platform GDPR tenant
// purge (PRD 7.4).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.records.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete scim users for organization: %w", err)
	}

	return res.DeletedCount, nil
}

func (s *Store) findOne(ctx context.Context, filter bson.M) (scim.Record, error) {
	var record scim.Record

	err := s.records.FindOne(ctx, filter).Decode(&record)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return scim.Record{}, scim.ErrNotFound
	}

	if err != nil {
		return scim.Record{}, fmt.Errorf("find scim user: %w", err)
	}

	return record, nil
}

var _ scim.TokenStore = (*TokenStore)(nil)

// TokenStore persists per-organization SCIM provisioning tokens (hashed).
type TokenStore struct {
	tokens *drivermongo.Collection
}

// NewTokenStore constructs a TokenStore.
func NewTokenStore(db *drivermongo.Database) *TokenStore {
	return &TokenStore{tokens: db.Collection("scim_tokens")}
}

// EnsureIndexes creates SCIM token indexes.
func (s *TokenStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.tokens.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: fieldTokenHash, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure scim token indexes: %w", err)
	}

	return nil
}

// StoreToken replaces any existing token for the organization.
func (s *TokenStore) StoreToken(
	ctx context.Context,
	organizationID, tokenHash string,
	issuedAt, expiresAt time.Time,
) error {
	_, err := s.tokens.ReplaceOne(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		bson.M{
			fieldOrganizationID: organizationID,
			fieldTokenHash:      tokenHash,
			fieldIssuedAt:       issuedAt,
			fieldExpiresAt:      expiresAt,
		},
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("store scim token: %w", err)
	}

	return nil
}

// ResolveOrganization returns the organization a token hash belongs to, plus
// the token's expiry (zero for tokens issued before expiry existed).
func (s *TokenStore) ResolveOrganization(ctx context.Context, tokenHash string) (string, time.Time, error) {
	var doc struct {
		OrganizationID string    `bson:"organizationId"`
		ExpiresAt      time.Time `bson:"expiresAt"`
	}

	err := s.tokens.FindOne(ctx, bson.M{fieldTokenHash: tokenHash}).Decode(&doc)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return "", time.Time{}, scim.ErrUnauthorized
	}

	if err != nil {
		return "", time.Time{}, fmt.Errorf("resolve scim token: %w", err)
	}

	return doc.OrganizationID, doc.ExpiresAt, nil
}

// DeleteForOrganization removes every provisioning token of the organization
// and returns the number deleted. It serves only the platform GDPR tenant
// purge (PRD 7.4).
func (s *TokenStore) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.tokens.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete scim tokens for organization: %w", err)
	}

	return res.DeletedCount, nil
}
