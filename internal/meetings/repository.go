package meetings

import (
	"context"
	"time"
)

// Repository persists meetings.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, meeting Meeting) error
	GetByIDForOrganization(ctx context.Context, organizationID, id string) (Meeting, error)
	Update(ctx context.Context, meeting Meeting) error
	// ListByOrganization returns meetings for a tenant, soonest first. An empty
	// status returns every status.
	ListByOrganization(ctx context.Context, organizationID, status string) ([]Meeting, error)
	// ListByAttendee returns one employee's meetings, soonest first.
	ListByAttendee(ctx context.Context, organizationID, employeeID string) ([]Meeting, error)
	ListUpcomingUnreminded(ctx context.Context, from, to time.Time) ([]Meeting, error)
	// DeleteForOrganization removes all of a tenant's meetings (GDPR purge).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// ConnectionRepository persists calendar connections.
type ConnectionRepository interface {
	EnsureIndexes(ctx context.Context) error
	// Upsert creates or replaces the tenant's connection for a provider,
	// encrypting the token at rest.
	Upsert(ctx context.Context, conn CalendarConnection) error
	// Get loads the tenant's connection for a provider, decrypting the token.
	Get(ctx context.Context, organizationID, provider string) (CalendarConnection, error)
	Delete(ctx context.Context, organizationID, provider string) error
	// DeleteForOrganization removes all of a tenant's connections (GDPR purge).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}
