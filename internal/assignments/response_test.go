package assignments_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"launchpad/internal/assignments"
)

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

func TestJourneyAssignmentResponseMatchesDocumentJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	due := now.Add(72 * time.Hour)

	assignment := assignments.JourneyAssignment{
		ID:                "asg-1",
		OrganizationID:    "org-1",
		EmployeeID:        "emp-1",
		JourneyTemplateID: "tpl-1",
		TemplateVersion:   3,
		Status:            "in_progress",
		StartsAt:          now,
		DueAt:             &due,
		ProgressPercent:   42.5,
		CompletedAt:       &due,
		CreatedAt:         now,
	}

	if got, want := marshalToMap(t, assignment.ToResponse()), marshalToMap(t, assignment); !reflect.DeepEqual(got, want) {
		t.Errorf("response JSON = %v, want %v", got, want)
	}

	empty := assignments.JourneyAssignment{}
	if got, want := marshalToMap(t, empty.ToResponse()), marshalToMap(t, empty); !reflect.DeepEqual(got, want) {
		t.Errorf("empty response JSON = %v, want %v", got, want)
	}
}

func TestStepAssignmentResponseMatchesDocumentJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	score := 88.5

	step := assignments.StepAssignment{
		ID:                  "step-1",
		OrganizationID:      "org-1",
		JourneyAssignmentID: "asg-1",
		JourneyStepID:       "tpl-step-1",
		EmployeeID:          "emp-1",
		StepType:            "quiz",
		Title:               "Security quiz",
		Instructions:        "Answer all questions",
		Position:            2,
		Status:              "completed",
		DueAt:               &now,
		Submission:          map[string]any{"q1": "a"},
		Score:               &score,
		CompletedAt:         &now,
		CreatedAt:           now,
	}

	if got, want := marshalToMap(t, step.ToResponse()), marshalToMap(t, step); !reflect.DeepEqual(got, want) {
		t.Errorf("response JSON = %v, want %v", got, want)
	}

	empty := assignments.StepAssignment{}
	if got, want := marshalToMap(t, empty.ToResponse()), marshalToMap(t, empty); !reflect.DeepEqual(got, want) {
		t.Errorf("empty response JSON = %v, want %v", got, want)
	}
}

func TestApprovalResponseMatchesDocumentJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	approval := assignments.Approval{
		ID:               "apr-1",
		OrganizationID:   "org-1",
		StepAssignmentID: "step-1",
		ApproverUserID:   "user-1",
		Status:           "approved",
		Note:             "Looks good",
		DecidedAt:        &now,
		CreatedAt:        now,
	}

	if got, want := marshalToMap(t, approval.ToResponse()), marshalToMap(t, approval); !reflect.DeepEqual(got, want) {
		t.Errorf("response JSON = %v, want %v", got, want)
	}

	empty := assignments.Approval{}
	if got, want := marshalToMap(t, empty.ToResponse()), marshalToMap(t, empty); !reflect.DeepEqual(got, want) {
		t.Errorf("empty response JSON = %v, want %v", got, want)
	}
}

func TestAssignResultResponseMatchesDocumentJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	result := assignments.AssignResult{
		Assignment: assignments.JourneyAssignment{ID: "asg-1", CreatedAt: now},
		Steps: []assignments.StepAssignment{
			{ID: "step-1", CreatedAt: now},
			{ID: "step-2", CreatedAt: now},
		},
	}

	if got, want := marshalToMap(t, result.ToResponse()), marshalToMap(t, result); !reflect.DeepEqual(got, want) {
		t.Errorf("response JSON = %v, want %v", got, want)
	}
}
