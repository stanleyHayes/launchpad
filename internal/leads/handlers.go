// Package leads implements lead capture use cases and HTTP handlers.
package leads

import (
	"errors"
	"log/slog"
	"net/http"

	"launchpad/pkg/httpx"
)

// Handler exposes lead HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs a leads Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleCreate captures a public lead.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	lead, err := h.svc.Create(r.Context(), in)
	if err != nil {
		writeLeadError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusCreated, lead)
}

// HandleList lists leads for platform staff.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list leads failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list leads")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

func writeLeadError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	default:
		slog.ErrorContext(r.Context(), "lead handler error", "error", err)
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
