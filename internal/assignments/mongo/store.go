// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/assignments"
)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldEmployeeID     = "employeeId"
	fieldCreatedAt      = "createdAt"

	statusScheduled  = "scheduled"
	statusInProgress = "in_progress"
)

var _ assignments.Repository = (*Store)(nil)

// Store persists journey and step assignments.
type Store struct {
	assignments *drivermongo.Collection
	steps       *drivermongo.Collection
	approvals   *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{
		assignments: db.Collection("journey_assignments"),
		steps:       db.Collection("step_assignments"),
		approvals:   db.Collection("approvals"),
	}
}

// EnsureIndexes creates assignment indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.assignments.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys: bson.D{
				{Key: fieldOrganizationID, Value: 1},
				{Key: fieldEmployeeID, Value: 1},
				{Key: "journeyTemplateId", Value: 1},
			},
		},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure journey assignment indexes: %w", err)
	}

	_, err = s.steps.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys: bson.D{
				{Key: fieldOrganizationID, Value: 1},
				{Key: "journeyAssignmentId", Value: 1},
				{Key: "position", Value: 1},
			},
		},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldEmployeeID, Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure step assignment indexes: %w", err)
	}

	_, err = s.approvals.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: "status", Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: "stepAssignmentId", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure approval indexes: %w", err)
	}

	return nil
}

// CreateAssignment inserts a journey assignment.
func (s *Store) CreateAssignment(ctx context.Context, assignment assignments.JourneyAssignment) error {
	_, err := s.assignments.InsertOne(ctx, assignment)
	if err != nil {
		return fmt.Errorf("insert journey assignment: %w", err)
	}

	return nil
}

// CreateStepAssignments inserts many step assignments.
func (s *Store) CreateStepAssignments(ctx context.Context, steps []assignments.StepAssignment) error {
	if len(steps) == 0 {
		return nil
	}

	docs := make([]any, 0, len(steps))
	for _, step := range steps {
		docs = append(docs, step)
	}

	_, err := s.steps.InsertMany(ctx, docs)
	if err != nil {
		return fmt.Errorf("insert step assignments: %w", err)
	}

	return nil
}

// CreateApproval inserts an approval.
func (s *Store) CreateApproval(ctx context.Context, approval assignments.Approval) error {
	_, err := s.approvals.InsertOne(ctx, approval)
	if err != nil {
		return fmt.Errorf("insert approval: %w", err)
	}

	return nil
}

// FindActiveAssignment finds an in-progress/scheduled assignment for employee+template.
func (s *Store) FindActiveAssignment(
	ctx context.Context,
	organizationID, employeeID, templateID string,
) (assignments.JourneyAssignment, error) {
	var assignment assignments.JourneyAssignment

	err := s.assignments.FindOne(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldEmployeeID:     employeeID,
		"journeyTemplateId": templateID,
		"status":            bson.M{"$in": []string{statusScheduled, statusInProgress}},
	}).Decode(&assignment)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return assignments.JourneyAssignment{}, assignments.ErrNotFound
	}

	if err != nil {
		return assignments.JourneyAssignment{}, fmt.Errorf("find active assignment: %w", err)
	}

	return assignment, nil
}

// GetAssignment returns one assignment.
func (s *Store) GetAssignment(
	ctx context.Context,
	organizationID, assignmentID string,
) (assignments.JourneyAssignment, error) {
	var assignment assignments.JourneyAssignment

	err := s.assignments.FindOne(ctx, bson.M{
		fieldID:             assignmentID,
		fieldOrganizationID: organizationID,
	}).Decode(&assignment)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return assignments.JourneyAssignment{}, assignments.ErrNotFound
	}

	if err != nil {
		return assignments.JourneyAssignment{}, fmt.Errorf("find assignment: %w", err)
	}

	return assignment, nil
}

// ListAssignments lists organization assignments.
func (s *Store) ListAssignments(ctx context.Context, organizationID string) ([]assignments.JourneyAssignment, error) {
	return s.findAssignments(ctx, bson.M{fieldOrganizationID: organizationID})
}

// ListAssignmentsForEmployee lists assignments for one employee.
func (s *Store) ListAssignmentsForEmployee(
	ctx context.Context,
	organizationID, employeeID string,
) ([]assignments.JourneyAssignment, error) {
	return s.findAssignments(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldEmployeeID:     employeeID,
	})
}

// UpdateAssignment replaces an assignment.
func (s *Store) UpdateAssignment(ctx context.Context, assignment assignments.JourneyAssignment) error {
	res, err := s.assignments.ReplaceOne(ctx, bson.M{
		fieldID:             assignment.ID,
		fieldOrganizationID: assignment.OrganizationID,
	}, assignment)
	if err != nil {
		return fmt.Errorf("replace assignment: %w", err)
	}

	if res.MatchedCount == 0 {
		return assignments.ErrNotFound
	}

	return nil
}

// ListSteps lists step assignments for a journey assignment.
func (s *Store) ListSteps(
	ctx context.Context,
	organizationID, journeyAssignmentID string,
) ([]assignments.StepAssignment, error) {
	cursor, err := s.steps.Find(
		ctx,
		bson.M{
			fieldOrganizationID:   organizationID,
			"journeyAssignmentId": journeyAssignmentID,
		},
		options.Find().SetSort(bson.D{{Key: "position", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find step assignments: %w", err)
	}

	items := make([]assignments.StepAssignment, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	return items, joinCursorErrors("step assignments", decodeErr, closeErr)
}

// GetStep returns one step assignment.
func (s *Store) GetStep(
	ctx context.Context,
	organizationID, stepAssignmentID string,
) (assignments.StepAssignment, error) {
	var step assignments.StepAssignment

	err := s.steps.FindOne(ctx, bson.M{
		fieldID:             stepAssignmentID,
		fieldOrganizationID: organizationID,
	}).Decode(&step)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return assignments.StepAssignment{}, assignments.ErrStepNotFound
	}

	if err != nil {
		return assignments.StepAssignment{}, fmt.Errorf("find step assignment: %w", err)
	}

	return step, nil
}

// UpdateStep replaces a step assignment.
func (s *Store) UpdateStep(ctx context.Context, step assignments.StepAssignment) error {
	res, err := s.steps.ReplaceOne(ctx, bson.M{
		fieldID:             step.ID,
		fieldOrganizationID: step.OrganizationID,
	}, step)
	if err != nil {
		return fmt.Errorf("replace step assignment: %w", err)
	}

	if res.MatchedCount == 0 {
		return assignments.ErrStepNotFound
	}

	return nil
}

// GetApproval returns one approval.
func (s *Store) GetApproval(ctx context.Context, organizationID, approvalID string) (assignments.Approval, error) {
	var approval assignments.Approval

	err := s.approvals.FindOne(ctx, bson.M{
		fieldID:             approvalID,
		fieldOrganizationID: organizationID,
	}).Decode(&approval)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return assignments.Approval{}, assignments.ErrNotFound
	}

	if err != nil {
		return assignments.Approval{}, fmt.Errorf("find approval: %w", err)
	}

	return approval, nil
}

// ListApprovals lists approvals for an organization.
func (s *Store) ListApprovals(ctx context.Context, organizationID string) ([]assignments.Approval, error) {
	cursor, err := s.approvals.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find approvals: %w", err)
	}

	items := make([]assignments.Approval, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	return items, joinCursorErrors("approvals", decodeErr, closeErr)
}

// GetApprovalByStep returns approval for a step.
func (s *Store) GetApprovalByStep(
	ctx context.Context,
	organizationID, stepAssignmentID string,
) (assignments.Approval, error) {
	var approval assignments.Approval

	err := s.approvals.FindOne(ctx, bson.M{
		fieldOrganizationID: organizationID,
		"stepAssignmentId":  stepAssignmentID,
	}).Decode(&approval)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return assignments.Approval{}, assignments.ErrNotFound
	}

	if err != nil {
		return assignments.Approval{}, fmt.Errorf("find approval by step: %w", err)
	}

	return approval, nil
}

// UpdateApproval replaces an approval.
func (s *Store) UpdateApproval(ctx context.Context, approval assignments.Approval) error {
	res, err := s.approvals.ReplaceOne(ctx, bson.M{
		fieldID:             approval.ID,
		fieldOrganizationID: approval.OrganizationID,
	}, approval)
	if err != nil {
		return fmt.Errorf("replace approval: %w", err)
	}

	if res.MatchedCount == 0 {
		return assignments.ErrNotFound
	}

	return nil
}

func (s *Store) findAssignments(ctx context.Context, filter bson.M) ([]assignments.JourneyAssignment, error) {
	cursor, err := s.assignments.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find assignments: %w", err)
	}

	items := make([]assignments.JourneyAssignment, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	return items, joinCursorErrors("assignments", decodeErr, closeErr)
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
