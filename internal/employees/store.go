package employees

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store is the MongoDB employee repository.
type Store struct {
	col *mongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *mongo.Database) *Store {
	return &Store{col: db.Collection("employees")}
}

// EnsureIndexes creates employee indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: fieldOrganizationID, Value: 1},
				{Key: fieldWorkEmail, Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldUserID, Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure employee indexes: %w", err)
	}

	return nil
}

// Create inserts an employee.
func (s *Store) Create(ctx context.Context, employee Employee) error {
	_, err := s.col.InsertOne(ctx, employee)
	if mongo.IsDuplicateKeyError(err) {
		return ErrEmailTaken
	}

	if err != nil {
		return fmt.Errorf("insert employee: %w", err)
	}

	return nil
}

// GetByID returns an employee scoped to an organization.
func (s *Store) GetByID(ctx context.Context, organizationID, employeeID string) (Employee, error) {
	var employee Employee

	err := s.col.FindOne(ctx, bson.M{
		fieldID:             employeeID,
		fieldOrganizationID: organizationID,
	}).Decode(&employee)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Employee{}, ErrNotFound
	}

	if err != nil {
		return Employee{}, fmt.Errorf("find employee: %w", err)
	}

	return employee, nil
}

// GetByUserID returns an employee linked to a user, scoped to an organization.
func (s *Store) GetByUserID(ctx context.Context, organizationID, userID string) (Employee, error) {
	var employee Employee

	err := s.col.FindOne(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldUserID:         userID,
	}).Decode(&employee)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Employee{}, ErrNotFound
	}

	if err != nil {
		return Employee{}, fmt.Errorf("find employee by user: %w", err)
	}

	return employee, nil
}

// List returns employees for an organization.
func (s *Store) List(ctx context.Context, organizationID string, limit int64) ([]Employee, error) {
	if limit <= 0 || limit > maxListLimit {
		limit = defaultListLimit
	}

	cursor, err := s.col.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}).SetLimit(limit),
	)
	if err != nil {
		return nil, fmt.Errorf("find employees: %w", err)
	}

	items := make([]Employee, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode employees: %w", decodeErr),
			fmt.Errorf("close employees cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode employees: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close employees cursor: %w", closeErr)
	}

	return items, nil
}

// Update replaces an employee document.
func (s *Store) Update(ctx context.Context, employee Employee) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{
		fieldID:             employee.ID,
		fieldOrganizationID: employee.OrganizationID,
	}, employee)
	if err != nil {
		return fmt.Errorf("replace employee: %w", err)
	}

	if res.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// ProvisionAccess links an employee to a user if it is not already linked.
func (s *Store) ProvisionAccess(ctx context.Context, organizationID, employeeID, userID string) error {
	res, err := s.col.UpdateOne(ctx, bson.M{
		fieldID:             employeeID,
		fieldOrganizationID: organizationID,
		fieldUserID:         "",
	}, bson.M{"$set": bson.M{fieldUserID: userID}})
	if err != nil {
		return fmt.Errorf("provision employee access: %w", err)
	}

	if res.MatchedCount > 0 {
		return nil
	}

	employee, err := s.GetByID(ctx, organizationID, employeeID)
	if err != nil {
		return fmt.Errorf("get employee after provision access: %w", err)
	}

	if employee.UserID != "" {
		return ErrAlreadyProvisioned
	}

	return ErrNotFound
}
