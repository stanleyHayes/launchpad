package assistant

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes AI assistant HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs an assistant Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleAsk answers a question grounded in the tenant's knowledge base.
func (h *Handler) HandleAsk(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Question string `json:"question"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	answer, err := h.svc.Ask(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		body.Question,
		organizations.CanManageOrganization(principal.RoleCode),
	)
	if err != nil {
		writeAssistantError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, answer)
}

// HandleFeedback records whether an answer was helpful.
func (h *Handler) HandleFeedback(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Helpful bool `json:"helpful"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	err := h.svc.Feedback(r.Context(), principal.OrganizationID, chi.URLParam(r, "interactionID"), body.Helpful)
	if err != nil {
		writeAssistantError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func writeAssistantError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Assistant record not found")
	default:
		slog.ErrorContext(r.Context(), "assistant handler error", "error", err)
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
