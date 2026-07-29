package privacy

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes the privacy HTTP endpoints: the tenant data export and the
// platform tenant purge.
type Handler struct {
	export *ExportService
	purge  *PurgeService
	audit  *audit.Service
}

// NewHandler constructs a privacy Handler.
func NewHandler(exportSvc *ExportService, purgeSvc *PurgeService, auditSvc *audit.Service) *Handler {
	return &Handler{export: exportSvc, purge: purgeSvc, audit: auditSvc}
}

// HandleExport returns the GDPR data export of the caller's organization.
// Mounted with the data.export permission gate (PRD 7.4).
func (h *Handler) HandleExport(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	export, err := h.export.Export(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "organization data export failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to export organization data")

		return
	}

	// Privileged read: audit who exported what, best-effort so a broken audit
	// store never blocks the data subject's right to access.
	orgID := principal.OrganizationID

	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		"organization.data_exported",
		"organization",
		orgID,
		nil,
	); err != nil {
		slog.WarnContext(r.Context(), "audit data export failed", "error", err)
	}

	writeJSON(w, r, export)
}

// HandlePurgeOrganization irreversibly deletes all data of a tenant. Mounted
// on the platform route group, which already restricts access to platform
// staff. The request body must confirm the organization slug.
func (h *Handler) HandlePurgeOrganization(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	var body struct {
		Confirm string `json:"confirm"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	result, err := h.purge.Purge(r.Context(), chi.URLParam(r, "organizationID"), body.Confirm, principal.UserID)
	if err != nil {
		writePurgeError(w, r, err)

		return
	}

	writeJSON(w, r, result)
}

func writePurgeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, organizations.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Organization not found")
	case errors.Is(err, ErrConfirmationMismatch):
		writeError(w, r, http.StatusBadRequest, "CONFIRMATION_MISMATCH", "confirm must equal the organization slug")
	default:
		slog.ErrorContext(r.Context(), "organization purge failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to purge organization")
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
