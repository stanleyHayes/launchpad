package organizations

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	fieldUserID         = "userId"
	fieldOrganizationID = "organizationId"
	fieldID             = "_id"
)

// Repository persists organizations and memberships.
type Repository interface {
	CreateOrganization(ctx context.Context, org Organization) error
	GetByID(ctx context.Context, id string) (Organization, error)
	GetBySlug(ctx context.Context, slug string) (Organization, error)
	Update(ctx context.Context, org Organization) error
	List(ctx context.Context) ([]Organization, error)
	CountByStatus(ctx context.Context) (map[string]int64, error)
	CreateMembership(ctx context.Context, membership Membership) error
	GetMembership(ctx context.Context, organizationID, userID string) (Membership, error)
	ListMembershipsByUser(ctx context.Context, userID string) ([]Membership, error)
}

// Store is the MongoDB organization repository.
type Store struct {
	orgs        *mongo.Collection
	memberships *mongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *mongo.Database) *Store {
	return &Store{
		orgs:        db.Collection("organizations"),
		memberships: db.Collection("organization_memberships"),
	}
}

// EnsureIndexes creates organization indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.orgs.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return fmt.Errorf("ensure organization indexes: %w", err)
	}

	_, err = s.memberships.Indexes().CreateMany(ctx, []mongo.IndexModel{
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
func (s *Store) CreateOrganization(ctx context.Context, org Organization) error {
	_, err := s.orgs.InsertOne(ctx, org)
	if mongo.IsDuplicateKeyError(err) {
		return ErrSlugTaken
	}

	if err != nil {
		return fmt.Errorf("insert organization: %w", err)
	}

	return nil
}

// GetByID loads an organization by id.
func (s *Store) GetByID(ctx context.Context, id string) (Organization, error) {
	var org Organization

	err := s.orgs.FindOne(ctx, bson.M{fieldID: id}).Decode(&org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Organization{}, ErrNotFound
	}

	if err != nil {
		return Organization{}, fmt.Errorf("find organization by id: %w", err)
	}

	return org, nil
}

// GetBySlug loads an organization by slug.
func (s *Store) GetBySlug(ctx context.Context, slug string) (Organization, error) {
	var org Organization

	err := s.orgs.FindOne(ctx, bson.M{"slug": slug}).Decode(&org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Organization{}, ErrNotFound
	}

	if err != nil {
		return Organization{}, fmt.Errorf("find organization by slug: %w", err)
	}

	return org, nil
}

// List returns all organizations ordered by creation time.
func (s *Store) List(ctx context.Context) ([]Organization, error) {
	cursor, err := s.orgs.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("find organizations: %w", err)
	}

	items := make([]Organization, 0)
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
	pipeline := mongo.Pipeline{
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
func (s *Store) Update(ctx context.Context, org Organization) error {
	res, err := s.orgs.ReplaceOne(ctx, bson.M{fieldID: org.ID}, org)
	if err != nil {
		return fmt.Errorf("replace organization: %w", err)
	}

	if res.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// CreateMembership inserts a membership.
func (s *Store) CreateMembership(ctx context.Context, membership Membership) error {
	_, err := s.memberships.InsertOne(ctx, membership)
	if err != nil {
		return fmt.Errorf("insert membership: %w", err)
	}

	return nil
}

// GetMembership loads an active membership.
func (s *Store) GetMembership(ctx context.Context, organizationID, userID string) (Membership, error) {
	var membership Membership

	err := s.memberships.FindOne(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldUserID:         userID,
		"status":            membershipStatusActive,
	}).Decode(&membership)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Membership{}, ErrNotFound
	}

	if err != nil {
		return Membership{}, fmt.Errorf("find membership: %w", err)
	}

	return membership, nil
}

// ListMembershipsByUser lists active memberships for a user.
func (s *Store) ListMembershipsByUser(ctx context.Context, userID string) ([]Membership, error) {
	cursor, err := s.memberships.Find(ctx, bson.M{fieldUserID: userID, "status": membershipStatusActive})
	if err != nil {
		return nil, fmt.Errorf("find memberships: %w", err)
	}

	items := make([]Membership, 0)
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
