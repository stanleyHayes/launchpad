package departments

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store is the MongoDB repository for departments and job roles.
type Store struct {
	departments *mongo.Collection
	jobRoles    *mongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *mongo.Database) *Store {
	return &Store{
		departments: db.Collection("departments"),
		jobRoles:    db.Collection("job_roles"),
	}
}

// EnsureIndexes creates department and job-role indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.departments.Indexes().CreateMany(ctx, []mongo.IndexModel{
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

	_, err = s.jobRoles.Indexes().CreateMany(ctx, []mongo.IndexModel{
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
func (s *Store) CreateDepartment(ctx context.Context, department Department) error {
	_, err := s.departments.InsertOne(ctx, department)
	if mongo.IsDuplicateKeyError(err) {
		return ErrNameTaken
	}

	if err != nil {
		return fmt.Errorf("insert department: %w", err)
	}

	return nil
}

// ListDepartments lists departments for an organization.
func (s *Store) ListDepartments(ctx context.Context, organizationID string) ([]Department, error) {
	cursor, err := s.departments.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldName, Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find departments: %w", err)
	}

	items := make([]Department, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if err := joinCursorErrors("departments", decodeErr, closeErr); err != nil {
		return nil, err
	}

	return items, nil
}

// GetDepartment returns a department scoped to an organization.
func (s *Store) GetDepartment(ctx context.Context, organizationID, departmentID string) (Department, error) {
	var department Department

	err := s.departments.FindOne(ctx, bson.M{
		"_id":               departmentID,
		fieldOrganizationID: organizationID,
	}).Decode(&department)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Department{}, ErrNotFound
	}

	if err != nil {
		return Department{}, fmt.Errorf("find department: %w", err)
	}

	return department, nil
}

// CreateJobRole inserts a job role.
func (s *Store) CreateJobRole(ctx context.Context, role JobRole) error {
	_, err := s.jobRoles.InsertOne(ctx, role)
	if mongo.IsDuplicateKeyError(err) {
		return ErrRoleNameTaken
	}

	if err != nil {
		return fmt.Errorf("insert job role: %w", err)
	}

	return nil
}

// ListJobRoles lists job roles for an organization.
func (s *Store) ListJobRoles(ctx context.Context, organizationID string) ([]JobRole, error) {
	cursor, err := s.jobRoles.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldName, Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find job roles: %w", err)
	}

	items := make([]JobRole, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if err := joinCursorErrors("job roles", decodeErr, closeErr); err != nil {
		return nil, err
	}

	return items, nil
}

// GetJobRole returns a job role scoped to an organization.
func (s *Store) GetJobRole(ctx context.Context, organizationID, roleID string) (JobRole, error) {
	var role JobRole

	err := s.jobRoles.FindOne(ctx, bson.M{
		"_id":               roleID,
		fieldOrganizationID: organizationID,
	}).Decode(&role)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return JobRole{}, ErrRoleNotFound
	}

	if err != nil {
		return JobRole{}, fmt.Errorf("find job role: %w", err)
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
