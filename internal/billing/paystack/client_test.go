package paystack_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"launchpad/internal/billing"
	"launchpad/internal/billing/paystack"
)

func TestInitializeTransactionPostsAuthorizedRequest(t *testing.T) {
	t.Parallel()

	var (
		gotAuth, gotPath string
		gotBody          struct {
			Email     string `json:"email"`
			Amount    int    `json:"amount"`
			Currency  string `json:"currency"`
			Reference string `json:"reference"`
		}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path

		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status": true,
			"message": "Authorization URL created",
			"data": {
				"authorization_url": "https://checkout.paystack.com/abc",
				"access_code": "abc",
				"reference": "lp_inv-1"
			}
		}`))
	}))
	defer server.Close()

	client := paystack.NewClient("sk_test_123", server.URL, server.Client())

	session, err := client.InitializeTransaction(context.Background(), billing.InitializeTransactionInput{
		Email:       "owner@example.com",
		AmountCents: 9900,
		Currency:    "USD",
		Reference:   "lp_inv-1",
		Metadata:    map[string]string{"invoiceId": "inv-1"},
	})
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	if gotAuth != "Bearer sk_test_123" {
		t.Fatalf("authorization header = %q", gotAuth)
	}

	if gotPath != "/transaction/initialize" {
		t.Fatalf("path = %q", gotPath)
	}

	if gotBody.Email != "owner@example.com" || gotBody.Amount != 9900 ||
		gotBody.Currency != "USD" || gotBody.Reference != "lp_inv-1" {
		t.Fatalf("request body mismatch: %+v", gotBody)
	}

	if session.AuthorizationURL != "https://checkout.paystack.com/abc" || session.Reference != "lp_inv-1" {
		t.Fatalf("unexpected session: %+v", session)
	}
}

func TestInitializeTransactionSurfacesRejectedRequests(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status": false, "message": "Invalid amount"}`))
	}))
	defer server.Close()

	client := paystack.NewClient("sk_test_123", server.URL, server.Client())

	_, err := client.InitializeTransaction(context.Background(), billing.InitializeTransactionInput{})
	if err == nil || !strings.Contains(err.Error(), "paystack rejected the request") {
		t.Fatalf("got %v, want errRequestRejected", err)
	}
}

func TestVerifyTransactionMapsSuccessToPaid(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		_, _ = w.Write([]byte(`{
			"status": true,
			"message": "Verification successful",
			"data": {
				"status": "success",
				"reference": "lp_inv-1",
				"amount": 9900,
				"currency": "USD",
				"paid_at": "2026-07-28T12:00:00.000Z"
			}
		}`))
	}))
	defer server.Close()

	client := paystack.NewClient("sk_test_123", server.URL, server.Client())

	verified, err := client.VerifyTransaction(context.Background(), "lp_inv-1")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if gotPath != "/transaction/verify/lp_inv-1" {
		t.Fatalf("path = %q", gotPath)
	}

	if !verified.Paid || verified.AmountCents != 9900 || verified.Currency != "USD" ||
		verified.PaidAt.IsZero() {
		t.Fatalf("unexpected verification: %+v", verified)
	}
}

func TestVerifyTransactionErrorStatuses(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status": false, "message": "Transaction not found"}`))
	}))
	defer server.Close()

	client := paystack.NewClient("sk_test_123", server.URL, server.Client())

	_, err := client.VerifyTransaction(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "paystack returned an error status") {
		t.Fatalf("got %v, want errUnexpectedStatus", err)
	}
}

func TestCreateRefundPostsProviderReferenceAndAmount(t *testing.T) {
	t.Parallel()
	var gotPath string
	var gotBody struct {
		Transaction  string `json:"transaction"`
		Amount       int    `json:"amount"`
		Currency     string `json:"currency"`
		CustomerNote string `json:"customer_note"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode refund request: %v", err)
		}
		_, _ = w.Write([]byte(`{"status":true,"message":"queued","data":{"id":3018284,"status":"pending"}}`))
	}))
	defer server.Close()
	client := paystack.NewClient("sk_test_123", server.URL, server.Client())
	got, err := client.CreateRefund(t.Context(), "lp_inv-1", 2500, "USD", "duplicate charge")
	if err != nil {
		t.Fatalf("create refund: %v", err)
	}
	if gotPath != "/refund" || gotBody.Transaction != "lp_inv-1" || gotBody.Amount != 2500 ||
		gotBody.Currency != "USD" || gotBody.CustomerNote != "duplicate charge" {
		t.Fatalf("refund request mismatch: path=%q body=%+v", gotPath, gotBody)
	}
	if got.ID != "3018284" || got.Status != "pending" {
		t.Fatalf("unexpected refund result: %+v", got)
	}
}
