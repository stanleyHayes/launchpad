package requests

import "context"

// Repository persists equipment and access requests.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, request Request) error
	GetByIDForOrganization(ctx context.Context, organizationID, id string) (Request, error)
	Update(ctx context.Context, request Request) error
	// ListByOrganization returns requests for a tenant, newest first. An empty
	// status returns every status.
	ListByOrganization(ctx context.Context, organizationID, status string) ([]Request, error)
	ListByRequester(ctx context.Context, organizationID, employeeID string) ([]Request, error)
	// DeleteForOrganization removes all of a tenant's requests (GDPR purge).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}
