package security_test

import (
	"testing"
	"time"

	"launchpad/pkg/security"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	t.Parallel()

	hash, err := security.HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if !security.CheckPassword(hash, "correct-horse-battery") {
		t.Fatal("expected password to match")
	}

	if security.CheckPassword(hash, "wrong-password") {
		t.Fatal("expected password mismatch")
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	t.Parallel()

	principal := security.Principal{
		UserID:         "user-1",
		Email:          "owner@example.com",
		OrganizationID: "org-1",
		RoleCode:       "organization_owner",
		SessionID:      "session-1",
	}

	token, err := security.IssueAccessToken("test-secret", time.Minute, principal)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	parsed, err := security.ParseAccessToken("test-secret", token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed != principal {
		t.Fatalf("parsed=%+v want=%+v", parsed, principal)
	}
}
