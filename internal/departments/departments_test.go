package departments_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/departments"
)

func TestCreateDepartmentRejectsEmptyName(t *testing.T) {
	t.Parallel()

	svc := departments.NewService(nil)

	_, err := svc.CreateDepartment(context.Background(), "org-1", departments.CreateDepartmentInput{
		Name: "  ",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !errors.Is(err, departments.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, departments.ErrInvalidInput)
	}
}

func TestCreateJobRoleRejectsEmptyOrganization(t *testing.T) {
	t.Parallel()

	svc := departments.NewService(nil)

	_, err := svc.CreateJobRole(context.Background(), "", departments.CreateJobRoleInput{
		Name: "Engineer",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !errors.Is(err, departments.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, departments.ErrInvalidInput)
	}
}
