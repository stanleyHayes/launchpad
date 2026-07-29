// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/employees"
)

const (
	fieldID                   = "_id"
	fieldOrganizationID       = "organizationId"
	fieldUserID               = "userId"
	fieldWorkEmail            = "workEmail"
	fieldCreatedAt            = "createdAt"
	fieldUpdatedAt            = "updatedAt"
	fieldStatus               = "status"
	statusActive              = "active"
	defaultListLimit    int64 = 50
	maxListLimit        int64 = 100
)

var _ employees.Repository = (*Store)(nil)

// Store is the MongoDB employee repository.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("employees")}
}

// EnsureIndexes creates employee indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
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
func (s *Store) Create(ctx context.Context, employee employees.Employee) error {
	_, err := s.col.InsertOne(ctx, employee)
	if drivermongo.IsDuplicateKeyError(err) {
		return employees.ErrEmailTaken
	}

	if err != nil {
		return fmt.Errorf("insert employee: %w", err)
	}

	return nil
}

// GetByID returns an employee scoped to an organization.
func (s *Store) GetByID(ctx context.Context, organizationID, employeeID string) (employees.Employee, error) {
	var employee employees.Employee

	err := s.col.FindOne(ctx, bson.M{
		fieldID:             employeeID,
		fieldOrganizationID: organizationID,
	}).Decode(&employee)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return employees.Employee{}, employees.ErrNotFound
	}

	if err != nil {
		return employees.Employee{}, fmt.Errorf("find employee: %w", err)
	}

	return employee, nil
}

// GetByUserID returns an employee linked to a user, scoped to an organization.
func (s *Store) GetByUserID(ctx context.Context, organizationID, userID string) (employees.Employee, error) {
	var employee employees.Employee

	err := s.col.FindOne(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldUserID:         userID,
	}).Decode(&employee)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return employees.Employee{}, employees.ErrNotFound
	}

	if err != nil {
		return employees.Employee{}, fmt.Errorf("find employee by user: %w", err)
	}

	return employee, nil
}

// GetByWorkEmail returns an employee by work email, scoped to an organization.
func (s *Store) GetByWorkEmail(ctx context.Context, organizationID, workEmail string) (employees.Employee, error) {
	var employee employees.Employee

	err := s.col.FindOne(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldWorkEmail:      workEmail,
	}).Decode(&employee)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return employees.Employee{}, employees.ErrNotFound
	}

	if err != nil {
		return employees.Employee{}, fmt.Errorf("find employee by work email: %w", err)
	}

	return employee, nil
}

// List returns a page of employees for an organization, most recent first.
func (s *Store) List(
	ctx context.Context,
	organizationID string,
	offset, limit int64,
) ([]employees.Employee, error) {
	if offset < 0 {
		offset = 0
	}

	switch {
	case limit <= 0:
		limit = defaultListLimit
	case limit > maxListLimit:
		limit = maxListLimit
	}

	cursor, err := s.col.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().
			SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}).
			SetSkip(offset).
			SetLimit(limit),
	)
	if err != nil {
		return nil, fmt.Errorf("find employees: %w", err)
	}

	items := make([]employees.Employee, 0)
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

// Count returns the number of employees in an organization.
func (s *Store) Count(ctx context.Context, organizationID string) (int64, error) {
	count, err := s.col.CountDocuments(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("count employees: %w", err)
	}

	return count, nil
}

// Update replaces an employee document.
func (s *Store) Update(ctx context.Context, employee employees.Employee) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{
		fieldID:             employee.ID,
		fieldOrganizationID: employee.OrganizationID,
	}, employee)
	if err != nil {
		return fmt.Errorf("replace employee: %w", err)
	}

	if res.MatchedCount == 0 {
		return employees.ErrNotFound
	}

	return nil
}

// ProvisionAccess links an employee to a user if it is not already linked.
// The link activates the employee, matching the previous check-then-act
// LinkUser behavior but as a single conditional update.
func (s *Store) ProvisionAccess(ctx context.Context, organizationID, employeeID, userID string) error {
	res, err := s.col.UpdateOne(ctx, bson.M{
		fieldID:             employeeID,
		fieldOrganizationID: organizationID,
		fieldUserID:         "",
	}, bson.M{"$set": bson.M{
		fieldUserID:    userID,
		fieldStatus:    statusActive,
		fieldUpdatedAt: time.Now().UTC(),
	}})
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
		return employees.ErrAlreadyProvisioned
	}

	return employees.ErrNotFound
}

// DeleteForOrganization removes every employee document of the organization
// and returns the number deleted. It serves only the platform GDPR tenant
// purge (PRD 7.4).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.col.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete employees for organization: %w", err)
	}

	return res.DeletedCount, nil
}
