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

// errPasswordResetsNotConfigured indicates the reset store was never wired
// (WithPasswordResets not called).
var errPasswordResetsNotConfigured = errors.New("password resets are not configured")

const (
	actionPasswordResetRequested = "auth.password_reset.requested"
	actionPasswordResetCompleted = "auth.password_reset.completed"
	// passwordResetTTL bounds how long a reset link stays valid.
	passwordResetTTL = time.Hour
)

// RequestPasswordReset issues a single-use, 1-hour reset token for the account
// with the given email and, when a mailer is configured, emails the reset
// link. Unknown emails return success exactly like known ones — the HTTP
// handler always answers 202 — so the endpoint cannot be used to enumerate
// accounts.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" || !strings.Contains(normalizedEmail, "@") {
		return ErrInvalidInput
	}

	if s.passwordResets == nil {
		return errPasswordResetsNotConfigured
	}

	user, err := s.users.GetByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			// Unknown account: same outcome as a known one (no enumeration).
			return nil
		}

		return fmt.Errorf("load user for password reset: %w", err)
	}

	raw, err := security.NewRefreshToken()
	if err != nil {
		return fmt.Errorf("generate password reset token: %w", err)
	}

	token := passwordResetTokenPrefix + raw
	now := time.Now().UTC()

	reset := PasswordReset{
		ID:        uuid.NewString(),
		TokenHash: security.HashToken(token),
		UserID:    user.ID,
		Email:     normalizedEmail,
		ExpiresAt: now.Add(passwordResetTTL),
		CreatedAt: now,
	}
	if err := s.passwordResets.Save(ctx, reset); err != nil {
		return fmt.Errorf("save password reset: %w", err)
	}

	if err := s.audit.Record(ctx, nil, user.ID, actionPasswordResetRequested, "user", user.ID, map[string]any{
		fieldEmail: normalizedEmail,
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	s.sendPasswordResetEmail(ctx, normalizedEmail, token)

	return nil
}

// ResetPassword redeems a reset token: it sets the user's new password and
// revokes ALL of the user's existing sessions (a reset implies the old
// credentials — and any sessions established with them — are suspect).
//
// Like AcceptInvitation, the token is consumed atomically (findOneAndDelete
// gated on expiry) BEFORE the password is changed, so the token is strictly
// single-use under concurrent redemption; a transient failure after
// consumption burns the token and the user simply requests a fresh reset. A
// weak-password attempt is rejected before consumption, so it does not burn
// the token.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrPasswordResetInvalid
	}

	if len(newPassword) < s.cfg.PasswordMinLen {
		return ErrWeakPassword
	}

	if s.passwordResets == nil {
		return errPasswordResetsNotConfigured
	}

	reset, err := s.passwordResets.Consume(ctx, security.HashToken(token))
	if err != nil {
		return ErrPasswordResetInvalid
	}

	user, err := s.users.GetByID(ctx, reset.UserID)
	if err != nil {
		return ErrPasswordResetInvalid
	}

	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user.PasswordHash = hash
	user.UpdatedAt = time.Now().UTC()

	if err := s.users.Update(ctx, user); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	if err := s.sessions.DeleteForUser(ctx, user.ID); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}

	if err := s.audit.Record(ctx, nil, user.ID, actionPasswordResetCompleted, "user", user.ID, nil); err != nil {
		return fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	return nil
}

// sendPasswordResetEmail best-effort emails the reset link when a mailer is
// configured. Delivery failures are logged, never fatal: the request flow has
// already succeeded and must not start leaking which accounts exist via
// error responses.
func (s *Service) sendPasswordResetEmail(ctx context.Context, email, token string) {
	if s.mailer == nil || s.passwordResetBaseURL == "" {
		return
	}

	link := s.passwordResetBaseURL + "?token=" + url.QueryEscape(token)
	subject := "Reset your LaunchPad password"
	body := fmt.Sprintf(
		`<p>We received a request to reset your LaunchPad password.</p>`+
			`<p><a href="%s">Reset your password</a> — this link expires in 1 hour and can be used once.</p>`+
			`<p>If you did not request a reset, you can ignore this email.</p>`,
		link,
	)

	if err := s.mailer.Send(ctx, email, subject, body); err != nil {
		slog.ErrorContext(ctx, "send password reset email", "error", err)
	}
}
