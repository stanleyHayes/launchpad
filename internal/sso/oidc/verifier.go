// Package oidc verifies OIDC callbacks against a tenant's identity provider: it
// exchanges the authorization code for tokens and cryptographically verifies the
// id_token (RS256 signature via the IdP's JWKS, plus issuer, audience, expiry,
// and nonce). It implements sso.Verifier over stdlib net/http and golang-jwt.
package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"launchpad/internal/sso"
	"launchpad/pkg/safehttp"
)

const (
	defaultTimeout    = 10 * time.Second
	maxBodyBytes      = 1 << 20
	signingMethodName = "RS256"
)

var (
	_ sso.Verifier = (*Client)(nil)

	errTokenStatus      = errors.New("token endpoint returned an error status")
	errJWKSStatus       = errors.New("jwks endpoint returned an error status")
	errNoIDToken        = errors.New("token response contained no id_token")
	errKeyNotFound      = errors.New("no matching jwks key for token")
	errNonceMismatch    = errors.New("id_token nonce does not match")
	errBadKey           = errors.New("malformed jwks key")
	errEmailNotVerified = errors.New("id_token email is not verified")
)

// Client verifies OIDC callbacks.
type Client struct {
	httpClient *http.Client
}

// NewClient constructs a Client. When client is nil it applies an SSRF-hardened
// default (no redirects; refuses non-public addresses) because the token/JWKS
// endpoints are tenant-controlled.
func NewClient(client *http.Client) *Client {
	if client == nil {
		client = safehttp.Client(defaultTimeout)
	}

	return &Client{httpClient: client}
}

type tokenResponse struct {
	IDToken string `json:"id_token"` //nolint:tagliatelle // OIDC wire format
}

type idTokenClaims struct {
	jwt.RegisteredClaims

	Email         string `json:"email"`
	EmailVerified *bool  `json:"email_verified"` //nolint:tagliatelle // OIDC wire format
	Nonce         string `json:"nonce"`
}

type jwks struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// Verify exchanges the code and verifies the resulting id_token.
func (c *Client) Verify(ctx context.Context, config sso.Config, code, expectedNonce string) (sso.Claims, error) {
	idToken, err := c.exchangeCode(ctx, config, code)
	if err != nil {
		return sso.Claims{}, err
	}

	return c.verifyIDToken(ctx, config, idToken, expectedNonce)
}

func (c *Client) exchangeCode(ctx context.Context, config sso.Config, code string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", config.RedirectURI)
	form.Set("client_id", config.ClientID)
	form.Set("client_secret", config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: %d", errTokenStatus, resp.StatusCode)
	}

	var decoded tokenResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if decoded.IDToken == "" {
		return "", errNoIDToken
	}

	return decoded.IDToken, nil
}

func (c *Client) verifyIDToken(
	ctx context.Context,
	config sso.Config,
	idToken, expectedNonce string,
) (sso.Claims, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{signingMethodName}),
		jwt.WithIssuer(config.Issuer),
		jwt.WithAudience(config.ClientID),
		jwt.WithExpirationRequired(),
	)

	var claims idTokenClaims

	keyfunc := func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)

		return c.publicKey(ctx, config.JWKSURI, kid)
	}

	if _, err := parser.ParseWithClaims(idToken, &claims, keyfunc); err != nil {
		return sso.Claims{}, fmt.Errorf("verify id_token: %w", err)
	}

	if claims.Nonce != expectedNonce {
		return sso.Claims{}, errNonceMismatch
	}

	// Refuse an email the IdP explicitly marks unverified — accepting it would
	// let a permissive IdP assert another member's email (account takeover). An
	// omitted claim is trusted to the org's configured IdP.
	if claims.EmailVerified != nil && !*claims.EmailVerified {
		return sso.Claims{}, errEmailNotVerified
	}

	return sso.Claims{Subject: claims.Subject, Email: claims.Email}, nil
}

func (c *Client) publicKey(ctx context.Context, jwksURI, kid string) (*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, fmt.Errorf("build jwks request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read jwks: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: %d", errJWKSStatus, resp.StatusCode)
	}

	var set jwks
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}

	for _, key := range set.Keys {
		if key.Kty == "RSA" && (kid == "" || key.Kid == kid) {
			return rsaPublicKey(key)
		}
	}

	return nil, errKeyNotFound
}

func rsaPublicKey(key jwkKey) (*rsa.PublicKey, error) {
	modulus, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("%w: modulus", errBadKey)
	}

	exponent, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("%w: exponent", errBadKey)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulus),
		E: int(new(big.Int).SetBytes(exponent).Int64()),
	}, nil
}
