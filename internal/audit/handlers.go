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

	limit := int64(0)

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeAuditError(w, r, http.StatusBadRequest, "INVALID_LIMIT", "limit must be an integer")

			return
		}

		limit = parsed
	}

	events, err := h.svc.List(r.Context(), principal.OrganizationID, limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "list audit events failed", "error", err)
		writeAuditError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list audit events")

		return
	}

	if err := httpx.WriteJSON(w, http.StatusOK, events); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

func writeAuditError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write error response", "error", err)
	}
}
