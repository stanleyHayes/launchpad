package jobs

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes scheduler health and controlled manual execution to platform operators.
type Handler struct {
	scheduler *Scheduler
	audit     *audit.Service
}

func NewHandler(scheduler *Scheduler, auditSvc *audit.Service) *Handler {
	return &Handler{scheduler: scheduler, audit: auditSvc}
}

func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	if err := httpx.WriteJSON(w, http.StatusOK, h.scheduler.Statuses()); err != nil {
		slog.ErrorContext(r.Context(), "write scheduler statuses", "error", err)
	}
}

func (h *Handler) HandleRun(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		_ = httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	name := chi.URLParam(r, "name")
	if err := h.scheduler.RunNow(r.Context(), name); err != nil {
		_ = httpx.WriteError(w, http.StatusConflict, "SWEEP_NOT_RUN", err.Error())
		return
	}
	if err := h.audit.Record(r.Context(), nil, principal.UserID, "job.run", "scheduled_sweep", name, nil); err != nil {
		slog.ErrorContext(r.Context(), "audit manual sweep", "error", err)
		_ = httpx.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")
		return
	}
	if err := httpx.WriteJSON(w, http.StatusOK, h.scheduler.Statuses()); err != nil {
		slog.ErrorContext(r.Context(), "write scheduler statuses", "error", err)
	}
}
