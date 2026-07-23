// Package support implements support ticket use cases and HTTP handlers.
package support

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes support HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs a support Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleOrgList lists tickets for the current organization.
func (h *Handler) HandleOrgList(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListForOrganization(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list support tickets failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list tickets")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleOrgCreate creates a support ticket.
func (h *Handler) HandleOrgCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Subject  string `json:"subject"`
		Body     string `json:"body"`
		Priority string `json:"priority"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	ticket, err := h.svc.Create(r.Context(), CreateTicketInput{
		OrganizationID:  principal.OrganizationID,
		CreatedByUserID: principal.UserID,
		Subject:         body.Subject,
		Body:            body.Body,
		Priority:        body.Priority,
	})
	if err != nil {
		writeSupportError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusCreated, ticket)
}

// HandleOrgGet returns one ticket for the current organization.
func (h *Handler) HandleOrgGet(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	ticket, err := h.svc.GetForOrganization(
		r.Context(),
		principal.OrganizationID,
		chi.URLParam(r, "ticketID"),
	)
	if err != nil {
		writeSupportError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, ticket)
}

// HandlePlatformList lists all support tickets.
func (h *Handler) HandlePlatformList(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list support tickets failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list tickets")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandlePlatformGet returns one support ticket.
func (h *Handler) HandlePlatformGet(w http.ResponseWriter, r *http.Request) {
	ticket, err := h.svc.Get(r.Context(), chi.URLParam(r, "ticketID"))
	if err != nil {
		writeSupportError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, ticket)
}

// HandlePlatformUpdateStatus updates ticket workflow state.
func (h *Handler) HandlePlatformUpdateStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status         string  `json:"status"`
		AssigneeUserID *string `json:"assigneeUserId"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	ticket, err := h.svc.UpdateStatus(r.Context(), UpdateTicketStatusInput{
		TicketID:       chi.URLParam(r, "ticketID"),
		Status:         body.Status,
		AssigneeUserID: body.AssigneeUserID,
	})
	if err != nil {
		writeSupportError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, ticket)
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func writeSupportError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Support ticket not found")
	default:
		slog.ErrorContext(r.Context(), "support handler error", "error", err)
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
