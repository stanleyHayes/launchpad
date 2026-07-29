package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"launchpad/internal/audit"
	"launchpad/internal/auth"
	"launchpad/internal/organizations"
	"launchpad/pkg/security"
)

// --- fakes -----------------------------------------------------------------

type fakeResets struct {
	byHash map[string]auth.PasswordReset
}

func newFakeResets() *fakeResets {
	return &fakeResets{byHash: map[string]auth.PasswordReset{}}
}

func (f *fakeResets) EnsureIndexes(context.Context) error { return nil }

func (f *fakeResets) Save(_ context.Context, reset auth.PasswordReset) error {
	f.byHash[reset.TokenHash] = reset

	return nil
}

func (f *fakeResets) Consume(_ context.Context, tokenHash string) (auth.PasswordReset, error) {
	reset, ok := f.byHash[tokenHash]
	if !ok || !reset.ExpiresAt.After(time.Now().UTC()) {
		return auth.PasswordReset{}, auth.ErrPasswordResetInvalid
	}

	delete(f.byHash, tokenHash)

	return reset, nil
}

type sentMail struct {
	to      string
	subject string
	html    string
}

type fakeMailer struct {
	sent []sentMail
}

func (f *fakeMailer) Send(_ context.Context, to, subject, html string) error {
	f.sent = append(f.sent, sentMail{to: to, subject: subject, html: html})

	return nil
}

type resetFixture struct {
	svc      *auth.Service
	users    *fakeUsers
	resets   *fakeResets
	sessions *fakeSessions
	mailer   *fakeMailer
}

func newResetFixture(t *testing.T) resetFixture {
	t.Helper()

	users := newFakeUsers()
	resets := newFakeResets()
	sessions := newFakeSessions()
	mailer := &fakeMailer{}
	orgs := newFakeOrgs()
	orgs.orgs[testOrg] = organizations.Organization{ID: testOrg, Name: "Acme", Slug: "acme", Status: "active"}

	cfg := auth.Config{
		JWTSecret: "test-secret-value", AccessTTL: time.Minute, RefreshTTL: time.Hour,
		InviteTTL: time.Hour, PasswordMinLen: 10,
	}
	svc := auth.NewService(users, orgs, audit.NewService(noopAuditRepo{}), sessions, newFakeInvites(), cfg, nil)
	svc = svc.
		WithPasswordResets(resets).
		WithMailer(mailer, "https://admin.example/accept-invitation", "https://admin.example/reset-password")

	return resetFixture{svc: svc, users: users, resets: resets, sessions: sessions, mailer: mailer}
}

// addMember seeds an active user account and returns its ID.
func addMember(fx resetFixture, email string) string {
	userID := "user-" + email
	fx.users.byID[userID] = auth.User{ID: userID, Email: email, Status: "active"}
	fx.users.byEmail[email] = fx.users.byID[userID]

	return userID
}

// requestReset issues a reset and returns the raw token the mailer delivered.
func requestReset(t *testing.T, fx resetFixture, email string) string {
	t.Helper()

	if err := fx.svc.RequestPasswordReset(context.Background(), email); err != nil {
		t.Fatalf("request reset: %v", err)
	}

	if len(fx.mailer.sent) == 0 {
		t.Fatal("reset email not sent")
	}

	mail := fx.mailer.sent[len(fx.mailer.sent)-1]

	const marker = "?token="

	idx := strings.Index(mail.html, marker)
	if idx < 0 {
		t.Fatalf("reset email has no token link: %q", mail.html)
	}

	token := mail.html[idx+len(marker):]
	if end := strings.IndexAny(token, `"'<`); end >= 0 {
		token = token[:end]
	}

	return token
}

// --- tests -----------------------------------------------------------------

func TestRequestPasswordResetUnknownEmailIsIndistinguishable(t *testing.T) {
	t.Parallel()

	fx := newResetFixture(t)

	// Unknown email: no error (the handler answers 202 either way), no token
	// stored, no email sent — nothing an enumeration oracle could use.
	if err := fx.svc.RequestPasswordReset(context.Background(), "ghost@acme.test"); err != nil {
		t.Fatalf("unknown email got %v, want nil", err)
	}

	if len(fx.resets.byHash) != 0 || len(fx.mailer.sent) != 0 {
		t.Fatalf("unknown email stored %d resets and sent %d mails, want 0/0",
			len(fx.resets.byHash), len(fx.mailer.sent))
	}

	if err := fx.svc.RequestPasswordReset(context.Background(), "not-an-email"); !errors.Is(err, auth.ErrInvalidInput) {
		t.Fatalf("malformed email got %v, want ErrInvalidInput", err)
	}
}

func TestRequestPasswordResetStoresHashedTokenAndEmailsLink(t *testing.T) {
	t.Parallel()

	fx := newResetFixture(t)
	userID := addMember(fx, "member@acme.test")

	token := requestReset(t, fx, "Member@Acme.Test")

	if !strings.HasPrefix(token, "reset_") {
		t.Fatalf("token missing prefix: %q", token)
	}

	reset, ok := fx.resets.byHash[security.HashToken(token)]
	if !ok || reset.UserID != userID || reset.Email != "member@acme.test" {
		t.Fatalf("reset not stored correctly: %+v", reset)
	}

	if ttl := time.Until(reset.ExpiresAt); ttl <= 0 || ttl > time.Hour {
		t.Fatalf("reset TTL out of bounds: %v", ttl)
	}

	mail := fx.mailer.sent[0]
	if mail.to != "member@acme.test" || !strings.Contains(mail.html, "https://admin.example/reset-password?token=") {
		t.Fatalf("reset email malformed: %+v", mail)
	}
}

func TestResetPasswordSetsPasswordAndIsSingleUse(t *testing.T) {
	t.Parallel()

	fx := newResetFixture(t)
	ctx := context.Background()

	oldHash, err := security.HashPassword("old-password-1")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	userID := addMember(fx, "member@acme.test")

	user := fx.users.byID[userID]
	user.PasswordHash = oldHash
	fx.users.byID[userID] = user
	fx.users.byEmail["member@acme.test"] = user

	token := requestReset(t, fx, "member@acme.test")

	if err := fx.svc.ResetPassword(ctx, token, goodPass); err != nil {
		t.Fatalf("reset: %v", err)
	}

	updated := fx.users.byID[userID]
	if !security.CheckPassword(updated.PasswordHash, goodPass) {
		t.Fatal("password not updated")
	}

	// The token is strictly single-use.
	if err := fx.svc.ResetPassword(ctx, token, goodPass); !errors.Is(err, auth.ErrPasswordResetInvalid) {
		t.Fatalf("second reset got %v, want ErrPasswordResetInvalid", err)
	}
}

func TestResetPasswordRevokesAllUserSessions(t *testing.T) {
	t.Parallel()

	fx := newResetFixture(t)
	ctx := context.Background()

	userID := addMember(fx, "member@acme.test")

	// Two sessions for the resetting user and one for someone else.
	_ = fx.sessions.Save(ctx, "s-1", userID, testOrg, "hash-1")
	_ = fx.sessions.Save(ctx, "s-2", userID, testOrg, "hash-2")
	_ = fx.sessions.Save(ctx, "s-3", "user-other", testOrg, "hash-3")

	token := requestReset(t, fx, "member@acme.test")

	if err := fx.svc.ResetPassword(ctx, token, goodPass); err != nil {
		t.Fatalf("reset: %v", err)
	}

	if _, _, _, err := fx.sessions.Get(ctx, "s-1"); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("session s-1 survived reset: %v", err)
	}

	if _, _, _, err := fx.sessions.Get(ctx, "s-2"); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("session s-2 survived reset: %v", err)
	}

	if _, _, _, err := fx.sessions.Get(ctx, "s-3"); err != nil {
		t.Fatalf("other user's session s-3 was revoked: %v", err)
	}
}

func TestResetPasswordRejectsExpired(t *testing.T) {
	t.Parallel()

	fx := newResetFixture(t)
	addMember(fx, "stale@acme.test")

	token := requestReset(t, fx, "stale@acme.test")

	// Force the stored reset to be expired.
	hash := security.HashToken(token)
	reset := fx.resets.byHash[hash]
	reset.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	fx.resets.byHash[hash] = reset

	if err := fx.svc.ResetPassword(context.Background(), token, goodPass); !errors.Is(err, auth.ErrPasswordResetInvalid) {
		t.Fatalf("expired reset got %v, want ErrPasswordResetInvalid", err)
	}
}

func TestResetPasswordRejectsWeakPasswordWithoutConsuming(t *testing.T) {
	t.Parallel()

	fx := newResetFixture(t)
	ctx := context.Background()

	addMember(fx, "weakpass@acme.test")

	token := requestReset(t, fx, "weakpass@acme.test")

	if err := fx.svc.ResetPassword(ctx, token, "short"); !errors.Is(err, auth.ErrWeakPassword) {
		t.Fatalf("weak password got %v, want ErrWeakPassword", err)
	}

	// A weak-password attempt must not have consumed the token.
	if err := fx.svc.ResetPassword(ctx, token, goodPass); err != nil {
		t.Fatalf("token should still be valid after a weak-password attempt, got %v", err)
	}
}

func TestIssueInvitationEmailsAcceptLinkWhenMailerConfigured(t *testing.T) {
	t.Parallel()

	fx := newResetFixture(t)

	token, err := fx.svc.IssueInvitation(context.Background(), testOrg, "hire@acme.test", "Hire", "employee", testActor)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if len(fx.mailer.sent) != 1 {
		t.Fatalf("expected 1 invitation email, got %d", len(fx.mailer.sent))
	}

	mail := fx.mailer.sent[0]
	if mail.to != "hire@acme.test" {
		t.Fatalf("invitation email went to %q", mail.to)
	}

	if !strings.Contains(mail.html, "https://admin.example/accept-invitation?token="+token) {
		t.Fatalf("invitation email missing accept link: %q", mail.html)
	}
}

func TestIssueInvitationWithoutMailerKeepsTokenInResponse(t *testing.T) {
	t.Parallel()

	// newInviteService wires no mailer: behavior stays as before email existed.
	svc, _, _ := newInviteService(t)

	token, err := svc.IssueInvitation(context.Background(), testOrg, "hire@acme.test", "Hire", "employee", testActor)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if !strings.HasPrefix(token, "invite_") {
		t.Fatalf("token missing prefix: %q", token)
	}
}

func (f *fakeResets) DeleteForUsers(context.Context, []string) (int64, error) {
	return 0, nil
}
