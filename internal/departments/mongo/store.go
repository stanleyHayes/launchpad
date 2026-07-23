// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/departments"
)

const (
	fieldOrganizationID = "organizationId"
	fieldName           = "name"
	fieldCreatedAt      = "createdAt"
)

var _ departments.Repository = (*Store)(nil)

// Store is the MongoDB repository for departments and job roles.
type Store struct {
	departments *drivermongo.Collection
	jobRoles    *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{
		departments: db.Collection("departments"),
		jobRoles:    db.Collection("job_roles"),
	}
}

// EnsureIndexes creates department and job-role indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.departments.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys: bson.D{
				{Key: fieldOrganizationID, Value: 1},
				{Key: fieldName, Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure department indexes: %w", err)
	}

	_, err = s.jobRoles.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys: bson.D{
				{Key: fieldOrganizationID, Value: 1},
				{Key: fieldName, Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure job role indexes: %w", err)
	}

	return nil
}

// CreateDepartment inserts a department.
func (s *Store) CreateDepartment(ctx context.Context, department departments.Department) error {
	_, err := s.departments.InsertOne(ctx, department)
	if drivermongo.IsDuplicateKeyError(err) {
		return departments.ErrNameTaken
	}

	if err != nil {
		return fmt.Errorf("insert department: %w", err)
	}

	return nil
}

// ListDepartments lists departments for an organization.
func (s *Store) ListDepartments(ctx context.Context, organizationID string) ([]departments.Department, error) {
	cursor, err := s.departments.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldName, Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find departments: %w", err)
	}

	items := make([]departments.Department, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if err := joinCursorErrors("departments", decodeErr, closeErr); err != nil {
		return nil, err
	}

	return items, nil
}

// GetDepartment returns a department scoped to an organization.
func (s *Store) GetDepartment(
	ctx context.Context,
	organizationID, departmentID string,
) (departments.Department, error) {
	var department departments.Department

	err := s.departments.FindOne(ctx, bson.M{
		"_id":               departmentID,
		fieldOrganizationID: organizationID,
	}).Decode(&department)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return departments.Department{}, departments.ErrNotFound
	}

	if err != nil {
		return departments.Department{}, fmt.Errorf("find department: %w", err)
	}

	return department, nil
}

// CreateJobRole inserts a job role.
func (s *Store) CreateJobRole(ctx context.Context, role departments.JobRole) error {
	_, err := s.jobRoles.InsertOne(ctx, role)
	if drivermongo.IsDuplicateKeyError(err) {
		return departments.ErrRoleNameTaken
	}

	if err != nil {
		return fmt.Errorf("insert job role: %w", err)
	}

	return nil
}

// ListJobRoles lists job roles for an organization.
func (s *Store) ListJobRoles(ctx context.Context, organizationID string) ([]departments.JobRole, error) {
	cursor, err := s.jobRoles.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldName, Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find job roles: %w", err)
	}

	items := make([]departments.JobRole, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if err := joinCursorErrors("job roles", decodeErr, closeErr); err != nil {
		return nil, err
	}

	return items, nil
}

// GetJobRole returns a job role scoped to an organization.
func (s *Store) GetJobRole(ctx context.Context, organizationID, roleID string) (departments.JobRole, error) {
	var role departments.JobRole

	err := s.jobRoles.FindOne(ctx, bson.M{
		"_id":               roleID,
		fieldOrganizationID: organizationID,
	}).Decode(&role)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return departments.JobRole{}, departments.ErrRoleNotFound
	}

	if err != nil {
		return departments.JobRole{}, fmt.Errorf("find job role: %w", err)
	}

	return role, nil
}

func joinCursorErrors(label string, decodeErr, closeErr error) error {
	if decodeErr != nil && closeErr != nil {
		return errors.Join(
			fmt.Errorf("decode %s: %w", label, decodeErr),
			fmt.Errorf("close %s cursor: %w", label, closeErr),
		)
	}

	if decodeErr != nil {
		return fmt.Errorf("decode %s: %w", label, decodeErr)
	}

	if closeErr != nil {
		return fmt.Errorf("close %s cursor: %w", label, closeErr)
	}

	return nil
}
