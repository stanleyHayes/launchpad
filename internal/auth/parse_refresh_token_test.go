package auth_test

import (
	"testing"

	"launchpad/internal/auth"
)

func TestParseRefreshToken(t *testing.T) {
	t.Parallel()

	t.Run("valid token", func(t *testing.T) {
		t.Parallel()

		refresh, sessionID, err := auth.ParseRefreshToken("refreshvalue.session-id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if refresh != "refreshvalue" {
			t.Fatalf("refresh=%q", refresh)
		}

		if sessionID != "session-id" {
			t.Fatalf("sessionID=%q", sessionID)
		}
	})

	t.Run("invalid formats", func(t *testing.T) {
		t.Parallel()

		cases := []string{"", "onlyone", ".session", "refresh.", "a.b.c"}
		for _, combined := range cases {
			_, _, err := auth.ParseRefreshToken(combined)
			if err == nil {
				t.Fatalf("expected error for %q", combined)
			}
		}
	})
}
