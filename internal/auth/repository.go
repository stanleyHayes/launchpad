package auth

import (
	"context"

	"launchpad/internal/organizations"
)

// UserRepository persists users.
type UserRepository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, user User) error
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	// Update replaces a user document (used to set a password / activate an
	// invited account).
	Update(ctx context.Context, user User) error
}

// SessionRepository persists refresh sessions.
type SessionRepository interface {
	Save(ctx context.Context, sessionID, userID, orgID, refreshHash string) error
	Get(ctx context.Context, sessionID string) (userID, orgID, refreshHash string, err error)
	Delete(ctx context.Context, sessionID string) error
	// DeleteForUser revokes every session belonging to a user (password reset
	// implies all sessions established with the old credentials are suspect).
	DeleteForUser(ctx context.Context, userID string) error
}

// InvitationStore persists single-use, expiring account invitations.
type InvitationStore interface {
	EnsureIndexes(ctx context.Context) error
	Save(ctx context.Context, invitation Invitation) error
	// Consume atomically finds and deletes a non-expired invitation by token
	// hash, returning ErrInvitationInvalid if none matches (unknown/expired or
	// already consumed). Atomicity makes the token single-use under
	// concurrent redemption.
	Consume(ctx context.Context, tokenHash string) (Invitation, error)
	// DeleteForOrganization removes every invitation of the organization and
	// returns the number deleted. Called only by the platform GDPR tenant
	// purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

type InvitationManagementStore interface {
	ListForOrganization(ctx context.Context, organizationID string) ([]Invitation, error)
	GetForOrganization(ctx context.Context, organizationID, invitationID string) (Invitation, error)
	DeleteForOrganizationByID(ctx context.Context, organizationID, invitationID string) error
	DeleteForUser(ctx context.Context, organizationID, userID string) error
}

// PasswordResetStore persists single-use, expiring password-reset tokens.
type PasswordResetStore interface {
	EnsureIndexes(ctx context.Context) error
	Save(ctx context.Context, reset PasswordReset) error
	// Consume atomically finds and deletes a non-expired reset by token hash,
	// returning ErrPasswordResetInvalid if none matches (unknown/expired or
	// already consumed). Atomicity makes the token single-use under concurrent
	// redemption.
	Consume(ctx context.Context, tokenHash string) (PasswordReset, error)
	// DeleteForUsers removes every password-reset token of the given users
	// and returns the number deleted. Reset tokens carry no organization
	// scope, so the platform GDPR tenant purge (PRD 7.4) deletes them by the
	// user ids collected from the organization's memberships.
	DeleteForUsers(ctx context.Context, userIDs []string) (int64, error)
}

// MFAStore persists per-user TOTP enrollments, scoped to an organization
// (an empty organizationID is the platform-staff scope).
type MFAStore interface {
	EnsureIndexes(ctx context.Context) error
	// Get loads the enrollment for the scope, returning ErrMFANotEnrolled when
	// none exists.
	Get(ctx context.Context, organizationID, userID string) (MFAEnrollment, error)
	// Upsert replaces the enrollment for the scope.
	Upsert(ctx context.Context, enrollment MFAEnrollment) error
	// ConsumeBackupCode atomically removes one matching backup-code hash and
	// reports whether one matched, making backup codes single-use under
	// concurrent redemption.
	ConsumeBackupCode(ctx context.Context, organizationID, userID, codeHash string) (bool, error)
	Delete(ctx context.Context, organizationID, userID string) error
	// DeleteForOrganization removes all of a tenant's enrollments (GDPR purge).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// MFATicketStore persists single-use, expiring MFA login tickets.
type MFATicketStore interface {
	EnsureIndexes(ctx context.Context) error
	Save(ctx context.Context, ticket MFATicket) error
	// Consume atomically finds and deletes a non-expired ticket by hash,
	// returning ErrMFATicketInvalid if none matches (unknown/expired or already
	// consumed). Atomicity makes the ticket single-use under concurrent
	// redemption.
	Consume(ctx context.Context, ticketHash string) (MFATicket, error)
}

// OrgDirectory is the slice of the organizations service the auth flows depend
// on. *organizations.Service satisfies it; depending on the interface keeps the
// auth service testable in isolation.
type OrgDirectory interface {
	CreateWithOwner(
		ctx context.Context,
		in organizations.CreateInput,
	) (organizations.Organization, organizations.Membership, error)
	Get(ctx context.Context, id string) (organizations.Organization, error)
	Membership(ctx context.Context, organizationID, userID string) (organizations.Membership, error)
	ListMembershipsForUser(ctx context.Context, userID string) ([]organizations.Membership, error)
	AddMember(ctx context.Context, organizationID, userID, roleCode string) (organizations.Membership, error)
}
