package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/scim"
)

const fieldDisplayName = "displayName"

var _ scim.GroupStore = (*GroupStore)(nil)

// GroupStore persists SCIM group records, tenant-scoped by organizationId.
type GroupStore struct {
	groups *drivermongo.Collection
}

// NewGroupStore constructs a GroupStore.
func NewGroupStore(db *drivermongo.Database) *GroupStore {
	return &GroupStore{groups: db.Collection("scim_groups")}
}

// EnsureIndexes creates SCIM group indexes.
func (s *GroupStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.groups.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldDisplayName, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure scim group indexes: %w", err)
	}

	return nil
}

// Create inserts a SCIM group record.
func (s *GroupStore) Create(ctx context.Context, group scim.GroupRecord) error {
	_, err := s.groups.InsertOne(ctx, group)
	if drivermongo.IsDuplicateKeyError(err) {
		return scim.ErrConflict
	}

	if err != nil {
		return fmt.Errorf("insert scim group: %w", err)
	}

	return nil
}

// GetByID loads a group by resource id within a tenant.
func (s *GroupStore) GetByID(ctx context.Context, organizationID, id string) (scim.GroupRecord, error) {
	return s.findOne(ctx, bson.M{fieldID: id, fieldOrganizationID: organizationID})
}

// GetByDisplayName loads a group by displayName within a tenant.
func (s *GroupStore) GetByDisplayName(
	ctx context.Context,
	organizationID, displayName string,
) (scim.GroupRecord, error) {
	return s.findOne(ctx, bson.M{fieldOrganizationID: organizationID, fieldDisplayName: displayName})
}

// List returns a page of tenant groups (sorted by displayName) plus the total
// matching count. limit <= 0 returns no groups, only the total.
func (s *GroupStore) List(
	ctx context.Context,
	organizationID, displayNameFilter string,
	offset, limit int,
) ([]scim.GroupRecord, int, error) {
	filter := bson.M{fieldOrganizationID: organizationID}
	if displayNameFilter != "" {
		filter[fieldDisplayName] = displayNameFilter
	}

	total, err := s.groups.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count scim groups: %w", err)
	}

	if limit <= 0 {
		return []scim.GroupRecord{}, int(total), nil
	}

	cursor, err := s.groups.Find(ctx, filter, pageOptions(fieldDisplayName, offset, limit))
	if err != nil {
		return nil, 0, fmt.Errorf("find scim groups: %w", err)
	}

	records, err := collectCursor[scim.GroupRecord](ctx, cursor, "scim groups")
	if err != nil {
		return nil, 0, err
	}

	return records, int(total), nil
}

// Update replaces a group record.
func (s *GroupStore) Update(ctx context.Context, group scim.GroupRecord) error {
	res, err := s.groups.ReplaceOne(
		ctx,
		bson.M{fieldID: group.ID, fieldOrganizationID: group.OrganizationID},
		group,
	)
	if drivermongo.IsDuplicateKeyError(err) {
		// A rename collided with another group's displayName in this tenant.
		return scim.ErrConflict
	}

	if err != nil {
		return fmt.Errorf("replace scim group: %w", err)
	}

	if res.MatchedCount == 0 {
		return scim.ErrNotFound
	}

	return nil
}

// Delete removes a group within a tenant.
func (s *GroupStore) Delete(ctx context.Context, organizationID, id string) error {
	res, err := s.groups.DeleteOne(ctx, bson.M{fieldID: id, fieldOrganizationID: organizationID})
	if err != nil {
		return fmt.Errorf("delete scim group: %w", err)
	}

	if res.DeletedCount == 0 {
		return scim.ErrNotFound
	}

	return nil
}

// DeleteForOrganization removes every SCIM group record of the organization
// and returns the number deleted. It serves only the platform GDPR tenant
// purge (PRD 7.4).
func (s *GroupStore) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.groups.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete scim groups for organization: %w", err)
	}

	return res.DeletedCount, nil
}

func (s *GroupStore) findOne(ctx context.Context, filter bson.M) (scim.GroupRecord, error) {
	var record scim.GroupRecord

	err := s.groups.FindOne(ctx, filter).Decode(&record)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return scim.GroupRecord{}, scim.ErrNotFound
	}

	if err != nil {
		return scim.GroupRecord{}, fmt.Errorf("find scim group: %w", err)
	}

	return record, nil
}
