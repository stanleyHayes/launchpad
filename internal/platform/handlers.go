// Package platform implements platform staff use cases and HTTP handlers.
package platform

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
)

// Handler exposes platform HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs a platform Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleOverview returns platform-wide metrics.
func (h *Handler) HandleOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.svc.Overview(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "platform overview failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load overview")

		return
	}

	writeJSON(w, r, overview)
}

// HandleListOrganizations lists all tenant organizations.
func (h *Handler) HandleListOrganizations(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListOrganizations(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list organizations failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list organizations")

		return
	}

	writeJSON(w, r, items)
}

// HandleGetOrganization returns one tenant organization.
func (h *Handler) HandleGetOrganization(w http.ResponseWriter, r *http.Request) {
	org, err := h.svc.GetOrganization(r.Context(), chi.URLParam(r, "organizationID"))
	if err != nil {
		writeOrganizationError(w, r, err)

		return
	}

	writeJSON(w, r, org)
}

// HandleSuspendOrganization suspends a tenant organization.
func (h *Handler) HandleSuspendOrganization(w http.ResponseWriter, r *http.Request) {
	org, err := h.svc.SetOrganizationStatus(
		r.Context(),
		chi.URLParam(r, "organizationID"),
		organizations.StatusSuspended(),
	)
	if err != nil {
		writeOrganizationError(w, r, err)

		return
	}

	writeJSON(w, r, org)
}

// HandleActivateOrganization activates a tenant organization.
func (h *Handler) HandleActivateOrganization(w http.ResponseWriter, r *http.Request) {
	org, err := h.svc.SetOrganizationStatus(
		r.Context(),
		chi.URLParam(r, "organizationID"),
		organizations.StatusActive(),
	)
	if err != nil {
		writeOrganizationError(w, r, err)

		return
	}

	writeJSON(w, r, org)
}

func writeOrganizationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, organizations.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Organization not found")
	case errors.Is(err, organizations.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	default:
		slog.ErrorContext(r.Context(), "platform organization handler error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected error")
	}
}

func writeJSON(w http.ResponseWriter, r *http.Request, data any) {
	if err := httpx.WriteJSON(w, http.StatusOK, data); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write error response", "error", err)
	}
}
