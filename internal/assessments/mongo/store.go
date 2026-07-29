// Package mongo is the MongoDB persistence adapter for the assessments domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/assessments"
)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldAssessmentID   = "assessmentId"
	fieldEmployeeID     = "employeeId"
	fieldStatus         = "status"
	fieldCreatedAt      = "createdAt"
	fieldSubmittedAt    = "submittedAt"
	fieldIssuedAt       = "issuedAt"
	fieldSerial         = "serial"
)

var _ assessments.Repository = (*Store)(nil)

// Store is the MongoDB assessments repository covering the assessments,
// assessment_attempts, and certificates collections.
type Store struct {
	assessments  *drivermongo.Collection
	attempts     *drivermongo.Collection
	certificates *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{
		assessments:  db.Collection("assessments"),
		attempts:     db.Collection("assessment_attempts"),
		certificates: db.Collection("certificates"),
	}
}

// EnsureIndexes creates the assessments, attempts, and certificates indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	if _, err := s.assessments.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldStatus, Value: 1}}},
	}); err != nil {
		return fmt.Errorf("ensure assessment indexes: %w", err)
	}

	if _, err := s.attempts.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{
			{Key: fieldOrganizationID, Value: 1},
			{Key: fieldAssessmentID, Value: 1},
			{Key: fieldSubmittedAt, Value: -1},
		}},
		{Keys: bson.D{
			{Key: fieldOrganizationID, Value: 1},
			{Key: fieldAssessmentID, Value: 1},
			{Key: fieldEmployeeID, Value: 1},
			{Key: fieldSubmittedAt, Value: -1},
		}},
	}); err != nil {
		return fmt.Errorf("ensure assessment attempt indexes: %w", err)
	}

	if _, err := s.certificates.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys: bson.D{
				{Key: fieldOrganizationID, Value: 1},
				{Key: fieldAssessmentID, Value: 1},
				{Key: fieldEmployeeID, Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{
			{Key: fieldOrganizationID, Value: 1},
			{Key: fieldEmployeeID, Value: 1},
			{Key: fieldIssuedAt, Value: -1},
		}},
		{
			Keys:    bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldSerial, Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}); err != nil {
		return fmt.Errorf("ensure certificate indexes: %w", err)
	}

	return nil
}

// CreateAssessment inserts an assessment.
func (s *Store) CreateAssessment(ctx context.Context, assessment assessments.Assessment) error {
	if _, err := s.assessments.InsertOne(ctx, assessment); err != nil {
		return fmt.Errorf("insert assessment: %w", err)
	}

	return nil
}

// GetAssessment loads an assessment scoped to a tenant.
func (s *Store) GetAssessment(
	ctx context.Context,
	organizationID, assessmentID string,
) (assessments.Assessment, error) {
	var assessment assessments.Assessment

	err := s.assessments.FindOne(ctx, bson.M{
		fieldID:             assessmentID,
		fieldOrganizationID: organizationID,
	}).Decode(&assessment)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return assessments.Assessment{}, assessments.ErrNotFound
	}

	if err != nil {
		return assessments.Assessment{}, fmt.Errorf("find assessment: %w", err)
	}

	return assessment, nil
}

// ListAssessments returns a tenant's assessments newest first.
func (s *Store) ListAssessments(
	ctx context.Context,
	organizationID string,
) ([]assessments.Assessment, error) {
	cursor, err := s.assessments.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find assessments: %w", err)
	}

	items := make([]assessments.Assessment, 0)
	if err := drainCursor(ctx, cursor, &items, "assessments"); err != nil {
		return nil, err
	}

	return items, nil
}

// UpdateAssessment replaces an assessment scoped to its tenant.
func (s *Store) UpdateAssessment(ctx context.Context, assessment assessments.Assessment) error {
	res, err := s.assessments.ReplaceOne(ctx, bson.M{
		fieldID:             assessment.ID,
		fieldOrganizationID: assessment.OrganizationID,
	}, assessment)
	if err != nil {
		return fmt.Errorf("replace assessment: %w", err)
	}

	if res.MatchedCount == 0 {
		return assessments.ErrNotFound
	}

	return nil
}

// CreateAttempt inserts an attempt.
func (s *Store) CreateAttempt(ctx context.Context, attempt assessments.Attempt) error {
	if _, err := s.attempts.InsertOne(ctx, attempt); err != nil {
		return fmt.Errorf("insert assessment attempt: %w", err)
	}

	return nil
}

// GetAttempt loads an attempt scoped to its tenant and assessment.
func (s *Store) GetAttempt(
	ctx context.Context,
	organizationID, assessmentID, attemptID string,
) (assessments.Attempt, error) {
	var attempt assessments.Attempt

	err := s.attempts.FindOne(ctx, bson.M{
		fieldID:             attemptID,
		fieldOrganizationID: organizationID,
		fieldAssessmentID:   assessmentID,
	}).Decode(&attempt)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return assessments.Attempt{}, assessments.ErrAttemptNotFound
	}

	if err != nil {
		return assessments.Attempt{}, fmt.Errorf("find assessment attempt: %w", err)
	}

	return attempt, nil
}

// CountAttempts counts an employee's submitted attempts on an assessment.
func (s *Store) CountAttempts(
	ctx context.Context,
	organizationID, assessmentID, employeeID string,
) (int64, error) {
	count, err := s.attempts.CountDocuments(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldAssessmentID:   assessmentID,
		fieldEmployeeID:     employeeID,
	})
	if err != nil {
		return 0, fmt.Errorf("count assessment attempts: %w", err)
	}

	return count, nil
}

// ListAttempts returns every attempt on an assessment, newest first.
func (s *Store) ListAttempts(
	ctx context.Context,
	organizationID, assessmentID string,
) ([]assessments.Attempt, error) {
	cursor, err := s.attempts.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID, fieldAssessmentID: assessmentID},
		options.Find().SetSort(bson.D{{Key: fieldSubmittedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find assessment attempts: %w", err)
	}

	items := make([]assessments.Attempt, 0)
	if err := drainCursor(ctx, cursor, &items, "assessment attempts"); err != nil {
		return nil, err
	}

	return items, nil
}

// LatestAttempt returns the employee's most recent attempt on an assessment.
func (s *Store) LatestAttempt(
	ctx context.Context,
	organizationID, assessmentID, employeeID string,
) (assessments.Attempt, error) {
	var attempt assessments.Attempt

	err := s.attempts.FindOne(
		ctx,
		bson.M{
			fieldOrganizationID: organizationID,
			fieldAssessmentID:   assessmentID,
			fieldEmployeeID:     employeeID,
		},
		options.FindOne().SetSort(bson.D{{Key: fieldSubmittedAt, Value: -1}}),
	).Decode(&attempt)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return assessments.Attempt{}, assessments.ErrAttemptNotFound
	}

	if err != nil {
		return assessments.Attempt{}, fmt.Errorf("find latest assessment attempt: %w", err)
	}

	return attempt, nil
}

// UpdateAttempt replaces an attempt scoped to its tenant and assessment.
func (s *Store) UpdateAttempt(ctx context.Context, attempt assessments.Attempt) error {
	res, err := s.attempts.ReplaceOne(ctx, bson.M{
		fieldID:             attempt.ID,
		fieldOrganizationID: attempt.OrganizationID,
		fieldAssessmentID:   attempt.AssessmentID,
	}, attempt)
	if err != nil {
		return fmt.Errorf("replace assessment attempt: %w", err)
	}

	if res.MatchedCount == 0 {
		return assessments.ErrAttemptNotFound
	}

	return nil
}

// CreateCertificate inserts a certificate.
func (s *Store) CreateCertificate(ctx context.Context, certificate assessments.Certificate) error {
	if _, err := s.certificates.InsertOne(ctx, certificate); err != nil {
		return fmt.Errorf("insert certificate: %w", err)
	}

	return nil
}

// FindCertificate returns the employee's certificate for an assessment.
func (s *Store) FindCertificate(
	ctx context.Context,
	organizationID, assessmentID, employeeID string,
) (assessments.Certificate, error) {
	var certificate assessments.Certificate

	err := s.certificates.FindOne(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldAssessmentID:   assessmentID,
		fieldEmployeeID:     employeeID,
	}).Decode(&certificate)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return assessments.Certificate{}, assessments.ErrNotFound
	}

	if err != nil {
		return assessments.Certificate{}, fmt.Errorf("find certificate: %w", err)
	}

	return certificate, nil
}

// ListCertificatesForEmployee returns an employee's certificates newest first.
func (s *Store) ListCertificatesForEmployee(
	ctx context.Context,
	organizationID, employeeID string,
) ([]assessments.Certificate, error) {
	cursor, err := s.certificates.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID, fieldEmployeeID: employeeID},
		options.Find().SetSort(bson.D{{Key: fieldIssuedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find certificates: %w", err)
	}

	items := make([]assessments.Certificate, 0)
	if err := drainCursor(ctx, cursor, &items, "certificates"); err != nil {
		return nil, err
	}

	return items, nil
}

// drainCursor decodes every cursor document into items and closes the cursor,
// joining a decode failure with a close failure.
func drainCursor(ctx context.Context, cursor *drivermongo.Cursor, items any, label string) error {
	decodeErr := cursor.All(ctx, items)
	closeErr := cursor.Close(ctx)

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
