// Package config loads and validates process configuration.
package config

import (
	"errors"
	"fmt"
	"os"
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
	defaultCORSOrigin     = "http://localhost:3000"
	defaultPasswordMinLen = 10
	localDevJWTSecret     = "local" + "-dev-only-change-me"
)

var (
	errJWTSecretRequired = errors.New("JWT_SECRET is required")
	errJWTSecretInsecure = errors.New("JWT_SECRET must be changed outside local")
)

// Config holds process configuration.
type Config struct {
	AppEnv         string
	HTTPAddr       string
	MongoURI       string
	MongoDatabase  string
	RedisURL       string
	JWTSecret      string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	PasswordMinLen int
	CORSOrigins    []string
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	cfg := Config{
		AppEnv:         getenv("APP_ENV", defaultAppEnv),
		HTTPAddr:       getenv("HTTP_ADDR", defaultHTTPAddr),
		MongoURI:       getenv("MONGODB_URI", defaultMongoURI),
		MongoDatabase:  getenv("MONGODB_DATABASE", defaultMongoDatabase),
		RedisURL:       getenv("REDIS_URL", defaultRedisURL),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		PasswordMinLen: defaultPasswordMinLen,
		CORSOrigins:    splitCSV(getenv("CORS_ORIGINS", defaultCORSOrigin)),
	}

	accessTTL, err := time.ParseDuration(getenv("JWT_ACCESS_TTL", defaultAccessTTL))
	if err != nil {
		return Config{}, fmt.Errorf("JWT_ACCESS_TTL: %w", err)
	}

	refreshTTL, err := time.ParseDuration(getenv("JWT_REFRESH_TTL", defaultRefreshTTL))
	if err != nil {
		return Config{}, fmt.Errorf("JWT_REFRESH_TTL: %w", err)
	}

	cfg.AccessTTL = accessTTL
	cfg.RefreshTTL = refreshTTL

	if cfg.JWTSecret == "" {
		return Config{}, errJWTSecretRequired
	}

	if cfg.AppEnv != defaultAppEnv && cfg.JWTSecret == localDevJWTSecret {
		return Config{}, errJWTSecretInsecure
	}

	return cfg, nil
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
