// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/notifications"
	"launchpad/pkg/security"
)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldUserID         = "userId"
	fieldCreatedAt      = "createdAt"
	defaultListLimit    = int64(50)
)

var _ notifications.Repository = (*Store)(nil)

// Store persists in-app notifications.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a notification Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("notifications")}
}

// EnsureIndexes creates notification indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
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
func (s *Store) Create(ctx context.Context, notification notifications.Notification) error {
	_, err := s.col.InsertOne(ctx, notification)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}

	return nil
}

// ListForUser returns notifications belonging to a user in an organization.
func (s *Store) ListForUser(ctx context.Context, organizationID, userID string) ([]notifications.Notification, error) {
	cursor, err := s.col.Find(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldUserID:         userID,
	}, options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}).SetLimit(defaultListLimit))
	if err != nil {
		return nil, fmt.Errorf("find notifications: %w", err)
	}

	items := make([]notifications.Notification, 0)
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
func (s *Store) Get(
	ctx context.Context,
	organizationID, userID, notificationID string,
) (notifications.Notification, error) {
	var notification notifications.Notification

	err := s.col.FindOne(ctx, bson.M{
		"_id":               notificationID,
		fieldOrganizationID: organizationID,
		fieldUserID:         userID,
	}).Decode(&notification)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return notifications.Notification{}, notifications.ErrNotFound
	}

	if err != nil {
		return notifications.Notification{}, fmt.Errorf("find notification: %w", err)
	}

	return notification, nil
}

// Update replaces a notification scoped to its recipient and organization.
func (s *Store) Update(ctx context.Context, notification notifications.Notification) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{
		fieldID:             notification.ID,
		fieldOrganizationID: notification.OrganizationID,
		fieldUserID:         notification.UserID,
	}, notification)
	if err != nil {
		return fmt.Errorf("replace notification: %w", err)
	}

	if res.MatchedCount == 0 {
		return notifications.ErrNotFound
	}

	return nil
}

var _ notifications.ChannelStore = (*ChannelStore)(nil)

// ChannelStore persists per-organization outbound channel configuration, keyed
// by organization id.
type ChannelStore struct {
	col *drivermongo.Collection
}

// NewChannelStore constructs a ChannelStore.
func NewChannelStore(db *drivermongo.Database) *ChannelStore {
	return &ChannelStore{col: db.Collection("notification_channels")}
}

// EnsureIndexes is a no-op: the collection is keyed by _id (organization id).
func (s *ChannelStore) EnsureIndexes(context.Context) error { return nil }

// GetChannels loads a tenant's channel config.
func (s *ChannelStore) GetChannels(ctx context.Context, organizationID string) (notifications.ChannelConfig, error) {
	var config notifications.ChannelConfig

	err := s.col.FindOne(ctx, bson.M{fieldID: organizationID}).Decode(&config)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return notifications.ChannelConfig{}, notifications.ErrNotFound
	}

	if err != nil {
		return notifications.ChannelConfig{}, fmt.Errorf("find channel config: %w", err)
	}

	if config.SlackWebhookURL, err = security.DecryptSecret(config.SlackWebhookURL); err != nil {
		return notifications.ChannelConfig{}, fmt.Errorf("decrypt slack webhook url: %w", err)
	}

	if config.TeamsWebhookURL, err = security.DecryptSecret(config.TeamsWebhookURL); err != nil {
		return notifications.ChannelConfig{}, fmt.Errorf("decrypt teams webhook url: %w", err)
	}

	return config, nil
}

// SetChannels upserts a tenant's channel config. Webhook URLs (bearer
// credentials) are encrypted at rest when ENCRYPTION_KEY is configured.
func (s *ChannelStore) SetChannels(ctx context.Context, config notifications.ChannelConfig) error {
	var err error

	if config.SlackWebhookURL, err = security.EncryptSecret(config.SlackWebhookURL); err != nil {
		return fmt.Errorf("encrypt slack webhook url: %w", err)
	}

	if config.TeamsWebhookURL, err = security.EncryptSecret(config.TeamsWebhookURL); err != nil {
		return fmt.Errorf("encrypt teams webhook url: %w", err)
	}

	_, err = s.col.ReplaceOne(
		ctx,
		bson.M{fieldID: config.OrganizationID},
		config,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("upsert channel config: %w", err)
	}

	return nil
}

// DeleteForOrganization removes every notification of the organization and
// returns the number deleted. It serves only the platform GDPR tenant purge
// (PRD 7.4).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete notifications for organization: %w", err)
	}

	return res.DeletedCount, nil
}

// DeleteForOrganization removes the organization's channel config (keyed by
// organization id) and returns the number of documents deleted. It serves
// only the platform GDPR tenant purge (PRD 7.4).
func (s *ChannelStore) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete notification channels for organization: %w", err)
	}

	return res.DeletedCount, nil
}
