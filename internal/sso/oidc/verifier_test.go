package oidc_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"launchpad/internal/sso"
	"launchpad/internal/sso/oidc"
)

const (
	clientID   = "client-123"
	testKID    = "test-key"
	loginNonce = "nonce-1"
)

func jwksJSON(pub *rsa.PublicKey) string {
	modulus := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	exponent := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())

	return fmt.Sprintf(`{"keys":[{"kty":"RSA","kid":%q,"n":%q,"e":%q}]}`, testKID, modulus, exponent)
}

func signIDToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = testKID

	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign id_token: %v", err)
	}

	return signed
}

// newIDP starts a fake identity provider whose id_token is produced by
// idTokenFor(issuer), where issuer is the server's own base URL.
func newIDP(t *testing.T, key *rsa.PrivateKey, idTokenFor func(issuer string) jwt.MapClaims) *httptest.Server {
	t.Helper()

	var server *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(jwksJSON(&key.PublicKey)))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		idToken := signIDToken(t, key, idTokenFor(server.URL))
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": idToken})
	})

	server = httptest.NewServer(mux)

	return server
}

func configFor(server *httptest.Server) sso.Config {
	return sso.Config{
		OrganizationID:        "org-1",
		Enabled:               true,
		Issuer:                server.URL,
		ClientID:              clientID,
		ClientSecret:          "secret",
		AuthorizationEndpoint: server.URL + "/authorize",
		TokenEndpoint:         server.URL + "/token",
		JWKSURI:               server.URL + "/jwks",
		RedirectURI:           "https://app.example.com/callback",
	}
}

func TestVerifySucceeds(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	server := newIDP(t, key, func(issuer string) jwt.MapClaims {
		return jwt.MapClaims{
			"iss":   issuer,
			"aud":   clientID,
			"exp":   time.Now().Add(time.Hour).Unix(),
			"nonce": loginNonce,
			"email": "jane@co.com",
			"sub":   "idp-sub",
		}
	})
	defer server.Close()

	claims, err := oidc.NewClient(server.Client()).Verify(context.Background(), configFor(server), "code", loginNonce)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if claims.Email != "jane@co.com" || claims.Subject != "idp-sub" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerifyRejectsWrongNonce(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	server := newIDP(t, key, func(issuer string) jwt.MapClaims {
		return jwt.MapClaims{
			"iss": issuer, "aud": clientID, "exp": time.Now().Add(time.Hour).Unix(),
			"nonce": "attacker-nonce", "email": "jane@co.com", "sub": "s",
		}
	})
	defer server.Close()

	client := oidc.NewClient(server.Client())
	if _, err := client.Verify(context.Background(), configFor(server), "code", loginNonce); err == nil {
		t.Fatalf("expected nonce mismatch to fail verification")
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	server := newIDP(t, key, func(issuer string) jwt.MapClaims {
		return jwt.MapClaims{
			"iss": issuer, "aud": clientID, "exp": time.Now().Add(-time.Hour).Unix(),
			"nonce": loginNonce, "email": "jane@co.com", "sub": "s",
		}
	})
	defer server.Close()

	client := oidc.NewClient(server.Client())
	if _, err := client.Verify(context.Background(), configFor(server), "code", loginNonce); err == nil {
		t.Fatalf("expected expired token to fail verification")
	}
}
