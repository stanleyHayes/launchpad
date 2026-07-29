package billing_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"launchpad/internal/billing"
)

func TestPlanResponseMatchesDocumentJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	plan := billing.Plan{
		Code:              "growth",
		Name:              "Growth",
		Description:       "For growing teams",
		PriceMonthlyCents: 9900,
		Currency:          "USD",
		Features:          []string{"core_onboarding", "analytics"},
		Active:            true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if got, want := marshalToMap(t, plan.ToResponse()), marshalToMap(t, plan); !reflect.DeepEqual(got, want) {
		t.Errorf("response JSON = %v, want %v", got, want)
	}

	empty := billing.Plan{}
	if got, want := marshalToMap(t, empty.ToResponse()), marshalToMap(t, empty); !reflect.DeepEqual(got, want) {
		t.Errorf("empty response JSON = %v, want %v", got, want)
	}
}

func TestSubscriptionResponseMatchesDocumentJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	subscription := billing.Subscription{
		ID:               "sub-1",
		OrganizationID:   "org-1",
		PlanCode:         "growth",
		Status:           "active",
		CurrentPeriodEnd: &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	got := marshalToMap(t, subscription.ToResponse())
	want := marshalToMap(t, subscription)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("response JSON = %v, want %v", got, want)
	}

	empty := billing.Subscription{}

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
