package organizations_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"launchpad/internal/organizations"
)

func TestOrganizationResponseMatchesDocumentJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	org := organizations.Organization{
		ID:       "org-1",
		Name:     "Acme",
		Slug:     "acme",
		Status:   "active",
		PlanCode: "growth",
		Timezone: "UTC",
		Branding: organizations.Branding{
			PrimaryColor:      "#112233",
			PrimaryHoverColor: "#223344",
			AccentColor:       "#334455",
			LogoURL:           "https://example.com/logo.png",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if got, want := marshalToMap(t, org.ToResponse()), marshalToMap(t, org); !reflect.DeepEqual(got, want) {
		t.Errorf("response JSON = %v, want %v", got, want)
	}

	empty := organizations.Organization{}
	if got, want := marshalToMap(t, empty.ToResponse()), marshalToMap(t, empty); !reflect.DeepEqual(got, want) {
		t.Errorf("empty response JSON = %v, want %v", got, want)
	}
}

func marshalToMap(t *testing.T, value any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	return out
}
