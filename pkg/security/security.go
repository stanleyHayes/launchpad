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
}

// IssueAccessToken creates a signed JWT access token.
func IssueAccessToken(secret string, ttl time.Duration, principal Principal) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		Email:          principal.Email,
		OrganizationID: principal.OrganizationID,
		RoleCode:       principal.RoleCode,
		SessionID:      principal.SessionID,
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
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(parsed *jwt.Token) (any, error) {
		if parsed.Method != jwt.SigningMethodHS256 {
			return nil, errUnexpectedSigningMethod
		}

		return []byte(secret), nil
	})
	if err != nil {
		return Principal{}, fmt.Errorf("parse access token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return Principal{}, errInvalidToken
	}

	return Principal{
		UserID:         claims.Subject,
		Email:          claims.Email,
		OrganizationID: claims.OrganizationID,
		RoleCode:       claims.RoleCode,
		SessionID:      claims.SessionID,
	}, nil
}
