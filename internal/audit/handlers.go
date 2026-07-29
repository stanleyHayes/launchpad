package audit

import (
	"log/slog"
	"net/http"
	"strconv"

	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes audit HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs an audit Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleList lists audit events for the current organization.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeAuditError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	limit, ok := parseLimit(w, r)
	if !ok {
		return
	}

	events, err := h.svc.List(r.Context(), principal.OrganizationID, limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "list audit events failed", "error", err)
		writeAuditError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list audit events")

		return
	}

	if err := httpx.WriteJSON(w, http.StatusOK, toEventResponses(events)); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

// HandlePlatformList lists recent audit events across all organizations,
// including platform-level events. Mounted on the platform route group, which
// already restricts access to platform staff.
func (h *Handler) HandlePlatformList(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseLimit(w, r)
	if !ok {
		return
	}

	events, err := h.svc.ListAll(r.Context(), limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "list platform audit events failed", "error", err)
		writeAuditError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list audit events")

		return
	}

	if err := httpx.WriteJSON(w, http.StatusOK, toEventResponses(events)); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

// parseLimit reads the optional limit query parameter, writing a 400 and
// returning ok=false when it is not an integer.
func parseLimit(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 0, true
	}

	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		writeAuditError(w, r, http.StatusBadRequest, "INVALID_LIMIT", "limit must be an integer")

		return 0, false
	}

	return parsed, true
}

func writeAuditError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write error response", "error", err)
	}
}
