// Package billing implements billing use cases and HTTP handlers.
package billing

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes billing HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs a billing Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandlePlatformListPlans lists all billing plans.
func (h *Handler) HandlePlatformListPlans(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListPlans(r.Context(), false)
	if err != nil {
		slog.ErrorContext(r.Context(), "list billing plans failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list plans")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandlePlatformCreatePlan creates a billing plan.
func (h *Handler) HandlePlatformCreatePlan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code              string   `json:"code"`
		Name              string   `json:"name"`
		Description       string   `json:"description"`
		PriceMonthlyCents int      `json:"priceMonthlyCents"`
		Currency          string   `json:"currency"`
		Features          []string `json:"features"`
		Active            bool     `json:"active"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	plan, err := h.svc.CreatePlan(r.Context(), CreatePlanInput{
		Code:              body.Code,
		Name:              body.Name,
		Description:       body.Description,
		PriceMonthlyCents: body.PriceMonthlyCents,
		Currency:          body.Currency,
		Features:          body.Features,
		Active:            body.Active,
	})
	if err != nil {
		writeBillingError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusCreated, plan)
}

// HandlePlatformPatchPlan updates a billing plan.
func (h *Handler) HandlePlatformPatchPlan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name              *string   `json:"name"`
		Description       *string   `json:"description"`
		PriceMonthlyCents *int      `json:"priceMonthlyCents"`
		Currency          *string   `json:"currency"`
		Features          *[]string `json:"features"`
		Active            *bool     `json:"active"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	plan, err := h.svc.UpdatePlan(r.Context(), chi.URLParam(r, "code"), UpdatePlanInput{
		Name:              body.Name,
		Description:       body.Description,
		PriceMonthlyCents: body.PriceMonthlyCents,
		Currency:          body.Currency,
		Features:          body.Features,
		Active:            body.Active,
	})
	if err != nil {
		writeBillingError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, plan)
}

// HandlePlatformListSubscriptions lists all subscriptions.
func (h *Handler) HandlePlatformListSubscriptions(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListSubscriptions(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list billing subscriptions failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list subscriptions")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandlePlatformSetOrganizationSubscription assigns a plan to an organization.
func (h *Handler) HandlePlatformSetOrganizationSubscription(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		PlanCode string `json:"planCode"`
		Status   string `json:"status"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	organizationID := chi.URLParam(r, "organizationID")

	subscription, err := h.svc.SetOrganizationPlan(r.Context(), SetOrganizationPlanInput{
		OrganizationID: organizationID,
		PlanCode:       body.PlanCode,
		Status:         body.Status,
	})
	if err != nil {
		writeBillingError(w, r, err)

		return
	}

	if err := h.audit.Record(
		r.Context(),
		&organizationID,
		principal.UserID,
		"subscription.updated",
		"subscription",
		subscription.ID,
		map[string]any{"planCode": subscription.PlanCode, "status": subscription.Status},
	); err != nil {
		slog.ErrorContext(r.Context(), "audit subscription update failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeJSON(w, r, http.StatusOK, subscription)
}

// HandleOrgListPlans lists active billing plans.
func (h *Handler) HandleOrgListPlans(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListPlans(r.Context(), true)
	if err != nil {
		slog.ErrorContext(r.Context(), "list billing plans failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list plans")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleOrgGetSubscription returns the current organization subscription.
func (h *Handler) HandleOrgGetSubscription(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	subscription, err := h.svc.GetOrCreateSubscription(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "get billing subscription failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load subscription")

		return
	}

	writeJSON(w, r, http.StatusOK, subscription)
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func writeBillingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Billing record not found")
	case errors.Is(err, ErrCodeTaken):
		writeError(w, r, http.StatusConflict, "CODE_TAKEN", err.Error())
	default:
		slog.ErrorContext(r.Context(), "billing handler error", "error", err)
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
