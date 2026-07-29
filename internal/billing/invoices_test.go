package billing_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"launchpad/internal/billing"
)

// --- shared test doubles ----------------------------------------------------

type stubPaymentProvider struct {
	initializeIn  billing.InitializeTransactionInput
	initializeOut billing.CheckoutSession
	initializeErr error
	verifyOut     billing.VerifiedPayment
	verifyErr     error
	verifyCalls   int
	refundOut     billing.ProviderRefund
	refundErr     error
}

func (s *stubPaymentProvider) InitializeTransaction(
	_ context.Context,
	in billing.InitializeTransactionInput,
) (billing.CheckoutSession, error) {
	s.initializeIn = in

	if s.initializeErr != nil {
		return billing.CheckoutSession{}, s.initializeErr
	}

	if s.initializeOut.Reference == "" {
		s.initializeOut.Reference = in.Reference
	}

	return s.initializeOut, nil
}

func (s *stubPaymentProvider) CreateRefund(
	_ context.Context, _ string, _ int, _, _ string,
) (billing.ProviderRefund, error) {
	return s.refundOut, s.refundErr
}

func (s *stubPaymentProvider) VerifyTransaction(
	_ context.Context,
	_ string,
) (billing.VerifiedPayment, error) {
	s.verifyCalls++

	if s.verifyErr != nil {
		return billing.VerifiedPayment{}, s.verifyErr
	}

	return s.verifyOut, nil
}

func signWebhookBody(t *testing.T, body []byte, secret string) string {
	t.Helper()

	mac := hmac.New(sha512.New, []byte(secret))
	_, _ = mac.Write(body)

	return hex.EncodeToString(mac.Sum(nil))
}

// assignPaidPlan seeds a paid plan, assigns it to org-1, and returns the
// resulting subscription.
func assignPaidPlan(t *testing.T, svc *billing.Service, orgs *stubOrgDirectory) billing.Subscription {
	t.Helper()

	seedPlan(t, svc, "growth", true)

	orgs.orgs["org-1"] = billing.OrganizationSummary{ID: "org-1", PlanCode: "starter", Status: "active"}

	subscription, err := svc.SetOrganizationPlan(context.Background(), billing.SetOrganizationPlanInput{
		OrganizationID: "org-1", PlanCode: "growth",
	})
	if err != nil {
		t.Fatalf("assign plan: %v", err)
	}

	return subscription
}

// --- invoice generation -----------------------------------------------------

func TestSetOrganizationPlanIssuesInvoiceForPaidPlan(t *testing.T) {
	t.Parallel()

	svc, repo, orgs := newBillingService()
	ctx := context.Background()

	subscription := assignPaidPlan(t, svc, orgs)

	invoices, err := svc.ListOrganizationInvoices(ctx, "org-1")
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}

	if len(invoices) != 1 {
		t.Fatalf("expected 1 invoice, got %d", len(invoices))
	}

	invoice := invoices[0]
	if invoice.Number != "INV-000001" {
		t.Fatalf("first invoice number = %q, want INV-000001", invoice.Number)
	}

	if invoice.Status != "open" {
		t.Fatalf("invoice status = %q, want open", invoice.Status)
	}

	if invoice.SubscriptionID != subscription.ID || invoice.PlanCode != "growth" {
		t.Fatalf("invoice not linked to subscription: %+v", invoice)
	}

	if invoice.AmountCents != 100 || invoice.Currency != "USD" {
		t.Fatalf("invoice should carry the plan price: %+v", invoice)
	}

	if !invoice.PeriodEnd.After(invoice.PeriodStart) || !invoice.DueAt.After(invoice.PeriodStart) {
		t.Fatalf("invoice period/due dates incoherent: %+v", invoice)
	}

	// A second paid assignment continues the per-organization sequence.
	if _, err := svc.SetOrganizationPlan(ctx, billing.SetOrganizationPlanInput{
		OrganizationID: "org-1", PlanCode: "growth",
	}); err != nil {
		t.Fatalf("re-assign: %v", err)
	}

	invoices, err = svc.ListOrganizationInvoices(ctx, "org-1")
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}

	if len(invoices) != 2 || repo.invoiceSeq["org-1"] != 2 {
		t.Fatalf("sequence should advance per assignment: %+v (seq %d)", invoices, repo.invoiceSeq["org-1"])
	}
}

func TestSetOrganizationPlanSkipsInvoiceForFreePlan(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newBillingService()
	ctx := context.Background()

	_, err := svc.CreatePlan(ctx, billing.CreatePlanInput{
		Code: "starter", Name: "Starter", PriceMonthlyCents: 0, Active: true,
	})
	if err != nil {
		t.Fatalf("seed free plan: %v", err)
	}

	orgs.orgs["org-1"] = billing.OrganizationSummary{ID: "org-1", PlanCode: "starter", Status: "trial"}

	if _, err := svc.SetOrganizationPlan(ctx, billing.SetOrganizationPlanInput{
		OrganizationID: "org-1", PlanCode: "starter",
	}); err != nil {
		t.Fatalf("assign: %v", err)
	}

	invoices, err := svc.ListOrganizationInvoices(ctx, "org-1")
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}

	if len(invoices) != 0 {
		t.Fatalf("free plans must not generate invoices, got %+v", invoices)
	}
}

func TestListOrganizationInvoicesIsOrgScoped(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newBillingService()
	ctx := context.Background()

	assignPaidPlan(t, svc, orgs)

	invoices, err := svc.ListOrganizationInvoices(ctx, "org-2")
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}

	if len(invoices) != 0 {
		t.Fatalf("invoices leaked across organizations: %+v", invoices)
	}
}

func TestAdjustInvoiceAppliesCouponAndTax(t *testing.T) {
	t.Parallel()
	svc, _, orgs := newBillingService()
	assignPaidPlan(t, svc, orgs)
	if _, err := svc.CreateCoupon(t.Context(), billing.CreateCouponInput{
		Code: "save20", PercentOffBasisPoints: 2000, MaxRedemptions: 1,
	}); err != nil {
		t.Fatalf("create coupon: %v", err)
	}
	invoices, _ := svc.ListOrganizationInvoices(t.Context(), "org-1")
	got, err := svc.AdjustInvoice(t.Context(), invoices[0].ID, billing.AdjustInvoiceInput{
		CouponCode: "save20", TaxRateBasisPoints: 1250,
	})
	if err != nil {
		t.Fatalf("adjust invoice: %v", err)
	}
	if got.SubtotalCents != 100 || got.DiscountCents != 20 || got.TaxCents != 10 ||
		got.AmountCents != 90 || got.CouponCode != "SAVE20" {
		t.Fatalf("unexpected adjusted invoice: %+v", got)
	}
}

func TestRefundInvoiceRecordsPartialAndFullRefund(t *testing.T) {
	t.Parallel()
	svc, repo, orgs := newBillingService()
	assignPaidPlan(t, svc, orgs)
	invoices, _ := svc.ListOrganizationInvoices(t.Context(), "org-1")
	invoice := invoices[0]
	invoice.Status = "paid"
	invoice.PaymentRef = "pay-ref-1"
	repo.invoices[invoice.ID] = invoice
	svc.SetPayments(&stubPaymentProvider{}, "secret")

	partial, err := svc.RefundInvoice(t.Context(), invoice.ID, 25, "service credit")
	if err != nil {
		t.Fatalf("partial refund: %v", err)
	}
	if partial.Status != "paid" || partial.RefundAmountCents != 25 || partial.RefundedAt == nil {
		t.Fatalf("unexpected partial refund: %+v", partial)
	}

	full, err := svc.RefundInvoice(t.Context(), invoice.ID, invoice.AmountCents, "duplicate charge")
	if err != nil {
		t.Fatalf("full refund: %v", err)
	}
	if full.Status != "refunded" {
		t.Fatalf("full refund status = %q", full.Status)
	}
}

func TestDunningAdvancesOverdueInvoicesAndSubscription(t *testing.T) {
	t.Parallel()
	svc, repo, orgs := newBillingService()
	assignPaidPlan(t, svc, orgs)
	invoices, _ := svc.ListOrganizationInvoices(t.Context(), "org-1")
	invoice := invoices[0]
	invoice.DueAt = time.Now().UTC().Add(-time.Hour)
	repo.invoices[invoice.ID] = invoice

	for range 3 {
		if err := svc.RunDunning(t.Context()); err != nil {
			t.Fatalf("run dunning: %v", err)
		}
	}
	got := repo.invoices[invoice.ID]
	if got.Status != "uncollectible" || got.DunningAttempts != 3 || got.LastDunningAt == nil {
		t.Fatalf("unexpected dunning invoice: %+v", got)
	}
	if repo.subs["org-1"].Status != "past_due" {
		t.Fatalf("subscription not marked past due: %+v", repo.subs["org-1"])
	}
}

// --- pay flow ---------------------------------------------------------------

func TestInitiateInvoicePaymentRequiresProvider(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newBillingService()
	subscription := assignPaidPlan(t, svc, orgs)

	invoices, err := svc.ListOrganizationInvoices(context.Background(), subscription.OrganizationID)
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}

	_, err = svc.InitiateInvoicePayment(context.Background(), "org-1", invoices[0].ID, "owner@example.com")
	if !errors.Is(err, billing.ErrNotConfigured) {
		t.Fatalf("got %v, want ErrNotConfigured", err)
	}
}

func TestInitiateInvoicePaymentReturnsCheckoutSession(t *testing.T) {
	t.Parallel()

	svc, repo, orgs := newBillingService()
	ctx := context.Background()

	assignPaidPlan(t, svc, orgs)

	provider := &stubPaymentProvider{
		initializeOut: billing.CheckoutSession{AuthorizationURL: "https://checkout.paystack.com/abc"},
	}
	svc.SetPayments(provider, "wh-secret")

	invoices, err := svc.ListOrganizationInvoices(ctx, "org-1")
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}

	invoice := invoices[0]

	session, err := svc.InitiateInvoicePayment(ctx, "org-1", invoice.ID, "owner@example.com")
	if err != nil {
		t.Fatalf("initiate payment: %v", err)
	}

	if session.AuthorizationURL != "https://checkout.paystack.com/abc" {
		t.Fatalf("unexpected checkout session: %+v", session)
	}

	// The provider saw the invoice amount and a deterministic reference.
	if provider.initializeIn.AmountCents != invoice.AmountCents ||
		provider.initializeIn.Currency != invoice.Currency ||
		provider.initializeIn.Email != "owner@example.com" {
		t.Fatalf("provider input mismatch: %+v", provider.initializeIn)
	}

	wantRef := "lp_" + invoice.ID
	if provider.initializeIn.Reference != wantRef || session.Reference != wantRef {
		t.Fatalf("reference = %q/%q, want %q", provider.initializeIn.Reference, session.Reference, wantRef)
	}

	// The reference is persisted for webhook idempotency and reused on retry.
	stored := repo.invoices[invoice.ID]
	if stored.PaymentRef != wantRef {
		t.Fatalf("payment ref not persisted: %+v", stored)
	}

	if _, err := svc.InitiateInvoicePayment(ctx, "org-1", invoice.ID, "owner@example.com"); err != nil {
		t.Fatalf("retry initiate payment: %v", err)
	}

	if provider.initializeIn.Reference != wantRef {
		t.Fatalf("retry should reuse the stored reference, got %q", provider.initializeIn.Reference)
	}
}

func TestInitiateInvoicePaymentErrorPaths(t *testing.T) {
	t.Parallel()

	svc, repo, orgs := newBillingService()
	ctx := context.Background()

	assignPaidPlan(t, svc, orgs)
	svc.SetPayments(&stubPaymentProvider{}, "wh-secret")

	invoices, err := svc.ListOrganizationInvoices(ctx, "org-1")
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}

	invoice := invoices[0]

	if _, err := svc.InitiateInvoicePayment(ctx, "org-2", invoice.ID, "a@b.c"); !errors.Is(err, billing.ErrNotFound) {
		t.Fatalf("cross-org pay got %v, want ErrNotFound", err)
	}

	paid := repo.invoices[invoice.ID]
	paid.Status = "paid"
	repo.invoices[invoice.ID] = paid

	if _, err := svc.InitiateInvoicePayment(ctx, "org-1", invoice.ID, "a@b.c"); !errors.Is(
		err, billing.ErrInvoiceNotPayable,
	) {
		t.Fatalf("paying a paid invoice got %v, want ErrInvoiceNotPayable", err)
	}
}

// --- webhook ----------------------------------------------------------------

func webhookBody(reference string) []byte {
	return fmt.Appendf(nil, `{"event":"charge.success","data":{"reference":%q}}`, reference)
}

func TestPaystackWebhookRequiresSecret(t *testing.T) {
	t.Parallel()

	svc, _, _ := newBillingService()

	_, err := svc.HandlePaystackWebhook(context.Background(), webhookBody("ref"), "sig")
	if !errors.Is(err, billing.ErrNotConfigured) {
		t.Fatalf("got %v, want ErrNotConfigured", err)
	}
}

func TestPaystackWebhookRejectsBadSignature(t *testing.T) {
	t.Parallel()

	svc, _, _ := newBillingService()
	svc.SetPayments(nil, "wh-secret")

	body := webhookBody("ref")

	if _, err := svc.HandlePaystackWebhook(context.Background(), body, "deadbeef"); !errors.Is(
		err, billing.ErrInvalidSignature,
	) {
		t.Fatalf("got %v, want ErrInvalidSignature", err)
	}

	// A valid signature for a different secret must also fail.
	forged := signWebhookBody(t, body, "other-secret")
	if _, err := svc.HandlePaystackWebhook(context.Background(), body, forged); !errors.Is(
		err, billing.ErrInvalidSignature,
	) {
		t.Fatalf("forged signature got %v, want ErrInvalidSignature", err)
	}
}

func TestPaystackWebhookSettlesInvoiceAndActivatesSubscription(t *testing.T) {
	t.Parallel()

	svc, repo, orgs := newBillingService()
	ctx := context.Background()

	subscription := assignPaidPlan(t, svc, orgs)

	// Past-due subscriptions reactivate on settlement.
	pastDue := repo.subs["org-1"]
	pastDue.Status = "past_due"
	repo.subs["org-1"] = pastDue

	provider := &stubPaymentProvider{}
	svc.SetPayments(provider, "wh-secret")

	invoices, err := svc.ListOrganizationInvoices(ctx, "org-1")
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}

	invoice := invoices[0]

	if _, err := svc.InitiateInvoicePayment(ctx, "org-1", invoice.ID, "a@b.c"); err != nil {
		t.Fatalf("initiate payment: %v", err)
	}

	reference := "lp_" + invoice.ID
	provider.verifyOut = billing.VerifiedPayment{
		Reference:   reference,
		AmountCents: invoice.AmountCents,
		Currency:    invoice.Currency,
		Paid:        true,
		PaidAt:      time.Now().UTC(),
	}

	body := webhookBody(reference)
	signature := signWebhookBody(t, body, "wh-secret")

	outcome, err := svc.HandlePaystackWebhook(ctx, body, signature)
	if err != nil {
		t.Fatalf("webhook: %v", err)
	}

	if !outcome.Settled || outcome.InvoiceID != invoice.ID || outcome.OrganizationID != "org-1" {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}

	settled := repo.invoices[invoice.ID]
	if settled.Status != "paid" || settled.PaidAt == nil {
		t.Fatalf("invoice not settled: %+v", settled)
	}

	if provider.verifyCalls != 1 {
		t.Fatalf("provider verify calls = %d, want 1", provider.verifyCalls)
	}

	if repo.subs["org-1"].Status != "active" {
		t.Fatalf("subscription not activated: %+v", repo.subs["org-1"])
	}

	if repo.subs["org-1"].ID != subscription.ID {
		t.Fatalf("settlement must not replace the subscription: %+v", repo.subs["org-1"])
	}

	// Idempotent replay: no error, no new settlement, PaidAt unchanged.
	replay, err := svc.HandlePaystackWebhook(ctx, body, signature)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if replay.Settled {
		t.Fatalf("replay reported a new settlement: %+v", replay)
	}

	if !repo.invoices[invoice.ID].PaidAt.Equal(*settled.PaidAt) {
		t.Fatalf("replay changed PaidAt: %+v", repo.invoices[invoice.ID])
	}
}

func TestPaystackWebhookIgnoresUnknownAndNonSuccessEvents(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newBillingService()
	ctx := context.Background()

	assignPaidPlan(t, svc, orgs)
	svc.SetPayments(nil, "wh-secret")

	for _, body := range [][]byte{
		webhookBody("unknown-reference"),
		[]byte(`{"event":"charge.failed","data":{"reference":"lp_x"}}`),
		[]byte(`{"event":"charge.success","data":{}}`),
	} {
		outcome, err := svc.HandlePaystackWebhook(ctx, body, signWebhookBody(t, body, "wh-secret"))
		if err != nil {
			t.Fatalf("body %s: got %v, want acknowledged", body, err)
		}

		if outcome.Settled {
			t.Fatalf("body %s: unexpected settlement %+v", body, outcome)
		}
	}
}

func TestPaystackWebhookRejectsAmountMismatch(t *testing.T) {
	t.Parallel()

	svc, repo, orgs := newBillingService()
	ctx := context.Background()

	assignPaidPlan(t, svc, orgs)

	provider := &stubPaymentProvider{}
	svc.SetPayments(provider, "wh-secret")

	invoices, err := svc.ListOrganizationInvoices(ctx, "org-1")
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}

	invoice := invoices[0]

	if _, err := svc.InitiateInvoicePayment(ctx, "org-1", invoice.ID, "a@b.c"); err != nil {
		t.Fatalf("initiate payment: %v", err)
	}

	reference := "lp_" + invoice.ID
	provider.verifyOut = billing.VerifiedPayment{
		Reference:   reference,
		AmountCents: invoice.AmountCents - 1,
		Currency:    invoice.Currency,
		Paid:        true,
	}

	body := webhookBody(reference)

	_, err = svc.HandlePaystackWebhook(ctx, body, signWebhookBody(t, body, "wh-secret"))
	if !errors.Is(err, billing.ErrPaymentMismatch) {
		t.Fatalf("got %v, want ErrPaymentMismatch", err)
	}

	if repo.invoices[invoice.ID].Status != "open" {
		t.Fatalf("mismatched payment must not settle the invoice: %+v", repo.invoices[invoice.ID])
	}
}

// --- revenue summary ----------------------------------------------------------

func TestRevenueSummaryComputesMRRFromActiveSubscriptions(t *testing.T) {
	t.Parallel()

	svc, repo, orgs := newBillingService()
	ctx := context.Background()

	_, err := svc.CreatePlan(ctx, billing.CreatePlanInput{
		Code: "growth", Name: "Growth", PriceMonthlyCents: 9900, Currency: "USD", Active: true,
	})
	if err != nil {
		t.Fatalf("seed growth: %v", err)
	}

	_, err = svc.CreatePlan(ctx, billing.CreatePlanInput{
		Code: "scale", Name: "Scale", PriceMonthlyCents: 24900, Currency: "USD", Active: true,
	})
	if err != nil {
		t.Fatalf("seed scale: %v", err)
	}

	for i, planCode := range []string{"growth", "growth", "scale"} {
		orgID := fmt.Sprintf("org-%d", i)
		orgs.orgs[orgID] = billing.OrganizationSummary{ID: orgID, PlanCode: "starter", Status: "active"}

		if _, err := svc.SetOrganizationPlan(ctx, billing.SetOrganizationPlanInput{
			OrganizationID: orgID, PlanCode: planCode,
		}); err != nil {
			t.Fatalf("assign %s: %v", planCode, err)
		}
	}

	// Trialing and canceled subscriptions do not count toward MRR.
	trial := repo.subs["org-0"]
	trial.Status = "trialing"
	repo.subs["org-0"] = trial

	canceled := repo.subs["org-2"]
	canceled.Status = "canceled"
	repo.subs["org-2"] = canceled

	summary, err := svc.RevenueSummary(ctx)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}

	if summary.ActiveSubscriptions != 1 {
		t.Fatalf("active subscriptions = %d, want 1", summary.ActiveSubscriptions)
	}

	if summary.MRRTotalCents != 9900 {
		t.Fatalf("MRR = %d, want 9900 (one active growth)", summary.MRRTotalCents)
	}

	if summary.ARRTotalCents != 9900*12 {
		t.Fatalf("ARR = %d, want %d", summary.ARRTotalCents, 9900*12)
	}

	if len(summary.Plans) != 1 || summary.Plans[0].PlanCode != "growth" ||
		summary.Plans[0].MRRCents != 9900 || summary.Plans[0].ActiveSubscriptions != 1 {
		t.Fatalf("per-plan breakdown wrong: %+v", summary.Plans)
	}
}

func TestRevenueSummaryEmpty(t *testing.T) {
	t.Parallel()

	svc, _, _ := newBillingService()

	summary, err := svc.RevenueSummary(context.Background())
	if err != nil {
		t.Fatalf("summary: %v", err)
	}

	if summary.MRRTotalCents != 0 || summary.ARRTotalCents != 0 || summary.ActiveSubscriptions != 0 {
		t.Fatalf("empty summary should be zero: %+v", summary)
	}
}
