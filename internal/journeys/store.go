package journeys

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store persists journey templates and steps.
type Store struct {
	templates *mongo.Collection
	steps     *mongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *mongo.Database) *Store {
	return &Store{
		templates: db.Collection("journey_templates"),
		steps:     db.Collection("journey_steps"),
	}
}

// EnsureIndexes creates journey indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.templates.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure journey template indexes: %w", err)
	}

	_, err = s.steps.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: fieldOrganizationID, Value: 1},
				{Key: fieldTemplateID, Value: 1},
				{Key: fieldVersion, Value: 1},
				{Key: fieldPosition, Value: 1},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("ensure journey step indexes: %w", err)
	}

	return nil
}

// CreateTemplate inserts a journey template.
func (s *Store) CreateTemplate(ctx context.Context, template Template) error {
	_, err := s.templates.InsertOne(ctx, template)
	if err != nil {
		return fmt.Errorf("insert journey template: %w", err)
	}

	return nil
}

// GetTemplate returns a template scoped to an organization.
func (s *Store) GetTemplate(ctx context.Context, organizationID, templateID string) (Template, error) {
	var template Template

	err := s.templates.FindOne(ctx, bson.M{
		"_id":               templateID,
		fieldOrganizationID: organizationID,
	}).Decode(&template)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Template{}, ErrNotFound
	}

	if err != nil {
		return Template{}, fmt.Errorf("find journey template: %w", err)
	}

	return template, nil
}

// ListTemplates lists templates for an organization.
func (s *Store) ListTemplates(ctx context.Context, organizationID string) ([]Template, error) {
	cursor, err := s.templates.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find journey templates: %w", err)
	}

	items := make([]Template, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	return items, joinCursorErrors("journey templates", decodeErr, closeErr)
}

// UpdateTemplate replaces a template document.
func (s *Store) UpdateTemplate(ctx context.Context, template Template) error {
	res, err := s.templates.ReplaceOne(ctx, bson.M{
		"_id":               template.ID,
		fieldOrganizationID: template.OrganizationID,
	}, template)
	if err != nil {
		return fmt.Errorf("replace journey template: %w", err)
	}

	if res.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// CreateStep inserts a journey step.
func (s *Store) CreateStep(ctx context.Context, step Step) error {
	_, err := s.steps.InsertOne(ctx, step)
	if err != nil {
		return fmt.Errorf("insert journey step: %w", err)
	}

	return nil
}

// ListSteps lists steps for a template version.
func (s *Store) ListSteps(
	ctx context.Context,
	organizationID, templateID string,
	version int,
) ([]Step, error) {
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

	items := make([]Step, 0)
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
