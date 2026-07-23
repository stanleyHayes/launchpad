package analytics

import (
	"errors"
	"log/slog"
	"net/http"

	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes analytics HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleOnboardingSummary returns organization onboarding analytics.
func (h *Handler) HandleOnboardingSummary(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireManager(w, r)
	if !ok {
		return
	}

	summary, err := h.svc.OnboardingSummary(r.Context(), principal.OrganizationID)
	if err != nil {
		writeAnalyticsError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, summary)
}

func requireManager(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	if !organizations.CanManageOrganization(principal.RoleCode) {
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")

		return security.Principal{}, false
	}

	return principal, true
}

func writeAnalyticsError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	default:
		slog.ErrorContext(r.Context(), "analytics handler error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected error")
	}
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	if err := httpx.WriteJSON(w, status, data); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write error response", "error", err)
	}
}
