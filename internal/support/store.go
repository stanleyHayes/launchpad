package support

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository persists support tickets.
type Repository interface {
	Create(ctx context.Context, ticket Ticket) error
	GetByID(ctx context.Context, id string) (Ticket, error)
	GetByIDForOrganization(ctx context.Context, organizationID, id string) (Ticket, error)
	ListByOrganization(ctx context.Context, organizationID string) ([]Ticket, error)
	ListAll(ctx context.Context) ([]Ticket, error)
	Update(ctx context.Context, ticket Ticket) error
	CountOpen(ctx context.Context) (int64, error)
}

// Store is the MongoDB support repository.
type Store struct {
	col *mongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *mongo.Database) *Store {
	return &Store{col: db.Collection("support_tickets")}
}

// EnsureIndexes creates support ticket indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: fieldStatus, Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure support ticket indexes: %w", err)
	}

	return nil
}

// Create inserts a support ticket.
func (s *Store) Create(ctx context.Context, ticket Ticket) error {
	_, err := s.col.InsertOne(ctx, ticket)
	if err != nil {
		return fmt.Errorf("insert support ticket: %w", err)
	}

	return nil
}

// GetByID loads a ticket by id.
func (s *Store) GetByID(ctx context.Context, id string) (Ticket, error) {
	var ticket Ticket

	err := s.col.FindOne(ctx, bson.M{fieldID: id}).Decode(&ticket)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Ticket{}, ErrNotFound
	}

	if err != nil {
		return Ticket{}, fmt.Errorf("find support ticket: %w", err)
	}

	return ticket, nil
}

// GetByIDForOrganization loads a ticket scoped to a tenant.
func (s *Store) GetByIDForOrganization(ctx context.Context, organizationID, id string) (Ticket, error) {
	var ticket Ticket

	err := s.col.FindOne(ctx, bson.M{
		fieldID:             id,
		fieldOrganizationID: organizationID,
	}).Decode(&ticket)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Ticket{}, ErrNotFound
	}

	if err != nil {
		return Ticket{}, fmt.Errorf("find support ticket for organization: %w", err)
	}

	return ticket, nil
}

// ListByOrganization returns tickets for a tenant.
func (s *Store) ListByOrganization(ctx context.Context, organizationID string) ([]Ticket, error) {
	cursor, err := s.col.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find support tickets: %w", err)
	}

	items := make([]Ticket, 0)
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

// ListAll returns all support tickets.
func (s *Store) ListAll(ctx context.Context) ([]Ticket, error) {
	cursor, err := s.col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("find all support tickets: %w", err)
	}

	items := make([]Ticket, 0)
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
func (s *Store) Update(ctx context.Context, ticket Ticket) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{fieldID: ticket.ID}, ticket)
	if err != nil {
		return fmt.Errorf("replace support ticket: %w", err)
	}

	if res.MatchedCount == 0 {
		return ErrNotFound
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
