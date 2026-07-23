package notifications

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes notification HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs a notification Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleList lists notifications for the authenticated user.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	items, err := h.svc.ListForUser(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeNotificationError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleMarkRead marks the authenticated user's notification as read.
func (h *Handler) HandleMarkRead(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	notification, err := h.svc.MarkRead(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "id"),
	)
	if err != nil {
		writeNotificationError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, notification)
}

func writeNotificationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Notification not found")
	default:
		slog.ErrorContext(r.Context(), "notification handler error", "error", err)
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
