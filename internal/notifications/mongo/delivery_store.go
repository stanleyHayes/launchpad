package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/notifications"
)

var _ notifications.DeliveryStore = (*DeliveryStore)(nil)

type DeliveryStore struct {
	col *drivermongo.Collection
}

func NewDeliveryStore(db *drivermongo.Database) *DeliveryStore {
	return &DeliveryStore{col: db.Collection("notification_deliveries")}
}

func (s *DeliveryStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "nextAttemptAt", Value: 1}}},
		{Keys: bson.D{{Key: "organizationId", Value: 1}, {Key: "createdAt", Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure notification delivery indexes: %w", err)
	}
	return nil
}

func (s *DeliveryStore) CreateDelivery(ctx context.Context, delivery notifications.Delivery) error {
	if _, err := s.col.InsertOne(ctx, delivery); err != nil {
		return fmt.Errorf("insert notification delivery: %w", err)
	}
	return nil
}

func (s *DeliveryStore) UpdateDelivery(ctx context.Context, delivery notifications.Delivery) error {
	result, err := s.col.ReplaceOne(ctx, bson.M{"_id": delivery.ID}, delivery)
	if err != nil {
		return fmt.Errorf("replace notification delivery: %w", err)
	}
	if result.MatchedCount == 0 {
		return notifications.ErrNotFound
	}
	return nil
}

func (s *DeliveryStore) GetDelivery(ctx context.Context, id string) (notifications.Delivery, error) {
	var delivery notifications.Delivery
	err := s.col.FindOne(ctx, bson.M{"_id": id}).Decode(&delivery)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return notifications.Delivery{}, notifications.ErrNotFound
	}
	if err != nil {
		return notifications.Delivery{}, fmt.Errorf("find notification delivery: %w", err)
	}
	return delivery, nil
}

func (s *DeliveryStore) ListDeliveries(ctx context.Context) ([]notifications.Delivery, error) {
	cursor, err := s.col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(500))
	if err != nil {
		return nil, fmt.Errorf("list notification deliveries: %w", err)
	}
	items := make([]notifications.Delivery, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode notification deliveries: %w", err)
	}
	if err := cursor.Close(ctx); err != nil {
		return nil, fmt.Errorf("close notification deliveries: %w", err)
	}
	return items, nil
}

func (s *DeliveryStore) ListDueDeliveries(ctx context.Context, now time.Time) ([]notifications.Delivery, error) {
	cursor, err := s.col.Find(ctx, bson.M{
		"status":        notifications.DeliveryRetrying,
		"nextAttemptAt": bson.M{"$lte": now},
	}, options.Find().SetSort(bson.D{{Key: "nextAttemptAt", Value: 1}}).SetLimit(100))
	if err != nil {
		return nil, fmt.Errorf("list due notification deliveries: %w", err)
	}
	items := make([]notifications.Delivery, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode due notification deliveries: %w", err)
	}
	if err := cursor.Close(ctx); err != nil {
		return nil, fmt.Errorf("close due notification deliveries: %w", err)
	}
	return items, nil
}

func (s *DeliveryStore) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	result, err := s.col.DeleteMany(ctx, bson.M{"organizationId": organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete notification deliveries: %w", err)
	}
	return result.DeletedCount, nil
}
