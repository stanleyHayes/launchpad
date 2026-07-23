// Package auth implements authentication use cases and HTTP handlers.
package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes auth HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs an auth Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleRegister registers a new owner and organization.
func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var in RegisterInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	result, err := h.svc.Register(r.Context(), in)
	if err != nil {
		writeAuthError(w, r, err)

		return
	}

	writeHTTPJSON(w, r, http.StatusCreated, result)
}

// HandleLogin authenticates a user.
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var in LoginInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	result, err := h.svc.Login(r.Context(), in)
	if err != nil {
		writeAuthError(w, r, err)

		return
	}

	writeHTTPJSON(w, r, http.StatusOK, result)
}

// HandleRefresh rotates tokens.
func (h *Handler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	refresh, sessionID, err := ParseRefreshToken(body.RefreshToken)
	if err != nil {
		writeHTTPError(w, r, http.StatusUnauthorized, "INVALID_SESSION", "Refresh token is invalid")

		return
	}

	tokens, err := h.svc.Refresh(r.Context(), sessionID, refresh)
	if err != nil {
		writeAuthError(w, r, err)

		return
	}

	writeHTTPJSON(w, r, http.StatusOK, tokens)
}

// HandleLogout revokes the current session.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	if err := h.svc.Logout(r.Context(), principal.SessionID); err != nil {
		slog.ErrorContext(r.Context(), "logout failed", "error", err)
		writeHTTPError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to logout")

		return
	}

	writeHTTPJSON(w, r, http.StatusOK, map[string]string{"status": "logged_out"})
}

// HandleMe returns the authenticated profile.
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	me, err := h.svc.Me(r.Context(), principal)
	if err != nil {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Unable to load profile")

		return
	}

	writeHTTPJSON(w, r, http.StatusOK, me)
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrWeakPassword):
		writeHTTPError(w, r, http.StatusBadRequest, "WEAK_PASSWORD", err.Error())
	case errors.Is(err, ErrEmailTaken):
		writeHTTPError(w, r, http.StatusConflict, "EMAIL_TAKEN", err.Error())
	case errors.Is(err, organizations.ErrSlugTaken):
		writeHTTPError(w, r, http.StatusConflict, "SLUG_TAKEN", err.Error())
	case errors.Is(err, organizations.ErrInvalidInput):
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrSessionInvalid):
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials or session")
	case errors.Is(err, ErrAuditFailed):
		slog.ErrorContext(r.Context(), "audit failure", "error", err)
		writeHTTPError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected error")
	default:
		slog.ErrorContext(r.Context(), "auth handler error", "error", err)
		writeHTTPError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected error")
	}
}

func writeHTTPJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	if err := httpx.WriteJSON(w, status, data); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

func writeHTTPError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write error response", "error", err)
	}
}
