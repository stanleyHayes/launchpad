package cms

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes CMS HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
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

	writeJSON(w, r, http.StatusOK, items)
}

// HandlePlatformCreate creates a draft CMS page.
func (h *Handler) HandlePlatformCreate(w http.ResponseWriter, r *http.Request) {
	if !requirePlatform(w, r) {
		return
	}

	var body struct {
		Slug    string `json:"slug"`
		Title   string `json:"title"`
		Summary string `json:"summary"`
		Body    string `json:"body"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	page, err := h.svc.Create(r.Context(), CreateInput{
		Slug:    body.Slug,
		Title:   body.Title,
		Summary: body.Summary,
		Body:    body.Body,
	})
	if err != nil {
		writeCMSError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusCreated, page)
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

	writeJSON(w, r, http.StatusOK, page)
}

// HandlePlatformUpdate updates a draft CMS page.
func (h *Handler) HandlePlatformUpdate(w http.ResponseWriter, r *http.Request) {
	if !requirePlatform(w, r) {
		return
	}

	var body struct {
		Title   *string `json:"title"`
		Summary *string `json:"summary"`
		Body    *string `json:"body"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	page, err := h.svc.Update(r.Context(), chi.URLParam(r, "pageID"), UpdateInput{
		Title:   body.Title,
		Summary: body.Summary,
		Body:    body.Body,
	})
	if err != nil {
		writeCMSError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, page)
}

// HandlePlatformPublish publishes a draft CMS page.
func (h *Handler) HandlePlatformPublish(w http.ResponseWriter, r *http.Request) {
	if !requirePlatform(w, r) {
		return
	}

	page, err := h.svc.Publish(r.Context(), chi.URLParam(r, "pageID"))
	if err != nil {
		writeCMSError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, page)
}

// HandlePublicGetBySlug returns a published page.
func (h *Handler) HandlePublicGetBySlug(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.GetPublishedBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeCMSError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, page)
}

func requirePlatform(w http.ResponseWriter, r *http.Request) bool {
	_, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

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
