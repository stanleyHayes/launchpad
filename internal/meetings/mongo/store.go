// Package mongo is the MongoDB persistence adapter for the meetings domain.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/meetings"
)

const (
	fieldID                 = "_id"
	fieldOrganizationID     = "organizationId"
	fieldAttendeeEmployeeID = "attendeeEmployeeId"
	fieldStatus             = "status"
	fieldStartsAt           = "startsAt"
	fieldReminderNotifiedAt = "reminderNotifiedAt"
)

var _ meetings.Repository = (*Store)(nil)

// Store is the MongoDB meetings repository.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("meetings")}
}

// EnsureIndexes creates meeting indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldStatus, Value: 1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldAttendeeEmployeeID, Value: 1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldStartsAt, Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure meeting indexes: %w", err)
	}

	return nil
}

// Create inserts a meeting.
func (s *Store) Create(ctx context.Context, meeting meetings.Meeting) error {
	_, err := s.col.InsertOne(ctx, meeting)
	if err != nil {
		return fmt.Errorf("insert meeting: %w", err)
	}

	return nil
}

// GetByIDForOrganization loads a meeting scoped to a tenant.
func (s *Store) GetByIDForOrganization(
	ctx context.Context,
	organizationID, id string,
) (meetings.Meeting, error) {
	var meeting meetings.Meeting

	err := s.col.FindOne(ctx, bson.M{
		fieldID:             id,
		fieldOrganizationID: organizationID,
	}).Decode(&meeting)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return meetings.Meeting{}, meetings.ErrNotFound
	}

	if err != nil {
		return meetings.Meeting{}, fmt.Errorf("find meeting for organization: %w", err)
	}

	return meeting, nil
}

// Update replaces a meeting.
func (s *Store) Update(ctx context.Context, meeting meetings.Meeting) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{
		fieldID:             meeting.ID,
		fieldOrganizationID: meeting.OrganizationID,
	}, meeting)
	if err != nil {
		return fmt.Errorf("replace meeting: %w", err)
	}

	if res.MatchedCount == 0 {
		return meetings.ErrNotFound
	}

	return nil
}

// ListByOrganization returns meetings for a tenant, soonest first. An empty
// status returns every status.
func (s *Store) ListByOrganization(
	ctx context.Context,
	organizationID, status string,
) ([]meetings.Meeting, error) {
	filter := bson.M{fieldOrganizationID: organizationID}
	if status != "" {
		filter[fieldStatus] = status
	}

	return s.list(ctx, filter)
}

// ListByAttendee returns one employee's meetings, soonest first.
func (s *Store) ListByAttendee(
	ctx context.Context,
	organizationID, employeeID string,
) ([]meetings.Meeting, error) {
	return s.list(ctx, bson.M{
		fieldOrganizationID:     organizationID,
		fieldAttendeeEmployeeID: employeeID,
	})
}

func (s *Store) ListUpcomingUnreminded(ctx context.Context, from, to time.Time) ([]meetings.Meeting, error) {
	return s.list(ctx, bson.M{
		fieldStatus:             "scheduled",
		fieldStartsAt:           bson.M{"$gte": from, "$lte": to},
		fieldReminderNotifiedAt: bson.M{"$exists": false},
	})
}

// DeleteForOrganization removes every meeting belonging to a tenant (GDPR purge).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	result, err := s.col.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete meetings for organization: %w", err)
	}

	return result.DeletedCount, nil
}

func (s *Store) list(ctx context.Context, filter bson.M) ([]meetings.Meeting, error) {
	cursor, err := s.col.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: fieldStartsAt, Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find meetings: %w", err)
	}

	items := make([]meetings.Meeting, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode meetings: %w", decodeErr),
			fmt.Errorf("close meetings cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode meetings: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close meetings cursor: %w", closeErr)
	}

	return items, nil
}
