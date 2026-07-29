// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/organizations"
)

var _ organizations.Repository = (*Store)(nil)

const (
	fieldUserID         = "userId"
	fieldOrganizationID = "organizationId"
	fieldID             = "_id"
	fieldStatus         = "status"

	membershipStatusActive = "active"
)

// Store is the MongoDB organization repository.
type Store struct {
	orgs        *drivermongo.Collection
	memberships *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{
		orgs:        db.Collection("organizations"),
		memberships: db.Collection("organization_memberships"),
	}
}

// EnsureIndexes creates organization indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.orgs.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return fmt.Errorf("ensure organization indexes: %w", err)
	}

	_, err = s.memberships.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldUserID, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: fieldUserID, Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure membership indexes: %w", err)
	}

	return nil
}

// CreateOrganization inserts an organization.
func (s *Store) CreateOrganization(ctx context.Context, org organizations.Organization) error {
	_, err := s.orgs.InsertOne(ctx, org)
	if drivermongo.IsDuplicateKeyError(err) {
		return organizations.ErrSlugTaken
	}

	if err != nil {
		return fmt.Errorf("insert organization: %w", err)
	}

	return nil
}

// DeleteOrganization removes an organization by id. It is used to compensate a
// failed CreateWithOwner so the slug is not burned.
func (s *Store) DeleteOrganization(ctx context.Context, id string) error {
	if _, err := s.orgs.DeleteOne(ctx, bson.M{fieldID: id}); err != nil {
		return fmt.Errorf("delete organization: %w", err)
	}

	return nil
}

// GetByID loads an organization by id.
func (s *Store) GetByID(ctx context.Context, id string) (organizations.Organization, error) {
	var org organizations.Organization

	err := s.orgs.FindOne(ctx, bson.M{fieldID: id}).Decode(&org)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return organizations.Organization{}, organizations.ErrNotFound
	}

	if err != nil {
		return organizations.Organization{}, fmt.Errorf("find organization by id: %w", err)
	}

	return org, nil
}

// GetBySlug loads an organization by slug.
func (s *Store) GetBySlug(ctx context.Context, slug string) (organizations.Organization, error) {
	var org organizations.Organization

	err := s.orgs.FindOne(ctx, bson.M{"slug": slug}).Decode(&org)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return organizations.Organization{}, organizations.ErrNotFound
	}

	if err != nil {
		return organizations.Organization{}, fmt.Errorf("find organization by slug: %w", err)
	}

	return org, nil
}

// List returns all organizations ordered by creation time.
func (s *Store) List(ctx context.Context) ([]organizations.Organization, error) {
	cursor, err := s.orgs.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("find organizations: %w", err)
	}

	items := make([]organizations.Organization, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode organizations: %w", decodeErr),
			fmt.Errorf("close organizations cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode organizations: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close organizations cursor: %w", closeErr)
	}

	return items, nil
}

// CountByStatus returns organization counts grouped by status.
func (s *Store) CountByStatus(ctx context.Context) (map[string]int64, error) {
	pipeline := drivermongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":   "$status",
			"count": bson.M{"$sum": 1},
		}}},
	}

	cursor, err := s.orgs.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate organization counts: %w", err)
	}

	type statusCount struct {
		Status string `bson:"_id"`
		Count  int64  `bson:"count"`
	}

	counts := make([]statusCount, 0)
	decodeErr := cursor.All(ctx, &counts)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode organization counts: %w", decodeErr),
			fmt.Errorf("close organization counts cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode organization counts: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close organization counts cursor: %w", closeErr)
	}

	out := make(map[string]int64, len(counts))
	for _, item := range counts {
		out[item.Status] = item.Count
	}

	return out, nil
}

// Update replaces an organization document.
func (s *Store) Update(ctx context.Context, org organizations.Organization) error {
	res, err := s.orgs.ReplaceOne(ctx, bson.M{fieldID: org.ID}, org)
	if err != nil {
		return fmt.Errorf("replace organization: %w", err)
	}

	if res.MatchedCount == 0 {
		return organizations.ErrNotFound
	}

	return nil
}

// CreateMembership inserts a membership. A duplicate (organization, user)
// pair maps to organizations.ErrAlreadyMember so callers can treat an
// existing membership as the goal state on retry.
func (s *Store) CreateMembership(ctx context.Context, membership organizations.Membership) error {
	_, err := s.memberships.InsertOne(ctx, membership)
	if drivermongo.IsDuplicateKeyError(err) {
		return organizations.ErrAlreadyMember
	}

	if err != nil {
		return fmt.Errorf("insert membership: %w", err)
	}

	return nil
}

// UpdateMembershipStatus sets a membership's status, matching by tenant and user
// regardless of the current status so a suspended membership can be reactivated
// in place.
func (s *Store) UpdateMembershipStatus(ctx context.Context, organizationID, userID, status string) error {
	res, err := s.memberships.UpdateOne(
		ctx,
		bson.M{fieldOrganizationID: organizationID, fieldUserID: userID},
		bson.M{"$set": bson.M{fieldStatus: status}},
	)
	if err != nil {
		return fmt.Errorf("update membership status: %w", err)
	}

	if res.MatchedCount == 0 {
		return organizations.ErrNotFound
	}

	return nil
}

// MembershipExists reports whether the user has a membership in the organization
// of any status (active or suspended).
func (s *Store) MembershipExists(ctx context.Context, organizationID, userID string) (bool, error) {
	count, err := s.memberships.CountDocuments(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldUserID:         userID,
	})
	if err != nil {
		return false, fmt.Errorf("count membership: %w", err)
	}

	return count > 0, nil
}

// GetMembership loads an active membership.
func (s *Store) GetMembership(ctx context.Context, organizationID, userID string) (organizations.Membership, error) {
	var membership organizations.Membership

	err := s.memberships.FindOne(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldUserID:         userID,
		fieldStatus:         membershipStatusActive,
	}).Decode(&membership)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return organizations.Membership{}, organizations.ErrNotFound
	}

	if err != nil {
		return organizations.Membership{}, fmt.Errorf("find membership: %w", err)
	}

	return membership, nil
}

// ListMembershipsByUser lists active memberships for a user.
func (s *Store) ListMembershipsByUser(ctx context.Context, userID string) ([]organizations.Membership, error) {
	cursor, err := s.memberships.Find(ctx, bson.M{fieldUserID: userID, fieldStatus: membershipStatusActive})
	if err != nil {
		return nil, fmt.Errorf("find memberships: %w", err)
	}

	items := make([]organizations.Membership, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode memberships: %w", decodeErr),
			fmt.Errorf("close memberships cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode memberships: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close memberships cursor: %w", closeErr)
	}

	return items, nil
}

// ListMemberships lists active memberships in an organization, ordered by
// creation time.
func (s *Store) ListMemberships(ctx context.Context, organizationID string) ([]organizations.Membership, error) {
	cursor, err := s.memberships.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID, fieldStatus: membershipStatusActive},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find memberships: %w", err)
	}

	items := make([]organizations.Membership, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode memberships: %w", decodeErr),
			fmt.Errorf("close memberships cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode memberships: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close memberships cursor: %w", closeErr)
	}

	return items, nil
}

// UpdateMembershipRole sets a membership's role, matching by tenant and user
// regardless of the current status so a suspended member's role can be
// corrected before reactivation.
func (s *Store) UpdateMembershipRole(ctx context.Context, organizationID, userID, roleCode string) error {
	res, err := s.memberships.UpdateOne(
		ctx,
		bson.M{fieldOrganizationID: organizationID, fieldUserID: userID},
		bson.M{"$set": bson.M{"roleCode": roleCode}},
	)
	if err != nil {
		return fmt.Errorf("update membership role: %w", err)
	}

	if res.MatchedCount == 0 {
		return organizations.ErrNotFound
	}

	return nil
}

// CountMembershipsByRole counts active memberships with a role in an
// organization, used to guard demotion of the last organization_owner.
func (s *Store) CountMembershipsByRole(ctx context.Context, organizationID, roleCode string) (int64, error) {
	count, err := s.memberships.CountDocuments(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldStatus:         membershipStatusActive,
		"roleCode":          roleCode,
	})
	if err != nil {
		return 0, fmt.Errorf("count memberships by role: %w", err)
	}

	return count, nil
}

// DeleteForOrganization removes every membership of the organization and the
// organization document itself, returning the number of documents deleted.
// It serves only the platform GDPR tenant purge (PRD 7.4); unlike
// DeleteOrganization it is not a compensation for a failed signup.
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.memberships.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete memberships for organization: %w", err)
	}

	deleted := res.DeletedCount

	res, err = s.orgs.DeleteMany(ctx, bson.M{fieldID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete organization document: %w", err)
	}

	return deleted + res.DeletedCount, nil
}
