package cms

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes CMS HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandlePlatformList lists all CMS pages.
func (h *Handler) HandlePlatformList(w http.ResponseWriter, r *http.Request) {
	if !requirePlatform(w, r) {
		return
	}

	items, err := h.svc.List(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list cms pages failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list pages")

		return
	}

	writeJSON(w, r, http.StatusOK, toPageResponses(items))
}

// HandlePlatformCreate creates a draft CMS page.
func (h *Handler) HandlePlatformCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Slug        string `json:"slug"`
		Title       string `json:"title"`
		Summary     string `json:"summary"`
		Body        string `json:"body"`
		ContentType string `json:"contentType"`
		NavLabel    string `json:"navLabel"`
		NavOrder    int    `json:"navOrder"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	page, err := h.svc.Create(r.Context(), CreateInput{
		Slug:        body.Slug,
		Title:       body.Title,
		Summary:     body.Summary,
		Body:        body.Body,
		ContentType: body.ContentType,
		NavLabel:    body.NavLabel,
		NavOrder:    body.NavOrder,
	})
	if err != nil {
		writeCMSError(w, r, err)

		return
	}

	if !h.recordAudit(w, r, principal, "cms_page.created", page.ID) {
		return
	}

	writeJSON(w, r, http.StatusCreated, page.ToResponse())
}

// HandlePlatformGet returns one CMS page.
func (h *Handler) HandlePlatformGet(w http.ResponseWriter, r *http.Request) {
	if !requirePlatform(w, r) {
		return
	}

	page, err := h.svc.Get(r.Context(), chi.URLParam(r, "pageID"))
	if err != nil {
		writeCMSError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, page.ToResponse())
}

// HandlePlatformUpdate updates a draft CMS page.
func (h *Handler) HandlePlatformUpdate(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Title       *string `json:"title"`
		Summary     *string `json:"summary"`
		Body        *string `json:"body"`
		ContentType *string `json:"contentType"`
		NavLabel    *string `json:"navLabel"`
		NavOrder    *int    `json:"navOrder"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	page, err := h.svc.Update(r.Context(), chi.URLParam(r, "pageID"), UpdateInput{
		Title:       body.Title,
		Summary:     body.Summary,
		Body:        body.Body,
		ContentType: body.ContentType,
		NavLabel:    body.NavLabel,
		NavOrder:    body.NavOrder,
	})
	if err != nil {
		writeCMSError(w, r, err)

		return
	}

	if !h.recordAudit(w, r, principal, "cms_page.updated", page.ID) {
		return
	}

	writeJSON(w, r, http.StatusOK, page.ToResponse())
}

func (h *Handler) HandlePlatformSchedule(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	var body struct {
		PublishAt time.Time `json:"publishAt"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
		return
	}
	page, err := h.svc.Schedule(r.Context(), chi.URLParam(r, "pageID"), body.PublishAt)
	if err != nil {
		writeCMSError(w, r, err)
		return
	}
	if !h.recordAudit(w, r, principal, "cms_page.scheduled", page.ID) {
		return
	}
	writeJSON(w, r, http.StatusOK, page.ToResponse())
}

// HandlePlatformPublish publishes a draft CMS page.
func (h *Handler) HandlePlatformPublish(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	page, err := h.svc.Publish(r.Context(), chi.URLParam(r, "pageID"))
	if err != nil {
		writeCMSError(w, r, err)

		return
	}

	if !h.recordAudit(w, r, principal, "cms_page.published", page.ID) {
		return
	}

	writeJSON(w, r, http.StatusOK, page.ToResponse())
}

// HandlePublicGetBySlug returns a published page.
func (h *Handler) HandlePublicGetBySlug(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.GetPublishedBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeCMSError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, page.ToResponse())
}

func (h *Handler) HandlePublicNavigation(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Navigation(r.Context())
	if err != nil {
		writeCMSError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, toPageResponses(items))
}

func requirePlatform(w http.ResponseWriter, r *http.Request) bool {
	_, ok := requirePrincipal(w, r)

	return ok
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

// recordAudit records a platform audit event for a CMS page change.
// CMS pages are global marketing content, so the audit event carries no organization.
func (h *Handler) recordAudit(
	w http.ResponseWriter,
	r *http.Request,
	principal security.Principal,
	action, pageID string,
) bool {
	if err := h.audit.Record(
		r.Context(),
		nil,
		principal.UserID,
		action,
		"cms_page",
		pageID,
		nil,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit cms page action failed", "error", err, "action", action)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return false
	}

	return true
}

func writeCMSError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, ErrSlugTaken), errors.Is(err, ErrNotDraft):
		writeError(w, r, http.StatusConflict, "INVALID_STATE", err.Error())
	default:
		slog.ErrorContext(r.Context(), "cms handler error", "error", err)
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
