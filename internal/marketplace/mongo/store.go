package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/marketplace"
)

var _ marketplace.Repository = (*Store)(nil)

type Store struct {
	templates     *drivermongo.Collection
	installations *drivermongo.Collection
	ratings       *drivermongo.Collection
}

func NewStore(db *drivermongo.Database) *Store {
	return &Store{
		templates:     db.Collection("marketplace_templates"),
		installations: db.Collection("marketplace_installations"),
		ratings:       db.Collection("marketplace_ratings"),
	}
}

func (s *Store) EnsureIndexes(ctx context.Context) error {
	if _, err := s.templates.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "featured", Value: -1}, {Key: "updatedAt", Value: -1}}},
	}); err != nil {
		return fmt.Errorf("ensure marketplace template indexes: %w", err)
	}
	if _, err := s.installations.Indexes().CreateOne(ctx, drivermongo.IndexModel{
		Keys: bson.D{{Key: "organizationId", Value: 1}, {Key: "templateId", Value: 1}, {Key: "installedAt", Value: -1}},
	}); err != nil {
		return fmt.Errorf("ensure marketplace installation indexes: %w", err)
	}
	if _, err := s.ratings.Indexes().CreateOne(ctx, drivermongo.IndexModel{
		Keys:    bson.D{{Key: "templateId", Value: 1}, {Key: "organizationId", Value: 1}, {Key: "userId", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return fmt.Errorf("ensure marketplace rating indexes: %w", err)
	}
	return nil
}

func (s *Store) Create(ctx context.Context, item marketplace.Template) error {
	if _, err := s.templates.InsertOne(ctx, item); err != nil {
		return fmt.Errorf("insert marketplace template: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id string) (marketplace.Template, error) {
	var item marketplace.Template
	err := s.templates.FindOne(ctx, bson.M{"_id": id}).Decode(&item)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return marketplace.Template{}, marketplace.ErrNotFound
	}
	if err != nil {
		return marketplace.Template{}, fmt.Errorf("find marketplace template: %w", err)
	}
	return item, nil
}

func (s *Store) List(ctx context.Context, status string) ([]marketplace.Template, error) {
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}
	cursor, err := s.templates.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "featured", Value: -1}, {Key: "updatedAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("find marketplace templates: %w", err)
	}
	var items []marketplace.Template
	decodeErr, closeErr := cursor.All(ctx, &items), cursor.Close(ctx)
	if decodeErr != nil || closeErr != nil {
		return nil, errors.Join(decodeErr, closeErr)
	}
	if items == nil {
		items = []marketplace.Template{}
	}
	return items, nil
}

func (s *Store) Update(ctx context.Context, item marketplace.Template) error {
	result, err := s.templates.ReplaceOne(ctx, bson.M{"_id": item.ID}, item)
	if err != nil {
		return fmt.Errorf("replace marketplace template: %w", err)
	}
	if result.MatchedCount == 0 {
		return marketplace.ErrNotFound
	}
	return nil
}

func (s *Store) CreateInstallation(ctx context.Context, item marketplace.Installation) error {
	if _, err := s.installations.InsertOne(ctx, item); err != nil {
		return fmt.Errorf("insert marketplace installation: %w", err)
	}
	return nil
}

func (s *Store) UpsertRating(ctx context.Context, item marketplace.Rating) error {
	filter := bson.M{"templateId": item.TemplateID, "organizationId": item.OrganizationID, "userId": item.UserID}
	update := bson.M{"$set": bson.M{"score": item.Score, "updatedAt": item.UpdatedAt}, "$setOnInsert": bson.M{"_id": item.ID, "createdAt": item.CreatedAt}}
	if _, err := s.ratings.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true)); err != nil {
		return fmt.Errorf("upsert marketplace rating: %w", err)
	}
	return nil
}

func (s *Store) ListRatings(ctx context.Context, templateID string) ([]marketplace.Rating, error) {
	cursor, err := s.ratings.Find(ctx, bson.M{"templateId": templateID})
	if err != nil {
		return nil, fmt.Errorf("find marketplace ratings: %w", err)
	}
	var items []marketplace.Rating
	decodeErr, closeErr := cursor.All(ctx, &items), cursor.Close(ctx)
	if decodeErr != nil || closeErr != nil {
		return nil, errors.Join(decodeErr, closeErr)
	}
	return items, nil
}
