package supportsessions

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Audit actions recorded for the support session lifecycle.
const (
	ActionSessionStarted = "support_session.started"
	ActionSessionEnded   = "support_session.ended"

	resourceType = "support_session"
)

// Handler exposes the platform support session HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs a support session Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandleCreate starts a support session and returns its impersonation token.
// Mounted on the platform route group, which already restricts access to
// platform staff via middleware.RequirePlatform.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		OrganizationID  string `json:"organizationId"`
		Reason          string `json:"reason"`
		DurationMinutes int    `json:"durationMinutes"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	session, token, err := h.svc.Create(r.Context(), CreateInput{
		OrganizationID:  body.OrganizationID,
		AgentUserID:     principal.UserID,
		AgentEmail:      principal.Email,
		Reason:          body.Reason,
		DurationMinutes: body.DurationMinutes,
	})
	if err != nil {
		writeServiceError(w, r, err)

		return
	}

	if !h.recordAudit(w, r, principal, session, ActionSessionStarted) {
		return
	}

	writeJSON(w, r, http.StatusCreated, CreateSessionResponse{
		Session:        session.ToResponse(),
		Token:          token,
		TokenExpiresAt: session.CreatedAt.Add(TokenTTL),
	})
}

// HandleEnd ends a support session early. The body is optional; an empty
// body records the default end reason.
func (h *Handler) HandleEnd(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		EndReason string `json:"endReason"`
	}
	if r.ContentLength > 0 {
		if err := httpx.DecodeJSON(r, &body); err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

			return
		}
	}

	session, err := h.svc.End(r.Context(), chi.URLParam(r, "sessionID"), body.EndReason)
	if err != nil {
		writeServiceError(w, r, err)

		return
	}

	if !h.recordAudit(w, r, principal, session, ActionSessionEnded) {
		return
	}

	writeJSON(w, r, http.StatusOK, session.ToResponse())
}

// HandleList returns the support session audit trail of one organization.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.svc.List(r.Context(), r.URL.Query().Get("organizationId"))
	if err != nil {
		writeServiceError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, toSessionResponses(sessions))
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

// recordAudit writes the session lifecycle audit event. The audited tenant
// is the target organization; the actor is the platform support agent. A
// failed audit write fails the request (500): support impersonation must
// never run unaudited. It returns false after writing the error response so
// the caller stops before writing a success body.
func (h *Handler) recordAudit(
	w http.ResponseWriter,
	r *http.Request,
	principal security.Principal,
	session Session,
	action string,
) bool {
	orgID := session.OrganizationID

	err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		action,
		resourceType,
		session.ID,
		map[string]any{
			"reason":    session.Reason,
			"expiresAt": session.ExpiresAt.Format(time.RFC3339),
		},
	)
	if err != nil {
		slog.ErrorContext(r.Context(), "audit support session action failed", "error", err, "action", action)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return false
	}

	return true
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid support session input")
	case errors.Is(err, organizations.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Organization not found")
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Support session not found")
	case errors.Is(err, ErrSessionEnded):
		writeError(w, r, http.StatusConflict, "SESSION_ENDED", "Support session already ended")
	default:
		slog.ErrorContext(r.Context(), "support session handler error", "error", err)
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
