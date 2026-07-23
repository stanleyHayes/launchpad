package cms_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/cms"
)

type memoryRepo struct {
	pages map[string]cms.Page
}

func (m *memoryRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryRepo) Create(_ context.Context, page cms.Page) error {
	if m.pages == nil {
		m.pages = map[string]cms.Page{}
	}

	for _, existing := range m.pages {
		if existing.Slug == page.Slug {
			return cms.ErrSlugTaken
		}
	}

	m.pages[page.ID] = page

	return nil
}

func (m *memoryRepo) GetByID(_ context.Context, id string) (cms.Page, error) {
	page, ok := m.pages[id]
	if !ok {
		return cms.Page{}, cms.ErrNotFound
	}

	return page, nil
}

func (m *memoryRepo) GetBySlug(_ context.Context, slug string) (cms.Page, error) {
	for _, page := range m.pages {
		if page.Slug == slug {
			return page, nil
		}
	}

	return cms.Page{}, cms.ErrNotFound
}

func (m *memoryRepo) List(context.Context) ([]cms.Page, error) {
	items := make([]cms.Page, 0, len(m.pages))
	for _, page := range m.pages {
		items = append(items, page)
	}

	return items, nil
}

func (m *memoryRepo) Update(_ context.Context, page cms.Page) error {
	if _, ok := m.pages[page.ID]; !ok {
		return cms.ErrNotFound
	}

	m.pages[page.ID] = page

	return nil
}

func TestCreateAndPublish(t *testing.T) {
	t.Parallel()

	svc := cms.NewService(&memoryRepo{})

	page, err := svc.Create(context.Background(), cms.CreateInput{
		Slug:  "pricing",
		Title: "Pricing",
		Body:  "Starter and growth plans.",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if page.Status != "draft" {
		t.Fatalf("status = %s, want draft", page.Status)
	}

	published, err := svc.Publish(context.Background(), page.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if published.Status != "published" || published.PublishedAt == nil {
		t.Fatalf("expected published page, got %+v", published)
	}

	publicPage, err := svc.GetPublishedBySlug(context.Background(), "pricing")
	if err != nil {
		t.Fatalf("get published: %v", err)
	}

	if publicPage.ID != page.ID {
		t.Fatalf("public page id mismatch")
	}
}

func TestCreateRejectsInvalidSlug(t *testing.T) {
	t.Parallel()

	svc := cms.NewService(&memoryRepo{})

	_, err := svc.Create(context.Background(), cms.CreateInput{
		Slug:  "Bad Slug",
		Title: "Pricing",
		Body:  "Body",
	})
	if !errors.Is(err, cms.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, cms.ErrInvalidInput)
	}
}
