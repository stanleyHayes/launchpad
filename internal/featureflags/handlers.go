// Package featureflags implements feature flag use cases and HTTP handlers.
package featureflags

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes feature flag HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs a feature flags Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandlePlatformList lists global feature flags.
func (h *Handler) HandlePlatformList(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListFlags(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list feature flags failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list feature flags")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandlePlatformCreate creates a global feature flag.
func (h *Handler) HandlePlatformCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key         string   `json:"key"`
		Description string   `json:"description"`
		Enabled     bool     `json:"enabled"`
		PlanCodes   []string `json:"planCodes"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	flag, err := h.svc.CreateFlag(r.Context(), CreateFlagInput{
		Key:         body.Key,
		Description: body.Description,
		Enabled:     body.Enabled,
		PlanCodes:   body.PlanCodes,
	})
	if err != nil {
		writeFeatureFlagError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusCreated, flag)
}

// HandlePlatformPatch updates a global feature flag.
func (h *Handler) HandlePlatformPatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Description *string   `json:"description"`
		Enabled     *bool     `json:"enabled"`
		PlanCodes   *[]string `json:"planCodes"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	flag, err := h.svc.UpdateFlag(r.Context(), chi.URLParam(r, "key"), UpdateFlagInput{
		Description: body.Description,
		Enabled:     body.Enabled,
		PlanCodes:   body.PlanCodes,
	})
	if err != nil {
		writeFeatureFlagError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, flag)
}

// HandlePlatformSetOverride sets a tenant feature flag override.
func (h *Handler) HandlePlatformSetOverride(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	override, err := h.svc.SetOverride(r.Context(), SetOverrideInput{
		OrganizationID: chi.URLParam(r, "organizationID"),
		Key:            chi.URLParam(r, "key"),
		Enabled:        body.Enabled,
		UpdatedBy:      principal.UserID,
	})
	if err != nil {
		writeFeatureFlagError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, override)
}

// HandleOrgList returns resolved feature flags for the current organization.
func (h *Handler) HandleOrgList(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	flags, err := h.svc.Resolve(r.Context(), principal.OrganizationID, "")
	if err != nil {
		slog.ErrorContext(r.Context(), "resolve feature flags failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to resolve feature flags")

		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{"flags": flags})
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func writeFeatureFlagError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Feature flag not found")
	case errors.Is(err, ErrKeyTaken):
		writeError(w, r, http.StatusConflict, "KEY_TAKEN", err.Error())
	default:
		slog.ErrorContext(r.Context(), "feature flag handler error", "error", err)
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
