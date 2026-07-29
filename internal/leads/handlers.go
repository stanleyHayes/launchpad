// Package leads implements lead capture use cases and HTTP handlers.
package leads

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

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

// HandleList lists a page of leads for platform staff. Optional query params:
// limit (page size, server-capped) and before (RFC 3339 keyset cursor — only
// leads created before that instant are returned).
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	limit := int64(0)

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_LIMIT", "limit must be an integer")

			return
		}

		limit = parsed
	}

	var before time.Time

	if raw := r.URL.Query().Get("before"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "before must be an RFC 3339 timestamp")

			return
		}

		before = parsed
	}

	items, err := h.svc.List(r.Context(), limit, before)
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
