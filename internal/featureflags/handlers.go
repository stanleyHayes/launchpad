// Package featureflags implements feature flag use cases and HTTP handlers.
package featureflags

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes feature flag HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs a feature flags Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
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
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Key               string   `json:"key"`
		Description       string   `json:"description"`
		Enabled           bool     `json:"enabled"`
		PlanCodes         []string `json:"planCodes"`
		RolloutPercentage int      `json:"rolloutPercentage"`
		CohortUserIDs     []string `json:"cohortUserIds"`
		ExpiresAt         string   `json:"expiresAt"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}
	expiresAt := parseOptionalTime(body.ExpiresAt)
	if strings.TrimSpace(body.ExpiresAt) != "" && expiresAt == nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", "expiresAt must be RFC3339")
		return
	}

	flag, err := h.svc.CreateFlag(r.Context(), CreateFlagInput{
		Key:               body.Key,
		Description:       body.Description,
		Enabled:           body.Enabled,
		PlanCodes:         body.PlanCodes,
		RolloutPercentage: body.RolloutPercentage,
		CohortUserIDs:     body.CohortUserIDs,
		ExpiresAt:         expiresAt,
		ActorUserID:       principal.UserID,
	})
	if err != nil {
		writeFeatureFlagError(w, r, err)

		return
	}

	h.recordFlagAudit(r, nil, principal, "feature_flag.created", flag.Key)

	writeJSON(w, r, http.StatusCreated, flag)
}

// HandlePlatformPatch updates a global feature flag.
func (h *Handler) HandlePlatformPatch(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Description       *string   `json:"description"`
		Enabled           *bool     `json:"enabled"`
		PlanCodes         *[]string `json:"planCodes"`
		RolloutPercentage *int      `json:"rolloutPercentage"`
		CohortUserIDs     *[]string `json:"cohortUserIds"`
		ExpiresAt         *string   `json:"expiresAt"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	expiresAt, validExpiry := parsePatchTime(body.ExpiresAt)
	if !validExpiry {
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", "expiresAt must be RFC3339 or null")
		return
	}

	flag, err := h.svc.UpdateFlag(r.Context(), chi.URLParam(r, "key"), UpdateFlagInput{
		Description:       body.Description,
		Enabled:           body.Enabled,
		PlanCodes:         body.PlanCodes,
		RolloutPercentage: body.RolloutPercentage,
		CohortUserIDs:     body.CohortUserIDs,
		ExpiresAt:         expiresAt,
		UpdatedBy:         principal.UserID,
	})
	if err != nil {
		writeFeatureFlagError(w, r, err)

		return
	}

	h.recordFlagAudit(r, nil, principal, "feature_flag.updated", flag.Key)

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

	organizationID := chi.URLParam(r, "organizationID")

	override, err := h.svc.SetOverride(r.Context(), SetOverrideInput{
		OrganizationID: organizationID,
		Key:            chi.URLParam(r, "key"),
		Enabled:        body.Enabled,
		UpdatedBy:      principal.UserID,
	})
	if err != nil {
		writeFeatureFlagError(w, r, err)

		return
	}

	h.recordFlagAudit(r, &organizationID, principal, "feature_flag_override.set", override.Key)

	writeJSON(w, r, http.StatusOK, override)
}

// HandleOrgList returns resolved feature flags for the current organization.
func (h *Handler) HandleOrgList(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	flags, err := h.svc.Resolve(r.Context(), principal.OrganizationID, "", principal.UserID)
	if err != nil {
		slog.ErrorContext(r.Context(), "resolve feature flags failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to resolve feature flags")

		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{"flags": flags})
}

// HandlePlatformHistory lists rollout mutations for one flag.
func (h *Handler) HandlePlatformHistory(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	items, err := h.svc.ListHistory(r.Context(), chi.URLParam(r, "key"), limit)
	if err != nil {
		writeFeatureFlagError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, items)
}

func parseOptionalTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &value
}

func parsePatchTime(raw *string) (**time.Time, bool) {
	if raw == nil {
		return nil, true
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		var cleared *time.Time
		return &cleared, true
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, false
	}
	parsedPtr := &parsed
	return &parsedPtr, true
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

// recordFlagAudit records a platform audit event for a feature flag change.
// Global flag changes carry no organization; tenant overrides carry the target
// organization. The flag change is already committed at this point, so an
// audit failure is logged but does not fail the request.
func (h *Handler) recordFlagAudit(
	r *http.Request,
	organizationID *string,
	principal security.Principal,
	action, key string,
) {
	if err := h.audit.Record(
		r.Context(),
		organizationID,
		principal.UserID,
		action,
		"feature_flag",
		key,
		nil,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit feature flag action failed", "error", err, "action", action)
	}
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
