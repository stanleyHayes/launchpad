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

// HandleOnboardingBreakdown returns completion rates grouped by department or
// job role (?by=department|jobRole, default department).
func (h *Handler) HandleOnboardingBreakdown(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	by := r.URL.Query().Get("by")
	if by == "" {
		by = BreakdownByDepartment
	}

	breakdown, err := h.svc.OnboardingBreakdown(r.Context(), principal.OrganizationID, by)
	if err != nil {
		writeAnalyticsError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, breakdown)
}

// HandleAssistantReport returns assistant usage and answer-quality analytics.
func (h *Handler) HandleAssistantReport(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	report, err := h.svc.AssistantReport(r.Context(), principal.OrganizationID)
	if err != nil {
		writeAnalyticsError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, report)
}

func (h *Handler) HandleFunnelReport(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	report, err := h.svc.FunnelReport(r.Context(), principal.OrganizationID)
	if err != nil {
		writeAnalyticsError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, report)
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
	case errors.Is(err, ErrSourceNotConfigured):
		writeError(w, r, http.StatusServiceUnavailable, "SOURCE_NOT_CONFIGURED", err.Error())
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
