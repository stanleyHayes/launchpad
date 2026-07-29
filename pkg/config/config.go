// Package config loads and validates process configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppEnv         = "local"
	defaultHTTPAddr       = ":8080"
	defaultMongoURI       = "mongodb://localhost:27017"
	defaultMongoDatabase  = "launchpad"
	defaultRedisURL       = "redis://localhost:6379/0"
	defaultAccessTTL      = "15m"
	defaultRefreshTTL     = "168h"
	defaultInviteTTL      = "72h"
	defaultCORSOrigin     = "http://localhost:3000"
	defaultPasswordMinLen = 10
	defaultEmailBaseURL   = "https://api.resend.com"
	defaultOrgAdminURL    = "http://localhost:3002"
	defaultAPIPublicURL   = "http://localhost:8080"
	defaultSchedulerTick  = "15m"
	localDevJWTSecret     = "local" + "-dev-only-change-me"
	minJWTSecretBytes     = 32
)

var (
	errJWTSecretRequired     = errors.New("JWT_SECRET is required")
	errJWTSecretInsecure     = errors.New("JWT_SECRET must be changed outside local")
	errJWTSecretTooShort     = errors.New("JWT_SECRET must be at least 32 bytes outside local")
	errMongoURIRequired      = errors.New("MONGODB_URI is required outside local")
	errRedisURLRequired      = errors.New("REDIS_URL is required outside local")
	errPasswordMinLenInvalid = errors.New("PASSWORD_MIN_LENGTH must be a positive integer")
	errSchedulerTickInvalid  = errors.New("SCHEDULER_INTERVAL must be a positive duration")
)

// Config holds process configuration.
type Config struct {
	AppEnv                        string
	HTTPAddr                      string
	MongoURI                      string
	MongoDatabase                 string
	RedisURL                      string
	JWTSecret                     string
	EncryptionKey                 string
	AccessTTL                     time.Duration
	RefreshTTL                    time.Duration
	InviteTTL                     time.Duration
	PasswordMinLen                int
	CORSOrigins                   []string
	PlatformOwnerEmail            string
	PlatformOwnerPassword         string
	PlatformOwnerName             string
	AnthropicAPIKey               string
	AssistantModel                string
	EmailAPIKey                   string
	EmailFrom                     string
	EmailBaseURL                  string
	SMSAPIKey                     string
	SMSFrom                       string
	SMSBaseURL                    string
	ErrorTrackingURL              string
	TracingExportURL              string
	ObservabilityToken            string
	OrgAdminURL                   string
	APIPublicURL                  string
	GoogleCalendarClientID        string
	GoogleCalendarClientSecret    string
	MicrosoftCalendarClientID     string
	MicrosoftCalendarClientSecret string
	MicrosoftCalendarTenant       string
	SAMLSPPrivateKey              string
	SAMLSPCertificate             string
	SchedulerInterval             time.Duration
	PaystackSecretKey             string
	PaystackWebhookSecret         string
	PaystackBaseURL               string
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	cfg := Config{
		AppEnv:                        getenv("APP_ENV", defaultAppEnv),
		HTTPAddr:                      getenv("HTTP_ADDR", defaultHTTPAddr),
		MongoURI:                      getenv("MONGODB_URI", defaultMongoURI),
		MongoDatabase:                 getenv("MONGODB_DATABASE", defaultMongoDatabase),
		RedisURL:                      getenv("REDIS_URL", defaultRedisURL),
		JWTSecret:                     os.Getenv("JWT_SECRET"),
		EncryptionKey:                 strings.TrimSpace(os.Getenv("ENCRYPTION_KEY")),
		CORSOrigins:                   splitCSV(getenv("CORS_ORIGINS", defaultCORSOrigin)),
		PlatformOwnerEmail:            strings.TrimSpace(os.Getenv("PLATFORM_OWNER_EMAIL")),
		PlatformOwnerPassword:         os.Getenv("PLATFORM_OWNER_PASSWORD"),
		PlatformOwnerName:             strings.TrimSpace(os.Getenv("PLATFORM_OWNER_NAME")),
		AnthropicAPIKey:               strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")),
		AssistantModel:                strings.TrimSpace(os.Getenv("ASSISTANT_MODEL")),
		EmailAPIKey:                   strings.TrimSpace(os.Getenv("EMAIL_API_KEY")),
		EmailFrom:                     strings.TrimSpace(os.Getenv("EMAIL_FROM")),
		EmailBaseURL:                  strings.TrimSpace(getenv("EMAIL_BASE_URL", defaultEmailBaseURL)),
		SMSAPIKey:                     strings.TrimSpace(os.Getenv("SMS_API_KEY")),
		SMSFrom:                       strings.TrimSpace(os.Getenv("SMS_FROM")),
		SMSBaseURL:                    strings.TrimSpace(os.Getenv("SMS_BASE_URL")),
		ErrorTrackingURL:              strings.TrimSpace(os.Getenv("ERROR_TRACKING_URL")),
		TracingExportURL:              strings.TrimSpace(os.Getenv("TRACING_EXPORT_URL")),
		ObservabilityToken:            strings.TrimSpace(os.Getenv("OBSERVABILITY_TOKEN")),
		OrgAdminURL:                   strings.TrimRight(strings.TrimSpace(getenv("ORG_ADMIN_URL", defaultOrgAdminURL)), "/"),
		APIPublicURL:                  strings.TrimRight(strings.TrimSpace(getenv("API_PUBLIC_URL", defaultAPIPublicURL)), "/"),
		GoogleCalendarClientID:        strings.TrimSpace(os.Getenv("GOOGLE_CALENDAR_CLIENT_ID")),
		GoogleCalendarClientSecret:    strings.TrimSpace(os.Getenv("GOOGLE_CALENDAR_CLIENT_SECRET")),
		MicrosoftCalendarClientID:     strings.TrimSpace(os.Getenv("MICROSOFT_CALENDAR_CLIENT_ID")),
		MicrosoftCalendarClientSecret: strings.TrimSpace(os.Getenv("MICROSOFT_CALENDAR_CLIENT_SECRET")),
		MicrosoftCalendarTenant:       strings.TrimSpace(getenv("MICROSOFT_CALENDAR_TENANT", "common")),
		SAMLSPPrivateKey:              strings.TrimSpace(os.Getenv("SAML_SP_PRIVATE_KEY")),
		SAMLSPCertificate:             strings.TrimSpace(os.Getenv("SAML_SP_CERTIFICATE")),
		PaystackSecretKey:             strings.TrimSpace(os.Getenv("PAYSTACK_SECRET_KEY")),
		PaystackWebhookSecret:         strings.TrimSpace(os.Getenv("PAYSTACK_WEBHOOK_SECRET")),
		PaystackBaseURL:               strings.TrimSpace(os.Getenv("PAYSTACK_BASE_URL")),
	}

	accessTTL, err := time.ParseDuration(getenv("JWT_ACCESS_TTL", defaultAccessTTL))
	if err != nil {
		return Config{}, fmt.Errorf("JWT_ACCESS_TTL: %w", err)
	}

	refreshTTL, err := time.ParseDuration(getenv("JWT_REFRESH_TTL", defaultRefreshTTL))
	if err != nil {
		return Config{}, fmt.Errorf("JWT_REFRESH_TTL: %w", err)
	}

	inviteTTL, err := time.ParseDuration(getenv("INVITE_TTL", defaultInviteTTL))
	if err != nil {
		return Config{}, fmt.Errorf("INVITE_TTL: %w", err)
	}

	cfg.AccessTTL = accessTTL
	cfg.RefreshTTL = refreshTTL
	cfg.InviteTTL = inviteTTL

	passwordMinLen, err := parsePasswordMinLen(getenv("PASSWORD_MIN_LENGTH", ""))
	if err != nil {
		return Config{}, err
	}

	cfg.PasswordMinLen = passwordMinLen

	schedulerInterval, err := parseSchedulerInterval(getenv("SCHEDULER_INTERVAL", defaultSchedulerTick))
	if err != nil {
		return Config{}, err
	}

	cfg.SchedulerInterval = schedulerInterval

	if cfg.JWTSecret == "" {
		return Config{}, errJWTSecretRequired
	}

	if cfg.AppEnv != defaultAppEnv {
		if err := validateNonLocal(cfg); err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

// validateNonLocal enforces the stricter requirements outside local development.
func validateNonLocal(cfg Config) error {
	if cfg.JWTSecret == localDevJWTSecret {
		return errJWTSecretInsecure
	}

	if len(cfg.JWTSecret) < minJWTSecretBytes {
		return errJWTSecretTooShort
	}

	if os.Getenv("MONGODB_URI") == "" {
		return errMongoURIRequired
	}

	if os.Getenv("REDIS_URL") == "" {
		return errRedisURLRequired
	}

	return nil
}

// parsePasswordMinLen returns the default when PASSWORD_MIN_LENGTH is unset
// and validates the value otherwise.
func parsePasswordMinLen(value string) (int, error) {
	if value == "" {
		return defaultPasswordMinLen, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%w: %q", errPasswordMinLenInvalid, value)
	}

	return parsed, nil
}

// parseSchedulerInterval validates SCHEDULER_INTERVAL; the caller passes the
// default when the variable is unset.
func parseSchedulerInterval(value string) (time.Duration, error) {
	interval, err := time.ParseDuration(value)
	if err != nil || interval <= 0 {
		return 0, errSchedulerTickInvalid
	}

	return interval, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}
