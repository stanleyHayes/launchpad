package cms

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Service implements CMS use cases.
type Service struct {
	repo Repository
}

// NewService constructs a Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create creates a draft page.
func (s *Service) Create(ctx context.Context, in CreateInput) (Page, error) {
	slug := strings.ToLower(strings.TrimSpace(in.Slug))
	title := strings.TrimSpace(in.Title)
	summary := strings.TrimSpace(in.Summary)
	body := strings.TrimSpace(in.Body)

	if !slugPattern.MatchString(slug) || title == "" || body == "" {
		return Page{}, ErrInvalidInput
	}

	now := time.Now().UTC()

	page := Page{
		ID:        uuid.NewString(),
		Slug:      slug,
		Title:     title,
		Summary:   summary,
		Body:      body,
		Status:    statusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, page); err != nil {
		return Page{}, fmt.Errorf("create cms page: %w", err)
	}

	return page, nil
}

// List returns all CMS pages for platform editors.
func (s *Service) List(ctx context.Context) ([]Page, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list cms pages: %w", err)
	}

	return items, nil
}

// Get returns one page by id.
func (s *Service) Get(ctx context.Context, id string) (Page, error) {
	if strings.TrimSpace(id) == "" {
		return Page{}, ErrInvalidInput
	}

	page, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Page{}, fmt.Errorf("get cms page: %w", err)
	}

	return page, nil
}

// GetPublishedBySlug returns a published page for public rendering.
func (s *Service) GetPublishedBySlug(ctx context.Context, slug string) (Page, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return Page{}, ErrInvalidInput
	}

	page, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return Page{}, fmt.Errorf("get cms page by slug: %w", err)
	}

	if page.Status != statusPublished {
		return Page{}, ErrNotFound
	}

	return page, nil
}

// Update updates draft page content.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (Page, error) {
	page, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Page{}, fmt.Errorf("get cms page for update: %w", err)
	}

	if page.Status != statusDraft {
		return Page{}, ErrNotDraft
	}

	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			return Page{}, ErrInvalidInput
		}

		page.Title = title
	}

	if in.Summary != nil {
		page.Summary = strings.TrimSpace(*in.Summary)
	}

	if in.Body != nil {
		body := strings.TrimSpace(*in.Body)
		if body == "" {
			return Page{}, ErrInvalidInput
		}

		page.Body = body
	}

	page.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, page); err != nil {
		return Page{}, fmt.Errorf("update cms page: %w", err)
	}

	return page, nil
}

// Publish marks a draft page as published.
func (s *Service) Publish(ctx context.Context, id string) (Page, error) {
	page, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Page{}, fmt.Errorf("get cms page for publish: %w", err)
	}

	if page.Status != statusDraft {
		return Page{}, ErrNotDraft
	}

	now := time.Now().UTC()
	page.Status = statusPublished
	page.PublishedAt = &now
	page.UpdatedAt = now

	if err := s.repo.Update(ctx, page); err != nil {
		return Page{}, fmt.Errorf("publish cms page: %w", err)
	}

	return page, nil
}
