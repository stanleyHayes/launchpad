// Package organizations implements organization use cases and HTTP handlers.
package organizations

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// jsonRoleCode is the role-code field name shared by the member payloads.
const jsonRoleCode = "roleCode"

// Handler exposes organization HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs an organization Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandleGetCurrent returns the caller's organization.
func (h *Handler) HandleGetCurrent(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeOrgError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	org, err := h.svc.Get(r.Context(), principal.OrganizationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeOrgError(w, r, http.StatusNotFound, "NOT_FOUND", "Organization not found")

			return
		}

		slog.ErrorContext(r.Context(), "load organization failed", "error", err)
		writeOrgError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load organization")

		return
	}

	writeOrgJSON(w, r, http.StatusOK, org.ToResponse())
}

// HandleUpdateCurrent updates the caller's organization. The route declares
// no permission, so the handler keeps its own stricter gate: org settings are
// owner/hr_admin only.
func (h *Handler) HandleUpdateCurrent(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeOrgError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	if !CanManageOrganization(principal.RoleCode) {
		writeOrgError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")

		return
	}

	input, err := decodeUpdateInput(r)
	if err != nil {
		writeOrgError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	org, err := h.svc.Update(r.Context(), principal.OrganizationID, input)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			writeOrgError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())

			return
		}

		slog.ErrorContext(r.Context(), "update organization failed", "error", err)
		writeOrgError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update organization")

		return
	}

	orgID := org.ID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		"organization.updated",
		"organization",
		org.ID,
		nil,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit organization update failed", "error", err)
		writeOrgError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeOrgJSON(w, r, http.StatusOK, org.ToResponse())
}

// HandleUpdateSetupProgress persists the guided setup wizard checkpoint.
func (h *Handler) HandleUpdateSetupProgress(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeOrgError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	if !CanManageOrganization(principal.RoleCode) {
		writeOrgError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}
	var body struct {
		Step      int  `json:"step"`
		Completed bool `json:"completed"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeOrgError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
		return
	}
	org, err := h.svc.UpdateSetupProgress(r.Context(), principal.OrganizationID, SetupProgressInput{
		Step: body.Step, Completed: body.Completed,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			writeOrgError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		slog.ErrorContext(r.Context(), "update setup progress failed", "error", err)
		writeOrgError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update setup progress")
		return
	}
	orgID := org.ID
	if err := h.audit.Record(r.Context(), &orgID, principal.UserID, "organization.setup_progressed", "organization", org.ID, map[string]any{
		"step": body.Step, "completed": body.Completed,
	}); err != nil {
		slog.ErrorContext(r.Context(), "audit setup progress failed", "error", err)
		writeOrgError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")
		return
	}
	writeOrgJSON(w, r, http.StatusOK, org.ToResponse())
}

func decodeUpdateInput(r *http.Request) (UpdateInput, error) {
	var body struct {
		Name         *string `json:"name"`
		Timezone     *string `json:"timezone"`
		CustomDomain *string `json:"customDomain"`
		Branding     *struct {
			PrimaryColor      string `json:"primaryColor"`
			PrimaryHoverColor string `json:"primaryHoverColor"`
			AccentColor       string `json:"accentColor"`
			LogoURL           string `json:"logoUrl"`
		} `json:"branding"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		return UpdateInput{}, fmt.Errorf("decode organization update: %w", err)
	}

	var branding *Branding
	if body.Branding != nil {
		branding = &Branding{
			PrimaryColor:      body.Branding.PrimaryColor,
			PrimaryHoverColor: body.Branding.PrimaryHoverColor,
			AccentColor:       body.Branding.AccentColor,
			LogoURL:           body.Branding.LogoURL,
		}
	}

	return UpdateInput{
		Name: body.Name, Timezone: body.Timezone, Branding: branding, CustomDomain: body.CustomDomain,
	}, nil
}

// HandleInviteMember invites an HR admin to the current organization. The
// route gates members.invite; the handler intentionally stays stricter
// (owner/hr_admin only) because the invite flow provisions an hr_admin
// account directly, bypassing the invitation lifecycle.
func (h *Handler) HandleInviteMember(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeOrgError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	if !CanManageOrganization(principal.RoleCode) {
		writeOrgError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")

		return
	}

	var body struct {
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
		RoleCode    string `json:"roleCode"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeOrgError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	membership, err := h.svc.InviteMember(r.Context(), principal.OrganizationID, InviteMemberInput{
		Email:       body.Email,
		DisplayName: body.DisplayName,
		Password:    body.Password,
		RoleCode:    body.RoleCode,
	})
	if err != nil {
		writeInviteError(w, r, err)

		return
	}

	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		"membership.invited",
		"membership",
		membership.UserID,
		map[string]any{jsonRoleCode: membership.RoleCode, "email": body.Email},
	); err != nil {
		slog.ErrorContext(r.Context(), "audit member invite failed", "error", err)
		writeOrgError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeOrgJSON(w, r, http.StatusCreated, map[string]any{
		"userId":         membership.UserID,
		"organizationId": principal.OrganizationID,
		jsonRoleCode:     membership.RoleCode,
	})
}

func writeInviteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInviteInvalidInput):
		writeOrgError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrInviteWeakPassword):
		writeOrgError(w, r, http.StatusBadRequest, "WEAK_PASSWORD", err.Error())
	case errors.Is(err, ErrInviteEmailTaken):
		writeOrgError(w, r, http.StatusConflict, "EMAIL_TAKEN", err.Error())
	default:
		slog.ErrorContext(r.Context(), "invite member failed", "error", err)
		writeOrgError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to invite member")
	}
}

// HandleListMembers lists the current organization's members with their
// account display info. Authorization is enforced by the route-level
// RequirePermission (members.read).
func (h *Handler) HandleListMembers(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeOrgError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	members, err := h.svc.ListMembers(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list members failed", "error", err)
		writeOrgError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list members")

		return
	}

	writeOrgJSON(w, r, http.StatusOK, ToMemberResponses(members))
}

// HandleUpdateMemberRole changes a member's role. Callers cannot change
// their own role, and the change may not demote the last organization_owner.
// Authorization is enforced by the route-level RequirePermission
// (members.update).
func (h *Handler) HandleUpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeOrgError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	targetUserID := chi.URLParam(r, "userID")

	var body struct {
		RoleCode string `json:"roleCode"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeOrgError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	membership, err := h.svc.ChangeMemberRole(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		targetUserID,
		body.RoleCode,
	)
	if err != nil {
		writeRoleChangeError(w, r, err)

		return
	}

	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		"membership.role_changed",
		"membership",
		membership.ID,
		map[string]any{"targetUserId": targetUserID, jsonRoleCode: membership.RoleCode},
	); err != nil {
		slog.ErrorContext(r.Context(), "audit member role change failed", "error", err)
		writeOrgError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeOrgJSON(w, r, http.StatusOK, map[string]any{
		"userId":         membership.UserID,
		"organizationId": principal.OrganizationID,
		jsonRoleCode:     membership.RoleCode,
	})
}

func writeRoleChangeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrUnknownRole), errors.Is(err, ErrCannotChangeOwnRole):
		writeOrgError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeOrgError(w, r, http.StatusNotFound, "NOT_FOUND", "Membership not found")
	case errors.Is(err, ErrLastOwner):
		writeOrgError(w, r, http.StatusConflict, "LAST_OWNER", err.Error())
	default:
		slog.ErrorContext(r.Context(), "change member role failed", "error", err)
		writeOrgError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to change member role")
	}
}

func writeOrgJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	if err := httpx.WriteJSON(w, status, data); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

func writeOrgError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write error response", "error", err)
	}
}
