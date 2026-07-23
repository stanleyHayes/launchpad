package journeys

import "context"

// Repository persists journey templates and steps.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	CreateTemplate(ctx context.Context, template Template) error
	GetTemplate(ctx context.Context, organizationID, templateID string) (Template, error)
	ListTemplates(ctx context.Context, organizationID string) ([]Template, error)
	UpdateTemplate(ctx context.Context, template Template) error
	CreateStep(ctx context.Context, step Step) error
	ListSteps(ctx context.Context, organizationID, templateID string, version int) ([]Step, error)
	CountSteps(ctx context.Context, organizationID, templateID string, version int) (int64, error)
}
