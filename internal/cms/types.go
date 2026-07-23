// Package cms manages platform marketing content pages.
package cms

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound indicates the page does not exist.
	ErrNotFound = errors.New("cms page not found")
	// ErrSlugTaken indicates the slug is already used.
	ErrSlugTaken = errors.New("cms page slug already taken")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid cms input")
	// ErrNotDraft indicates the page is not editable as a draft.
	ErrNotDraft = errors.New("cms page is not a draft")
)

const (
	statusDraft     = "draft"
	statusPublished = "published"
)

// Page is a CMS-managed marketing page.
type Page struct {
	ID          string     `bson:"_id"                   json:"id"`
	Slug        string     `bson:"slug"                  json:"slug"`
	Title       string     `bson:"title"                 json:"title"`
	Summary     string     `bson:"summary"               json:"summary"`
	Body        string     `bson:"body"                  json:"body"`
	Status      string     `bson:"status"                json:"status"`
	PublishedAt *time.Time `bson:"publishedAt,omitempty" json:"publishedAt,omitempty"`
	CreatedAt   time.Time  `bson:"createdAt"             json:"createdAt"`
	UpdatedAt   time.Time  `bson:"updatedAt"             json:"updatedAt"`
}

// CreateInput creates a draft CMS page.
type CreateInput struct {
	Slug    string
	Title   string
	Summary string
	Body    string
}

// UpdateInput updates mutable CMS page fields.
type UpdateInput struct {
	Title   *string
	Summary *string
	Body    *string
}

// Repository persists CMS pages.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, page Page) error
	GetByID(ctx context.Context, id string) (Page, error)
	GetBySlug(ctx context.Context, slug string) (Page, error)
	List(ctx context.Context) ([]Page, error)
	Update(ctx context.Context, page Page) error
}
