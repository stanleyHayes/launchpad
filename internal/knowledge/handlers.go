package knowledge

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/entitlements"
	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes knowledge document HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs a knowledge Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandleList lists knowledge documents visible to the caller.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.List(
		r.Context(),
		principal.OrganizationID,
		organizations.CanManageOrganization(principal.RoleCode),
	)
	if err != nil {
		writeKnowledgeError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleGet returns one knowledge document.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	doc, err := h.svc.Get(
		r.Context(),
		principal.OrganizationID,
		chi.URLParam(r, "documentID"),
		organizations.CanManageOrganization(principal.RoleCode),
	)
	if err != nil {
		writeKnowledgeError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, doc)
}

// HandleCreate registers a new draft knowledge document.
// Authorization: route-level RequirePermission (knowledge.manage).
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Title         string     `json:"title"`
		Source        string     `json:"source"`
		URI           string     `json:"uri"`
		Body          string     `json:"body"`
		Tags          []string   `json:"tags"`
		AccessScope   string     `json:"accessScope"`
		OwnerUserID   string     `json:"ownerUserId"`
		ReviewDate    *time.Time `json:"reviewDate"`
		RetentionDays int        `json:"retentionDays"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	doc, err := h.svc.Create(r.Context(), principal.OrganizationID, principal.UserID, CreateInput{
		Title:         body.Title,
		Source:        body.Source,
		URI:           body.URI,
		Body:          body.Body,
		Tags:          body.Tags,
		AccessScope:   body.AccessScope,
		OwnerUserID:   body.OwnerUserID,
		ReviewDate:    body.ReviewDate,
		RetentionDays: body.RetentionDays,
	})
	if err != nil {
		writeKnowledgeError(w, r, err)

		return
	}

	h.recordAudit(r, principal, "knowledge_document.created", doc.ID)

	writeJSON(w, r, http.StatusCreated, doc)
}

// HandleUpdate mutates editable fields of a draft document.
// Authorization: route-level RequirePermission (knowledge.manage).
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Title         *string    `json:"title"`
		URI           *string    `json:"uri"`
		Body          *string    `json:"body"`
		Tags          *[]string  `json:"tags"`
		AccessScope   *string    `json:"accessScope"`
		OwnerUserID   *string    `json:"ownerUserId"`
		ReviewDate    *time.Time `json:"reviewDate"`
		RetentionDays *int       `json:"retentionDays"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	doc, err := h.svc.Update(r.Context(), principal.OrganizationID, chi.URLParam(r, "documentID"), UpdateInput{
		Title:         body.Title,
		URI:           body.URI,
		Body:          body.Body,
		Tags:          body.Tags,
		AccessScope:   body.AccessScope,
		OwnerUserID:   body.OwnerUserID,
		ReviewDate:    body.ReviewDate,
		RetentionDays: body.RetentionDays,
	})
	if err != nil {
		writeKnowledgeError(w, r, err)

		return
	}

	h.recordAudit(r, principal, "knowledge_document.updated", doc.ID)

	writeJSON(w, r, http.StatusOK, doc)
}

// HandleHistory returns immutable snapshots plus the current document version.
func (h *Handler) HandleHistory(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	items, err := h.svc.History(r.Context(), principal.OrganizationID, chi.URLParam(r, "documentID"))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, items)
}

// HandleCreateNewVersion reopens a published document as a new draft.
func (h *Handler) HandleCreateNewVersion(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	doc, err := h.svc.CreateNewVersion(r.Context(), principal.OrganizationID, chi.URLParam(r, "documentID"))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	h.recordAudit(r, principal, "knowledge_document.version_created", doc.ID)
	writeJSON(w, r, http.StatusOK, doc)
}

// HandleSync refreshes a connector-backed document and returns changed
// content to draft for human review.
func (h *Handler) HandleSync(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	doc, err := h.svc.Sync(r.Context(), principal.OrganizationID, chi.URLParam(r, "documentID"))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	h.recordAudit(r, principal, "knowledge_document.synced", doc.ID)
	writeJSON(w, r, http.StatusOK, doc)
}

// HandleApprove approves a draft document for AI indexing.
// Authorization: route-level RequirePermission (knowledge.manage).
func (h *Handler) HandleApprove(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	doc, err := h.svc.Approve(
		r.Context(),
		principal.OrganizationID,
		chi.URLParam(r, "documentID"),
		principal.UserID,
	)
	if err != nil {
		writeKnowledgeError(w, r, err)

		return
	}

	h.recordAudit(r, principal, "knowledge_document.approved", doc.ID)

	writeJSON(w, r, http.StatusOK, doc)
}

// HandleIndex hands an approved document to the AI indexer.
// Authorization: route-level RequirePermission (knowledge.manage).
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	doc, err := h.svc.Index(r.Context(), principal.OrganizationID, chi.URLParam(r, "documentID"))
	if err != nil {
		writeKnowledgeError(w, r, err)

		return
	}

	h.recordAudit(r, principal, "knowledge_document.indexed", doc.ID)

	writeJSON(w, r, http.StatusOK, doc)
}

// HandleArchive withdraws a document from the AI index.
// Authorization: route-level RequirePermission (knowledge.manage).
func (h *Handler) HandleArchive(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	doc, err := h.svc.Archive(r.Context(), principal.OrganizationID, chi.URLParam(r, "documentID"))
	if err != nil {
		writeKnowledgeError(w, r, err)

		return
	}

	h.recordAudit(r, principal, "knowledge_document.archived", doc.ID)

	writeJSON(w, r, http.StatusOK, doc)
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

// recordAudit writes a tenant-scoped audit event for a knowledge document
// action. The state change is already committed, so a failed audit write is
// logged but never fails the request.
func (h *Handler) recordAudit(
	r *http.Request,
	principal security.Principal,
	action, documentID string,
) {
	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		action,
		"knowledge_document",
		documentID,
		nil,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit knowledge action failed", "error", err, "action", action)
	}
}

func writeKnowledgeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, entitlements.ErrLimitExceeded):
		writeError(w, r, http.StatusConflict, "PLAN_LIMIT_EXCEEDED", err.Error())
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Knowledge document not found")
	case errors.Is(err, ErrInvalidState):
		writeError(w, r, http.StatusConflict, "INVALID_STATE", err.Error())
	default:
		slog.ErrorContext(r.Context(), "knowledge handler error", "error", err)
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
