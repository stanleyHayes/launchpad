package auth_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/auth"
	"launchpad/pkg/security"
)

func TestAccountProfilePreferencesAndPassword(t *testing.T) {
	t.Parallel()
	svc, users, _ := newAuthService(t)
	registered := registerOwner(t, svc, "employee-settings@example.com")
	ctx := context.Background()
	orgID := registered.Organization.ID
	userID := registered.User.ID

	profile, err := svc.UpdateProfile(ctx, orgID, userID, "Ama Owusu")
	if err != nil || profile.DisplayName != "Ama Owusu" {
		t.Fatalf("update profile: %v (%+v)", err, profile)
	}

	preferences := auth.UserPreferences{
		EmailNotifications: false,
		InAppNotifications: true,
		DigestFrequency:    "weekly",
		Locale:             "fr",
		Timezone:           "Africa/Accra",
	}
	saved, err := svc.UpdatePreferences(ctx, orgID, userID, preferences)
	if err != nil || saved != preferences {
		t.Fatalf("update preferences: %v (%+v)", err, saved)
	}

	if err := svc.ChangePassword(ctx, orgID, userID, "wrong-password", "new-password-123"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("wrong current password = %v, want invalid credentials", err)
	}
	if err := svc.ChangePassword(ctx, orgID, userID, goodPass, "new-password-123"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if !security.CheckPassword(users.byID[userID].PasswordHash, "new-password-123") {
		t.Fatal("new password was not persisted")
	}
}

func TestAccountSettingsValidateInput(t *testing.T) {
	t.Parallel()
	svc, _, _ := newAuthService(t)
	registered := registerOwner(t, svc, "employee-validation@example.com")

	if _, err := svc.UpdateProfile(context.Background(), registered.Organization.ID, registered.User.ID, " "); !errors.Is(err, auth.ErrInvalidInput) {
		t.Fatalf("blank display name = %v, want invalid input", err)
	}
	if _, err := svc.UpdatePreferences(context.Background(), registered.Organization.ID, registered.User.ID, auth.UserPreferences{
		DigestFrequency: "sometimes", Locale: "en", Timezone: "UTC",
	}); !errors.Is(err, auth.ErrInvalidInput) {
		t.Fatalf("invalid digest = %v, want invalid input", err)
	}
}
