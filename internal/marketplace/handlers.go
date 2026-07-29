package marketplace

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

type Handler struct {
	svc   *Service
	audit *audit.Service
}

func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

func (h *Handler) HandlePublicList(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListPublished(r.Context())
	h.write(w, r, items, err, http.StatusOK)
}

func (h *Handler) HandlePlatformList(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListAll(r.Context())
	h.write(w, r, items, err, http.StatusOK)
}

func (h *Handler) HandlePlatformCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := principal(r)
	if !ok {
		h.unauthorized(w, r)
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Steps       []Step `json:"steps"`
	}
	if httpx.DecodeJSON(r, &body) != nil {
		h.invalidJSON(w, r)
		return
	}
	item, err := h.svc.Create(r.Context(), CreateInput{
		Name: body.Name, Description: body.Description, Category: body.Category,
		Official: true, Steps: body.Steps, CreatedBy: principal.UserID,
	})
	if err == nil {
		h.record(r, principal, nil, "marketplace_template.created", item.ID)
	}
	h.write(w, r, item, err, http.StatusCreated)
}

func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	principal, ok := principal(r)
	if !ok {
		h.unauthorized(w, r)
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Steps       []Step `json:"steps"`
	}
	if httpx.DecodeJSON(r, &body) != nil {
		h.invalidJSON(w, r)
		return
	}
	item, err := h.svc.Create(r.Context(), CreateInput{
		Name: body.Name, Description: body.Description, Category: body.Category,
		Official: false, SubmittedByOrganizationID: principal.OrganizationID,
		Steps: body.Steps, CreatedBy: principal.UserID,
	})
	if err == nil {
		orgID := principal.OrganizationID
		h.record(r, principal, &orgID, "marketplace_template.submitted", item.ID)
	}
	h.write(w, r, item, err, http.StatusCreated)
}

func (h *Handler) HandlePublish(w http.ResponseWriter, r *http.Request) {
	h.handleStatus(w, r, StatusPublished, "marketplace_template.published")
}
func (h *Handler) HandleRemove(w http.ResponseWriter, r *http.Request) {
	h.handleStatus(w, r, StatusRemoved, "marketplace_template.removed")
}
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request, status, action string) {
	principal, ok := principal(r)
	if !ok {
		h.unauthorized(w, r)
		return
	}
	item, err := h.svc.SetStatus(r.Context(), chi.URLParam(r, "templateID"), status)
	if err == nil {
		h.record(r, principal, nil, action, item.ID)
	}
	h.write(w, r, item, err, http.StatusOK)
}

func (h *Handler) HandleFeature(w http.ResponseWriter, r *http.Request) {
	principal, ok := principal(r)
	if !ok {
		h.unauthorized(w, r)
		return
	}
	var body struct {
		Featured bool `json:"featured"`
	}
	if httpx.DecodeJSON(r, &body) != nil {
		h.invalidJSON(w, r)
		return
	}
	item, err := h.svc.SetFeatured(r.Context(), chi.URLParam(r, "templateID"), body.Featured)
	if err == nil {
		h.record(r, principal, nil, "marketplace_template.featured", item.ID)
	}
	h.write(w, r, item, err, http.StatusOK)
}

func (h *Handler) HandleVersion(w http.ResponseWriter, r *http.Request) {
	principal, ok := principal(r)
	if !ok {
		h.unauthorized(w, r)
		return
	}
	var body struct {
		Steps []Step `json:"steps"`
	}
	if httpx.DecodeJSON(r, &body) != nil {
		h.invalidJSON(w, r)
		return
	}
	item, err := h.svc.NewVersion(r.Context(), chi.URLParam(r, "templateID"), body.Steps)
	if err == nil {
		h.record(r, principal, nil, "marketplace_template.versioned", item.ID)
	}
	h.write(w, r, item, err, http.StatusOK)
}

func (h *Handler) HandleInstall(w http.ResponseWriter, r *http.Request) {
	principal, ok := principal(r)
	if !ok {
		h.unauthorized(w, r)
		return
	}
	item, err := h.svc.Install(r.Context(), chi.URLParam(r, "templateID"), principal.OrganizationID, principal.UserID)
	if err == nil {
		orgID := principal.OrganizationID
		h.record(r, principal, &orgID, "marketplace_template.installed", item.TemplateID)
	}
	h.write(w, r, item, err, http.StatusCreated)
}

func (h *Handler) HandleRate(w http.ResponseWriter, r *http.Request) {
	principal, ok := principal(r)
	if !ok {
		h.unauthorized(w, r)
		return
	}
	var body struct {
		Score int `json:"score"`
	}
	if httpx.DecodeJSON(r, &body) != nil {
		h.invalidJSON(w, r)
		return
	}
	item, err := h.svc.Rate(r.Context(), chi.URLParam(r, "templateID"), principal.OrganizationID, principal.UserID, body.Score)
	if err == nil {
		orgID := principal.OrganizationID
		h.record(r, principal, &orgID, "marketplace_template.rated", item.ID)
	}
	h.write(w, r, item, err, http.StatusOK)
}

func principal(r *http.Request) (security.Principal, bool) {
	return security.PrincipalFromContext(r.Context())
}
func (h *Handler) record(r *http.Request, p security.Principal, orgID *string, action, id string) {
	if err := h.audit.Record(r.Context(), orgID, p.UserID, action, "marketplace_template", id, nil); err != nil {
		slog.ErrorContext(r.Context(), "audit marketplace action", "error", err)
	}
}
func (h *Handler) write(w http.ResponseWriter, r *http.Request, data any, err error, status int) {
	if err == nil {
		_ = httpx.WriteJSON(w, status, data)
		return
	}
	switch {
	case errors.Is(err, ErrInvalidInput):
		_ = httpx.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		_ = httpx.WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, ErrInvalidState):
		_ = httpx.WriteError(w, http.StatusConflict, "INVALID_STATE", err.Error())
	default:
		slog.ErrorContext(r.Context(), "marketplace handler", "error", err)
		_ = httpx.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected error")
	}
}
func (h *Handler) invalidJSON(w http.ResponseWriter, _ *http.Request) {
	_ = httpx.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
}
func (h *Handler) unauthorized(w http.ResponseWriter, _ *http.Request) {
	_ = httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
}
