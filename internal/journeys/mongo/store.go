// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/journeys"
)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldCreatedAt      = "createdAt"
	fieldTemplateID     = "journeyTemplateId"
	fieldVersion        = "version"
	fieldPosition       = "position"
)

var _ journeys.Repository = (*Store)(nil)

// Store persists journey templates and steps.
type Store struct {
	templates *drivermongo.Collection
	steps     *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{
		templates: db.Collection("journey_templates"),
		steps:     db.Collection("journey_steps"),
	}
}

// EnsureIndexes creates journey indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.templates.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure journey template indexes: %w", err)
	}

	_, err = s.steps.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{
			Keys: bson.D{
				{Key: fieldOrganizationID, Value: 1},
				{Key: fieldTemplateID, Value: 1},
				{Key: fieldVersion, Value: 1},
				{Key: fieldPosition, Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure journey step indexes: %w", err)
	}

	return nil
}

// CreateTemplate inserts a journey template.
func (s *Store) CreateTemplate(ctx context.Context, template journeys.Template) error {
	_, err := s.templates.InsertOne(ctx, template)
	if err != nil {
		return fmt.Errorf("insert journey template: %w", err)
	}

	return nil
}

// GetTemplate returns a template scoped to an organization.
func (s *Store) GetTemplate(ctx context.Context, organizationID, templateID string) (journeys.Template, error) {
	var template journeys.Template

	err := s.templates.FindOne(ctx, bson.M{
		fieldID:             templateID,
		fieldOrganizationID: organizationID,
	}).Decode(&template)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return journeys.Template{}, journeys.ErrNotFound
	}

	if err != nil {
		return journeys.Template{}, fmt.Errorf("find journey template: %w", err)
	}

	return template, nil
}

// ListTemplates lists templates for an organization.
func (s *Store) ListTemplates(ctx context.Context, organizationID string) ([]journeys.Template, error) {
	cursor, err := s.templates.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find journey templates: %w", err)
	}

	items := make([]journeys.Template, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	return items, joinCursorErrors("journey templates", decodeErr, closeErr)
}

// UpdateTemplate replaces a template document.
func (s *Store) UpdateTemplate(ctx context.Context, template journeys.Template) error {
	res, err := s.templates.ReplaceOne(ctx, bson.M{
		fieldID:             template.ID,
		fieldOrganizationID: template.OrganizationID,
	}, template)
	if err != nil {
		return fmt.Errorf("replace journey template: %w", err)
	}

	if res.MatchedCount == 0 {
		return journeys.ErrNotFound
	}

	return nil
}

// CreateStep inserts a journey step.
func (s *Store) CreateStep(ctx context.Context, step journeys.Step) error {
	_, err := s.steps.InsertOne(ctx, step)
	if drivermongo.IsDuplicateKeyError(err) {
		return journeys.ErrStepPositionTaken
	}

	if err != nil {
		return fmt.Errorf("insert journey step: %w", err)
	}

	return nil
}

// UpdateStep replaces a step document.
func (s *Store) UpdateStep(ctx context.Context, step journeys.Step) error {
	res, err := s.steps.ReplaceOne(ctx, bson.M{
		fieldID:             step.ID,
		fieldOrganizationID: step.OrganizationID,
	}, step)
	if err != nil {
		return fmt.Errorf("replace journey step: %w", err)
	}

	if res.MatchedCount == 0 {
		return journeys.ErrStepNotFound
	}

	return nil
}

// DeleteStep removes a step from a template version.
func (s *Store) DeleteStep(
	ctx context.Context,
	organizationID, templateID string,
	version int,
	stepID string,
) error {
	res, err := s.steps.DeleteOne(ctx, bson.M{
		fieldID:             stepID,
		fieldOrganizationID: organizationID,
		fieldTemplateID:     templateID,
		fieldVersion:        version,
	})
	if err != nil {
		return fmt.Errorf("delete journey step: %w", err)
	}

	if res.DeletedCount == 0 {
		return journeys.ErrStepNotFound
	}

	return nil
}

// ListSteps lists steps for a template version.
func (s *Store) ListSteps(
	ctx context.Context,
	organizationID, templateID string,
	version int,
) ([]journeys.Step, error) {
	cursor, err := s.steps.Find(
		ctx,
		bson.M{
			fieldOrganizationID: organizationID,
			fieldTemplateID:     templateID,
			fieldVersion:        version,
		},
		options.Find().SetSort(bson.D{{Key: fieldPosition, Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find journey steps: %w", err)
	}

	items := make([]journeys.Step, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	return items, joinCursorErrors("journey steps", decodeErr, closeErr)
}

// CountSteps counts steps for a template version.
func (s *Store) CountSteps(
	ctx context.Context,
	organizationID, templateID string,
	version int,
) (int64, error) {
	count, err := s.steps.CountDocuments(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldTemplateID:     templateID,
		fieldVersion:        version,
	})
	if err != nil {
		return 0, fmt.Errorf("count journey steps: %w", err)
	}

	return count, nil
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

// DeleteForOrganization removes every journey template and step of the
// organization and returns the number of documents deleted. It serves only
// the platform GDPR tenant purge (PRD 7.4).
func (s *Store) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.templates.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete journey templates for organization: %w", err)
	}

	deleted := res.DeletedCount

	res, err = s.steps.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete journey steps for organization: %w", err)
	}

	return deleted + res.DeletedCount, nil
}
