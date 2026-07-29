// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/supportsessions"
)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldCreatedAt      = "createdAt"
)

var _ supportsessions.Repository = (*Store)(nil)

// Store is the MongoDB support session repository.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("support_sessions")}
}

// EnsureIndexes creates support session indexes. Expiry is enforced at
// validation time (Service.Active), so no TTL index is created: ended and
// expired sessions are part of the audit trail and must be retained.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure support session indexes: %w", err)
	}

	return nil
}

// Create inserts a support session.
func (s *Store) Create(ctx context.Context, session supportsessions.Session) error {
	_, err := s.col.InsertOne(ctx, session)
	if err != nil {
		return fmt.Errorf("insert support session: %w", err)
	}

	return nil
}

// GetByID loads a support session by id.
func (s *Store) GetByID(ctx context.Context, id string) (supportsessions.Session, error) {
	var session supportsessions.Session

	err := s.col.FindOne(ctx, bson.M{fieldID: id}).Decode(&session)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return supportsessions.Session{}, supportsessions.ErrNotFound
	}

	if err != nil {
		return supportsessions.Session{}, fmt.Errorf("find support session: %w", err)
	}

	return session, nil
}

// Update replaces a support session.
func (s *Store) Update(ctx context.Context, session supportsessions.Session) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{fieldID: session.ID}, session)
	if err != nil {
		return fmt.Errorf("replace support session: %w", err)
	}

	if res.MatchedCount == 0 {
		return supportsessions.ErrNotFound
	}

	return nil
}

// ListByOrganization returns the tenant's support sessions, newest first.
func (s *Store) ListByOrganization(
	ctx context.Context,
	organizationID string,
) ([]supportsessions.Session, error) {
	cursor, err := s.col.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find support sessions: %w", err)
	}

	items := make([]supportsessions.Session, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode support sessions: %w", decodeErr),
			fmt.Errorf("close support sessions cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode support sessions: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close support sessions cursor: %w", closeErr)
	}

	return items, nil
}
