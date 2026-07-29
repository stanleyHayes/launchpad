package assignments

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// RuleHandler exposes assignment rule HTTP endpoints.
type RuleHandler struct {
	svc   *RuleService
	audit *audit.Service
}

// NewRuleHandler constructs a RuleHandler.
func NewRuleHandler(svc *RuleService, auditSvc *audit.Service) *RuleHandler {
	return &RuleHandler{svc: svc, audit: auditSvc}
}

// HandleListRules lists assignment rules.
// Authorization: route-level RequirePermission (journeys.assign).
func (h *RuleHandler) HandleListRules(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListRules(r.Context(), principal.OrganizationID)
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleCreateRule creates an assignment rule.
// Authorization: route-level RequirePermission (journeys.assign).
func (h *RuleHandler) HandleCreateRule(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Name              string `json:"name"`
		JourneyTemplateID string `json:"journeyTemplateId"`
		DepartmentID      string `json:"departmentId"`
		JobRoleID         string `json:"jobRoleId"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	rule, err := h.svc.CreateRule(r.Context(), principal.OrganizationID, principal.UserID, CreateRuleInput{
		Name:              body.Name,
		JourneyTemplateID: body.JourneyTemplateID,
		DepartmentID:      body.DepartmentID,
		JobRoleID:         body.JobRoleID,
	})
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	h.recordRuleAudit(r, principal, "assignment_rule.created", rule.ID, map[string]any{
		"journeyTemplateId": rule.JourneyTemplateID,
		"departmentId":      rule.DepartmentID,
		"jobRoleId":         rule.JobRoleID,
	})

	writeJSON(w, r, http.StatusCreated, rule)
}

// HandleUpdateRule replaces a rule's mutable fields.
// Authorization: route-level RequirePermission (journeys.assign).
func (h *RuleHandler) HandleUpdateRule(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Name              string `json:"name"`
		JourneyTemplateID string `json:"journeyTemplateId"`
		DepartmentID      string `json:"departmentId"`
		JobRoleID         string `json:"jobRoleId"`
		Active            bool   `json:"active"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	ruleID := chi.URLParam(r, "ruleID")

	rule, err := h.svc.UpdateRule(r.Context(), principal.OrganizationID, ruleID, UpdateRuleInput{
		Name:              body.Name,
		JourneyTemplateID: body.JourneyTemplateID,
		DepartmentID:      body.DepartmentID,
		JobRoleID:         body.JobRoleID,
		Active:            body.Active,
	})
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	h.recordRuleAudit(r, principal, "assignment_rule.updated", rule.ID, map[string]any{
		"journeyTemplateId": rule.JourneyTemplateID,
		statusActive:        rule.Active,
	})

	writeJSON(w, r, http.StatusOK, rule)
}

// HandleDeleteRule removes a rule. Existing assignments are left untouched.
// Authorization: route-level RequirePermission (journeys.assign).
func (h *RuleHandler) HandleDeleteRule(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	ruleID := chi.URLParam(r, "ruleID")

	if err := h.svc.DeleteRule(r.Context(), principal.OrganizationID, ruleID); err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	h.recordRuleAudit(r, principal, "assignment_rule.deleted", ruleID, nil)

	writeJSON(w, r, http.StatusOK, map[string]string{"status": "deleted"})
}

// HandleRunRule applies a rule to every matching active employee.
// Authorization: route-level RequirePermission (journeys.assign).
func (h *RuleHandler) HandleRunRule(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	ruleID := chi.URLParam(r, "ruleID")

	result, err := h.svc.RunRule(r.Context(), principal.OrganizationID, principal.UserID, ruleID)
	if err != nil {
		writeAssignmentError(w, r, err)

		return
	}

	h.recordRuleAudit(r, principal, "assignment_rule.run", ruleID, map[string]any{
		"employees": result.Employees,
		"assigned":  result.Assigned,
		"skipped":   result.Skipped,
	})

	writeJSON(w, r, http.StatusOK, result)
}

// recordRuleAudit writes an audit event for a privileged rule action. The
// change is already persisted by this point, so a failed audit write is
// logged rather than reported as a request failure.
func (h *RuleHandler) recordRuleAudit(
	r *http.Request,
	principal security.Principal,
	action, ruleID string,
	metadata map[string]any,
) {
	orgID := principal.OrganizationID

	err := h.audit.Record(r.Context(), &orgID, principal.UserID, action, "assignment_rule", ruleID, metadata)
	if err != nil {
		slog.ErrorContext(r.Context(), "audit assignment rule failed", "action", action, "error", err)
	}
}
