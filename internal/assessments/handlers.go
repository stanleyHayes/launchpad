package assessments

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes assessment HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs an assessments Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandleList lists the tenant's assessments (answer keys included; manager
// view).
// Authorization: route-level RequirePermission (assessments.manage).
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.List(r.Context(), principal.OrganizationID)
	if err != nil {
		writeAssessmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleGet returns one assessment with its answer keys (manager view).
// Authorization: route-level RequirePermission (assessments.manage).
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	assessment, err := h.svc.Get(r.Context(), principal.OrganizationID, chi.URLParam(r, "assessmentID"))
	if err != nil {
		writeAssessmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, assessment)
}

// HandleCreate registers a new draft assessment.
// Authorization: route-level RequirePermission (assessments.manage).
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Title        string     `json:"title"`
		Description  string     `json:"description"`
		Questions    []Question `json:"questions"`
		PassingScore float64    `json:"passingScore"`
		MaxAttempts  int        `json:"maxAttempts"`
		Randomize    bool       `json:"randomize"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	assessment, err := h.svc.Create(r.Context(), principal.OrganizationID, principal.UserID, CreateAssessmentInput{
		Title:        body.Title,
		Description:  body.Description,
		Questions:    body.Questions,
		PassingScore: body.PassingScore,
		MaxAttempts:  body.MaxAttempts,
		Randomize:    body.Randomize,
	})
	if err != nil {
		writeAssessmentError(w, r, err)

		return
	}

	h.recordAudit(r, principal, "assessment.created", assessment.ID)

	writeJSON(w, r, http.StatusCreated, assessment)
}

// HandleUpdate mutates editable fields of a draft assessment.
// Authorization: route-level RequirePermission (assessments.manage).
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Title        *string     `json:"title"`
		Description  *string     `json:"description"`
		Questions    *[]Question `json:"questions"`
		PassingScore *float64    `json:"passingScore"`
		MaxAttempts  *int        `json:"maxAttempts"`
		Randomize    *bool       `json:"randomize"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	assessment, err := h.svc.Update(
		r.Context(),
		principal.OrganizationID,
		chi.URLParam(r, "assessmentID"),
		UpdateAssessmentInput{
			Title:        body.Title,
			Description:  body.Description,
			Questions:    body.Questions,
			PassingScore: body.PassingScore,
			MaxAttempts:  body.MaxAttempts,
			Randomize:    body.Randomize,
		},
	)
	if err != nil {
		writeAssessmentError(w, r, err)

		return
	}

	h.recordAudit(r, principal, "assessment.updated", assessment.ID)

	writeJSON(w, r, http.StatusOK, assessment)
}

// HandlePublish publishes a draft assessment.
// Authorization: route-level RequirePermission (assessments.manage).
func (h *Handler) HandlePublish(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	assessment, err := h.svc.Publish(r.Context(), principal.OrganizationID, chi.URLParam(r, "assessmentID"))
	if err != nil {
		writeAssessmentError(w, r, err)

		return
	}

	h.recordAudit(r, principal, "assessment.published", assessment.ID)

	writeJSON(w, r, http.StatusOK, assessment)
}

// HandleArchive archives an assessment.
// Authorization: route-level RequirePermission (assessments.manage).
func (h *Handler) HandleArchive(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	assessment, err := h.svc.Archive(r.Context(), principal.OrganizationID, chi.URLParam(r, "assessmentID"))
	if err != nil {
		writeAssessmentError(w, r, err)

		return
	}

	h.recordAudit(r, principal, "assessment.archived", assessment.ID)

	writeJSON(w, r, http.StatusOK, assessment)
}

// HandleListAttempts lists every attempt on an assessment (manager view).
// Authorization: route-level RequirePermission (assessments.manage).
func (h *Handler) HandleListAttempts(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListAttempts(r.Context(), principal.OrganizationID, chi.URLParam(r, "assessmentID"))
	if err != nil {
		writeAssessmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleReviewAttempt finalizes a pending-review attempt with the manager's
// score and note.
// Authorization: route-level RequirePermission (assessments.manage).
func (h *Handler) HandleReviewAttempt(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Score float64 `json:"score"`
		Note  string  `json:"note"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	attempt, err := h.svc.ReviewAttempt(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "assessmentID"),
		chi.URLParam(r, "attemptID"),
		ReviewAttemptInput{Score: body.Score, Note: body.Note},
	)
	if err != nil {
		writeAssessmentError(w, r, err)

		return
	}

	h.recordAudit(r, principal, "assessment_attempt.reviewed", attempt.ID)

	writeJSON(w, r, http.StatusOK, attempt)
}

// HandleTake returns the answer-key-free questions and attempt budget for
// the calling employee.
// Authorization: any authenticated employee of the tenant.
func (h *Handler) HandleTake(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	view, err := h.svc.Take(r.Context(), principal.OrganizationID, principal.UserID, chi.URLParam(r, "assessmentID"))
	if err != nil {
		writeAssessmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, view)
}

// HandleSubmitAttempt grades the calling employee's submission server-side.
// Authorization: any authenticated employee of the tenant.
func (h *Handler) HandleSubmitAttempt(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Answers []Answer `json:"answers"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	attempt, err := h.svc.SubmitAttempt(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "assessmentID"),
		SubmitAttemptInput{Answers: body.Answers},
	)
	if err != nil {
		writeAssessmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusCreated, attempt)
}

// HandleListMyCertificates lists the calling employee's certificates.
// Authorization: any authenticated employee of the tenant.
func (h *Handler) HandleListMyCertificates(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListMyCertificates(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeAssessmentError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

// recordAudit writes a tenant-scoped audit event for an assessment action.
// The state change is already committed, so a failed audit write is logged
// but never fails the request.
func (h *Handler) recordAudit(
	r *http.Request,
	principal security.Principal,
	action, assessmentID string,
) {
	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		action,
		"assessment",
		assessmentID,
		nil,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit assessment action failed", "error", err, "action", action)
	}
}

func writeAssessmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Assessment not found")
	case errors.Is(err, ErrAttemptNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Assessment attempt not found")
	case errors.Is(err, ErrNotPublished):
		writeError(w, r, http.StatusConflict, "NOT_PUBLISHED", "Assessment is not published")
	case errors.Is(err, ErrAttemptsExhausted):
		writeError(w, r, http.StatusConflict, "ATTEMPTS_EXHAUSTED", "No assessment attempts remaining")
	case errors.Is(err, ErrInvalidState):
		writeError(w, r, http.StatusConflict, "INVALID_STATE", err.Error())
	default:
		slog.ErrorContext(r.Context(), "assessment handler error", "error", err)
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
