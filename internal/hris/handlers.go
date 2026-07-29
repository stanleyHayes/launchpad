package hris

import (
	"errors"
	"log/slog"
	"net/http"

	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes HRIS HTTP endpoints. Authorization is enforced by the
// route-level RequirePermission (integrations.manage); handlers only need the
// authenticated principal.
type Handler struct {
	svc *Service
}

// NewHandler constructs an HRIS Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleGetConfig returns the tenant's HRIS configuration (token omitted).
func (h *Handler) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	config, err := h.svc.GetConfig(r.Context(), principal.OrganizationID)
	if err != nil {
		writeHRISError(w, r, err)

		return
	}

	writeJSON(w, r, config)
}

// HandleSetConfig stores the tenant's HRIS configuration.
func (h *Handler) HandleSetConfig(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Provider  string `json:"provider"`
		Subdomain string `json:"subdomain"`
		APIToken  string `json:"apiToken"`
		BaseURL   string `json:"baseUrl"`
		Enabled   bool   `json:"enabled"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	config, err := h.svc.SetConfig(r.Context(), principal.OrganizationID, SetConfigInput{
		Provider:  body.Provider,
		Subdomain: body.Subdomain,
		APIToken:  body.APIToken,
		BaseURL:   body.BaseURL,
		Enabled:   body.Enabled,
	})
	if err != nil {
		writeHRISError(w, r, err)

		return
	}

	writeJSON(w, r, config)
}

// HandleSync triggers a sync now and returns the resulting run.
func (h *Handler) HandleSync(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	run, err := h.svc.Sync(r.Context(), principal.OrganizationID)
	if err != nil {
		writeHRISError(w, r, err)

		return
	}

	writeJSON(w, r, run)
}

// HandleGetState returns the latest synced directory and last run.
func (h *Handler) HandleGetState(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	state, err := h.svc.GetState(r.Context(), principal.OrganizationID)
	if err != nil {
		writeHRISError(w, r, err)

		return
	}

	writeJSON(w, r, state)
}

// HandleApply maps the tenant's latest synced directory snapshot into employee
// records and returns a summary (created / skipped / failed).
func (h *Handler) HandleApply(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	result, err := h.svc.Apply(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeHRISError(w, r, err)

		return
	}

	writeJSON(w, r, result)
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func writeHRISError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid HRIS configuration")
	case errors.Is(err, ErrUnsupportedProvider):
		writeError(w, r, http.StatusBadRequest, "UNSUPPORTED_PROVIDER", "Unsupported HRIS provider")
	case errors.Is(err, ErrNotConfigured):
		writeError(w, r, http.StatusNotFound, "HRIS_NOT_CONFIGURED", "HRIS is not configured")
	case errors.Is(err, ErrNoDirectory):
		writeError(w, r, http.StatusNotFound, "HRIS_NO_DIRECTORY", "No synced directory to apply. Run a sync first.")
	case errors.Is(err, ErrSyncFailed):
		writeError(w, r, http.StatusBadGateway, "SYNC_FAILED", "HRIS sync failed")
	default:
		slog.ErrorContext(r.Context(), "hris handler error", "error", err)
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
