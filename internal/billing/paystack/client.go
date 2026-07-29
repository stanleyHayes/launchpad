// Package paystack is a billing.PaymentProvider adapter for Paystack's
// transaction API: hosted checkout via /transaction/initialize and
// server-side confirmation via /transaction/verify. It implements
// billing.PaymentProvider over stdlib net/http.
package paystack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"launchpad/internal/billing"
	"launchpad/pkg/safehttp"
)

const (
	defaultBaseURL = "https://api.paystack.co"
	defaultTimeout = 15 * time.Second
	maxBodyBytes   = 1 << 20 // 1 MiB transaction payloads
)

var (
	_ billing.PaymentProvider = (*Client)(nil)
	_ billing.RefundProvider  = (*Client)(nil)

	errUnexpectedStatus = errors.New("paystack returned an error status")
	errRequestRejected  = errors.New("paystack rejected the request")
)

// Client calls the Paystack transaction API with a secret key.
type Client struct {
	httpClient *http.Client
	baseURL    string
	secretKey  string
}

type refundRequest struct {
	Transaction  string `json:"transaction"`
	Amount       int    `json:"amount"`
	Currency     string `json:"currency,omitempty"`
	CustomerNote string `json:"customer_note,omitempty"` //nolint:tagliatelle
	MerchantNote string `json:"merchant_note,omitempty"` //nolint:tagliatelle
}

type refundResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID     json.Number `json:"id"`
		Status string      `json:"status"`
	} `json:"data"`
}

// CreateRefund queues a full or partial Paystack refund for a settled transaction.
func (c *Client) CreateRefund(
	ctx context.Context,
	transactionReference string,
	amountCents int,
	currency, reason string,
) (billing.ProviderRefund, error) {
	payload, err := json.Marshal(refundRequest{
		Transaction: transactionReference, Amount: amountCents, Currency: currency,
		CustomerNote: reason, MerchantNote: reason,
	})
	if err != nil {
		return billing.ProviderRefund{}, fmt.Errorf("encode refund request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/refund", bytes.NewReader(payload))
	if err != nil {
		return billing.ProviderRefund{}, fmt.Errorf("build refund request: %w", err)
	}
	c.authorize(req)
	req.Header.Set("Content-Type", "application/json")
	body, err := c.do(req)
	if err != nil {
		return billing.ProviderRefund{}, fmt.Errorf("create refund: %w", err)
	}
	var decoded refundResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return billing.ProviderRefund{}, fmt.Errorf("decode refund response: %w", err)
	}
	if !decoded.Status {
		return billing.ProviderRefund{}, fmt.Errorf("%w: %s", errRequestRejected, decoded.Message)
	}
	return billing.ProviderRefund{ID: decoded.Data.ID.String(), Status: decoded.Data.Status}, nil
}

// NewClient constructs a Client. An empty baseURL selects the Paystack API
// default; tests pass an httptest URL. When client is nil it applies an
// SSRF-hardened default (no redirects; refuses non-public addresses) with a
// 15s timeout, matching the HRIS provider adapters.
func NewClient(secretKey, baseURL string, client *http.Client) *Client {
	if client == nil {
		client = safehttp.Client(defaultTimeout)
	}

	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Client{httpClient: client, baseURL: strings.TrimRight(baseURL, "/"), secretKey: secretKey}
}

type initializeRequest struct {
	Email       string            `json:"email"`
	Amount      int               `json:"amount"`
	Currency    string            `json:"currency"`
	Reference   string            `json:"reference"`
	CallbackURL string            `json:"callback_url,omitempty"` //nolint:tagliatelle // Paystack wire format
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type initializeResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AuthorizationURL string `json:"authorization_url"` //nolint:tagliatelle // Paystack wire format
		Reference        string `json:"reference"`
	} `json:"data"`
}

// InitializeTransaction starts a hosted checkout and returns its redirect
// URL. Amount is in the currency's smallest unit (kobo/cents), matching the
// Paystack API.
func (c *Client) InitializeTransaction(
	ctx context.Context,
	in billing.InitializeTransactionInput,
) (billing.CheckoutSession, error) {
	payload, err := json.Marshal(initializeRequest{
		Email:       in.Email,
		Amount:      in.AmountCents,
		Currency:    in.Currency,
		Reference:   in.Reference,
		CallbackURL: in.CallbackURL,
		Metadata:    in.Metadata,
	})
	if err != nil {
		return billing.CheckoutSession{}, fmt.Errorf("encode initialize request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/transaction/initialize",
		bytes.NewReader(payload),
	)
	if err != nil {
		return billing.CheckoutSession{}, fmt.Errorf("build initialize request: %w", err)
	}

	c.authorize(req)
	req.Header.Set("Content-Type", "application/json")

	body, err := c.do(req)
	if err != nil {
		return billing.CheckoutSession{}, fmt.Errorf("initialize transaction: %w", err)
	}

	var decoded initializeResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return billing.CheckoutSession{}, fmt.Errorf("decode initialize response: %w", err)
	}

	if !decoded.Status {
		return billing.CheckoutSession{}, fmt.Errorf("%w: %s", errRequestRejected, decoded.Message)
	}

	return billing.CheckoutSession{
		AuthorizationURL: decoded.Data.AuthorizationURL,
		Reference:        decoded.Data.Reference,
	}, nil
}

type verifyResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Status    string `json:"status"`
		Reference string `json:"reference"`
		Amount    int    `json:"amount"`
		Currency  string `json:"currency"`
		PaidAt    string `json:"paid_at"` //nolint:tagliatelle // Paystack wire format
	} `json:"data"`
}

// VerifyTransaction loads the provider-side state of a transaction.
func (c *Client) VerifyTransaction(ctx context.Context, reference string) (billing.VerifiedPayment, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/transaction/verify/"+reference,
		nil,
	)
	if err != nil {
		return billing.VerifiedPayment{}, fmt.Errorf("build verify request: %w", err)
	}

	c.authorize(req)

	body, err := c.do(req)
	if err != nil {
		return billing.VerifiedPayment{}, fmt.Errorf("verify transaction: %w", err)
	}

	var decoded verifyResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return billing.VerifiedPayment{}, fmt.Errorf("decode verify response: %w", err)
	}

	if !decoded.Status {
		return billing.VerifiedPayment{}, fmt.Errorf("%w: %s", errRequestRejected, decoded.Message)
	}

	paidAt, _ := time.Parse(time.RFC3339, decoded.Data.PaidAt)

	return billing.VerifiedPayment{
		Reference:   decoded.Data.Reference,
		AmountCents: decoded.Data.Amount,
		Currency:    decoded.Data.Currency,
		Paid:        strings.EqualFold(decoded.Data.Status, "success"),
		PaidAt:      paidAt,
	}, nil
}

func (c *Client) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	req.Header.Set("Accept", "application/json")
}

// do executes the request and returns the body of any 2xx response.
func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call paystack: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read paystack response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
	}

	return body, nil
}
