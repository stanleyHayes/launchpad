package requests

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes equipment and access request HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs a requests Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandleCreateMine raises a request for the caller (employee self-service).
func (h *Handler) HandleCreateMine(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Kind    string `json:"kind"`
		Item    string `json:"item"`
		Details string `json:"details"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	request, err := h.svc.CreateMine(r.Context(), principal.OrganizationID, principal.UserID, CreateInput{
		OrganizationID: principal.OrganizationID,
		EmployeeID:     "",
		Kind:           body.Kind,
		Item:           body.Item,
		Details:        body.Details,
	})
	if err != nil {
		writeRequestError(w, r, err)

		return
	}

	if !h.record(w, r, principal, "request.created", request, map[string]any{
		"kind": request.Kind,
		"item": request.Item,
	}) {
		return
	}

	writeJSON(w, r, http.StatusCreated, request.ToResponse())
}

// HandleListMine lists the caller's own requests.
func (h *Handler) HandleListMine(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListMine(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeRequestError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, toResponses(items))
}

// HandleCancelMine cancels one of the caller's own pending requests.
func (h *Handler) HandleCancelMine(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	request, err := h.svc.CancelMine(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "requestID"),
	)
	if err != nil {
		writeRequestError(w, r, err)

		return
	}

	if !h.record(w, r, principal, "request.cancelled", request, nil) {
		return
	}

	writeJSON(w, r, http.StatusOK, request.ToResponse())
}

// HandleList lists requests for the current organization, optionally filtered
// by ?status=.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.List(r.Context(), principal.OrganizationID, r.URL.Query().Get("status"))
	if err != nil {
		writeRequestError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, toResponses(items))
}

// HandleDecide approves or rejects a pending request.
func (h *Handler) HandleDecide(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	request, err := h.svc.Decide(r.Context(), DecideInput{
		OrganizationID: principal.OrganizationID,
		RequestID:      chi.URLParam(r, "requestID"),
		ApproverUserID: principal.UserID,
		Approve:        body.Approve,
		Note:           body.Note,
	})
	if err != nil {
		writeRequestError(w, r, err)

		return
	}

	if !h.record(w, r, principal, "request.decided", request, map[string]any{"status": request.Status}) {
		return
	}

	writeJSON(w, r, http.StatusOK, request.ToResponse())
}

// HandleFulfill marks an approved request as provisioned.
func (h *Handler) HandleFulfill(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	request, err := h.svc.Fulfill(
		r.Context(),
		principal.OrganizationID,
		chi.URLParam(r, "requestID"),
	)
	if err != nil {
		writeRequestError(w, r, err)

		return
	}

	if !h.record(w, r, principal, "request.fulfilled", request, nil) {
		return
	}

	writeJSON(w, r, http.StatusOK, request.ToResponse())
}

// record writes an audit event for a request action. Failures abort the
// response with a 500, matching the support module's audit policy. It returns
// false when the response was already written.
func (h *Handler) record(
	w http.ResponseWriter,
	r *http.Request,
	principal security.Principal,
	action string,
	request Request,
	metadata map[string]any,
) bool {
	orgID := request.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		action,
		"request",
		request.ID,
		metadata,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit request action failed", "action", action, "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return false
	}

	return true
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func writeRequestError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Request not found")
	case errors.Is(err, ErrForbidden):
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "You may not access this request")
	case errors.Is(err, ErrInvalidState):
		writeError(w, r, http.StatusConflict, "INVALID_STATE", err.Error())
	default:
		slog.ErrorContext(r.Context(), "requests handler error", "error", err)
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
