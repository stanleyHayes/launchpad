package employees_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"launchpad/internal/employees"
)

type noopReferences struct{}

func (noopReferences) EnsureDepartmentExists(context.Context, string, string) error {
	return nil
}

func (noopReferences) EnsureJobRoleExists(context.Context, string, string) error {
	return nil
}

func TestCreateRejectsInvalidEmail(t *testing.T) {
	t.Parallel()

	svc := employees.NewService(nil, noopReferences{})

	_, err := svc.Create(context.Background(), "org-1", employees.CreateInput{
		FirstName: "Ada",
		LastName:  "Lovelace",
		WorkEmail: "not-an-email",
		StartDate: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !errors.Is(err, employees.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, employees.ErrInvalidInput)
	}
}

func TestCreateRejectsZeroStartDate(t *testing.T) {
	t.Parallel()

	svc := employees.NewService(nil, noopReferences{})

	_, err := svc.Create(context.Background(), "org-1", employees.CreateInput{
		FirstName: "Ada",
		LastName:  "Lovelace",
		WorkEmail: "ada@example.com",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !errors.Is(err, employees.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, employees.ErrInvalidInput)
	}
}
