package assignments

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/employees"
	"launchpad/internal/organizations"
	"launchpad/internal/support"
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
// Authorization: route-level RequirePermission (journeys.assign).
func (h *Handler) HandleAssign(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
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
		// The assignment is already persisted; a failed audit write must not
		// turn a committed change into a reported failure.
		slog.ErrorContext(r.Context(), "audit assignment failed", "error", err)
	}

	writeJSON(w, r, http.StatusCreated, result.ToResponse())
}

// HandleAssignDepartment assigns a journey to every employee in a department.
// Authorization: route-level RequirePermission (journeys.assign).
func (h *Handler) HandleAssignDepartment(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		DepartmentID      string `json:"departmentId"`
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

	result, err := h.svc.AssignToDepartment(r.Context(), principal.OrganizationID, principal.UserID, AssignDepartmentInput{
		DepartmentID:      body.DepartmentID,
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
		"assignment.department_created",
		"journey_assignment",
		body.JourneyTemplateID,
		map[string]any{
			"departmentId": body.DepartmentID,
			"employees":    result.Employees,
			"assigned":     result.Assigned,
			"skipped":      result.Skipped,
		},
	); err != nil {
		// The assignments are already persisted; a failed audit write must not
		// turn a committed change into a reported failure.
		slog.ErrorContext(r.Context(), "audit department assignment failed", "error", err)
	}

	writeJSON(w, r, http.StatusCreated, result)
}

// HandleList lists organization assignments. The route deliberately declares
// no permission: org-wide reads are open to any org member.
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

	writeJSON(w, r, http.StatusOK, toJourneyAssignmentResponses(items))
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

	writeJSON(w, r, http.StatusOK, toJourneyAssignmentResponses(items))
}

// HandleGet returns one assignment.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	item, err := h.svc.GetForActor(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		organizations.CanManageOrganization(principal.RoleCode),
		chi.URLParam(r, "assignmentID"),
	)
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, item.ToResponse())
}

// HandleListSteps lists steps for an assignment.
func (h *Handler) HandleListSteps(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListStepsForActor(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		organizations.CanManageOrganization(principal.RoleCode),
		chi.URLParam(r, "assignmentID"),
		r.URL.Query().Get("locale"),
	)
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, toStepAssignmentResponses(items))
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

	writeJSON(w, r, http.StatusOK, step.ToResponse())
}

// HandleOverrideStep records a manager's explicit workflow intervention.
// Authorization is enforced by the route's assignments.manage permission.
func (h *Handler) HandleOverrideStep(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
		return
	}

	step, err := h.svc.OverrideStep(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "stepAssignmentID"),
		OverrideStepInput{Action: body.Action, Reason: body.Reason},
	)
	if err != nil {
		writeAssignmentError(w, r, err)
		return
	}

	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		"assignment.step_overridden",
		"step_assignment",
		step.ID,
		map[string]any{"action": body.Action, "reason": body.Reason},
	); err != nil {
		slog.ErrorContext(r.Context(), "audit assignment step override failed", "error", err)
	}

	writeJSON(w, r, http.StatusOK, step.ToResponse())
}

// HandleStartStep marks a step started by its owner.
// Authorization: the service enforces ownership — only the step's employee
// may start it.
func (h *Handler) HandleStartStep(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	step, err := h.svc.StartStep(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "stepAssignmentID"),
	)
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, step.ToResponse())
}

// HandleSubmitStep stores partial progress on a step without completing it.
// Authorization: the service enforces ownership — only the step's employee
// may submit it.
func (h *Handler) HandleSubmitStep(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Submission map[string]any `json:"submission"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	step, err := h.svc.SubmitStep(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "stepAssignmentID"),
		SubmitStepInput{Submission: body.Submission},
	)
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, step.ToResponse())
}

// HandleListApprovals lists approvals. The route deliberately declares no
// permission: org-wide reads are open to any org member.
func (h *Handler) HandleListApprovals(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListApprovals(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list approvals failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list approvals")

		return
	}

	writeJSON(w, r, http.StatusOK, toApprovalResponses(items))
}

// HandleDecideApproval decides an approval. The service enforces the
// ownership rule — only the designated approver may decide — so the handler
// needs no role gate of its own.
func (h *Handler) HandleDecideApproval(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, r, http.StatusOK, approval.ToResponse())
}

// HandleTeamRollup returns the manager's direct-reports summary.
// Authorization: route-level RequirePermission (assignments.read); the
// service scopes the result to the caller's direct reports.
func (h *Handler) HandleTeamRollup(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.TeamRollup(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleReportBlocker records a blocker for the caller's employee record.
// Authorization: any authenticated org member; the service ties the blocker
// to the caller's own employee record.
func (h *Handler) HandleReportBlocker(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		StepAssignmentID string `json:"stepAssignmentId"`
		Category         string `json:"category"`
		Message          string `json:"message"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	blocker, err := h.svc.ReportBlocker(r.Context(), principal.OrganizationID, principal.UserID, ReportBlockerInput{
		StepAssignmentID: body.StepAssignmentID,
		Category:         body.Category,
		Message:          body.Message,
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
		"blocker.reported",
		"blocker",
		blocker.ID,
		map[string]any{
			"category":         blocker.Category,
			"stepAssignmentId": blocker.StepAssignmentID,
			"ticketId":         blocker.TicketID,
		},
	); err != nil {
		// The blocker is already persisted; a failed audit write must not
		// turn a committed change into a reported failure.
		slog.ErrorContext(r.Context(), "audit blocker failed", "error", err)
	}

	writeJSON(w, r, http.StatusCreated, blocker.ToResponse())
}

// HandleListTeamBlockers lists blockers raised by the manager's direct
// reports. Authorization: route-level RequirePermission (assignments.read);
// the service scopes the result to the caller's direct reports.
func (h *Handler) HandleListTeamBlockers(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListTeamBlockers(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, support.ToBlockerResponses(items))
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func writeAssignmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput), errors.Is(err, support.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrStepNotFound), errors.Is(err, employees.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, ErrAlreadyAssigned), errors.Is(err, ErrInvalidState), errors.Is(err, ErrApprovalRequired):
		writeError(w, r, http.StatusConflict, "INVALID_STATE", err.Error())
	case errors.Is(err, ErrForbidden):
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
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
