// Package organizations implements organization use cases and HTTP handlers.
package organizations

import (
	"errors"
	"log/slog"
	"net/http"

	"launchpad/internal/audit"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

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

	writeOrgJSON(w, r, http.StatusOK, org)
}

// HandleUpdateCurrent updates the caller's organization.
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

	var body struct {
		Name     *string `json:"name"`
		Timezone *string `json:"timezone"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeOrgError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	org, err := h.svc.Update(r.Context(), principal.OrganizationID, UpdateInput{
		Name:     body.Name,
		Timezone: body.Timezone,
	})
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

	writeOrgJSON(w, r, http.StatusOK, org)
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
