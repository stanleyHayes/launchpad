package organizations_test

import (
	"testing"

	"launchpad/internal/organizations"
)

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"Acme Corp":     "acme-corp",
		"  Hello!!  ":   "hello",
		"LaunchPad Inc": "launchpad-inc",
	}
	for in, want := range tests {
		if got := organizations.Slugify(in); got != want {
			t.Fatalf("Slugify(%q)=%q want %q", in, got, want)
		}
	}
}
