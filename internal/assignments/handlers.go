package assignments

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

// Handler exposes assignment HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandleAssign assigns a journey to an employee.
func (h *Handler) HandleAssign(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireManager(w, r)
	if !ok {
		return
	}

	var body struct {
		EmployeeID        string `json:"employeeId"`
		JourneyTemplateID string `json:"journeyTemplateId"`
		StartsAt          string `json:"startsAt"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	startsAt := time.Now().UTC()

	if body.StartsAt != "" {
		parsed, err := time.Parse(time.RFC3339, body.StartsAt)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", body.StartsAt)
			if err != nil {
				writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", "startsAt must be RFC3339 or YYYY-MM-DD")

				return
			}
		}

		startsAt = parsed.UTC()
	}

	result, err := h.svc.Assign(r.Context(), principal.OrganizationID, principal.UserID, AssignInput{
		EmployeeID:        body.EmployeeID,
		JourneyTemplateID: body.JourneyTemplateID,
		StartsAt:          startsAt,
	})
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		"assignment.created",
		"journey_assignment",
		result.Assignment.ID,
		map[string]any{"employeeId": body.EmployeeID},
	); err != nil {
		slog.ErrorContext(r.Context(), "audit assignment failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeJSON(w, r, http.StatusCreated, result)
}

// HandleList lists organization assignments.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.List(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list assignments failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list assignments")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleListMine lists the caller's assignments.
func (h *Handler) HandleListMine(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListMine(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleGet returns one assignment.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	item, err := h.svc.Get(r.Context(), principal.OrganizationID, chi.URLParam(r, "assignmentID"))
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, item)
}

// HandleListSteps lists steps for an assignment.
func (h *Handler) HandleListSteps(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListSteps(r.Context(), principal.OrganizationID, chi.URLParam(r, "assignmentID"))
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleCompleteStep completes or submits a step.
func (h *Handler) HandleCompleteStep(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Submission map[string]any `json:"submission"`
		Score      *float64       `json:"score"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	step, err := h.svc.CompleteStep(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "stepAssignmentID"),
		CompleteStepInput{Submission: body.Submission, Score: body.Score},
	)
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, step)
}

// HandleListApprovals lists approvals.
func (h *Handler) HandleListApprovals(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireManager(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListApprovals(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list approvals failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list approvals")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleDecideApproval decides an approval.
func (h *Handler) HandleDecideApproval(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireManager(w, r)
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

	approval, err := h.svc.DecideApproval(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "approvalID"),
		DecideApprovalInput{Approve: body.Approve, Note: body.Note},
	)
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, approval)
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func requireManager(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return security.Principal{}, false
	}

	if !organizations.CanManageOrganization(principal.RoleCode) {
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")

		return security.Principal{}, false
	}

	return principal, true
}

func writeAssignmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrStepNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, ErrAlreadyAssigned), errors.Is(err, ErrInvalidState), errors.Is(err, ErrApprovalRequired):
		writeError(w, r, http.StatusConflict, "INVALID_STATE", err.Error())
	default:
		slog.ErrorContext(r.Context(), "assignment handler error", "error", err)
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
