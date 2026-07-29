package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/pkg/security"
)

const (
	actionInvitationIssued   = "auth.invitation.issued"
	actionInvitationAccepted = "auth.invitation.accepted"
	actionInvitationRevoked  = "auth.invitation.revoked"
	actionInvitationResent   = "auth.invitation.resent"
)

// IssueInvitation creates (or re-uses) a pending account for an email in the
// caller's organization, grants it a membership, and returns a single-use
// invitation token (shown once). The invitee redeems it via AcceptInvitation to
// set a password and activate the account.
//
// The flow is idempotent/retryable: an email that already has an ACTIVE account,
// or a pending invite belonging to a different organization, returns ErrEmailTaken
// (it never re-uses or grafts an active/foreign account); but a pending invite in
// THIS organization is re-used, so a partial failure on a previous attempt can be
// healed by simply inviting again (a fresh token is issued each time).
func (s *Service) IssueInvitation(
	ctx context.Context,
	organizationID, email, displayName, roleCode, actorUserID string,
) (string, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	normalizedName := strings.TrimSpace(displayName)

	role, err := normalizeInviteRole(roleCode)
	if err != nil {
		return "", err
	}

	if organizationID == "" || normalizedEmail == "" || normalizedName == "" || !strings.Contains(normalizedEmail, "@") {
		return "", ErrInvalidInput
	}

	user, err := s.resolveInviteUser(ctx, organizationID, normalizedEmail, normalizedName)
	if err != nil {
		return "", err
	}

	if err := s.ensureMembership(ctx, organizationID, user.ID, role); err != nil {
		return "", err
	}

	token, err := s.storeInvitation(ctx, user, organizationID, role, normalizedEmail)
	if err != nil {
		return "", err
	}

	orgID := organizationID
	if err := s.audit.Record(ctx, &orgID, actorUserID, actionInvitationIssued, "user", user.ID, map[string]any{
		fieldEmail: normalizedEmail,
		"role":     role,
	}); err != nil {
		return "", fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	s.sendInvitationEmail(ctx, normalizedEmail, normalizedName, token)

	return token, nil
}

// resolveInviteUser returns the pending user to invite: an existing pending
// invite for this organization is re-used (making retries idempotent); an active
// account, or a pending invite scoped to another organization, is refused; a new
// email creates a fresh pending user.
func (s *Service) resolveInviteUser(ctx context.Context, organizationID, email, displayName string) (User, error) {
	existing, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return s.createPendingUser(ctx, email, displayName)
		}

		return User{}, fmt.Errorf("check existing user: %w", err)
	}

	if existing.Status != userStatusInvited {
		return User{}, ErrEmailTaken
	}

	memberships, err := s.orgs.ListMembershipsForUser(ctx, existing.ID)
	if err != nil {
		return User{}, fmt.Errorf("list invited memberships: %w", err)
	}

	for _, membership := range memberships {
		if membership.OrganizationID != organizationID {
			return User{}, ErrEmailTaken
		}
	}

	return existing, nil
}

// ensureMembership adds the invited membership only if one does not already
// exist, so re-issuing an invitation does not create a duplicate.
func (s *Service) ensureMembership(ctx context.Context, organizationID, userID, role string) error {
	if _, err := s.orgs.Membership(ctx, organizationID, userID); err == nil {
		return nil
	}

	if _, err := s.orgs.AddMember(ctx, organizationID, userID, role); err != nil {
		return fmt.Errorf("add invited member: %w", err)
	}

	return nil
}

// AcceptInvitation redeems an invitation token: it sets the invited user's
// password, activates the account, and issues a session (logging the user in).
//
// The token is consumed atomically (findOneAndDelete gated on expiry) BEFORE
// the account is activated, so two concurrent redemptions of the same token
// cannot both succeed — the token is strictly single-use. If activation fails
// transiently after consumption the invite is burned, but a manager can heal
// the account by simply issuing a fresh invitation (IssueInvitation re-uses
// the pending user). An already-active account short-circuits with
// ErrInvitationInvalid, so a lingering token can never re-set the password of
// an activated account.
func (s *Service) AcceptInvitation(ctx context.Context, token, password string) (Result, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Result{}, ErrInvitationInvalid
	}

	if len(password) < s.cfg.PasswordMinLen {
		return Result{}, ErrWeakPassword
	}

	tokenHash := security.HashToken(token)

	invitation, err := s.invitations.Consume(ctx, tokenHash)
	if err != nil {
		return Result{}, ErrInvitationInvalid
	}

	user, err := s.users.GetByID(ctx, invitation.UserID)
	if err != nil {
		return Result{}, ErrInvitationInvalid
	}

	if user.Status == userStatusActive {
		return Result{}, ErrInvitationInvalid
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return Result{}, fmt.Errorf("hash password: %w", err)
	}

	user.PasswordHash = hash
	user.Status = userStatusActive

	user.UpdatedAt = time.Now().UTC()
	if err := s.users.Update(ctx, user); err != nil {
		return Result{}, fmt.Errorf("activate invited user: %w", err)
	}

	org, err := s.orgs.Get(ctx, invitation.OrganizationID)
	if err != nil {
		return Result{}, fmt.Errorf("load organization: %w", err)
	}

	orgID := org.ID
	if err := s.audit.Record(ctx, &orgID, user.ID, actionInvitationAccepted, "user", user.ID, nil); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	tokens, err := s.issueTokens(ctx, user, org.ID, invitation.RoleCode)
	if err != nil {
		return Result{}, fmt.Errorf("issue invitation tokens: %w", err)
	}

	return Result{User: toPublic(user), Organization: toOrganizationPublicPtr(org), Tokens: tokens}, nil
}

func (s *Service) storeInvitation(
	ctx context.Context,
	user User,
	organizationID, role, email string,
) (string, error) {
	if management, ok := s.invitations.(InvitationManagementStore); ok {
		if err := management.DeleteForUser(ctx, organizationID, user.ID); err != nil {
			return "", fmt.Errorf("revoke prior invitations: %w", err)
		}
	}
	raw, err := security.NewRefreshToken()
	if err != nil {
		return "", fmt.Errorf("generate invitation token: %w", err)
	}

	token := invitationTokenPrefix + raw
	now := time.Now().UTC()

	invitation := Invitation{
		ID:             uuid.NewString(),
		TokenHash:      security.HashToken(token),
		UserID:         user.ID,
		OrganizationID: organizationID,
		RoleCode:       role,
		Email:          email,
		ExpiresAt:      now.Add(s.cfg.InviteTTL),
		CreatedAt:      now,
	}
	if err := s.invitations.Save(ctx, invitation); err != nil {
		return "", fmt.Errorf("save invitation: %w", err)
	}

	return token, nil
}

func (s *Service) ListInvitations(ctx context.Context, organizationID string) ([]Invitation, error) {
	management, ok := s.invitations.(InvitationManagementStore)
	if !ok || organizationID == "" {
		return nil, ErrInvalidInput
	}
	items, err := management.ListForOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	return items, nil
}

func (s *Service) RevokeInvitation(
	ctx context.Context,
	organizationID, invitationID, actorUserID string,
) error {
	management, ok := s.invitations.(InvitationManagementStore)
	if !ok || organizationID == "" || invitationID == "" {
		return ErrInvalidInput
	}
	invitation, err := management.GetForOrganization(ctx, organizationID, invitationID)
	if err != nil {
		return err
	}
	if err := management.DeleteForOrganizationByID(ctx, organizationID, invitationID); err != nil {
		return err
	}
	orgID := organizationID
	if err := s.audit.Record(ctx, &orgID, actorUserID, actionInvitationRevoked, "invitation", invitationID, map[string]any{
		fieldEmail: invitation.Email,
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}
	return nil
}

func (s *Service) ResendInvitation(
	ctx context.Context,
	organizationID, invitationID, actorUserID string,
) (string, error) {
	management, ok := s.invitations.(InvitationManagementStore)
	if !ok || organizationID == "" || invitationID == "" {
		return "", ErrInvalidInput
	}
	invitation, err := management.GetForOrganization(ctx, organizationID, invitationID)
	if err != nil {
		return "", err
	}
	user, err := s.users.GetByID(ctx, invitation.UserID)
	if err != nil {
		return "", err
	}
	token, err := s.IssueInvitation(
		ctx, organizationID, invitation.Email, user.DisplayName, invitation.RoleCode, actorUserID,
	)
	if err != nil {
		return "", err
	}
	orgID := organizationID
	if err := s.audit.Record(ctx, &orgID, actorUserID, actionInvitationResent, "invitation", invitationID, nil); err != nil {
		return "", fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}
	return token, nil
}

// sendInvitationEmail best-effort emails the accept link when a mailer is
// configured. Delivery failures are logged, never fatal: the token is still
// returned in the API response (backward compatible with the pre-email flow,
// and a log-only sender keeps today's no-delivery behavior).
func (s *Service) sendInvitationEmail(ctx context.Context, email, displayName, token string) {
	if s.mailer == nil || s.inviteAcceptBaseURL == "" {
		return
	}

	link := s.inviteAcceptBaseURL + "?token=" + url.QueryEscape(token)
	subject := "You've been invited to LaunchPad"
	body := fmt.Sprintf(
		`<p>Hi %s,</p>`+
			`<p>You've been invited to join your organization on LaunchPad.</p>`+
			`<p><a href="%s">Accept the invitation</a> to set your password and activate your account.</p>`,
		displayName, link,
	)

	if err := s.mailer.Send(ctx, email, subject, body); err != nil {
		slog.ErrorContext(ctx, "send invitation email", "error", err)
	}
}

func (s *Service) createPendingUser(ctx context.Context, email, displayName string) (User, error) {
	now := time.Now().UTC()
	user := User{
		ID:          uuid.NewString(),
		Email:       email,
		DisplayName: displayName,
		Status:      userStatusInvited,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return User{}, fmt.Errorf("create pending user: %w", err)
	}

	return user, nil
}

// normalizeInviteRole restricts invitations to member roles (never owner).
func normalizeInviteRole(roleCode string) (string, error) {
	role := strings.TrimSpace(roleCode)
	if role == "" {
		return roleEmployee, nil
	}

	if role == roleHRAdmin || role == roleEmployee {
		return role, nil
	}

	return "", ErrInvalidInput
}
