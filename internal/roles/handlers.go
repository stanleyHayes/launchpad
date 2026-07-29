package roles

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

// Handler exposes role HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs a role Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandleList lists the caller organization's built-in and custom roles.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePermission(w, r, PermissionMembersRead)
	if !ok {
		return
	}

	items, err := h.svc.List(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list roles failed", "error", err)
		writeRoleError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list roles")

		return
	}

	writeRoleJSON(w, r, http.StatusOK, ToResponses(items))
}

// HandleCreate creates a custom role (enterprise plan only).
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePermission(w, r, PermissionMembersUpdate)
	if !ok {
		return
	}

	var body struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeRoleError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	role, err := h.svc.Create(r.Context(), principal.OrganizationID, CreateInput{
		Name:        body.Name,
		Permissions: body.Permissions,
	})
	if err != nil {
		writeRoleServiceError(w, r, err, "create role failed", "Unable to create role")

		return
	}

	if err := h.recordAudit(r, principal, "role.created", role.ID, map[string]any{
		"name":        role.Name,
		"permissions": role.Permissions,
	}); err != nil {
		writeRoleError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeRoleJSON(w, r, http.StatusCreated, role.ToResponse())
}

// HandleUpdate replaces a custom role's permission set.
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePermission(w, r, PermissionMembersUpdate)
	if !ok {
		return
	}

	var body struct {
		Permissions []string `json:"permissions"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeRoleError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	role, err := h.svc.Update(r.Context(), principal.OrganizationID, chi.URLParam(r, "roleID"), UpdateInput{
		Permissions: body.Permissions,
	})
	if err != nil {
		writeRoleServiceError(w, r, err, "update role failed", "Unable to update role")

		return
	}

	if err := h.recordAudit(r, principal, "role.updated", role.ID, map[string]any{
		"name":        role.Name,
		"permissions": role.Permissions,
	}); err != nil {
		writeRoleError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeRoleJSON(w, r, http.StatusOK, role.ToResponse())
}

// HandleDelete removes a custom role.
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePermission(w, r, PermissionMembersUpdate)
	if !ok {
		return
	}

	roleID := chi.URLParam(r, "roleID")

	if err := h.svc.Delete(r.Context(), principal.OrganizationID, roleID); err != nil {
		writeRoleServiceError(w, r, err, "delete role failed", "Unable to delete role")

		return
	}

	if err := h.recordAudit(r, principal, "role.deleted", roleID, nil); err != nil {
		writeRoleError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// requirePermission is the handler-level authorization gate: the caller's
// resolved permission set must contain permission. It resolves through the
// roles service itself, so built-in roles, custom roles, and unknown role
// codes (empty set, denied) all behave exactly as the routing middleware.
func (h *Handler) requirePermission(
	w http.ResponseWriter,
	r *http.Request,
	permission string,
) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeRoleError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	permissions, err := h.svc.ResolvePermissions(r.Context(), principal.OrganizationID, principal.RoleCode)
	if err != nil {
		slog.ErrorContext(r.Context(), "resolve permissions failed", "error", err)
		writeRoleError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to resolve permissions")

		return security.Principal{}, false
	}

	if _, granted := permissions[permission]; !granted {
		writeRoleError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")

		return security.Principal{}, false
	}

	return principal, true
}

// recordAudit writes a role audit event, logging and returning the error so
// the caller can fail the request: security-sensitive role changes must never
// go unrecorded.
func (h *Handler) recordAudit(
	r *http.Request,
	principal security.Principal,
	action, roleID string,
	metadata map[string]any,
) error {
	orgID := principal.OrganizationID
	if err := h.audit.Record(r.Context(), &orgID, principal.UserID, action, "role", roleID, metadata); err != nil {
		slog.ErrorContext(r.Context(), "audit role change failed", "action", action, "error", err)

		return fmt.Errorf("record role audit: %w", err)
	}

	return nil
}

func writeRoleServiceError(w http.ResponseWriter, r *http.Request, err error, logMsg, fallback string) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeRoleError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrBuiltinRole):
		writeRoleError(w, r, http.StatusBadRequest, "BUILTIN_ROLE", err.Error())
	case errors.Is(err, ErrNotFound):
		writeRoleError(w, r, http.StatusNotFound, "NOT_FOUND", "Role not found")
	case errors.Is(err, ErrNameTaken):
		writeRoleError(w, r, http.StatusConflict, "ROLE_NAME_TAKEN", err.Error())
	case errors.Is(err, ErrPlanNotEligible):
		writeRoleError(w, r, http.StatusForbidden, "PLAN_NOT_ELIGIBLE", err.Error())
	default:
		slog.ErrorContext(r.Context(), logMsg, "error", err)
		writeRoleError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func writeRoleJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	if err := httpx.WriteJSON(w, status, data); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

func writeRoleError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write error response", "error", err)
	}
}
