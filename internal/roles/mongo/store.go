// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/roles"
)

var _ roles.Repository = (*Store)(nil)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldName           = "name"
)

// Store is the MongoDB custom-role repository.
type Store struct {
	roles *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{roles: db.Collection("roles")}
}

// EnsureIndexes creates role indexes: role names are unique per organization.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.roles.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldName, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure role indexes: %w", err)
	}

	return nil
}

// Create inserts a custom role. A duplicate (organization, name) pair maps to
// roles.ErrNameTaken so a retried create reports the conflict cleanly.
func (s *Store) Create(ctx context.Context, role roles.Role) error {
	_, err := s.roles.InsertOne(ctx, role)
	if drivermongo.IsDuplicateKeyError(err) {
		return roles.ErrNameTaken
	}

	if err != nil {
		return fmt.Errorf("insert role: %w", err)
	}

	return nil
}

// GetByID loads a custom role by id within its organization.
func (s *Store) GetByID(ctx context.Context, organizationID, id string) (roles.Role, error) {
	return s.findOne(ctx, bson.M{fieldID: id, fieldOrganizationID: organizationID})
}

// GetByName loads a custom role by name within its organization. Permission
// resolution goes through here when a membership's roleCode is not built in.
func (s *Store) GetByName(ctx context.Context, organizationID, name string) (roles.Role, error) {
	return s.findOne(ctx, bson.M{fieldName: name, fieldOrganizationID: organizationID})
}

// List returns the organization's custom roles ordered by name.
func (s *Store) List(ctx context.Context, organizationID string) ([]roles.Role, error) {
	cursor, err := s.roles.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldName, Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find roles: %w", err)
	}

	items := make([]roles.Role, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode roles: %w", decodeErr),
			fmt.Errorf("close roles cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode roles: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close roles cursor: %w", closeErr)
	}

	return items, nil
}

// Update replaces a custom role document, scoped to its organization.
func (s *Store) Update(ctx context.Context, role roles.Role) error {
	res, err := s.roles.ReplaceOne(
		ctx,
		bson.M{fieldID: role.ID, fieldOrganizationID: role.OrganizationID},
		role,
	)
	if err != nil {
		return fmt.Errorf("replace role: %w", err)
	}

	if res.MatchedCount == 0 {
		return roles.ErrNotFound
	}

	return nil
}

// Delete removes a custom role by id, scoped to its organization.
func (s *Store) Delete(ctx context.Context, organizationID, id string) error {
	res, err := s.roles.DeleteOne(ctx, bson.M{fieldID: id, fieldOrganizationID: organizationID})
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}

	if res.DeletedCount == 0 {
		return roles.ErrNotFound
	}

	return nil
}

// DeleteForOrganization removes every custom role of the organization and
// returns the number deleted. It serves only the platform GDPR tenant purge
// (PRD 7.4).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.roles.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete roles for organization: %w", err)
	}

	return res.DeletedCount, nil
}

func (s *Store) findOne(ctx context.Context, filter bson.M) (roles.Role, error) {
	var role roles.Role

	err := s.roles.FindOne(ctx, filter).Decode(&role)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return roles.Role{}, roles.ErrNotFound
	}

	if err != nil {
		return roles.Role{}, fmt.Errorf("find role: %w", err)
	}

	return role, nil
}
