package journeys_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	"launchpad/internal/journeys"
)

const testOrg = "org-1"

// --- in-memory fake ---------------------------------------------------------

type fakeJourneyRepo struct {
	templates map[string]journeys.Template
	steps     []journeys.Step
}

func newFakeJourneyRepo() *fakeJourneyRepo {
	return &fakeJourneyRepo{templates: map[string]journeys.Template{}}
}

func templateKey(organizationID, templateID string) string { return organizationID + "|" + templateID }

func (f *fakeJourneyRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeJourneyRepo) CreateTemplate(_ context.Context, template journeys.Template) error {
	f.templates[templateKey(template.OrganizationID, template.ID)] = template

	return nil
}

func (f *fakeJourneyRepo) GetTemplate(
	_ context.Context,
	organizationID, templateID string,
) (journeys.Template, error) {
	if template, ok := f.templates[templateKey(organizationID, templateID)]; ok {
		return template, nil
	}

	return journeys.Template{}, journeys.ErrNotFound
}

func (f *fakeJourneyRepo) ListTemplates(_ context.Context, organizationID string) ([]journeys.Template, error) {
	out := make([]journeys.Template, 0)

	for _, template := range f.templates {
		if template.OrganizationID == organizationID {
			out = append(out, template)
		}
	}

	return out, nil
}

func (f *fakeJourneyRepo) UpdateTemplate(_ context.Context, template journeys.Template) error {
	key := templateKey(template.OrganizationID, template.ID)
	if _, ok := f.templates[key]; !ok {
		return journeys.ErrNotFound
	}

	f.templates[key] = template

	return nil
}

func (f *fakeJourneyRepo) CreateStep(_ context.Context, step journeys.Step) error {
	f.steps = append(f.steps, step)

	return nil
}

func (f *fakeJourneyRepo) UpdateStep(_ context.Context, step journeys.Step) error {
	for i, existing := range f.steps {
		if existing.ID == step.ID && existing.OrganizationID == step.OrganizationID {
			f.steps[i] = step

			return nil
		}
	}

	return journeys.ErrStepNotFound
}

func (f *fakeJourneyRepo) DeleteStep(
	_ context.Context,
	organizationID, templateID string,
	version int,
	stepID string,
) error {
	for i, existing := range f.steps {
		if existing.ID == stepID &&
			existing.OrganizationID == organizationID &&
			existing.JourneyTemplateID == templateID &&
			existing.Version == version {
			f.steps = append(f.steps[:i], f.steps[i+1:]...)

			return nil
		}
	}

	return journeys.ErrStepNotFound
}

func (f *fakeJourneyRepo) ListSteps(
	_ context.Context,
	organizationID, templateID string,
	version int,
) ([]journeys.Step, error) {
	out := make([]journeys.Step, 0)

	for _, step := range f.steps {
		if step.OrganizationID == organizationID && step.JourneyTemplateID == templateID && step.Version == version {
			out = append(out, step)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })

	return out, nil
}

func (f *fakeJourneyRepo) CountSteps(
	_ context.Context,
	organizationID, templateID string,
	version int,
) (int64, error) {
	var count int64

	for _, step := range f.steps {
		if step.OrganizationID == organizationID && step.JourneyTemplateID == templateID && step.Version == version {
			count++
		}
	}

	return count, nil
}

func newJourneyService() *journeys.Service {
	return journeys.NewService(newFakeJourneyRepo())
}

func createDraft(t *testing.T, svc *journeys.Service) journeys.Template {
	t.Helper()

	template, err := svc.CreateTemplate(context.Background(), testOrg, journeys.CreateTemplateInput{
		Name: "Onboarding", CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	return template
}

// --- tests ------------------------------------------------------------------

func TestCreateTemplateValidatesInput(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()
	ctx := context.Background()

	for _, tc := range []struct {
		organizationID string
		in             journeys.CreateTemplateInput
	}{
		{"", journeys.CreateTemplateInput{Name: "X", CreatedBy: "u"}},
		{testOrg, journeys.CreateTemplateInput{Name: "  ", CreatedBy: "u"}},
		{testOrg, journeys.CreateTemplateInput{Name: "X"}},
	} {
		if _, err := svc.CreateTemplate(ctx, tc.organizationID, tc.in); !errors.Is(err, journeys.ErrInvalidInput) {
			t.Fatalf("input %+v: got %v, want ErrInvalidInput", tc, err)
		}
	}
}

func TestCreateTemplateStartsAsDraftVersionOne(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()

	template := createDraft(t, svc)

	if template.Status != "draft" || template.CurrentVersion != 1 {
		t.Fatalf("new template should be a v1 draft, got status=%q version=%d", template.Status, template.CurrentVersion)
	}
}

func TestAddStepAppendsPositionsWithinDraft(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()
	ctx := context.Background()

	template := createDraft(t, svc)

	first, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "document", Title: "Read the handbook",
	})
	if err != nil {
		t.Fatalf("add first step: %v", err)
	}

	second, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "quiz", Title: "Handbook quiz",
		Config: map[string]any{
			"questions": []any{
				map[string]any{
					"id": "q1", "text": "Q?", "options": []any{"a", "b"}, "correctOption": 0,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("add second step: %v", err)
	}

	if first.Position != 1 || second.Position != 2 {
		t.Fatalf("positions should increment from 1, got %d and %d", first.Position, second.Position)
	}

	if first.Version != 1 || second.Version != 1 {
		t.Fatalf("steps should belong to the current draft version, got %d and %d", first.Version, second.Version)
	}

	if first.Config == nil {
		t.Fatal("nil config input should be stored as an empty map, not nil")
	}

	steps, err := svc.ListSteps(ctx, testOrg, template.ID)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}

	if len(steps) != 2 || steps[0].Position != 1 || steps[1].Position != 2 {
		t.Fatalf("steps not listed in position order: %+v", steps)
	}
}

func TestAddStepValidatesInputAndTenant(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()
	ctx := context.Background()

	template := createDraft(t, svc)

	if _, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "hologram", Title: "X",
	}); !errors.Is(err, journeys.ErrInvalidInput) {
		t.Fatalf("invalid step type got %v, want ErrInvalidInput", err)
	}

	if _, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "task", Title: "  ",
	}); !errors.Is(err, journeys.ErrInvalidInput) {
		t.Fatalf("blank title got %v, want ErrInvalidInput", err)
	}

	if _, err := svc.AddStep(ctx, testOrg, "missing", journeys.AddStepInput{
		StepType: "task", Title: "X",
	}); !errors.Is(err, journeys.ErrNotFound) {
		t.Fatalf("unknown template got %v, want ErrNotFound", err)
	}

	// Another tenant must not see (or mutate) this template.
	if _, err := svc.AddStep(ctx, "org-2", template.ID, journeys.AddStepInput{
		StepType: "task", Title: "X",
	}); !errors.Is(err, journeys.ErrNotFound) {
		t.Fatalf("cross-tenant add got %v, want ErrNotFound", err)
	}

	if _, err := svc.GetTemplate(ctx, "org-2", template.ID); !errors.Is(err, journeys.ErrNotFound) {
		t.Fatalf("cross-tenant get got %v, want ErrNotFound", err)
	}
}

func TestPublishRequiresSteps(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()

	template := createDraft(t, svc)

	if _, err := svc.Publish(context.Background(), testOrg, template.ID); !errors.Is(err, journeys.ErrNoSteps) {
		t.Fatalf("publishing an empty draft got %v, want ErrNoSteps", err)
	}
}

func TestPublishTransitionsAndLocksTheDraft(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()
	ctx := context.Background()

	template := createDraft(t, svc)

	if _, err := svc.RequirePublished(ctx, testOrg, template.ID); !errors.Is(err, journeys.ErrNotPublished) {
		t.Fatalf("draft got %v, want ErrNotPublished", err)
	}

	if _, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "task", Title: "Sign the contract",
	}); err != nil {
		t.Fatalf("add step: %v", err)
	}

	published, err := svc.Publish(ctx, testOrg, template.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if published.Status != "published" {
		t.Fatalf("status = %q, want published", published.Status)
	}

	if _, err := svc.RequirePublished(ctx, testOrg, template.ID); err != nil {
		t.Fatalf("published template should satisfy RequirePublished: %v", err)
	}

	// A published template is locked: no new steps, no re-publish.
	if _, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "task", Title: "Late step",
	}); !errors.Is(err, journeys.ErrNotDraft) {
		t.Fatalf("add step after publish got %v, want ErrNotDraft", err)
	}

	if _, err := svc.Publish(ctx, testOrg, template.ID); !errors.Is(err, journeys.ErrNotDraft) {
		t.Fatalf("re-publish got %v, want ErrNotDraft", err)
	}

	// The published version's steps remain readable by version.
	steps, err := svc.ListStepsForVersion(ctx, testOrg, template.ID, published.CurrentVersion)
	if err != nil {
		t.Fatalf("list steps for version: %v", err)
	}

	if len(steps) != 1 || steps[0].Title != "Sign the contract" {
		t.Fatalf("published version should keep its steps, got %+v", steps)
	}
}

func publishTemplate(t *testing.T, svc *journeys.Service, templateID string) journeys.Template {
	t.Helper()

	published, err := svc.Publish(context.Background(), testOrg, templateID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	return published
}

func TestCreateNewVersionCopiesStepsIntoEditableDraft(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()
	ctx := context.Background()

	template := createDraft(t, svc)

	for _, title := range []string{"Read the handbook", "Sign the contract"} {
		if _, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
			StepType: "task", Title: title,
		}); err != nil {
			t.Fatalf("add step: %v", err)
		}
	}

	// A draft cannot spawn a new version.
	if _, err := svc.CreateNewVersion(ctx, testOrg, template.ID); !errors.Is(err, journeys.ErrNotPublished) {
		t.Fatalf("new version on draft got %v, want ErrNotPublished", err)
	}

	publishTemplate(t, svc, template.ID)

	draft, err := svc.CreateNewVersion(ctx, testOrg, template.ID)
	if err != nil {
		t.Fatalf("create new version: %v", err)
	}

	if draft.Status != "draft" || draft.CurrentVersion != 2 {
		t.Fatalf("new version should be a v2 draft, got status=%q version=%d", draft.Status, draft.CurrentVersion)
	}

	copied, err := svc.ListSteps(ctx, testOrg, template.ID)
	if err != nil {
		t.Fatalf("list copied steps: %v", err)
	}

	if len(copied) != 2 || copied[0].Version != 2 || copied[1].Version != 2 {
		t.Fatalf("expected 2 copied steps at version 2, got %+v", copied)
	}

	if copied[0].Title != "Read the handbook" || copied[1].Title != "Sign the contract" {
		t.Fatalf("copied steps should preserve titles, got %+v", copied)
	}

	originals, err := svc.ListStepsForVersion(ctx, testOrg, template.ID, 1)
	if err != nil {
		t.Fatalf("list original steps: %v", err)
	}

	if copied[0].ID == originals[0].ID {
		t.Fatal("copied steps must have fresh IDs")
	}

	// The new draft is editable.
	added, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "approval", Title: "Manager sign-off",
	})
	if err != nil {
		t.Fatalf("add step to new draft: %v", err)
	}

	if added.Version != 2 || added.Position != 3 {
		t.Fatalf("new step should land at v2 position 3, got version=%d position=%d", added.Version, added.Position)
	}

	// The published version is untouched.
	if len(originals) != 2 {
		t.Fatalf("published v1 should still have 2 steps, got %d", len(originals))
	}
}

func TestPublishCycleBumpsVersion(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()
	ctx := context.Background()

	template := createDraft(t, svc)

	if _, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "document", Title: "Handbook",
	}); err != nil {
		t.Fatalf("add step: %v", err)
	}

	v1 := publishTemplate(t, svc, template.ID)

	if v1.CurrentVersion != 1 {
		t.Fatalf("first publish should stay at version 1, got %d", v1.CurrentVersion)
	}

	if _, err := svc.CreateNewVersion(ctx, testOrg, template.ID); err != nil {
		t.Fatalf("create new version: %v", err)
	}

	v2 := publishTemplate(t, svc, template.ID)

	if v2.Status != "published" || v2.CurrentVersion != 2 {
		t.Fatalf("second publish should publish version 2, got status=%q version=%d", v2.Status, v2.CurrentVersion)
	}

	versions, err := svc.ListVersions(ctx, testOrg, template.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}

	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %+v", versions)
	}

	if versions[0].Version != 1 || versions[0].Status != "published" || versions[0].StepCount != 1 {
		t.Fatalf("unexpected v1 summary: %+v", versions[0])
	}

	if versions[1].Version != 2 || versions[1].Status != "published" || versions[1].StepCount != 1 {
		t.Fatalf("unexpected v2 summary: %+v", versions[1])
	}
}

func TestCloneTemplateIsIndependent(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()
	ctx := context.Background()

	source := createDraft(t, svc)

	if _, err := svc.AddStep(ctx, testOrg, source.ID, journeys.AddStepInput{
		StepType: "task", Title: "Sign the contract",
	}); err != nil {
		t.Fatalf("add step: %v", err)
	}

	publishTemplate(t, svc, source.ID)

	clone, err := svc.CloneTemplate(ctx, testOrg, source.ID, "user-2")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	if clone.ID == source.ID {
		t.Fatal("clone must be a new template")
	}

	if clone.Name != "Onboarding (copy)" {
		t.Fatalf("clone name = %q, want %q", clone.Name, "Onboarding (copy)")
	}

	if clone.Status != "draft" || clone.CurrentVersion != 1 || clone.CreatedBy != "user-2" {
		t.Fatalf("clone should be a v1 draft by user-2, got %+v", clone)
	}

	cloneSteps, err := svc.ListSteps(ctx, testOrg, clone.ID)
	if err != nil {
		t.Fatalf("list clone steps: %v", err)
	}

	if len(cloneSteps) != 1 || cloneSteps[0].Title != "Sign the contract" || cloneSteps[0].JourneyTemplateID != clone.ID {
		t.Fatalf("clone should carry the source steps, got %+v", cloneSteps)
	}

	// Editing the clone must not affect the source.
	if _, err := svc.AddStep(ctx, testOrg, clone.ID, journeys.AddStepInput{
		StepType: "task", Title: "Clone-only step",
	}); err != nil {
		t.Fatalf("add step to clone: %v", err)
	}

	sourceSteps, err := svc.ListStepsForVersion(ctx, testOrg, source.ID, 1)
	if err != nil {
		t.Fatalf("list source steps: %v", err)
	}

	if len(sourceSteps) != 1 {
		t.Fatalf("source should be untouched, got %d steps", len(sourceSteps))
	}

	sourceAfter, err := svc.GetTemplate(ctx, testOrg, source.ID)
	if err != nil {
		t.Fatalf("get source: %v", err)
	}

	if sourceAfter.Status != "published" || sourceAfter.Name != "Onboarding" {
		t.Fatalf("source template mutated by clone edit: %+v", sourceAfter)
	}
}

func TestRollbackPublishesOlderVersion(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()
	ctx := context.Background()

	template := createDraft(t, svc)

	if _, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "document", Title: "Handbook v1",
	}); err != nil {
		t.Fatalf("add step: %v", err)
	}

	publishTemplate(t, svc, template.ID)

	if _, err := svc.CreateNewVersion(ctx, testOrg, template.ID); err != nil {
		t.Fatalf("create new version: %v", err)
	}

	if _, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "task", Title: "New in v2",
	}); err != nil {
		t.Fatalf("add v2 step: %v", err)
	}

	publishTemplate(t, svc, template.ID)

	// Invalid targets are rejected.
	for _, version := range []int{0, -1, 99} {
		if _, err := svc.Rollback(ctx, testOrg, template.ID, version); !errors.Is(err, journeys.ErrInvalidInput) {
			t.Fatalf("rollback to %d got %v, want ErrInvalidInput", version, err)
		}
	}

	rolledBack, err := svc.Rollback(ctx, testOrg, template.ID, 1)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if rolledBack.Status != "published" || rolledBack.CurrentVersion != 3 {
		t.Fatalf("rollback should publish the old content as v3, got status=%q version=%d",
			rolledBack.Status, rolledBack.CurrentVersion)
	}

	current, err := svc.ListSteps(ctx, testOrg, template.ID)
	if err != nil {
		t.Fatalf("list current steps: %v", err)
	}

	if len(current) != 1 || current[0].Title != "Handbook v1" || current[0].Version != 3 {
		t.Fatalf("current version should mirror v1 content, got %+v", current)
	}

	// Assignments pinned to older versions keep working: v1 and v2 steps survive.
	v1Steps, err := svc.ListStepsForVersion(ctx, testOrg, template.ID, 1)
	if err != nil {
		t.Fatalf("list v1 steps: %v", err)
	}

	if len(v1Steps) != 1 || v1Steps[0].Title != "Handbook v1" {
		t.Fatalf("pinned v1 steps should be intact, got %+v", v1Steps)
	}

	v2Steps, err := svc.ListStepsForVersion(ctx, testOrg, template.ID, 2)
	if err != nil {
		t.Fatalf("list v2 steps: %v", err)
	}

	if len(v2Steps) != 2 {
		t.Fatalf("pinned v2 steps should be intact, got %+v", v2Steps)
	}

	// Rolling back to the current version is a no-op.
	noop, err := svc.Rollback(ctx, testOrg, template.ID, rolledBack.CurrentVersion)
	if err != nil {
		t.Fatalf("no-op rollback: %v", err)
	}

	if noop.CurrentVersion != rolledBack.CurrentVersion {
		t.Fatalf("no-op rollback changed the version to %d", noop.CurrentVersion)
	}
}

func TestRollbackRequiresPublishedTemplate(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()

	template := createDraft(t, svc)

	if _, err := svc.Rollback(context.Background(), testOrg, template.ID, 1); !errors.Is(err, journeys.ErrNotPublished) {
		t.Fatalf("rollback on draft got %v, want ErrNotPublished", err)
	}
}

func TestDeleteStepRenumbersDraft(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()
	ctx := context.Background()

	template := createDraft(t, svc)

	ids := make([]string, 0, 3)

	for _, title := range []string{"One", "Two", "Three"} {
		step, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
			StepType: "task", Title: title,
		})
		if err != nil {
			t.Fatalf("add step: %v", err)
		}

		ids = append(ids, step.ID)
	}

	if err := svc.DeleteStep(ctx, testOrg, template.ID, ids[1]); err != nil {
		t.Fatalf("delete step: %v", err)
	}

	steps, err := svc.ListSteps(ctx, testOrg, template.ID)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}

	if len(steps) != 2 || steps[0].Title != "One" || steps[1].Title != "Three" {
		t.Fatalf("unexpected steps after delete: %+v", steps)
	}

	if steps[0].Position != 1 || steps[1].Position != 2 {
		t.Fatalf("positions should be renumbered to 1..2, got %d, %d", steps[0].Position, steps[1].Position)
	}

	// Appending after a delete must not collide with an existing position.
	appended, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "task", Title: "Four",
	})
	if err != nil {
		t.Fatalf("append after delete: %v", err)
	}

	if appended.Position != 3 {
		t.Fatalf("appended step should take position 3, got %d", appended.Position)
	}

	if err := svc.DeleteStep(ctx, testOrg, template.ID, "missing"); !errors.Is(err, journeys.ErrStepNotFound) {
		t.Fatalf("delete unknown step got %v, want ErrStepNotFound", err)
	}

	publishTemplate(t, svc, template.ID)

	if err := svc.DeleteStep(ctx, testOrg, template.ID, ids[0]); !errors.Is(err, journeys.ErrNotDraft) {
		t.Fatalf("delete on published template got %v, want ErrNotDraft", err)
	}
}

func TestVersionOperationsEnforceTenantIsolation(t *testing.T) {
	t.Parallel()

	svc := newJourneyService()
	ctx := context.Background()

	template := createDraft(t, svc)

	if _, err := svc.AddStep(ctx, testOrg, template.ID, journeys.AddStepInput{
		StepType: "task", Title: "Sign the contract",
	}); err != nil {
		t.Fatalf("add step: %v", err)
	}

	publishTemplate(t, svc, template.ID)

	if _, err := svc.CreateNewVersion(ctx, "org-2", template.ID); !errors.Is(err, journeys.ErrNotFound) {
		t.Fatalf("cross-tenant new version got %v, want ErrNotFound", err)
	}

	if _, err := svc.CloneTemplate(ctx, "org-2", template.ID, "user-9"); !errors.Is(err, journeys.ErrNotFound) {
		t.Fatalf("cross-tenant clone got %v, want ErrNotFound", err)
	}

	if _, err := svc.Rollback(ctx, "org-2", template.ID, 1); !errors.Is(err, journeys.ErrNotFound) {
		t.Fatalf("cross-tenant rollback got %v, want ErrNotFound", err)
	}

	if _, err := svc.ListVersions(ctx, "org-2", template.ID); !errors.Is(err, journeys.ErrNotFound) {
		t.Fatalf("cross-tenant list versions got %v, want ErrNotFound", err)
	}

	if err := svc.DeleteStep(ctx, "org-2", template.ID, "step-1"); !errors.Is(err, journeys.ErrNotFound) {
		t.Fatalf("cross-tenant delete step got %v, want ErrNotFound", err)
	}
}

func (f *fakeJourneyRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
