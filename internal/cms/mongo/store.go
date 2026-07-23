// Package mongo is the MongoDB persistence adapter for this domain.
package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/cms"
)

const (
	fieldID        = "_id"
	fieldSlug      = "slug"
	fieldCreatedAt = "createdAt"
)

var _ cms.Repository = (*Store)(nil)

// Store is the MongoDB CMS repository.
type Store struct {
	col *drivermongo.Collection
}

// NewStore constructs a Store.
func NewStore(db *drivermongo.Database) *Store {
	return &Store{col: db.Collection("cms_pages")}
}

// EnsureIndexes creates CMS indexes.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldSlug, Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure cms page indexes: %w", err)
	}

	return nil
}

// Create inserts a CMS page.
func (s *Store) Create(ctx context.Context, page cms.Page) error {
	_, err := s.col.InsertOne(ctx, page)
	if drivermongo.IsDuplicateKeyError(err) {
		return cms.ErrSlugTaken
	}

	if err != nil {
		return fmt.Errorf("insert cms page: %w", err)
	}

	return nil
}

// GetByID returns a page by id.
func (s *Store) GetByID(ctx context.Context, id string) (cms.Page, error) {
	var page cms.Page

	err := s.col.FindOne(ctx, bson.M{fieldID: id}).Decode(&page)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return cms.Page{}, cms.ErrNotFound
	}

	if err != nil {
		return cms.Page{}, fmt.Errorf("find cms page: %w", err)
	}

	return page, nil
}

// GetBySlug returns a page by slug.
func (s *Store) GetBySlug(ctx context.Context, slug string) (cms.Page, error) {
	var page cms.Page

	err := s.col.FindOne(ctx, bson.M{fieldSlug: slug}).Decode(&page)
	if errors.Is(err, drivermongo.ErrNoDocuments) {
		return cms.Page{}, cms.ErrNotFound
	}

	if err != nil {
		return cms.Page{}, fmt.Errorf("find cms page by slug: %w", err)
	}

	return page, nil
}

// List returns all CMS pages newest first.
func (s *Store) List(ctx context.Context) ([]cms.Page, error) {
	cursor, err := s.col.Find(
		ctx,
		bson.M{},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find cms pages: %w", err)
	}

	items := make([]cms.Page, 0)
	decodeErr := cursor.All(ctx, &items)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("decode cms pages: %w", decodeErr),
			fmt.Errorf("close cms pages cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode cms pages: %w", decodeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close cms pages cursor: %w", closeErr)
	}

	return items, nil
}

// Update replaces a CMS page document.
func (s *Store) Update(ctx context.Context, page cms.Page) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{fieldID: page.ID}, page)
	if err != nil {
		return fmt.Errorf("replace cms page: %w", err)
	}

	if res.MatchedCount == 0 {
		return cms.ErrNotFound
	}

	return nil
}
