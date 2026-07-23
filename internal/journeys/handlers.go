package journeys

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes journey HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandleList lists journey templates.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListTemplates(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list journeys failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list journeys")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleCreate creates a draft journey.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireManager(w, r)
	if !ok {
		return
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	template, err := h.svc.CreateTemplate(r.Context(), principal.OrganizationID, CreateTemplateInput{
		Name:        body.Name,
		Description: body.Description,
		CreatedBy:   principal.UserID,
	})
	if err != nil {
		writeJourneyError(w, r, err)

		return
	}

	if err := h.recordAudit(w, r, principal, "journey.created", "journey_template", template.ID, map[string]any{
		"name": template.Name,
	}); err != nil {
		return
	}

	writeJSON(w, r, http.StatusCreated, template)
}

// HandleGet returns a journey template.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	template, err := h.svc.GetTemplate(r.Context(), principal.OrganizationID, chi.URLParam(r, "journeyID"))
	if err != nil {
		writeJourneyError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, template)
}

// HandleListSteps lists steps for a journey.
func (h *Handler) HandleListSteps(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListSteps(r.Context(), principal.OrganizationID, chi.URLParam(r, "journeyID"))
	if err != nil {
		writeJourneyError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleAddStep adds a step to a draft journey.
func (h *Handler) HandleAddStep(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireManager(w, r)
	if !ok {
		return
	}

	var body struct {
		StepType      string         `json:"stepType"`
		Title         string         `json:"title"`
		Instructions  string         `json:"instructions"`
		DueOffsetDays int            `json:"dueOffsetDays"`
		Config        map[string]any `json:"config"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	step, err := h.svc.AddStep(r.Context(), principal.OrganizationID, chi.URLParam(r, "journeyID"), AddStepInput{
		StepType:      body.StepType,
		Title:         body.Title,
		Instructions:  body.Instructions,
		DueOffsetDays: body.DueOffsetDays,
		Config:        body.Config,
	})
	if err != nil {
		writeJourneyError(w, r, err)

		return
	}

	if err := h.recordAudit(w, r, principal, "journey.step_added", "journey_step", step.ID, map[string]any{
		"title":    step.Title,
		"stepType": step.StepType,
	}); err != nil {
		return
	}

	writeJSON(w, r, http.StatusCreated, step)
}

// HandlePublish publishes a draft journey.
func (h *Handler) HandlePublish(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireManager(w, r)
	if !ok {
		return
	}

	template, err := h.svc.Publish(r.Context(), principal.OrganizationID, chi.URLParam(r, "journeyID"))
	if err != nil {
		writeJourneyError(w, r, err)

		return
	}

	if err := h.recordAudit(w, r, principal, "journey.published", "journey_template", template.ID, nil); err != nil {
		return
	}

	writeJSON(w, r, http.StatusOK, template)
}

func (h *Handler) recordAudit(
	w http.ResponseWriter,
	r *http.Request,
	principal security.Principal,
	action, resourceType, resourceID string,
	metadata map[string]any,
) error {
	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		action,
		resourceType,
		resourceID,
		metadata,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit journey action failed", "error", err, "action", action)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return fmt.Errorf("record audit event: %w", err)
	}

	return nil
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

func writeJourneyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrStepNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, ErrNotDraft), errors.Is(err, ErrNotPublished), errors.Is(err, ErrNoSteps):
		writeError(w, r, http.StatusConflict, "INVALID_STATE", err.Error())
	default:
		slog.ErrorContext(r.Context(), "journey handler error", "error", err)
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
