package cms

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Service implements CMS use cases.
type Service struct {
	repo Repository
}

// Navigation returns published pages explicitly opted into public navigation.
func (s *Service) Navigation(ctx context.Context) ([]Page, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list cms navigation: %w", err)
	}
	navigation := make([]Page, 0)
	for _, page := range items {
		if page.Status == statusPublished && strings.TrimSpace(page.NavLabel) != "" {
			navigation = append(navigation, page)
		}
	}
	sort.SliceStable(navigation, func(i, j int) bool {
		if navigation[i].NavOrder == navigation[j].NavOrder {
			return navigation[i].NavLabel < navigation[j].NavLabel
		}
		return navigation[i].NavOrder < navigation[j].NavOrder
	})
	return navigation, nil
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
	contentType := strings.ToLower(strings.TrimSpace(in.ContentType))
	if contentType == "" {
		contentType = "page"
	}

	if !slugPattern.MatchString(slug) || title == "" || body == "" || !isValidContentType(contentType) || in.NavOrder < 0 {
		return Page{}, ErrInvalidInput
	}

	now := time.Now().UTC()

	page := Page{
		ID:          uuid.NewString(),
		Slug:        slug,
		Title:       title,
		Summary:     summary,
		Body:        body,
		ContentType: contentType,
		NavLabel:    strings.TrimSpace(in.NavLabel),
		NavOrder:    in.NavOrder,
		Status:      statusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
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

	// Site information is a small, structured settings document rather than
	// editorial page content. Platform editors can update it in place so
	// contact details and effective dates do not require a draft lifecycle.
	if page.Status != statusDraft && page.ContentType != "settings" {
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
	if in.ContentType != nil {
		value := strings.ToLower(strings.TrimSpace(*in.ContentType))
		if !isValidContentType(value) {
			return Page{}, ErrInvalidInput
		}
		page.ContentType = value
	}
	if in.NavLabel != nil {
		page.NavLabel = strings.TrimSpace(*in.NavLabel)
	}
	if in.NavOrder != nil {
		if *in.NavOrder < 0 {
			return Page{}, ErrInvalidInput
		}
		page.NavOrder = *in.NavOrder
	}

	page.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, page); err != nil {
		return Page{}, fmt.Errorf("update cms page: %w", err)
	}

	return page, nil
}

func isValidContentType(value string) bool {
	switch value {
	case "page", "blog", "faq", "legal", "settings":
		return true
	default:
		return false
	}
}

// Schedule queues a draft for publication at a future UTC instant.
func (s *Service) Schedule(ctx context.Context, id string, at time.Time) (Page, error) {
	page, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Page{}, fmt.Errorf("get cms page for scheduling: %w", err)
	}
	now := time.Now().UTC()
	at = at.UTC()
	if page.Status != statusDraft || !at.After(now) {
		return Page{}, ErrInvalidInput
	}
	page.Status, page.ScheduledAt, page.UpdatedAt = statusScheduled, &at, now
	if err := s.repo.Update(ctx, page); err != nil {
		return Page{}, fmt.Errorf("schedule cms page: %w", err)
	}
	return page, nil
}

// PublishDue publishes all scheduled pages whose publication time has arrived.
func (s *Service) PublishDue(ctx context.Context) error {
	items, err := s.repo.List(ctx)
	if err != nil {
		return fmt.Errorf("list scheduled cms pages: %w", err)
	}
	now := time.Now().UTC()
	for _, page := range items {
		if page.Status != statusScheduled || page.ScheduledAt == nil || page.ScheduledAt.After(now) {
			continue
		}
		page.Status, page.PublishedAt, page.ScheduledAt, page.UpdatedAt = statusPublished, &now, nil, now
		if err := s.repo.Update(ctx, page); err != nil {
			return fmt.Errorf("publish scheduled cms page %q: %w", page.ID, err)
		}
	}
	return nil
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
