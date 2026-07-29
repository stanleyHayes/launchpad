package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"launchpad/pkg/security"
)

func (s *Service) UpdateProfile(
	ctx context.Context,
	organizationID, userID, displayName string,
) (UserPublic, error) {
	displayName = strings.TrimSpace(displayName)
	if len(displayName) < 2 || len(displayName) > 100 {
		return UserPublic{}, ErrInvalidInput
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return UserPublic{}, fmt.Errorf("load profile: %w", err)
	}
	user.DisplayName = displayName
	user.UpdatedAt = time.Now().UTC()
	if err := s.users.Update(ctx, user); err != nil {
		return UserPublic{}, fmt.Errorf("update profile: %w", err)
	}
	if err := s.audit.Record(
		ctx, auditOrgPtr(organizationID), userID, "auth.profile_updated", "user", userID, nil,
	); err != nil {
		return UserPublic{}, fmt.Errorf("%w: %v", ErrAuditFailed, err)
	}
	return toPublic(user), nil
}

func (s *Service) ChangePassword(
	ctx context.Context,
	organizationID, userID, currentPassword, newPassword string,
) error {
	if len(newPassword) < s.cfg.PasswordMinLen {
		return ErrWeakPassword
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("load user: %w", err)
	}
	if !security.CheckPassword(user.PasswordHash, currentPassword) {
		return ErrInvalidCredentials
	}
	if security.CheckPassword(user.PasswordHash, newPassword) {
		return ErrInvalidInput
	}
	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	user.PasswordHash = hash
	user.UpdatedAt = time.Now().UTC()
	if err := s.users.Update(ctx, user); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if err := s.sessions.DeleteForUser(ctx, userID); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	if err := s.audit.Record(
		ctx, auditOrgPtr(organizationID), userID, "auth.password_changed", "user", userID, nil,
	); err != nil {
		return fmt.Errorf("%w: %v", ErrAuditFailed, err)
	}
	return nil
}

func (s *Service) UpdatePreferences(
	ctx context.Context,
	organizationID, userID string,
	preferences UserPreferences,
) (UserPreferences, error) {
	preferences.DigestFrequency = strings.ToLower(strings.TrimSpace(preferences.DigestFrequency))
	preferences.Locale = strings.ToLower(strings.TrimSpace(preferences.Locale))
	preferences.Timezone = strings.TrimSpace(preferences.Timezone)
	if !oneOf(preferences.DigestFrequency, "instant", "daily", "weekly", "off") ||
		!oneOf(preferences.Locale, "en", "fr") ||
		preferences.Timezone == "" {
		return UserPreferences{}, ErrInvalidInput
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return UserPreferences{}, fmt.Errorf("load preferences: %w", err)
	}
	user.Preferences = preferences
	user.UpdatedAt = time.Now().UTC()
	if err := s.users.Update(ctx, user); err != nil {
		return UserPreferences{}, fmt.Errorf("update preferences: %w", err)
	}
	if err := s.audit.Record(
		ctx, auditOrgPtr(organizationID), userID, "auth.preferences_updated", "user", userID, nil,
	); err != nil {
		return UserPreferences{}, fmt.Errorf("%w: %v", ErrAuditFailed, err)
	}
	return preferences, nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
