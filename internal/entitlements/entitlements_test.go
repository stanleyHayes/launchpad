package entitlements_test

import (
	"errors"
	"testing"

	"launchpad/internal/entitlements"
)

func TestPlanLimitsAndGuards(t *testing.T) {
	t.Parallel()

	if got := entitlements.ForPlan("starter").Employees; got != 25 {
		t.Fatalf("starter employees = %d, want 25", got)
	}
	if err := entitlements.Check("growth", entitlements.ResourceJourneyTemplates, 49); err != nil {
		t.Fatalf("growth below limit: %v", err)
	}
	if err := entitlements.Check("growth", entitlements.ResourceJourneyTemplates, 50); !errors.Is(err, entitlements.ErrLimitExceeded) {
		t.Fatalf("growth at limit = %v", err)
	}
	if err := entitlements.Check("enterprise", entitlements.ResourceEmployees, 100000); err != nil {
		t.Fatalf("enterprise should be unlimited: %v", err)
	}
}
