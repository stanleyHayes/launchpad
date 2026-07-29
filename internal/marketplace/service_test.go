package marketplace_test

import (
	"context"
	"testing"

	"launchpad/internal/marketplace"
)

type memoryRepo struct {
	templates     map[string]marketplace.Template
	installations []marketplace.Installation
	ratings       map[string]marketplace.Rating
}

func newRepo() *memoryRepo {
	return &memoryRepo{templates: map[string]marketplace.Template{}, ratings: map[string]marketplace.Rating{}}
}
func (*memoryRepo) EnsureIndexes(context.Context) error { return nil }
func (r *memoryRepo) Create(_ context.Context, item marketplace.Template) error {
	r.templates[item.ID] = item
	return nil
}
func (r *memoryRepo) Get(_ context.Context, id string) (marketplace.Template, error) {
	item, ok := r.templates[id]
	if !ok {
		return marketplace.Template{}, marketplace.ErrNotFound
	}
	return item, nil
}
func (r *memoryRepo) List(_ context.Context, status string) ([]marketplace.Template, error) {
	out := []marketplace.Template{}
	for _, item := range r.templates {
		if status == "" || item.Status == status {
			out = append(out, item)
		}
	}
	return out, nil
}
func (r *memoryRepo) Update(_ context.Context, item marketplace.Template) error {
	r.templates[item.ID] = item
	return nil
}
func (r *memoryRepo) CreateInstallation(_ context.Context, item marketplace.Installation) error {
	r.installations = append(r.installations, item)
	return nil
}
func (r *memoryRepo) UpsertRating(_ context.Context, item marketplace.Rating) error {
	r.ratings[item.TemplateID+"|"+item.OrganizationID+"|"+item.UserID] = item
	return nil
}
func (r *memoryRepo) ListRatings(_ context.Context, id string) ([]marketplace.Rating, error) {
	out := []marketplace.Rating{}
	for _, item := range r.ratings {
		if item.TemplateID == id {
			out = append(out, item)
		}
	}
	return out, nil
}

type installer struct{ calls int }

func (i *installer) InstallMarketplaceTemplate(_ context.Context, _, _, _, _ string, steps []marketplace.Step) (string, error) {
	i.calls++
	if len(steps) == 0 {
		return "", marketplace.ErrInvalidInput
	}
	return "journey-installed", nil
}

func TestModerationVersionInstallAndRatings(t *testing.T) {
	t.Parallel()
	repo, install := newRepo(), &installer{}
	svc := marketplace.NewService(repo, install)
	ctx := context.Background()
	item, err := svc.Create(ctx, marketplace.CreateInput{
		Name: "Engineer onboarding", Category: "engineering", Official: true, CreatedBy: "staff-1",
		Steps: []marketplace.Step{{StepType: "task", Title: "Ship a change"}},
	})
	if err != nil || item.Status != marketplace.StatusDraft || item.Version != 1 {
		t.Fatalf("create: %v (%+v)", err, item)
	}
	item, err = svc.SetStatus(ctx, item.ID, marketplace.StatusPublished)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	item, err = svc.SetFeatured(ctx, item.ID, true)
	if err != nil || !item.Featured {
		t.Fatalf("feature: %v (%+v)", err, item)
	}
	installation, err := svc.Install(ctx, item.ID, "org-1", "user-1")
	if err != nil || installation.JourneyTemplateID != "journey-installed" || install.calls != 1 {
		t.Fatalf("install: %v (%+v)", err, installation)
	}
	item, err = svc.Rate(ctx, item.ID, "org-1", "user-1", 5)
	if err != nil || item.RatingAverage != 5 || item.RatingCount != 1 {
		t.Fatalf("rate: %v (%+v)", err, item)
	}
	item, err = svc.NewVersion(ctx, item.ID, []marketplace.Step{{StepType: "document", Title: "Read handbook"}})
	if err != nil || item.Version != 2 || item.Status != marketplace.StatusDraft || item.Featured {
		t.Fatalf("version: %v (%+v)", err, item)
	}
}

func TestCustomerSubmissionRequiresReview(t *testing.T) {
	t.Parallel()
	svc := marketplace.NewService(newRepo(), &installer{})
	item, err := svc.Create(context.Background(), marketplace.CreateInput{
		Name: "Sales ramp", Category: "sales", SubmittedByOrganizationID: "org-1", CreatedBy: "user-1",
		Steps: []marketplace.Step{{StepType: "task", Title: "Practice pitch"}},
	})
	if err != nil || item.Status != marketplace.StatusSubmitted || item.Official {
		t.Fatalf("submission: %v (%+v)", err, item)
	}
}
