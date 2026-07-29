// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/support"
)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldEmployeeID     = "employeeId"
	fieldStatus         = "status"
	fieldCreatedAt      = "createdAt"

	statusOpen       = "open"
	statusInProgress = "in_progress"
	statusWaiting    = "waiting"
)

var _ support.Repository = (*Store)(nil)

// Store is the MongoDB support repository.
type Store struct {
	col      *drivermongo.Collection
	blockers *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{
		col:      db.Collection("support_tickets"),
		blockers: db.Collection("support_blockers"),
	}
}

// EnsureIndexes creates support ticket and blocker indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: fieldStatus, Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure support ticket indexes: %w", err)
	}

	_, err = s.blockers.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldEmployeeID, Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure support blocker indexes: %w", err)
	}

	return nil
}

// Create inserts a support ticket.
func (s *Store) Create(ctx context.Context, ticket support.Ticket) error {
	_, err := s.col.InsertOne(ctx, ticket)
	if err != nil {
		return fmt.Errorf("insert support ticket: %w", err)
	}

	return nil
}

// GetByID loads a ticket by id.
func (s *Store) GetByID(ctx context.Context, id string) (support.Ticket, error) {
	var ticket support.Ticket

	err := s.col.FindOne(ctx, bson.M{fieldID: id}).Decode(&ticket)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return support.Ticket{}, support.ErrNotFound
	}

	if err != nil {
		return support.Ticket{}, fmt.Errorf("find support ticket: %w", err)
	}

	return ticket, nil
}

// GetByIDForOrganization loads a ticket scoped to a tenant.
func (s *Store) GetByIDForOrganization(ctx context.Context, organizationID, id string) (support.Ticket, error) {
	var ticket support.Ticket

	err := s.col.FindOne(ctx, bson.M{
		fieldID:             id,
		fieldOrganizationID: organizationID,
	}).Decode(&ticket)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return support.Ticket{}, support.ErrNotFound
	}

	if err != nil {
		return support.Ticket{}, fmt.Errorf("find support ticket for organization: %w", err)
	}

	return ticket, nil
}

// ListByOrganization returns tickets for a tenant.
func (s *Store) ListByOrganization(ctx context.Context, organizationID string) ([]support.Ticket, error) {
	return listForOrganization[support.Ticket](ctx, s.col, organizationID, "support tickets")
}

// ListAll returns all support tickets.
func (s *Store) ListAll(ctx context.Context) ([]support.Ticket, error) {
	cursor, err := s.col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("find all support tickets: %w", err)
	}

	items := make([]support.Ticket, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode support tickets: %w", decodeErr),
			fmt.Errorf("close support tickets cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode support tickets: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close support tickets cursor: %w", closeErr)
	}

	return items, nil
}

// Update replaces a support ticket.
func (s *Store) Update(ctx context.Context, ticket support.Ticket) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{fieldID: ticket.ID}, ticket)
	if err != nil {
		return fmt.Errorf("replace support ticket: %w", err)
	}

	if res.MatchedCount == 0 {
		return support.ErrNotFound
	}

	return nil
}

// CountOpen returns tickets in open workflow states.
func (s *Store) CountOpen(ctx context.Context) (int64, error) {
	count, err := s.col.CountDocuments(ctx, bson.M{
		fieldStatus: bson.M{"$in": []string{statusOpen, statusInProgress, statusWaiting}},
	})
	if err != nil {
		return 0, fmt.Errorf("count open support tickets: %w", err)
	}

	return count, nil
}

// CreateBlocker inserts an employee blocker.
func (s *Store) CreateBlocker(ctx context.Context, blocker support.Blocker) error {
	_, err := s.blockers.InsertOne(ctx, blocker)
	if err != nil {
		return fmt.Errorf("insert blocker: %w", err)
	}

	return nil
}

// ListBlockers returns blockers for a tenant, newest first.
func (s *Store) ListBlockers(ctx context.Context, organizationID string) ([]support.Blocker, error) {
	return listForOrganization[support.Blocker](ctx, s.blockers, organizationID, "blockers")
}

// listForOrganization returns documents of one collection for a tenant,
// newest first.
func listForOrganization[T any](
	ctx context.Context,
	col *drivermongo.Collection,
	organizationID, entity string,
) ([]T, error) {
	cursor, err := col.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find %s: %w", entity, err)
	}

	items := make([]T, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode %s: %w", entity, decodeErr),
			fmt.Errorf("close %s cursor: %w", entity, closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode %s: %w", entity, decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close %s cursor: %w", entity, closeErr)
	}

	return items, nil
}

// DeleteForOrganization removes every support ticket and blocker of the
// organization and returns the number of documents deleted. It serves only
// the platform GDPR tenant purge (PRD 7.4).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete support tickets for organization: %w", err)
	}

	deleted := res.DeletedCount

	res, err = s.blockers.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete support blockers for organization: %w", err)
	}

	return deleted + res.DeletedCount, nil
}
