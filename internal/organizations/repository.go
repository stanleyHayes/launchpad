package organizations

import "context"

// Repository persists organizations and memberships.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	CreateOrganization(ctx context.Context, org Organization) error
	GetByID(ctx context.Context, id string) (Organization, error)
	GetBySlug(ctx context.Context, slug string) (Organization, error)
	Update(ctx context.Context, org Organization) error
	List(ctx context.Context) ([]Organization, error)
	CountByStatus(ctx context.Context) (map[string]int64, error)
	CreateMembership(ctx context.Context, membership Membership) error
	GetMembership(ctx context.Context, organizationID, userID string) (Membership, error)
	ListMembershipsByUser(ctx context.Context, userID string) ([]Membership, error)
}
