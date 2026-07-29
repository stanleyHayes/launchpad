// Package billing implements billing use cases and HTTP handlers.
package billing

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// maxWebhookBodyBytes bounds the Paystack webhook payload.
const maxWebhookBodyBytes = 1 << 20

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

	writeJSON(w, r, http.StatusOK, toPlanResponses(items))
}

// HandlePlatformCreatePlan creates a billing plan.
func (h *Handler) HandlePlatformCreatePlan(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

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

	if !h.recordPlanAudit(w, r, principal, "billing_plan.created", plan.Code) {
		return
	}

	writeJSON(w, r, http.StatusCreated, plan.ToResponse())
}

// HandlePlatformPatchPlan updates a billing plan.
func (h *Handler) HandlePlatformPatchPlan(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

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

	if !h.recordPlanAudit(w, r, principal, "billing_plan.updated", plan.Code) {
		return
	}

	writeJSON(w, r, http.StatusOK, plan.ToResponse())
}

// HandlePlatformListSubscriptions lists all subscriptions.
func (h *Handler) HandlePlatformListSubscriptions(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListSubscriptions(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list billing subscriptions failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list subscriptions")

		return
	}

	writeJSON(w, r, http.StatusOK, toSubscriptionResponses(items))
}

func (h *Handler) HandlePlatformListInvoices(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListInvoices(r.Context())
	if err != nil {
		writeBillingError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, toInvoiceResponses(items))
}

func (h *Handler) HandlePlatformAdjustInvoice(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	var body struct {
		TaxRateBasisPoints int    `json:"taxRateBasisPoints"`
		CouponCode         string `json:"couponCode"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
		return
	}
	invoice, err := h.svc.AdjustInvoice(r.Context(), chi.URLParam(r, "invoiceID"), AdjustInvoiceInput{
		TaxRateBasisPoints: body.TaxRateBasisPoints, CouponCode: body.CouponCode,
	})
	if err != nil {
		writeBillingError(w, r, err)
		return
	}
	if !h.recordInvoiceAudit(w, r, principal, "invoice.adjusted", invoice) {
		return
	}
	writeJSON(w, r, http.StatusOK, invoice.ToResponse())
}

func (h *Handler) HandlePlatformListCoupons(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListCoupons(r.Context())
	if err != nil {
		writeBillingError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, items)
}

func (h *Handler) HandlePlatformCreateCoupon(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	var body struct {
		Code                  string     `json:"code"`
		PercentOffBasisPoints int        `json:"percentOffBasisPoints"`
		AmountOffCents        int        `json:"amountOffCents"`
		Currency              string     `json:"currency"`
		MaxRedemptions        int        `json:"maxRedemptions"`
		ExpiresAt             *time.Time `json:"expiresAt"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
		return
	}
	coupon, err := h.svc.CreateCoupon(r.Context(), CreateCouponInput{
		Code: body.Code, PercentOffBasisPoints: body.PercentOffBasisPoints,
		AmountOffCents: body.AmountOffCents, Currency: body.Currency,
		MaxRedemptions: body.MaxRedemptions, ExpiresAt: body.ExpiresAt,
	})
	if err != nil {
		writeBillingError(w, r, err)
		return
	}
	if err := h.audit.Record(r.Context(), nil, principal.UserID, "billing_coupon.created", "billing_coupon", coupon.Code, nil); err != nil {
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")
		return
	}
	writeJSON(w, r, http.StatusCreated, coupon)
}

func (h *Handler) HandlePlatformRefundInvoice(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	var body struct {
		AmountCents int    `json:"amountCents"`
		Reason      string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
		return
	}
	invoice, err := h.svc.RefundInvoice(r.Context(), chi.URLParam(r, "invoiceID"), body.AmountCents, body.Reason)
	if err != nil {
		writeBillingError(w, r, err)
		return
	}
	if !h.recordInvoiceAudit(w, r, principal, "invoice.refunded", invoice) {
		return
	}
	writeJSON(w, r, http.StatusOK, invoice.ToResponse())
}

func (h *Handler) recordInvoiceAudit(w http.ResponseWriter, r *http.Request, principal security.Principal, action string, invoice Invoice) bool {
	if err := h.audit.Record(r.Context(), &invoice.OrganizationID, principal.UserID, action, "billing_invoice", invoice.ID, nil); err != nil {
		slog.ErrorContext(r.Context(), "audit invoice operation failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")
		return false
	}
	return true
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

	writeJSON(w, r, http.StatusOK, subscription.ToResponse())
}

// HandleOrgListPlans lists active billing plans.
func (h *Handler) HandleOrgListPlans(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListPlans(r.Context(), true)
	if err != nil {
		slog.ErrorContext(r.Context(), "list billing plans failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list plans")

		return
	}

	writeJSON(w, r, http.StatusOK, toPlanResponses(items))
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

	writeJSON(w, r, http.StatusOK, subscription.ToResponse())
}

// HandleOrgSetSubscription changes the current organization's plan.
func (h *Handler) HandleOrgSetSubscription(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		PlanCode string `json:"planCode"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	subscription, err := h.svc.SetOrganizationPlan(r.Context(), SetOrganizationPlanInput{
		OrganizationID: principal.OrganizationID,
		PlanCode:       body.PlanCode,
	})
	if err != nil {
		writeBillingError(w, r, err)

		return
	}

	if err := h.audit.Record(
		r.Context(),
		&principal.OrganizationID,
		principal.UserID,
		"subscription.updated",
		"subscription",
		subscription.ID,
		map[string]any{"planCode": subscription.PlanCode, "status": subscription.Status},
	); err != nil {
		slog.ErrorContext(r.Context(), "audit organization subscription update failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeJSON(w, r, http.StatusOK, subscription.ToResponse())
}

// HandleOrgListInvoices lists the current organization's invoices.
func (h *Handler) HandleOrgListInvoices(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListOrganizationInvoices(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list billing invoices failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list invoices")

		return
	}

	writeJSON(w, r, http.StatusOK, toInvoiceResponses(items))
}

// HandleOrgPayInvoice initializes hosted checkout for an open invoice.
func (h *Handler) HandleOrgPayInvoice(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	invoiceID := chi.URLParam(r, "invoiceID")

	session, err := h.svc.InitiateInvoicePayment(
		r.Context(),
		principal.OrganizationID,
		invoiceID,
		principal.Email,
	)
	if err != nil {
		writeBillingError(w, r, err)

		return
	}

	if err := h.audit.Record(
		r.Context(),
		&principal.OrganizationID,
		principal.UserID,
		"invoice.payment_initiated",
		"billing_invoice",
		invoiceID,
		map[string]any{"reference": session.Reference},
	); err != nil {
		slog.ErrorContext(r.Context(), "audit invoice payment failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeJSON(w, r, http.StatusOK, CheckoutSessionResponse{
		CheckoutURL: session.AuthorizationURL,
		Reference:   session.Reference,
	})
}

// HandlePlatformBillingSummary reports MRR/ARR from active subscriptions.
func (h *Handler) HandlePlatformBillingSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.RevenueSummary(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "billing revenue summary failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to compute billing summary")

		return
	}

	writeJSON(w, r, http.StatusOK, summary)
}

// HandlePaystackWebhook receives Paystack events on a public route. The
// x-paystack-signature header is the only credential; every outcome that is
// not a signature/config/parse failure returns 200 so Paystack stops
// retrying acknowledged events.
func (h *Handler) HandlePaystackWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodyBytes))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_BODY", "Request body is unreadable")

		return
	}

	outcome, err := h.svc.HandlePaystackWebhook(r.Context(), body, r.Header.Get("X-Paystack-Signature"))
	if err != nil {
		writeBillingError(w, r, err)

		return
	}

	if outcome.Settled {
		// Best-effort: a broken audit store must not make Paystack retry a
		// settled payment.
		organizationID := outcome.OrganizationID
		if err := h.audit.Record(
			r.Context(),
			&organizationID,
			"paystack-webhook",
			"invoice.paid",
			"billing_invoice",
			outcome.InvoiceID,
			nil,
		); err != nil {
			slog.ErrorContext(r.Context(), "audit invoice payment settlement failed", "error", err)
		}
	}

	writeJSON(w, r, http.StatusOK, map[string]bool{"received": true})
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

// recordPlanAudit records a platform audit event for a global billing plan change.
// Plans are not tenant-scoped, so the audit event carries no organization.
func (h *Handler) recordPlanAudit(
	w http.ResponseWriter,
	r *http.Request,
	principal security.Principal,
	action, planCode string,
) bool {
	if err := h.audit.Record(
		r.Context(),
		nil,
		principal.UserID,
		action,
		"billing_plan",
		planCode,
		nil,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit billing plan action failed", "error", err, "action", action)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return false
	}

	return true
}

func writeBillingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Billing record not found")
	case errors.Is(err, ErrCodeTaken):
		writeError(w, r, http.StatusConflict, "CODE_TAKEN", err.Error())
	case errors.Is(err, ErrNotConfigured):
		writeError(w, r, http.StatusNotFound, "PAYMENTS_NOT_CONFIGURED", "Payment provider is not configured")
	case errors.Is(err, ErrInvalidSignature):
		writeError(w, r, http.StatusUnauthorized, "INVALID_SIGNATURE", "Webhook signature verification failed")
	case errors.Is(err, ErrInvoiceNotPayable):
		writeError(w, r, http.StatusConflict, "INVOICE_NOT_PAYABLE", "Invoice is not open for payment")
	case errors.Is(err, ErrPaymentMismatch):
		writeError(w, r, http.StatusConflict, "PAYMENT_MISMATCH", "Verified payment does not match the invoice")
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
