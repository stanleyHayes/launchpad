// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/requests"
)

const (
	fieldID                  = "_id"
	fieldOrganizationID      = "organizationId"
	fieldRequesterEmployeeID = "requesterEmployeeId"
	fieldStatus              = "status"
	fieldCreatedAt           = "createdAt"
)

var _ requests.Repository = (*Store)(nil)

// Store is the MongoDB requests repository.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("requests")}
}

// EnsureIndexes creates request indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldStatus, Value: 1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldRequesterEmployeeID, Value: 1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure request indexes: %w", err)
	}

	return nil
}

// Create inserts a request.
func (s *Store) Create(ctx context.Context, request requests.Request) error {
	_, err := s.col.InsertOne(ctx, request)
	if err != nil {
		return fmt.Errorf("insert request: %w", err)
	}

	return nil
}

// GetByIDForOrganization loads a request scoped to a tenant.
func (s *Store) GetByIDForOrganization(
	ctx context.Context,
	organizationID, id string,
) (requests.Request, error) {
	var request requests.Request

	err := s.col.FindOne(ctx, bson.M{
		fieldID:             id,
		fieldOrganizationID: organizationID,
	}).Decode(&request)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return requests.Request{}, requests.ErrNotFound
	}

	if err != nil {
		return requests.Request{}, fmt.Errorf("find request for organization: %w", err)
	}

	return request, nil
}

// Update replaces a request.
func (s *Store) Update(ctx context.Context, request requests.Request) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{
		fieldID:             request.ID,
		fieldOrganizationID: request.OrganizationID,
	}, request)
	if err != nil {
		return fmt.Errorf("replace request: %w", err)
	}

	if res.MatchedCount == 0 {
		return requests.ErrNotFound
	}

	return nil
}

// ListByOrganization returns requests for a tenant, newest first. An empty
// status returns every status.
func (s *Store) ListByOrganization(
	ctx context.Context,
	organizationID, status string,
) ([]requests.Request, error) {
	filter := bson.M{fieldOrganizationID: organizationID}
	if status != "" {
		filter[fieldStatus] = status
	}

	return s.list(ctx, filter)
}

// ListByRequester returns one employee's requests, newest first.
func (s *Store) ListByRequester(
	ctx context.Context,
	organizationID, employeeID string,
) ([]requests.Request, error) {
	return s.list(ctx, bson.M{
		fieldOrganizationID:      organizationID,
		fieldRequesterEmployeeID: employeeID,
	})
}

// DeleteForOrganization removes every request belonging to a tenant (GDPR purge).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	result, err := s.col.DeleteMany(ctx, bson.M{"organizationId": organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete requests for organization: %w", err)
	}

	return result.DeletedCount, nil
}

func (s *Store) list(ctx context.Context, filter bson.M) ([]requests.Request, error) {
	cursor, err := s.col.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find requests: %w", err)
	}

	items := make([]requests.Request, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode requests: %w", decodeErr),
			fmt.Errorf("close requests cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode requests: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close requests cursor: %w", closeErr)
	}

	return items, nil
}
