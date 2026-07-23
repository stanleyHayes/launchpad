package organizations

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const fieldUserID = "userId"

// Repository persists organizations and memberships.
type Repository interface {
	CreateOrganization(ctx context.Context, org Organization) error
	GetByID(ctx context.Context, id string) (Organization, error)
	GetBySlug(ctx context.Context, slug string) (Organization, error)
	Update(ctx context.Context, org Organization) error
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
			Keys:    bson.D{{Key: "organizationId", Value: 1}, {Key: fieldUserID, Value: 1}},
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

	err := s.orgs.FindOne(ctx, bson.M{"_id": id}).Decode(&org)
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

// Update replaces an organization document.
func (s *Store) Update(ctx context.Context, org Organization) error {
	res, err := s.orgs.ReplaceOne(ctx, bson.M{"_id": org.ID}, org)
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
		"organizationId": organizationID,
		fieldUserID:      userID,
		"status":         membershipStatusActive,
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
