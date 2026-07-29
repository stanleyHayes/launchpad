// Package auth implements authentication use cases and HTTP handlers.
package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// responseStatusField is the JSON field name for simple status responses.
const responseStatusField = "status"

// Handler exposes auth HTTP endpoints.
type Handler struct {
	svc *Service
	// secureCookies marks session cookies Secure; enable outside local
	// development (the app wires config.AppEnv != "local").
	secureCookies bool
}

// NewHandler constructs an auth Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, secureCookies: false}
}

// WithSecureCookies controls the Secure attribute on session cookies and
// returns the handler for chaining.
func (h *Handler) WithSecureCookies(secure bool) *Handler {
	h.secureCookies = secure

	return h
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

	SetSessionCookies(w, result.Tokens, h.svc.cfg.RefreshTTL, h.secureCookies)
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

	// MFA-challenged logins hold no tokens yet: the client exchanges the
	// ticket (plus the second factor) via /auth/login/mfa, so no session
	// cookies are set here.
	if result.MFARequired {
		writeHTTPJSON(w, r, http.StatusOK, map[string]any{
			"mfaRequired": true,
			"mfaTicket":   result.MFATicket,
		})

		return
	}

	SetSessionCookies(w, result.Tokens, h.svc.cfg.RefreshTTL, h.secureCookies)
	writeHTTPJSON(w, r, http.StatusOK, result)
}

// HandleLoginMFA completes an MFA-challenged login by exchanging the
// single-use ticket plus a TOTP or backup code for a session. This route is
// public.
func (h *Handler) HandleLoginMFA(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ticket string `json:"ticket"`
		Code   string `json:"code"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	result, err := h.svc.CompleteMFALogin(r.Context(), body.Ticket, body.Code)
	if err != nil {
		writeAuthError(w, r, err)

		return
	}

	SetSessionCookies(w, result.Tokens, h.svc.cfg.RefreshTTL, h.secureCookies)
	writeHTTPJSON(w, r, http.StatusOK, result)
}

// HandleMFAEnroll starts TOTP enrollment for the authenticated principal,
// returning the secret, otpauth URL, and one-time backup codes.
func (h *Handler) HandleMFAEnroll(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	result, err := h.svc.MFAEnroll(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeAuthError(w, r, err)

		return
	}

	writeHTTPJSON(w, r, http.StatusCreated, result)
}

// HandleMFAConfirm enables MFA after verifying a code from the provisioned
// authenticator.
func (h *Handler) HandleMFAConfirm(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	if err := h.svc.MFAConfirm(r.Context(), principal.OrganizationID, principal.UserID, body.Code); err != nil {
		writeAuthError(w, r, err)

		return
	}

	writeHTTPJSON(w, r, http.StatusOK, map[string]string{responseStatusField: "mfa_enabled"})
}

// HandleMFADisable disables MFA after verifying a TOTP or backup code.
func (h *Handler) HandleMFADisable(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	if err := h.svc.MFADisable(r.Context(), principal.OrganizationID, principal.UserID, body.Code); err != nil {
		writeAuthError(w, r, err)

		return
	}

	writeHTTPJSON(w, r, http.StatusOK, map[string]string{responseStatusField: "mfa_disabled"})
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

	combined := strings.TrimSpace(body.RefreshToken)
	if combined == "" {
		// Browser clients hold the refresh token in an HttpOnly cookie
		// instead of the request body.
		if cookie, cookieErr := r.Cookie(RefreshTokenCookieName); cookieErr == nil {
			combined = cookie.Value
		}
	}

	refresh, sessionID, err := ParseRefreshToken(combined)
	if err != nil {
		writeHTTPError(w, r, http.StatusUnauthorized, "INVALID_SESSION", "Refresh token is invalid")

		return
	}

	tokens, err := h.svc.Refresh(r.Context(), sessionID, refresh)
	if err != nil {
		writeAuthError(w, r, err)

		return
	}

	SetSessionCookies(w, tokens, h.svc.cfg.RefreshTTL, h.secureCookies)
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

	ClearSessionCookies(w, h.secureCookies)
	writeHTTPJSON(w, r, http.StatusOK, map[string]string{responseStatusField: "logged_out"})
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

// HandleListOrganizations returns workspaces available to the authenticated user.
func (h *Handler) HandleListOrganizations(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	items, err := h.svc.ListOrganizations(r.Context(), principal.UserID)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeHTTPJSON(w, r, http.StatusOK, items)
}

// HandleSwitchOrganization rotates the session into another membership.
func (h *Handler) HandleSwitchOrganization(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	var body struct {
		OrganizationID string `json:"organizationId"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
		return
	}
	result, err := h.svc.SwitchOrganization(r.Context(), principal, body.OrganizationID)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	SetSessionCookies(w, result.Tokens, h.svc.cfg.RefreshTTL, h.secureCookies)
	writeHTTPJSON(w, r, http.StatusOK, result)
}

// HandleIssueInvitation issues a single-use invitation for a new member.
// Authorization is enforced by the route-level RequirePermission
// (members.invite); the handler only needs the authenticated principal.
func (h *Handler) HandleIssueInvitation(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	var body struct {
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
		Role        string `json:"role"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	token, err := h.svc.IssueInvitation(
		r.Context(), principal.OrganizationID, body.Email, body.DisplayName, body.Role, principal.UserID,
	)
	if err != nil {
		writeAuthError(w, r, err)

		return
	}

	writeHTTPJSON(w, r, http.StatusCreated, map[string]string{"token": token})
}

func (h *Handler) HandleListInvitations(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	items, err := h.svc.ListInvitations(r.Context(), principal.OrganizationID)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeHTTPJSON(w, r, http.StatusOK, items)
}

func (h *Handler) HandleRevokeInvitation(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	if err := h.svc.RevokeInvitation(
		r.Context(), principal.OrganizationID, chi.URLParam(r, "invitationID"), principal.UserID,
	); err != nil {
		writeAuthError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) HandleResendInvitation(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeHTTPError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	token, err := h.svc.ResendInvitation(
		r.Context(), principal.OrganizationID, chi.URLParam(r, "invitationID"), principal.UserID,
	)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeHTTPJSON(w, r, http.StatusOK, map[string]string{"token": token})
}

// HandleAcceptInvitation redeems an invitation token, sets the account's
// password, activates it, and returns a session. This route is public.
func (h *Handler) HandleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	result, err := h.svc.AcceptInvitation(r.Context(), body.Token, body.Password)
	if err != nil {
		writeAuthError(w, r, err)

		return
	}

	SetSessionCookies(w, result.Tokens, h.svc.cfg.RefreshTTL, h.secureCookies)
	writeHTTPJSON(w, r, http.StatusOK, result)
}

// HandleRequestPasswordReset issues a password-reset token and emails the
// reset link when a mailer is configured. It always answers 202 for valid
// input — whether or not the email is registered — so the endpoint cannot be
// used to enumerate accounts. This route is public.
func (h *Handler) HandleRequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	if err := h.svc.RequestPasswordReset(r.Context(), body.Email); err != nil {
		writeAuthError(w, r, err)

		return
	}

	writeHTTPJSON(w, r, http.StatusAccepted, map[string]string{responseStatusField: "reset_requested"})
}

// HandleConfirmPasswordReset consumes a reset token, sets the new password,
// and revokes all of the user's sessions. This route is public.
func (h *Handler) HandleConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	if err := h.svc.ResetPassword(r.Context(), body.Token, body.NewPassword); err != nil {
		writeAuthError(w, r, err)

		return
	}

	writeHTTPJSON(w, r, http.StatusOK, map[string]string{responseStatusField: "password_reset"})
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrWeakPassword):
		writeHTTPError(w, r, http.StatusBadRequest, "WEAK_PASSWORD", err.Error())
	case errors.Is(err, ErrEmailTaken):
		writeHTTPError(w, r, http.StatusConflict, "EMAIL_TAKEN", err.Error())
	case errors.Is(err, ErrInvitationInvalid):
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_INVITATION", "Invitation is invalid or expired")
	case errors.Is(err, ErrPasswordResetInvalid):
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_RESET_TOKEN", "Password reset token is invalid or expired")
	case writeMFAError(w, r, err):
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

// writeMFAError maps the MFA use-case errors and reports whether err matched
// one of them, keeping writeAuthError's switch readable.
func writeMFAError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, ErrMFACodeInvalid):
		writeHTTPError(w, r, http.StatusBadRequest, "INVALID_MFA_CODE", "The code is invalid")
	case errors.Is(err, ErrMFANotEnrolled):
		writeHTTPError(w, r, http.StatusBadRequest, "MFA_NOT_ENROLLED", "MFA is not enrolled")
	case errors.Is(err, ErrMFAAlreadyEnabled):
		writeHTTPError(w, r, http.StatusConflict, "MFA_ALREADY_ENABLED", "MFA is already enabled")
	case errors.Is(err, ErrMFATicketInvalid):
		writeHTTPError(w, r, http.StatusUnauthorized, "INVALID_MFA_TICKET", "MFA ticket is invalid or expired")
	default:
		return false
	}

	return true
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
