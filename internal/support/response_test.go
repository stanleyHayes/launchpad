package support_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"launchpad/internal/support"
)

func TestTicketResponseMatchesDocumentJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	ticket := support.Ticket{
		ID:              "tkt-1",
		OrganizationID:  "org-1",
		CreatedByUserID: "user-1",
		Subject:         "Cannot log in",
		Body:            "Login page returns an error.",
		Priority:        "high",
		Status:          "open",
		AssigneeUserID:  "staff-1",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if got, want := marshalToMap(t, ticket.ToResponse()), marshalToMap(t, ticket); !reflect.DeepEqual(got, want) {
		t.Errorf("response JSON = %v, want %v", got, want)
	}

	empty := support.Ticket{}
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
