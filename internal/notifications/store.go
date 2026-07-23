package notifications

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store persists in-app notifications.
type Store struct {
	col *mongo.Collection
}

// NewStore constructs a notification Store.
func NewStore(db *mongo.Database) *Store {
	return &Store{col: db.Collection("notifications")}
}

// EnsureIndexes creates notification indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{
			{Key: fieldOrganizationID, Value: 1},
			{Key: fieldUserID, Value: 1},
			{Key: fieldCreatedAt, Value: -1},
		}},
	})
	if err != nil {
		return fmt.Errorf("ensure notification indexes: %w", err)
	}

	return nil
}

// Create inserts a notification.
func (s *Store) Create(ctx context.Context, notification Notification) error {
	_, err := s.col.InsertOne(ctx, notification)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}

	return nil
}

// ListForUser returns notifications belonging to a user in an organization.
func (s *Store) ListForUser(ctx context.Context, organizationID, userID string) ([]Notification, error) {
	cursor, err := s.col.Find(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldUserID:         userID,
	}, options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("find notifications: %w", err)
	}

	items := make([]Notification, 0)
	decodeErr := cursor.All(ctx, &items)

	closeErr := cursor.Close(ctx)
	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode notifications: %w", decodeErr),
			fmt.Errorf("close notifications cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode notifications: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close notifications cursor: %w", closeErr)
	}

	return items, nil
}

// Get returns one notification scoped to its recipient and organization.
func (s *Store) Get(ctx context.Context, organizationID, userID, notificationID string) (Notification, error) {
	var notification Notification

	err := s.col.FindOne(ctx, bson.M{
		"_id":               notificationID,
		fieldOrganizationID: organizationID,
		fieldUserID:         userID,
	}).Decode(&notification)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Notification{}, ErrNotFound
	}

	if err != nil {
		return Notification{}, fmt.Errorf("find notification: %w", err)
	}

	return notification, nil
}

// Update replaces a notification scoped to its recipient and organization.
func (s *Store) Update(ctx context.Context, notification Notification) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{
		"_id":               notification.ID,
		fieldOrganizationID: notification.OrganizationID,
		fieldUserID:         notification.UserID,
	}, notification)
	if err != nil {
		return fmt.Errorf("replace notification: %w", err)
	}

	if res.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}
