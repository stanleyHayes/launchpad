package audit_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"launchpad/internal/audit"
)

func TestEventResponseMatchesDocumentJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	orgID := "org-1"

	event := audit.Event{
		ID:             "evt-1",
		OrganizationID: &orgID,
		ActorUserID:    "user-1",
		Action:         "assignment.created",
		ResourceType:   "journey_assignment",
		ResourceID:     "asg-1",
		IP:             "203.0.113.9",
		UserAgent:      "Mozilla/5.0",
		RequestID:      "req-1",
		Result:         audit.ResultSuccess,
		Metadata:       map[string]any{"employeeId": "emp-1"},
		CreatedAt:      now,
	}

	if got, want := marshalToMap(t, event.ToResponse()), marshalToMap(t, event); !reflect.DeepEqual(got, want) {
		t.Errorf("response JSON = %v, want %v", got, want)
	}

	empty := audit.Event{}
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
