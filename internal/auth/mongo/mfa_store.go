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

var (
	_ auth.MFAStore       = (*MFAStore)(nil)
	_ auth.MFATicketStore = (*MFATicketStore)(nil)
)

const (
	mfaDocIDField       = "_id"
	mfaBackupCodesField = "backupCodeHashes"

	mfaTicketHashField   = "ticketHash"
	mfaTicketExpiryField = "expiresAt"

	mongoGreaterThan = "$gt"
)

// MFAStore persists per-user TOTP enrollments in the auth_mfa collection,
// keyed by the (organization, user) scope id.
type MFAStore struct {
	col *drivermongo.Collection
}

// NewMFAStore constructs an MFAStore.
func NewMFAStore(db *drivermongo.Database) *MFAStore {
	return &MFAStore{col: db.Collection("auth_mfa")}
}

// EnsureIndexes creates the lookup index on the scope key fields. Uniqueness
// is enforced by the primary _id (the scope id).
func (s *MFAStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateOne(ctx, drivermongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}, {Key: "organizationId", Value: 1}},
	})
	if err != nil {
		return fmt.Errorf("ensure mfa indexes: %w", err)
	}

	return nil
}

// Get loads the enrollment for the scope.
func (s *MFAStore) Get(ctx context.Context, organizationID, userID string) (auth.MFAEnrollment, error) {
	var enrollment auth.MFAEnrollment

	err := s.col.FindOne(ctx, bson.M{mfaDocIDField: auth.MFAScopeID(organizationID, userID)}).Decode(&enrollment)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return auth.MFAEnrollment{}, auth.ErrMFANotEnrolled
	}

	if err != nil {
		return auth.MFAEnrollment{}, fmt.Errorf("load mfa enrollment: %w", err)
	}

	return enrollment, nil
}

// Upsert replaces the enrollment for the scope.
func (s *MFAStore) Upsert(ctx context.Context, enrollment auth.MFAEnrollment) error {
	if _, err := s.col.ReplaceOne(
		ctx,
		bson.M{mfaDocIDField: enrollment.ID},
		enrollment,
		options.Replace().SetUpsert(true),
	); err != nil {
		return fmt.Errorf("upsert mfa enrollment: %w", err)
	}

	return nil
}

// ConsumeBackupCode atomically pulls one matching backup-code hash from the
// enrollment, reporting whether one matched; the array-element filter makes
// the code single-use under concurrent redemption.
func (s *MFAStore) ConsumeBackupCode(
	ctx context.Context,
	organizationID, userID, codeHash string,
) (bool, error) {
	result, err := s.col.UpdateOne(
		ctx,
		bson.M{
			mfaDocIDField:       auth.MFAScopeID(organizationID, userID),
			mfaBackupCodesField: codeHash,
		},
		bson.M{"$pull": bson.M{mfaBackupCodesField: codeHash}},
	)
	if err != nil {
		return false, fmt.Errorf("consume backup code: %w", err)
	}

	return result.MatchedCount > 0, nil
}

// Delete removes the enrollment for the scope (MFA disabled).
func (s *MFAStore) Delete(ctx context.Context, organizationID, userID string) error {
	if _, err := s.col.DeleteOne(ctx, bson.M{mfaDocIDField: auth.MFAScopeID(organizationID, userID)}); err != nil {
		return fmt.Errorf("delete mfa enrollment: %w", err)
	}

	return nil
}

// MFATicketStore persists single-use, expiring MFA login tickets in the
// auth_mfa_tickets collection.
type MFATicketStore struct {
	col *drivermongo.Collection
}

// NewMFATicketStore constructs an MFATicketStore.
func NewMFATicketStore(db *drivermongo.Database) *MFATicketStore {
	return &MFATicketStore{col: db.Collection("auth_mfa_tickets")}
}

// EnsureIndexes creates a unique index on the ticket hash and a TTL index that
// purges expired tickets.
func (s *MFATicketStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: mfaTicketHashField, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: mfaTicketExpiryField, Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure mfa ticket indexes: %w", err)
	}

	return nil
}

// Save inserts an MFA login ticket.
func (s *MFATicketStore) Save(ctx context.Context, ticket auth.MFATicket) error {
	if _, err := s.col.InsertOne(ctx, ticket); err != nil {
		return fmt.Errorf("insert mfa ticket: %w", err)
	}

	return nil
}

// Consume atomically finds and deletes a non-expired ticket by hash, so
// concurrent redemptions of the same ticket cannot both succeed. The expiry
// filter guards the window before the TTL reaper removes an expired document.
func (s *MFATicketStore) Consume(ctx context.Context, ticketHash string) (auth.MFATicket, error) {
	var ticket auth.MFATicket

	err := s.col.FindOneAndDelete(ctx, bson.M{
		mfaTicketHashField:   ticketHash,
		mfaTicketExpiryField: bson.M{mongoGreaterThan: time.Now().UTC()},
	}).Decode(&ticket)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return auth.MFATicket{}, auth.ErrMFATicketInvalid
	}

	if err != nil {
		return auth.MFATicket{}, fmt.Errorf("consume mfa ticket: %w", err)
	}

	return ticket, nil
}

// DeleteForOrganization removes every MFA enrollment belonging to a tenant (GDPR purge).
func (s *MFAStore) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	result, err := s.col.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete mfa enrollments for organization: %w", err)
	}

	return result.DeletedCount, nil
}
