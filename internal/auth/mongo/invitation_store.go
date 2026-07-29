package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/auth"
)

const fieldOrganizationID = "organizationId"

var _ auth.InvitationStore = (*InvitationStore)(nil)

const (
	invitationTokenHashField = "tokenHash"
	invitationExpiresField   = "expiresAt"
)

// InvitationStore persists single-use, expiring account invitations.
type InvitationStore struct {
	col *drivermongo.Collection
}

// NewInvitationStore constructs an InvitationStore.
func NewInvitationStore(db *drivermongo.Database) *InvitationStore {
	return &InvitationStore{col: db.Collection("auth_invitations")}
}

// EnsureIndexes creates a unique index on the token hash and a TTL index that
// purges expired invitations.
func (s *InvitationStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: invitationTokenHashField, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: invitationExpiresField, Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure invitation indexes: %w", err)
	}

	return nil
}

// Save inserts an invitation.
func (s *InvitationStore) Save(ctx context.Context, invitation auth.Invitation) error {
	if _, err := s.col.InsertOne(ctx, invitation); err != nil {
		return fmt.Errorf("insert invitation: %w", err)
	}

	return nil
}

// Consume atomically finds and deletes a non-expired invitation by token
// hash, so concurrent redemptions of the same token cannot both succeed. The
// expiry filter guards the window before the TTL reaper removes an expired
// document.
func (s *InvitationStore) Consume(ctx context.Context, tokenHash string) (auth.Invitation, error) {
	var invitation auth.Invitation

	err := s.col.FindOneAndDelete(ctx, bson.M{
		invitationTokenHashField: tokenHash,
		invitationExpiresField:   bson.M{"$gt": time.Now().UTC()},
	}).Decode(&invitation)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return auth.Invitation{}, auth.ErrInvitationInvalid
	}

	if err != nil {
		return auth.Invitation{}, fmt.Errorf("consume invitation: %w", err)
	}

	return invitation, nil
}

func (s *InvitationStore) ListForOrganization(ctx context.Context, organizationID string) ([]auth.Invitation, error) {
	cursor, err := s.col.Find(ctx, bson.M{
		fieldOrganizationID:    organizationID,
		invitationExpiresField: bson.M{"$gt": time.Now().UTC()},
	}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()
	items := make([]auth.Invitation, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode invitations: %w", err)
	}
	return items, nil
}

func (s *InvitationStore) GetForOrganization(
	ctx context.Context, organizationID, invitationID string,
) (auth.Invitation, error) {
	var invitation auth.Invitation
	err := s.col.FindOne(ctx, bson.M{
		"_id": invitationID, fieldOrganizationID: organizationID,
		invitationExpiresField: bson.M{"$gt": time.Now().UTC()},
	}).Decode(&invitation)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return auth.Invitation{}, auth.ErrInvitationInvalid
	}
	if err != nil {
		return auth.Invitation{}, fmt.Errorf("get invitation: %w", err)
	}
	return invitation, nil
}

func (s *InvitationStore) DeleteForOrganizationByID(
	ctx context.Context, organizationID, invitationID string,
) error {
	result, err := s.col.DeleteOne(ctx, bson.M{"_id": invitationID, fieldOrganizationID: organizationID})
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	if result.DeletedCount == 0 {
		return auth.ErrInvitationInvalid
	}
	return nil
}

func (s *InvitationStore) DeleteForUser(ctx context.Context, organizationID, userID string) error {
	if _, err := s.col.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID, "userId": userID}); err != nil {
		return fmt.Errorf("delete prior user invitations: %w", err)
	}
	return nil
}

// DeleteForOrganization removes every invitation of the organization and
// returns the number deleted. It serves only the platform GDPR tenant purge
// (PRD 7.4).
func (s *InvitationStore) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete invitations for organization: %w", err)
	}

	return res.DeletedCount, nil
}
