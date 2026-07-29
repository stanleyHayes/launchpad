package platform

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/google/uuid"
)

// tempPasswordBytes sizes the generated temporary staff password.
const tempPasswordBytes = 16
const maxBreakGlassDuration = time.Hour

// CreateStaff provisions a new platform staff account: a user account with a
// generated temporary password plus an active staff record. When a mail
// sender is wired the credentials are emailed (Invited); otherwise the
// temporary password is returned so the operator can show it once.
func (s *Service) CreateStaff(ctx context.Context, in CreateStaffInput) (CreateStaffResult, error) {
	email, displayName, err := validateStaffInput(in)
	if err != nil {
		return CreateStaffResult{}, err
	}

	if s.accounts == nil {
		return CreateStaffResult{}, ErrProvisioningUnavailable
	}

	tempPassword, err := newTempPassword()
	if err != nil {
		return CreateStaffResult{}, err
	}

	userID, err := s.accounts.CreateAccount(ctx, email, displayName, tempPassword)
	if err != nil {
		return CreateStaffResult{}, fmt.Errorf("create staff user account: %w", err)
	}

	staff := Staff{
		ID:          uuid.NewString(),
		UserID:      userID,
		Email:       email,
		DisplayName: displayName,
		RoleCode:    in.RoleCode,
		Status:      staffStatusActive,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, staff); err != nil {
		return CreateStaffResult{}, fmt.Errorf("create platform staff: %w", err)
	}

	result := CreateStaffResult{Staff: staff, TempPassword: tempPassword, Invited: false}
	if s.mailer == nil {
		return result, nil
	}

	if err := s.sendStaffInvite(ctx, staff, tempPassword); err != nil {
		return CreateStaffResult{}, err
	}

	result.Invited = true
	result.TempPassword = ""

	return result, nil
}

// ListStaff returns all platform staff records, including deactivated ones.
func (s *Service) ListStaff(ctx context.Context) ([]Staff, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list platform staff: %w", err)
	}

	return items, nil
}

// GrantBreakGlass elevates an active staff account for at most one hour.
func (s *Service) GrantBreakGlass(
	ctx context.Context, approverUserID, staffID, reason string, duration time.Duration,
) (Staff, error) {
	reason = strings.TrimSpace(reason)
	if approverUserID == "" || reason == "" || duration <= 0 || duration > maxBreakGlassDuration {
		return Staff{}, ErrInvalidInput
	}
	staff, err := s.repo.GetByID(ctx, staffID)
	if err != nil {
		return Staff{}, fmt.Errorf("get break-glass staff: %w", err)
	}
	if staff.Status != staffStatusActive || staff.UserID == approverUserID {
		return Staff{}, ErrInvalidInput
	}
	now := time.Now().UTC()
	staff.BreakGlass = &BreakGlassGrant{
		RoleCode: rolePlatformOwner, Reason: reason, ApprovedBy: approverUserID,
		GrantedAt: now, ExpiresAt: now.Add(duration),
	}
	if err := s.repo.Update(ctx, staff); err != nil {
		return Staff{}, fmt.Errorf("grant break-glass access: %w", err)
	}
	return staff, nil
}

func (s *Service) RevokeBreakGlass(ctx context.Context, actorUserID, staffID string) (Staff, error) {
	staff, err := s.repo.GetByID(ctx, staffID)
	if err != nil {
		return Staff{}, fmt.Errorf("get break-glass staff: %w", err)
	}
	if staff.BreakGlass == nil || staff.BreakGlass.RevokedAt != nil {
		return Staff{}, ErrInvalidInput
	}
	now := time.Now().UTC()
	staff.BreakGlass.RevokedAt, staff.BreakGlass.RevokedBy = &now, actorUserID
	if err := s.repo.Update(ctx, staff); err != nil {
		return Staff{}, fmt.Errorf("revoke break-glass access: %w", err)
	}
	return staff, nil
}

// AccessReview returns privilege posture and flags accounts not attested in 90 days.
func (s *Service) AccessReview(ctx context.Context) ([]AccessReviewItem, error) {
	items, err := s.ListStaff(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	result := make([]AccessReviewItem, 0, len(items))
	for _, staff := range items {
		effectiveRole := staff.RoleCode
		if grant := staff.BreakGlass; grant != nil && grant.RevokedAt == nil && grant.ExpiresAt.After(now) {
			effectiveRole = grant.RoleCode
		}
		result = append(result, AccessReviewItem{
			Staff:             staff,
			ReviewDue:         staff.AccessReviewedAt == nil || staff.AccessReviewedAt.Before(now.AddDate(0, 0, -90)),
			EffectiveRoleCode: effectiveRole,
		})
	}
	return result, nil
}

func (s *Service) AttestAccess(ctx context.Context, reviewerUserID, staffID string) (Staff, error) {
	if reviewerUserID == "" {
		return Staff{}, ErrInvalidInput
	}
	staff, err := s.repo.GetByID(ctx, staffID)
	if err != nil {
		return Staff{}, fmt.Errorf("get staff for access review: %w", err)
	}
	now := time.Now().UTC()
	staff.AccessReviewedAt, staff.AccessReviewedBy = &now, reviewerUserID
	if err := s.repo.Update(ctx, staff); err != nil {
		return Staff{}, fmt.Errorf("attest staff access: %w", err)
	}
	return staff, nil
}

// UpdateStaffRole changes a staff member's role.
func (s *Service) UpdateStaffRole(ctx context.Context, staffID, roleCode string) (Staff, error) {
	roleCode = strings.TrimSpace(roleCode)
	if !IsValidRole(roleCode) {
		return Staff{}, fmt.Errorf("%w: unknown staff role %q", ErrInvalidInput, roleCode)
	}

	staff, err := s.repo.GetByID(ctx, staffID)
	if err != nil {
		return Staff{}, fmt.Errorf("get platform staff: %w", err)
	}

	staff.RoleCode = roleCode
	if err := s.repo.Update(ctx, staff); err != nil {
		return Staff{}, fmt.Errorf("update platform staff: %w", err)
	}

	return staff, nil
}

// SetStaffStatus activates or deactivates a staff account. Deactivating the
// caller's own account is rejected so an operator cannot lock themselves out.
func (s *Service) SetStaffStatus(ctx context.Context, actorUserID, staffID, status string) (Staff, error) {
	if status != staffStatusActive && status != staffStatusDeactivated {
		return Staff{}, fmt.Errorf("%w: unknown staff status %q", ErrInvalidInput, status)
	}

	staff, err := s.repo.GetByID(ctx, staffID)
	if err != nil {
		return Staff{}, fmt.Errorf("get platform staff: %w", err)
	}

	if status == staffStatusDeactivated && staff.UserID == actorUserID {
		return Staff{}, fmt.Errorf("%w: cannot deactivate your own staff account", ErrInvalidInput)
	}

	staff.Status = status
	if err := s.repo.Update(ctx, staff); err != nil {
		return Staff{}, fmt.Errorf("update platform staff: %w", err)
	}

	return staff, nil
}

// validateStaffInput normalizes and validates staff creation details.
func validateStaffInput(in CreateStaffInput) (string, string, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	displayName := strings.TrimSpace(in.DisplayName)

	if email == "" || displayName == "" || !strings.Contains(email, "@") {
		return "", "", fmt.Errorf("%w: email and display name are required", ErrInvalidInput)
	}

	if !IsValidRole(in.RoleCode) {
		return "", "", fmt.Errorf("%w: unknown staff role %q", ErrInvalidInput, in.RoleCode)
	}

	return email, displayName, nil
}

// newTempPassword generates a cryptographically random temporary password.
func newTempPassword() (string, error) {
	buf := make([]byte, tempPasswordBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate temporary password: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// sendStaffInvite emails the temporary credentials to the new staff member.
// User-supplied values are HTML-escaped before interpolation.
func (s *Service) sendStaffInvite(ctx context.Context, staff Staff, tempPassword string) error {
	subject := "Your LaunchPad platform staff account"
	body := fmt.Sprintf(
		"<p>Hi %s,</p>"+
			"<p>A LaunchPad platform staff account (%s) was created for %s. "+
			"Sign in with this temporary password and change it after your first login:</p>"+
			"<p><strong>%s</strong></p>",
		html.EscapeString(staff.DisplayName),
		html.EscapeString(staff.RoleCode),
		html.EscapeString(staff.Email),
		html.EscapeString(tempPassword),
	)

	if err := s.mailer.Send(ctx, staff.Email, subject, body); err != nil {
		return fmt.Errorf("send staff invite email: %w", err)
	}

	return nil
}
