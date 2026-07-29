// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/billing"
)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldCode           = "code"
	fieldActive         = "active"
	fieldCreatedAt      = "createdAt"
	fieldNumber         = "number"
	fieldPaymentRef     = "paymentRef"
	fieldSequence       = "seq"
)

var _ billing.Repository = (*Store)(nil)

// Store is the MongoDB billing repository.
type Store struct {
	plans         *drivermongo.Collection
	subscriptions *drivermongo.Collection
	invoices      *drivermongo.Collection
	sequences     *drivermongo.Collection
	coupons       *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{
		plans:         db.Collection("billing_plans"),
		subscriptions: db.Collection("billing_subscriptions"),
		invoices:      db.Collection("billing_invoices"),
		sequences:     db.Collection("billing_sequences"),
		coupons:       db.Collection("billing_coupons"),
	}
}

func (s *Store) CreateCoupon(ctx context.Context, coupon billing.Coupon) error {
	_, err := s.coupons.InsertOne(ctx, coupon)
	if drivermongo.IsDuplicateKeyError(err) {
		return billing.ErrCodeTaken
	}
	if err != nil {
		return fmt.Errorf("insert billing coupon: %w", err)
	}
	return nil
}

func (s *Store) GetCoupon(ctx context.Context, code string) (billing.Coupon, error) {
	var coupon billing.Coupon
	err := s.coupons.FindOne(ctx, bson.M{fieldID: code}).Decode(&coupon)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return billing.Coupon{}, billing.ErrNotFound
	}
	if err != nil {
		return billing.Coupon{}, fmt.Errorf("find billing coupon: %w", err)
	}
	return coupon, nil
}

func (s *Store) ListCoupons(ctx context.Context) ([]billing.Coupon, error) {
	cursor, err := s.coupons.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("find billing coupons: %w", err)
	}
	items := make([]billing.Coupon, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode billing coupons: %w", err)
	}
	if err := cursor.Close(ctx); err != nil {
		return nil, fmt.Errorf("close billing coupons: %w", err)
	}
	return items, nil
}

func (s *Store) UpdateCoupon(ctx context.Context, coupon billing.Coupon) error {
	result, err := s.coupons.ReplaceOne(ctx, bson.M{fieldID: coupon.Code}, coupon)
	if err != nil {
		return fmt.Errorf("replace billing coupon: %w", err)
	}
	if result.MatchedCount == 0 {
		return billing.ErrNotFound
	}
	return nil
}

// EnsureIndexes creates billing indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.subscriptions.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure billing subscription indexes: %w", err)
	}

	_, err = s.invoices.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldNumber, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			// Sparse: only invoices that reached hosted checkout carry a
			// payment reference; webhook idempotency keys on it.
			Keys:    bson.D{{Key: fieldPaymentRef, Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure billing invoice indexes: %w", err)
	}

	return nil
}

// UpsertPlan inserts or replaces a plan by code.
func (s *Store) UpsertPlan(ctx context.Context, plan billing.Plan) error {
	opts := options.Replace().SetUpsert(true)

	_, err := s.plans.ReplaceOne(ctx, bson.M{fieldID: plan.Code}, plan, opts)
	if err != nil {
		return fmt.Errorf("upsert billing plan: %w", err)
	}

	return nil
}

// CreatePlan inserts a new plan.
func (s *Store) CreatePlan(ctx context.Context, plan billing.Plan) error {
	_, err := s.plans.InsertOne(ctx, plan)
	if drivermongo.IsDuplicateKeyError(err) {
		return billing.ErrCodeTaken
	}

	if err != nil {
		return fmt.Errorf("insert billing plan: %w", err)
	}

	return nil
}

// GetPlan loads a plan by code.
func (s *Store) GetPlan(ctx context.Context, code string) (billing.Plan, error) {
	var plan billing.Plan

	err := s.plans.FindOne(ctx, bson.M{fieldID: code}).Decode(&plan)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return billing.Plan{}, billing.ErrNotFound
	}

	if err != nil {
		return billing.Plan{}, fmt.Errorf("find billing plan: %w", err)
	}

	return plan, nil
}

// ListPlans returns billing plans.
func (s *Store) ListPlans(ctx context.Context, activeOnly bool) ([]billing.Plan, error) {
	filter := bson.M{}
	if activeOnly {
		filter[fieldActive] = true
	}

	cursor, err := s.plans.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: fieldID, Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("find billing plans: %w", err)
	}

	items := make([]billing.Plan, 0)
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
func (s *Store) UpdatePlan(ctx context.Context, plan billing.Plan) error {
	res, err := s.plans.ReplaceOne(ctx, bson.M{fieldID: plan.Code}, plan)
	if err != nil {
		return fmt.Errorf("replace billing plan: %w", err)
	}

	if res.MatchedCount == 0 {
		return billing.ErrNotFound
	}

	return nil
}

// GetSubscriptionByOrganization loads a subscription for a tenant.
func (s *Store) GetSubscriptionByOrganization(
	ctx context.Context,
	organizationID string,
) (billing.Subscription, error) {
	var subscription billing.Subscription

	err := s.subscriptions.FindOne(ctx, bson.M{fieldOrganizationID: organizationID}).Decode(&subscription)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return billing.Subscription{}, billing.ErrNotFound
	}

	if err != nil {
		return billing.Subscription{}, fmt.Errorf("find billing subscription: %w", err)
	}

	return subscription, nil
}

// CreateSubscription inserts a subscription.
func (s *Store) CreateSubscription(ctx context.Context, subscription billing.Subscription) error {
	_, err := s.subscriptions.InsertOne(ctx, subscription)
	if drivermongo.IsDuplicateKeyError(err) {
		return billing.ErrInvalidInput
	}

	if err != nil {
		return fmt.Errorf("insert billing subscription: %w", err)
	}

	return nil
}

// UpdateSubscription replaces a subscription.
func (s *Store) UpdateSubscription(ctx context.Context, subscription billing.Subscription) error {
	res, err := s.subscriptions.ReplaceOne(ctx, bson.M{fieldID: subscription.ID}, subscription)
	if err != nil {
		return fmt.Errorf("replace billing subscription: %w", err)
	}

	if res.MatchedCount == 0 {
		return billing.ErrNotFound
	}

	return nil
}

// ListSubscriptions returns all subscriptions.
func (s *Store) ListSubscriptions(ctx context.Context) ([]billing.Subscription, error) {
	cursor, err := s.subscriptions.Find(
		ctx,
		bson.M{},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find billing subscriptions: %w", err)
	}

	items := make([]billing.Subscription, 0)
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

// NextInvoiceSequence atomically increments and returns the per-organization
// invoice sequence. The counter document is created on first use.
func (s *Store) NextInvoiceSequence(ctx context.Context, organizationID string) (int64, error) {
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var counter struct {
		Seq int64 `bson:"seq"`
	}

	err := s.sequences.FindOneAndUpdate(
		ctx,
		bson.M{fieldID: organizationID},
		bson.M{"$inc": bson.M{fieldSequence: 1}},
		opts,
	).Decode(&counter)
	if err != nil {
		return 0, fmt.Errorf("increment invoice sequence: %w", err)
	}

	return counter.Seq, nil
}

// CreateInvoice inserts an invoice.
func (s *Store) CreateInvoice(ctx context.Context, invoice billing.Invoice) error {
	_, err := s.invoices.InsertOne(ctx, invoice)
	if drivermongo.IsDuplicateKeyError(err) {
		return billing.ErrInvalidInput
	}

	if err != nil {
		return fmt.Errorf("insert billing invoice: %w", err)
	}

	return nil
}

// GetInvoice loads an org-scoped invoice.
func (s *Store) GetInvoice(ctx context.Context, organizationID, invoiceID string) (billing.Invoice, error) {
	var invoice billing.Invoice

	err := s.invoices.FindOne(
		ctx,
		bson.M{fieldID: invoiceID, fieldOrganizationID: organizationID},
	).Decode(&invoice)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return billing.Invoice{}, billing.ErrNotFound
	}

	if err != nil {
		return billing.Invoice{}, fmt.Errorf("find billing invoice: %w", err)
	}

	return invoice, nil
}

// GetInvoiceByPaymentRef loads an invoice by its payment provider reference.
func (s *Store) GetInvoiceByPaymentRef(ctx context.Context, reference string) (billing.Invoice, error) {
	var invoice billing.Invoice

	err := s.invoices.FindOne(ctx, bson.M{fieldPaymentRef: reference}).Decode(&invoice)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return billing.Invoice{}, billing.ErrNotFound
	}

	if err != nil {
		return billing.Invoice{}, fmt.Errorf("find billing invoice by payment ref: %w", err)
	}

	return invoice, nil
}

// ListInvoicesByOrganization returns an organization's invoices, newest first.
func (s *Store) ListInvoicesByOrganization(
	ctx context.Context,
	organizationID string,
) ([]billing.Invoice, error) {
	cursor, err := s.invoices.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find billing invoices: %w", err)
	}

	items := make([]billing.Invoice, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode billing invoices: %w", decodeErr),
			fmt.Errorf("close billing invoices cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode billing invoices: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close billing invoices cursor: %w", closeErr)
	}

	return items, nil
}

// ListInvoices returns every invoice for platform billing operations.
func (s *Store) ListInvoices(ctx context.Context) ([]billing.Invoice, error) {
	cursor, err := s.invoices.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("find all billing invoices: %w", err)
	}
	items := make([]billing.Invoice, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode all billing invoices: %w", decodeErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close all billing invoices: %w", closeErr)
	}
	return items, nil
}

// UpdateInvoice replaces an invoice.
func (s *Store) UpdateInvoice(ctx context.Context, invoice billing.Invoice) error {
	res, err := s.invoices.ReplaceOne(ctx, bson.M{fieldID: invoice.ID}, invoice)
	if err != nil {
		return fmt.Errorf("replace billing invoice: %w", err)
	}

	if res.MatchedCount == 0 {
		return billing.ErrNotFound
	}

	return nil
}

// DeleteForOrganization removes every billing subscription, invoice, and
// invoice sequence of the organization and returns the number deleted. Plans
// are platform-wide and are kept. It serves only the platform GDPR tenant
// purge (PRD 7.4).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	var deleted int64

	res, err := s.subscriptions.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete billing subscriptions for organization: %w", err)
	}

	deleted += res.DeletedCount

	invoiceRes, err := s.invoices.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete billing invoices for organization: %w", err)
	}

	deleted += invoiceRes.DeletedCount

	if _, err := s.sequences.DeleteOne(ctx, bson.M{fieldID: organizationID}); err != nil {
		return 0, fmt.Errorf("delete billing invoice sequence for organization: %w", err)
	}

	return deleted, nil
}
