// Package security provides password, token, and request-identity helpers.
package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	refreshTokenBytes = 32
	tokenIssuer       = "launchpad"
)

var (
	errUnexpectedSigningMethod = errors.New("unexpected signing method")
	errInvalidToken            = errors.New("invalid token")
)

// Principal is the authenticated caller.
type Principal struct {
	UserID         string
	Email          string
	OrganizationID string
	RoleCode       string
	SessionID      string
	// Impersonator marks a platform support impersonation token (PRD 5.2.2):
	// UserID stays the support agent, while OrganizationID/RoleCode address
	// the target tenant. ImpersonationSessionID identifies the backing
	// support session so audit events and token validation can reference it.
	Impersonator           bool
	ImpersonationSessionID string
}

// ImpersonatorPermissions returns the fixed read-only permission set granted
// to platform support impersonation tokens (PRD 5.2.2). It covers only gated
// reads; tenant read routes without a permission gate (journeys, support
// tickets, blockers) stay open to any organization member, and every write
// permission — billing.manage, members.*, integrations.*, and the rest — is
// deliberately absent so an impersonation token can never mutate tenant
// state, change billing/subscriptions, purge organizations, or change
// platform settings.
func ImpersonatorPermissions() map[string]struct{} {
	return map[string]struct{}{
		"employees.read":     {},
		"assignments.read":   {},
		"analytics.read":     {},
		"audit.read":         {},
		"notifications.read": {},
	}
}

type contextKey string

const principalKey contextKey = "principal"

// WithPrincipal stores a principal on the context.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey, principal)
}

// PrincipalFromContext loads a principal from context.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey).(Principal)

	return principal, ok
}

// HashPassword hashes a password with bcrypt.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}

	return string(hashed), nil
}

// CheckPassword compares a password with a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// HashToken returns a SHA-256 hex digest.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:])
}

// NewRefreshToken creates a cryptographically random refresh token.
func NewRefreshToken() (string, error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Claims are JWT access-token claims.
type Claims struct {
	jwt.RegisteredClaims

	Email          string `json:"email"`
	OrganizationID string `json:"organizationId"`
	RoleCode       string `json:"roleCode"`
	SessionID      string `json:"sessionId"`
	// Impersonator/ImpersonationSessionID mark platform support impersonation
	// tokens (PRD 5.2.2); both are empty on ordinary access tokens.
	Impersonator           bool   `json:"impersonator,omitempty"`
	ImpersonationSessionID string `json:"impersonationSessionId,omitempty"`
}

// IssueAccessToken creates a signed JWT access token.
func IssueAccessToken(secret string, ttl time.Duration, principal Principal) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		Email:                  principal.Email,
		OrganizationID:         principal.OrganizationID,
		RoleCode:               principal.RoleCode,
		SessionID:              principal.SessionID,
		Impersonator:           principal.Impersonator,
		ImpersonationSessionID: principal.ImpersonationSessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   principal.UserID,
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    tokenIssuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}

	return signed, nil
}

// ParseAccessToken validates and parses an access token.
func ParseAccessToken(secret, tokenString string) (Principal, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(parsed *jwt.Token) (any, error) {
			if parsed.Method != jwt.SigningMethodHS256 {
				return nil, errUnexpectedSigningMethod
			}

			return []byte(secret), nil
		},
		jwt.WithIssuer(tokenIssuer),
	)
	if err != nil {
		return Principal{}, fmt.Errorf("parse access token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return Principal{}, errInvalidToken
	}

	return Principal{
		UserID:                 claims.Subject,
		Email:                  claims.Email,
		OrganizationID:         claims.OrganizationID,
		RoleCode:               claims.RoleCode,
		SessionID:              claims.SessionID,
		Impersonator:           claims.Impersonator,
		ImpersonationSessionID: claims.ImpersonationSessionID,
	}, nil
}
