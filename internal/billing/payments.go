package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PaymentProvider is the port to a hosted-checkout payment processor
// (Paystack in production, stubbed in tests).
type PaymentProvider interface {
	InitializeTransaction(ctx context.Context, in InitializeTransactionInput) (CheckoutSession, error)
	VerifyTransaction(ctx context.Context, reference string) (VerifiedPayment, error)
}

type RefundProvider interface {
	CreateRefund(ctx context.Context, transactionReference string, amountCents int, currency, reason string) (ProviderRefund, error)
}

type ProviderRefund struct {
	ID     string
	Status string
}

// InitializeTransactionInput starts a hosted checkout for an invoice.
type InitializeTransactionInput struct {
	Email       string
	AmountCents int
	Currency    string
	Reference   string
	CallbackURL string
	Metadata    map[string]string
}

// CheckoutSession is a hosted-checkout redirect target.
type CheckoutSession struct {
	AuthorizationURL string
	Reference        string
}

// VerifiedPayment is the provider-confirmed state of a transaction.
type VerifiedPayment struct {
	Reference   string
	AmountCents int
	Currency    string
	Paid        bool
	PaidAt      time.Time
}

// paystackEvent is the documented Paystack webhook envelope. Only
// charge.success carries a settled payment; other events are acknowledged
// and ignored.
type paystackEvent struct {
	Event string `json:"event"`
	Data  struct {
		Reference string `json:"reference"`
	} `json:"data"`
}

// SetPayments configures the payment provider and webhook secret. Either may
// be absent, in which case the matching feature degrades to ErrNotConfigured
// (the HRIS fail-safe pattern): invoices are still generated and listed.
func (s *Service) SetPayments(provider PaymentProvider, webhookSecret string) {
	s.payments = provider
	s.webhookSecret = strings.TrimSpace(webhookSecret)
}

// ListOrganizationInvoices returns an organization's invoices, newest first.
func (s *Service) ListOrganizationInvoices(ctx context.Context, organizationID string) ([]Invoice, error) {
	items, err := s.repo.ListInvoicesByOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list billing invoices: %w", err)
	}

	return items, nil
}

func (s *Service) ListInvoices(ctx context.Context) ([]Invoice, error) {
	items, err := s.repo.ListInvoices(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all billing invoices: %w", err)
	}
	return items, nil
}

func (s *Service) CreateCoupon(ctx context.Context, in CreateCouponInput) (Coupon, error) {
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if code == "" || in.PercentOffBasisPoints < 0 || in.PercentOffBasisPoints > 10000 ||
		in.AmountOffCents < 0 || in.MaxRedemptions < 0 ||
		(in.PercentOffBasisPoints == 0) == (in.AmountOffCents == 0) ||
		(in.AmountOffCents > 0 && currency == "") {
		return Coupon{}, ErrInvalidInput
	}
	coupon := Coupon{
		Code: code, PercentOffBasisPoints: in.PercentOffBasisPoints,
		AmountOffCents: in.AmountOffCents, Currency: currency,
		MaxRedemptions: in.MaxRedemptions, ExpiresAt: in.ExpiresAt,
		Active: true, CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.CreateCoupon(ctx, coupon); err != nil {
		return Coupon{}, fmt.Errorf("create billing coupon: %w", err)
	}
	return coupon, nil
}

func (s *Service) ListCoupons(ctx context.Context) ([]Coupon, error) {
	items, err := s.repo.ListCoupons(ctx)
	if err != nil {
		return nil, fmt.Errorf("list billing coupons: %w", err)
	}
	return items, nil
}

// AdjustInvoice applies tax and a fixed coupon discount to an open invoice.
func (s *Service) AdjustInvoice(ctx context.Context, invoiceID string, in AdjustInvoiceInput) (Invoice, error) {
	if strings.TrimSpace(invoiceID) == "" || in.TaxRateBasisPoints < 0 ||
		in.TaxRateBasisPoints > 10000 {
		return Invoice{}, ErrInvalidInput
	}
	invoice, err := s.platformInvoice(ctx, invoiceID)
	if err != nil {
		return Invoice{}, err
	}
	if invoice.Status != invoiceStatusOpen {
		return Invoice{}, ErrInvoiceNotPayable
	}
	subtotal := invoice.SubtotalCents
	if subtotal == 0 {
		subtotal = invoice.AmountCents
	}
	discountCents := 0
	var coupon *Coupon
	if code := strings.ToUpper(strings.TrimSpace(in.CouponCode)); code != "" {
		found, err := s.repo.GetCoupon(ctx, code)
		if err != nil {
			return Invoice{}, fmt.Errorf("get billing coupon: %w", err)
		}
		now := time.Now().UTC()
		if !found.Active || (found.ExpiresAt != nil && !found.ExpiresAt.After(now)) ||
			(found.MaxRedemptions > 0 && found.RedemptionCount >= found.MaxRedemptions) ||
			(found.Currency != "" && !strings.EqualFold(found.Currency, invoice.Currency)) {
			return Invoice{}, ErrInvalidInput
		}
		discountCents = found.AmountOffCents
		if found.PercentOffBasisPoints > 0 {
			discountCents = subtotal * found.PercentOffBasisPoints / 10000
		}
		coupon = &found
	}
	if discountCents > subtotal {
		discountCents = subtotal
	}
	taxable := subtotal - discountCents
	invoice.SubtotalCents = subtotal
	invoice.DiscountCents = discountCents
	invoice.CouponCode = strings.ToUpper(strings.TrimSpace(in.CouponCode))
	invoice.TaxCents = taxable * in.TaxRateBasisPoints / 10000
	invoice.AmountCents = taxable + invoice.TaxCents
	if err := s.repo.UpdateInvoice(ctx, invoice); err != nil {
		return Invoice{}, fmt.Errorf("adjust invoice: %w", err)
	}
	if coupon != nil {
		coupon.RedemptionCount++
		if err := s.repo.UpdateCoupon(ctx, *coupon); err != nil {
			return Invoice{}, fmt.Errorf("record coupon redemption: %w", err)
		}
	}
	return invoice, nil
}

// RefundInvoice records an audited full or partial refund against a paid invoice.
func (s *Service) RefundInvoice(ctx context.Context, invoiceID string, amountCents int, reason string) (Invoice, error) {
	invoice, err := s.platformInvoice(ctx, invoiceID)
	if err != nil {
		return Invoice{}, err
	}
	reason = strings.TrimSpace(reason)
	if invoice.Status != invoiceStatusPaid || amountCents <= 0 || amountCents > invoice.AmountCents || reason == "" {
		return Invoice{}, ErrInvalidInput
	}
	refunder, ok := s.payments.(RefundProvider)
	if !ok || invoice.PaymentRef == "" {
		return Invoice{}, ErrNotConfigured
	}
	if _, err := refunder.CreateRefund(ctx, invoice.PaymentRef, amountCents, invoice.Currency, reason); err != nil {
		return Invoice{}, fmt.Errorf("create provider refund: %w", err)
	}
	now := time.Now().UTC()
	invoice.RefundAmountCents = amountCents
	invoice.RefundReason = reason
	invoice.RefundedAt = &now
	if amountCents == invoice.AmountCents {
		invoice.Status = invoiceStatusRefunded
	}
	if err := s.repo.UpdateInvoice(ctx, invoice); err != nil {
		return Invoice{}, fmt.Errorf("record invoice refund: %w", err)
	}
	return invoice, nil
}

// RunDunning advances overdue open invoices and marks their subscriptions past due.
func (s *Service) RunDunning(ctx context.Context) error {
	items, err := s.repo.ListInvoices(ctx)
	if err != nil {
		return fmt.Errorf("list invoices for dunning: %w", err)
	}
	now := time.Now().UTC()
	for _, invoice := range items {
		if invoice.Status != invoiceStatusOpen || invoice.DueAt.After(now) {
			continue
		}
		invoice.DunningAttempts++
		invoice.LastDunningAt = &now
		if invoice.DunningAttempts >= 3 {
			invoice.Status = invoiceStatusUncollectible
		}
		if err := s.repo.UpdateInvoice(ctx, invoice); err != nil {
			return fmt.Errorf("update dunning invoice %q: %w", invoice.ID, err)
		}
		subscription, err := s.repo.GetSubscriptionByOrganization(ctx, invoice.OrganizationID)
		if err == nil && subscription.Status != statusPastDue {
			subscription.Status, subscription.UpdatedAt = statusPastDue, now
			if err := s.repo.UpdateSubscription(ctx, subscription); err != nil {
				return fmt.Errorf("mark subscription past due: %w", err)
			}
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("get subscription for dunning: %w", err)
		}
	}
	return nil
}

func (s *Service) platformInvoice(ctx context.Context, invoiceID string) (Invoice, error) {
	items, err := s.repo.ListInvoices(ctx)
	if err != nil {
		return Invoice{}, fmt.Errorf("list invoices: %w", err)
	}
	for _, invoice := range items {
		if invoice.ID == invoiceID {
			return invoice, nil
		}
	}
	return Invoice{}, ErrNotFound
}

// InitiateInvoicePayment starts hosted checkout for an open invoice and
// returns the redirect target. The reference is deterministic per invoice so
// retries reuse one provider transaction and webhooks stay idempotent.
func (s *Service) InitiateInvoicePayment(
	ctx context.Context,
	organizationID, invoiceID, email string,
) (CheckoutSession, error) {
	if s.payments == nil {
		return CheckoutSession{}, ErrNotConfigured
	}

	invoice, err := s.repo.GetInvoice(ctx, organizationID, invoiceID)
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("get billing invoice: %w", err)
	}

	if invoice.Status != invoiceStatusOpen {
		return CheckoutSession{}, ErrInvoiceNotPayable
	}

	reference := invoice.PaymentRef
	if reference == "" {
		reference = "lp_" + invoice.ID
	}

	session, err := s.payments.InitializeTransaction(ctx, InitializeTransactionInput{
		Email:       strings.TrimSpace(email),
		AmountCents: invoice.AmountCents,
		Currency:    invoice.Currency,
		Reference:   reference,
		Metadata: map[string]string{
			"invoiceId":      invoice.ID,
			"organizationId": invoice.OrganizationID,
		},
	})
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("initialize payment: %w", err)
	}

	invoice.PaymentRef = session.Reference
	if err := s.repo.UpdateInvoice(ctx, invoice); err != nil {
		return CheckoutSession{}, fmt.Errorf("store payment reference: %w", err)
	}

	return session, nil
}

// HandlePaystackWebhook verifies the x-paystack-signature header
// (HMAC-SHA512 of the raw body, constant-time compare) and settles the
// referenced invoice. It is idempotent on the payment reference: replays,
// unknown references, and non-charge.success events are acknowledged without
// state changes.
func (s *Service) HandlePaystackWebhook(ctx context.Context, body []byte, signature string) (WebhookOutcome, error) {
	if s.webhookSecret == "" {
		return WebhookOutcome{}, ErrNotConfigured
	}

	if !validPaystackSignature(body, signature, s.webhookSecret) {
		return WebhookOutcome{}, ErrInvalidSignature
	}

	var event paystackEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return WebhookOutcome{}, fmt.Errorf("decode paystack webhook: %w (%w)", err, ErrInvalidInput)
	}

	if event.Event != "charge.success" || event.Data.Reference == "" {
		return WebhookOutcome{}, nil
	}

	return s.settlePayment(ctx, event.Data.Reference)
}

// settlePayment marks the invoice paid and activates the subscription. When
// a provider is configured the transaction is verified server-side first and
// must match the invoice amount and currency (Paystack's recommended
// defense against spoofed webhooks).
func (s *Service) settlePayment(ctx context.Context, reference string) (WebhookOutcome, error) {
	invoice, err := s.repo.GetInvoiceByPaymentRef(ctx, reference)
	if errors.Is(err, ErrNotFound) {
		return WebhookOutcome{}, nil
	}

	if err != nil {
		return WebhookOutcome{}, fmt.Errorf("get invoice by payment ref: %w", err)
	}

	if invoice.Status == invoiceStatusPaid {
		// Replay of an already-settled reference: acknowledge without
		// reporting a new settlement so downstream audit stays idempotent.
		return WebhookOutcome{}, nil
	}

	paidAt := time.Now().UTC()

	if s.payments != nil {
		verified, err := s.payments.VerifyTransaction(ctx, reference)
		if err != nil {
			return WebhookOutcome{}, fmt.Errorf("verify payment: %w", err)
		}

		if !verified.Paid ||
			verified.AmountCents != invoice.AmountCents ||
			!strings.EqualFold(verified.Currency, invoice.Currency) {
			return WebhookOutcome{}, ErrPaymentMismatch
		}

		if !verified.PaidAt.IsZero() {
			paidAt = verified.PaidAt
		}
	}

	invoice.Status = invoiceStatusPaid
	invoice.PaidAt = &paidAt

	if err := s.repo.UpdateInvoice(ctx, invoice); err != nil {
		return WebhookOutcome{}, fmt.Errorf("mark invoice paid: %w", err)
	}

	if err := s.activateSubscription(ctx, invoice.OrganizationID); err != nil {
		return WebhookOutcome{}, err
	}

	return WebhookOutcome{
		Settled:        true,
		InvoiceID:      invoice.ID,
		OrganizationID: invoice.OrganizationID,
	}, nil
}

func (s *Service) activateSubscription(ctx context.Context, organizationID string) error {
	subscription, err := s.repo.GetSubscriptionByOrganization(ctx, organizationID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("get billing subscription: %w", err)
	}

	if subscription.Status == statusActive {
		return nil
	}

	subscription.Status = statusActive
	subscription.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateSubscription(ctx, subscription); err != nil {
		return fmt.Errorf("activate billing subscription: %w", err)
	}

	return nil
}

// issueInvoice creates an open invoice for one subscription period. Numbers
// come from a per-organization sequence (INV-000001, ...).
func (s *Service) issueInvoice(ctx context.Context, subscription Subscription, plan Plan) (Invoice, error) {
	sequence, err := s.repo.NextInvoiceSequence(ctx, subscription.OrganizationID)
	if err != nil {
		return Invoice{}, fmt.Errorf("next invoice sequence: %w", err)
	}

	now := time.Now().UTC()

	invoice := Invoice{
		ID:             uuid.NewString(),
		OrganizationID: subscription.OrganizationID,
		Number:         fmt.Sprintf("INV-%06d", sequence),
		SubscriptionID: subscription.ID,
		PlanCode:       plan.Code,
		AmountCents:    plan.PriceMonthlyCents,
		SubtotalCents:  plan.PriceMonthlyCents,
		Currency:       plan.Currency,
		Status:         invoiceStatusOpen,
		PeriodStart:    now,
		PeriodEnd:      now.AddDate(0, 1, 0),
		DueAt:          now.AddDate(0, 0, invoiceDueWindowDays),
		CreatedAt:      now,
	}
	if err := s.repo.CreateInvoice(ctx, invoice); err != nil {
		return Invoice{}, fmt.Errorf("create billing invoice: %w", err)
	}

	return invoice, nil
}

// RevenueSummary computes MRR/ARR from active subscriptions at their plan's
// monthly price, broken down per plan.
func (s *Service) RevenueSummary(ctx context.Context) (RevenueSummary, error) {
	plans, err := s.repo.ListPlans(ctx, false)
	if err != nil {
		return RevenueSummary{}, fmt.Errorf("list billing plans: %w", err)
	}

	priceByCode := make(map[string]Plan, len(plans))
	for _, plan := range plans {
		priceByCode[plan.Code] = plan
	}

	subscriptions, err := s.repo.ListSubscriptions(ctx)
	if err != nil {
		return RevenueSummary{}, fmt.Errorf("list billing subscriptions: %w", err)
	}

	perPlan := make(map[string]*PlanRevenue)
	ordered := make([]string, 0)

	summary := RevenueSummary{Plans: make([]PlanRevenue, 0)}

	for _, subscription := range subscriptions {
		if subscription.Status != statusActive {
			continue
		}

		plan, ok := priceByCode[subscription.PlanCode]
		if !ok {
			continue
		}

		revenue, ok := perPlan[plan.Code]
		if !ok {
			revenue = &PlanRevenue{PlanCode: plan.Code, Currency: plan.Currency}
			perPlan[plan.Code] = revenue
			ordered = append(ordered, plan.Code)
		}

		revenue.ActiveSubscriptions++
		revenue.MRRCents += int64(plan.PriceMonthlyCents)

		summary.ActiveSubscriptions++
		summary.MRRTotalCents += int64(plan.PriceMonthlyCents)
	}

	summary.ARRTotalCents = summary.MRRTotalCents * monthsPerYear

	for _, code := range ordered {
		summary.Plans = append(summary.Plans, *perPlan[code])
	}

	return summary, nil
}

// validPaystackSignature compares the hex HMAC-SHA512 of the raw body in
// constant time.
func validPaystackSignature(body []byte, signature, secret string) bool {
	mac := hmac.New(sha512.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)

	provided, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return false
	}

	return hmac.Equal(expected, provided)
}
