package sso

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"launchpad/pkg/security"
)

const stateTTL = 10 * time.Minute

// Service implements SSO use cases.
type Service struct {
	configs  ConfigStore
	states   StateStore
	verifier Verifier
	issuer   SessionIssuer
	orgs     OrgResolver
}

// NewService constructs a Service.
func NewService(
	configs ConfigStore,
	states StateStore,
	verifier Verifier,
	issuer SessionIssuer,
	orgs OrgResolver,
) *Service {
	return &Service{configs: configs, states: states, verifier: verifier, issuer: issuer, orgs: orgs}
}

// GetConfig returns the tenant's OIDC configuration (secret omitted), or an
// empty disabled config when none is set.
func (s *Service) GetConfig(ctx context.Context, organizationID string) (Config, error) {
	if organizationID == "" {
		return Config{}, ErrInvalidInput
	}

	config, err := s.configs.GetByOrganization(ctx, organizationID)
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			return Config{OrganizationID: organizationID}, nil
		}

		return Config{}, fmt.Errorf("get sso config: %w", err)
	}

	return config, nil
}

// SetConfig validates and stores the tenant's OIDC configuration.
func (s *Service) SetConfig(ctx context.Context, organizationID string, in SetConfigInput) (Config, error) {
	if organizationID == "" {
		return Config{}, ErrInvalidInput
	}

	config, err := buildConfig(organizationID, in)
	if err != nil {
		return Config{}, err
	}

	if err := s.configs.SetConfig(ctx, config); err != nil {
		return Config{}, fmt.Errorf("set sso config: %w", err)
	}

	return config, nil
}

// Start begins the OIDC login for an organization slug, returning the IdP
// authorization URL the caller should redirect the browser to.
func (s *Service) Start(ctx context.Context, orgSlug string) (string, error) {
	organizationID, err := s.orgs.OrganizationIDBySlug(ctx, strings.TrimSpace(orgSlug))
	if err != nil {
		return "", ErrNotConfigured
	}

	config, err := s.configs.GetByOrganization(ctx, organizationID)
	if err != nil || !config.Enabled {
		return "", ErrNotConfigured
	}

	state, err := security.NewRefreshToken()
	if err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}

	nonce, err := security.NewRefreshToken()
	if err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	if err := s.states.Save(ctx, state, AuthState{OrganizationID: organizationID, Nonce: nonce}, stateTTL); err != nil {
		return "", fmt.Errorf("save sso state: %w", err)
	}

	return authorizationURL(config, state, nonce)
}

// Callback completes the OIDC login: it validates the state, verifies the code
// with the IdP, and issues a LaunchPad session for the mapped user.
func (s *Service) Callback(ctx context.Context, state, code string) (Session, error) {
	if strings.TrimSpace(state) == "" || strings.TrimSpace(code) == "" {
		return Session{}, ErrInvalidInput
	}

	authState, err := s.states.Consume(ctx, state)
	if err != nil {
		return Session{}, ErrInvalidState
	}

	config, err := s.configs.GetByOrganization(ctx, authState.OrganizationID)
	if err != nil || !config.Enabled {
		return Session{}, ErrNotConfigured
	}

	claims, err := s.verifier.Verify(ctx, config, code, authState.Nonce)
	if err != nil {
		return Session{}, fmt.Errorf("%w: %w", ErrVerification, err)
	}

	if strings.TrimSpace(claims.Email) == "" {
		return Session{}, ErrVerification
	}

	session, err := s.issuer.IssueFederatedSession(ctx, claims.Email, authState.OrganizationID)
	if err != nil {
		return Session{}, fmt.Errorf("issue federated session: %w", err)
	}

	return session, nil
}

func authorizationURL(config Config, state, nonce string) (string, error) {
	endpoint, err := url.Parse(config.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("parse authorization endpoint: %w", err)
	}

	query := endpoint.Query()
	query.Set("client_id", config.ClientID)
	query.Set("redirect_uri", config.RedirectURI)
	query.Set("response_type", "code")
	query.Set("scope", "openid email")
	query.Set("state", state)
	query.Set("nonce", nonce)
	endpoint.RawQuery = query.Encode()

	return endpoint.String(), nil
}

func buildConfig(organizationID string, in SetConfigInput) (Config, error) {
	config := Config{
		OrganizationID:        organizationID,
		Enabled:               in.Enabled,
		Issuer:                strings.TrimSpace(in.Issuer),
		ClientID:              strings.TrimSpace(in.ClientID),
		ClientSecret:          in.ClientSecret,
		AuthorizationEndpoint: strings.TrimSpace(in.AuthorizationEndpoint),
		TokenEndpoint:         strings.TrimSpace(in.TokenEndpoint),
		JWKSURI:               strings.TrimSpace(in.JWKSURI),
		RedirectURI:           strings.TrimSpace(in.RedirectURI),
		UpdatedAt:             time.Now().UTC(),
	}

	if err := validateConfig(config); err != nil {
		return Config{}, err
	}

	return config, nil
}

// validateConfig requires https endpoints and, when enabled, all fields present.
func validateConfig(config Config) error {
	endpoints := []string{
		config.Issuer,
		config.AuthorizationEndpoint,
		config.TokenEndpoint,
		config.JWKSURI,
		config.RedirectURI,
	}
	for _, endpoint := range endpoints {
		if endpoint != "" && !isHTTPSURL(endpoint) {
			return ErrInvalidInput
		}
	}

	if !config.Enabled {
		return nil
	}

	if config.ClientID == "" || config.ClientSecret == "" {
		return ErrInvalidInput
	}

	if slices.Contains(endpoints, "") {
		return ErrInvalidInput
	}

	return nil
}

func isHTTPSURL(raw string) bool {
	parsed, err := url.Parse(raw)

	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}
