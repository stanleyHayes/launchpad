package integrations

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/entitlements"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes integration HTTP endpoints. Authorization is enforced by the
// route-level RequirePermission (integrations.manage); handlers only need the
// authenticated principal.
type Handler struct {
	svc *Service
}

// NewHandler constructs an integrations Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleList lists the caller organization's connections (tokens omitted).
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	connections, err := h.svc.List(r.Context(), principal.OrganizationID)
	if err != nil {
		writeIntegrationError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, connections)
}

// HandleConnect validates and stores a provider connection.
func (h *Handler) HandleConnect(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Token   string `json:"token"`
		BaseURL string `json:"baseUrl"`
		Email   string `json:"email"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	conn, err := h.svc.Connect(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "provider"),
		ConnectInput{Token: body.Token, BaseURL: body.BaseURL, Email: body.Email},
	)
	if err != nil {
		writeIntegrationError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, conn)
}

// HandleDisconnect removes a provider connection.
func (h *Handler) HandleDisconnect(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	err := h.svc.Disconnect(r.Context(), principal.OrganizationID, principal.UserID, chi.URLParam(r, "provider"))
	if err != nil {
		writeIntegrationError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleHealth re-validates the stored credential and returns the connection.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	conn, err := h.svc.Health(r.Context(), principal.OrganizationID, chi.URLParam(r, "provider"))
	if err != nil {
		writeIntegrationError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, conn)
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func writeIntegrationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, entitlements.ErrLimitExceeded):
		writeError(w, r, http.StatusConflict, "PLAN_LIMIT_EXCEEDED", err.Error())
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid integration configuration")
	case errors.Is(err, ErrUnknownProvider):
		writeError(w, r, http.StatusBadRequest, "UNKNOWN_PROVIDER", "Unknown integration provider")
	case errors.Is(err, ErrInvalidCredential):
		writeError(w, r, http.StatusBadRequest, "INVALID_CREDENTIAL", "Provider rejected the credential")
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Integration not connected")
	default:
		slog.ErrorContext(r.Context(), "integrations handler error", "error", err)
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
