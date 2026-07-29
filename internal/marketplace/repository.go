package marketplace

import "context"

type Repository interface {
	EnsureIndexes(context.Context) error
	Create(context.Context, Template) error
	Get(context.Context, string) (Template, error)
	List(context.Context, string) ([]Template, error)
	Update(context.Context, Template) error
	CreateInstallation(context.Context, Installation) error
	UpsertRating(context.Context, Rating) error
	ListRatings(context.Context, string) ([]Rating, error)
}

type JourneyInstaller interface {
	InstallMarketplaceTemplate(ctx context.Context, organizationID, createdBy, name, description string, steps []Step) (string, error)
}
