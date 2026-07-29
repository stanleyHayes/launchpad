package organizations

import (
	"context"
	"errors"
)

// ErrAlreadyMember indicates a membership already exists for the
// (organization, user) pair. Provisioning flows treat it as the goal state:
// a retried attempt must not fail when the membership is already present.
var ErrAlreadyMember = errors.New("membership already exists")

// Repository persists organizations and memberships.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	CreateOrganization(ctx context.Context, org Organization) error
	DeleteOrganization(ctx context.Context, id string) error
	// DeleteForOrganization removes the organization document and all of its
	// memberships, returning the number of documents deleted. Called only by
	// the platform GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
	GetByID(ctx context.Context, id string) (Organization, error)
	GetBySlug(ctx context.Context, slug string) (Organization, error)
	Update(ctx context.Context, org Organization) error
	List(ctx context.Context) ([]Organization, error)
	CountByStatus(ctx context.Context) (map[string]int64, error)
	CreateMembership(ctx context.Context, membership Membership) error
	GetMembership(ctx context.Context, organizationID, userID string) (Membership, error)
	ListMemberships(ctx context.Context, organizationID string) ([]Membership, error)
	ListMembershipsByUser(ctx context.Context, userID string) ([]Membership, error)
	UpdateMembershipStatus(ctx context.Context, organizationID, userID, status string) error
	UpdateMembershipRole(ctx context.Context, organizationID, userID, roleCode string) error
	CountMembershipsByRole(ctx context.Context, organizationID, roleCode string) (int64, error)
	MembershipExists(ctx context.Context, organizationID, userID string) (bool, error)
}
