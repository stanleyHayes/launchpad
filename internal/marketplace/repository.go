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
	CreatePurchase(context.Context, Purchase) error
	GetPurchaseByReference(context.Context, string, string) (Purchase, error)
	UpdatePurchase(context.Context, Purchase) error
	HasPaidPurchase(context.Context, string, string) (bool, error)
}

type JourneyInstaller interface {
	InstallMarketplaceTemplate(ctx context.Context, organizationID, createdBy, name, description string, steps []Step) (string, error)
}
