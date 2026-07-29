package supportsessions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/notifications"
	"launchpad/internal/organizations"
	"launchpad/internal/roles"
	"launchpad/pkg/security"
)

// OrganizationReader validates that the target tenant exists. Satisfied by
// *organizations.Service.
type OrganizationReader interface {
	Get(ctx context.Context, id string) (organizations.Organization, error)
}

// MemberLister lists the tenant's memberships so the service can find the
// organization owners to notify. Satisfied by *organizations.Service.
type MemberLister interface {
	ListMembers(ctx context.Context, organizationID string) ([]organizations.Member, error)
}

// Notifier creates in-app notifications. Satisfied by
// *notifications.Service.
type Notifier interface {
	Create(ctx context.Context, organizationID string, in notifications.CreateInput) (notifications.Notification, error)
}

// SessionChecker is the subset of middleware.SessionChecker this package
// implements, declared locally so the domain does not import the HTTP layer.
type SessionChecker interface {
	SessionExists(ctx context.Context, sessionID string) (bool, error)
}

// Config carries the secrets the service needs to issue impersonation tokens.
type Config struct {
	JWTSecret string
}

// Service implements support session use cases.
type Service struct {
	repo     Repository
	orgs     OrganizationReader
	members  MemberLister
	notifier Notifier
	cfg      Config
}

// NewService constructs a support session Service.
func NewService(
	repo Repository,
	orgs OrganizationReader,
	members MemberLister,
	notifier Notifier,
	cfg Config,
) *Service {
	return &Service{repo: repo, orgs: orgs, members: members, notifier: notifier, cfg: cfg}
}

// Create validates the request, stores the session, issues the read-only
// impersonation token, and notifies the organization's owners. The
// notification is best-effort: a notification failure never blocks support
// access, but it is logged.
func (s *Service) Create(ctx context.Context, in CreateInput) (Session, string, error) {
	reason := strings.TrimSpace(in.Reason)
	if in.OrganizationID == "" || in.AgentUserID == "" || len(reason) < MinReasonLength {
		return Session{}, "", ErrInvalidInput
	}

	durationMinutes := in.DurationMinutes
	if durationMinutes == 0 {
		durationMinutes = MaxDurationMinutes
	}

	if durationMinutes < 0 || durationMinutes > MaxDurationMinutes {
		return Session{}, "", ErrInvalidInput
	}

	if _, err := s.orgs.Get(ctx, in.OrganizationID); err != nil {
		return Session{}, "", fmt.Errorf("load organization: %w", err)
	}

	now := time.Now().UTC()

	session := Session{
		ID:             uuid.NewString(),
		OrganizationID: in.OrganizationID,
		AgentUserID:    in.AgentUserID,
		Reason:         reason,
		CreatedAt:      now,
		ExpiresAt:      now.Add(time.Duration(durationMinutes) * time.Minute),
	}
	if err := s.repo.Create(ctx, session); err != nil {
		return Session{}, "", fmt.Errorf("create support session: %w", err)
	}

	// The token's subject stays the support agent so audit actors are always
	// the human responsible; the tenant role code is hr_admin-equivalent, but
	// the authorization middleware restricts impersonators to the fixed
	// read-only security.ImpersonatorPermissions set regardless. SessionID
	// doubles as the support session id so token validation can enforce
	// early end and expiry server-side.
	token, err := security.IssueAccessToken(s.cfg.JWTSecret, TokenTTL, security.Principal{
		UserID:                 in.AgentUserID,
		Email:                  in.AgentEmail,
		OrganizationID:         in.OrganizationID,
		RoleCode:               roles.RoleHRAdmin,
		SessionID:              session.ID,
		Impersonator:           true,
		ImpersonationSessionID: session.ID,
	})
	if err != nil {
		return Session{}, "", fmt.Errorf("issue impersonation token: %w", err)
	}

	s.notifyOwners(ctx, session)

	return session, token, nil
}

// End closes an active session early, invalidating its impersonation tokens
// at the next validation. Ending an already-ended session is a conflict;
// ending an expired-but-open session is allowed so the audit trail shows a
// deliberate close.
func (s *Service) End(ctx context.Context, sessionID, endReason string) (Session, error) {
	endReason = strings.TrimSpace(endReason)
	if endReason == "" {
		endReason = EndReasonEndedByAgent
	}

	if sessionID == "" || len(endReason) > maxEndReasonLength {
		return Session{}, ErrInvalidInput
	}

	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return Session{}, fmt.Errorf("load support session: %w", err)
	}

	if session.EndedAt != nil {
		return Session{}, ErrSessionEnded
	}

	now := time.Now().UTC()
	session.EndedAt = &now
	session.EndReason = endReason

	if err := s.repo.Update(ctx, session); err != nil {
		return Session{}, fmt.Errorf("end support session: %w", err)
	}

	return session, nil
}

// List returns the organization's support sessions, newest first.
func (s *Service) List(ctx context.Context, organizationID string) ([]Session, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	sessions, err := s.repo.ListByOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list support sessions: %w", err)
	}

	return sessions, nil
}

// Active reports whether the session still authorizes impersonation tokens:
// it exists, has not been ended, and has not passed its expiry. Expiry is
// enforced here at validation time rather than by a TTL sweeper so a dead
// scheduler can never extend access.
func (s *Service) Active(ctx context.Context, sessionID string) (bool, error) {
	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}

		return false, fmt.Errorf("load support session: %w", err)
	}

	if session.EndedAt != nil || !time.Now().UTC().Before(session.ExpiresAt) {
		return false, nil
	}

	return true, nil
}

// TokenChecker adapts the service to the session-store check the
// authentication middleware runs on every request: an impersonation token's
// sessionId claim is the support session id.
type TokenChecker struct {
	svc *Service
}

// NewTokenChecker constructs a TokenChecker.
func NewTokenChecker(svc *Service) TokenChecker {
	return TokenChecker{svc: svc}
}

// SessionExists reports whether the support session is still active.
func (c TokenChecker) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	return c.svc.Active(ctx, sessionID)
}

// ChainedSessionChecker tries each checker in order and allows the request
// when any of them finds an active session. Store errors are remembered and
// returned only when no checker matched, so a Redis outage still 503s
// ordinary sessions while a valid support session remains usable.
//
//nolint:ireturn // factory intentionally composes SessionChecker implementations
func ChainedSessionChecker(checkers ...SessionChecker) SessionChecker {
	return chainedSessionChecker{checkers: checkers}
}

type chainedSessionChecker struct {
	checkers []SessionChecker
}

func (c chainedSessionChecker) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	var firstErr error

	for _, checker := range c.checkers {
		exists, err := checker.SessionExists(ctx, sessionID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}

			continue
		}

		if exists {
			return true, nil
		}
	}

	if firstErr != nil {
		return false, firstErr
	}

	return false, nil
}

// notifyOwners informs every organization owner that a support session
// started. Best-effort: failures are logged, never returned.
func (s *Service) notifyOwners(ctx context.Context, session Session) {
	if s.members == nil || s.notifier == nil {
		return
	}

	members, err := s.members.ListMembers(ctx, session.OrganizationID)
	if err != nil {
		slog.WarnContext(ctx, "support session: list owners for notification", "error", err)

		return
	}

	for _, member := range members {
		if member.Membership.RoleCode != roles.RoleOrganizationOwner {
			continue
		}

		_, err := s.notifier.Create(ctx, session.OrganizationID, notifications.CreateInput{
			UserID: member.Membership.UserID,
			Type:   notifications.TypeSystem,
			Title:  "Platform support session started",
			Body: fmt.Sprintf(
				"A LaunchPad platform support agent started a read-only support session for your organization. "+
					"Reason: %s. The session expires at %s and every action is audited.",
				session.Reason,
				session.ExpiresAt.Format(time.RFC3339),
			),
			Link: "/support",
		})
		if err != nil {
			slog.WarnContext(ctx, "support session: notify owner", "error", err, "userId", member.Membership.UserID)
		}
	}
}
