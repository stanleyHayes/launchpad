package billing

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository persists billing plans and subscriptions.
type Repository interface {
	UpsertPlan(ctx context.Context, plan Plan) error
	GetPlan(ctx context.Context, code string) (Plan, error)
	ListPlans(ctx context.Context, activeOnly bool) ([]Plan, error)
	CreatePlan(ctx context.Context, plan Plan) error
	UpdatePlan(ctx context.Context, plan Plan) error
	GetSubscriptionByOrganization(ctx context.Context, organizationID string) (Subscription, error)
	CreateSubscription(ctx context.Context, subscription Subscription) error
	UpdateSubscription(ctx context.Context, subscription Subscription) error
	ListSubscriptions(ctx context.Context) ([]Subscription, error)
}

// Store is the MongoDB billing repository.
type Store struct {
	plans         *mongo.Collection
	subscriptions *mongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *mongo.Database) *Store {
	return &Store{
		plans:         db.Collection("billing_plans"),
		subscriptions: db.Collection("billing_subscriptions"),
	}
}

// EnsureIndexes creates billing indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.subscriptions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure billing subscription indexes: %w", err)
	}

	return nil
}

// UpsertPlan inserts or replaces a plan by code.
func (s *Store) UpsertPlan(ctx context.Context, plan Plan) error {
	opts := options.Replace().SetUpsert(true)

	_, err := s.plans.ReplaceOne(ctx, bson.M{fieldID: plan.Code}, plan, opts)
	if err != nil {
		return fmt.Errorf("upsert billing plan: %w", err)
	}

	return nil
}

// CreatePlan inserts a new plan.
func (s *Store) CreatePlan(ctx context.Context, plan Plan) error {
	_, err := s.plans.InsertOne(ctx, plan)
	if mongo.IsDuplicateKeyError(err) {
		return ErrCodeTaken
	}

	if err != nil {
		return fmt.Errorf("insert billing plan: %w", err)
	}

	return nil
}

// GetPlan loads a plan by code.
func (s *Store) GetPlan(ctx context.Context, code string) (Plan, error) {
	var plan Plan

	err := s.plans.FindOne(ctx, bson.M{fieldID: code}).Decode(&plan)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Plan{}, ErrNotFound
	}

	if err != nil {
		return Plan{}, fmt.Errorf("find billing plan: %w", err)
	}

	return plan, nil
}

// ListPlans returns billing plans.
func (s *Store) ListPlans(ctx context.Context, activeOnly bool) ([]Plan, error) {
	filter := bson.M{}
	if activeOnly {
		filter[fieldActive] = true
	}

	cursor, err := s.plans.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: fieldID, Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("find billing plans: %w", err)
	}

	items := make([]Plan, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode billing plans: %w", decodeErr),
			fmt.Errorf("close billing plans cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode billing plans: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close billing plans cursor: %w", closeErr)
	}

	return items, nil
}

// UpdatePlan replaces an existing plan.
func (s *Store) UpdatePlan(ctx context.Context, plan Plan) error {
	res, err := s.plans.ReplaceOne(ctx, bson.M{fieldID: plan.Code}, plan)
	if err != nil {
		return fmt.Errorf("replace billing plan: %w", err)
	}

	if res.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// GetSubscriptionByOrganization loads a subscription for a tenant.
func (s *Store) GetSubscriptionByOrganization(ctx context.Context, organizationID string) (Subscription, error) {
	var subscription Subscription

	err := s.subscriptions.FindOne(ctx, bson.M{fieldOrganizationID: organizationID}).Decode(&subscription)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Subscription{}, ErrNotFound
	}

	if err != nil {
		return Subscription{}, fmt.Errorf("find billing subscription: %w", err)
	}

	return subscription, nil
}

// CreateSubscription inserts a subscription.
func (s *Store) CreateSubscription(ctx context.Context, subscription Subscription) error {
	_, err := s.subscriptions.InsertOne(ctx, subscription)
	if mongo.IsDuplicateKeyError(err) {
		return ErrInvalidInput
	}

	if err != nil {
		return fmt.Errorf("insert billing subscription: %w", err)
	}

	return nil
}

// UpdateSubscription replaces a subscription.
func (s *Store) UpdateSubscription(ctx context.Context, subscription Subscription) error {
	res, err := s.subscriptions.ReplaceOne(ctx, bson.M{fieldID: subscription.ID}, subscription)
	if err != nil {
		return fmt.Errorf("replace billing subscription: %w", err)
	}

	if res.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// ListSubscriptions returns all subscriptions.
func (s *Store) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	cursor, err := s.subscriptions.Find(
		ctx,
		bson.M{},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find billing subscriptions: %w", err)
	}

	items := make([]Subscription, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode billing subscriptions: %w", decodeErr),
			fmt.Errorf("close billing subscriptions cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode billing subscriptions: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close billing subscriptions cursor: %w", closeErr)
	}

	return items, nil
}
