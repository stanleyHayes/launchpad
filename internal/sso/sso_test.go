package sso_test

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"launchpad/internal/sso"
)

const (
	orgID   = "org-1"
	orgSlug = "acme"
)

type fakeConfigStore struct {
	config *sso.Config
	saved  *sso.Config
}

func (f *fakeConfigStore) EnsureIndexes(context.Context) error { return nil }

func (f *fakeConfigStore) GetByOrganization(_ context.Context, _ string) (sso.Config, error) {
	if f.config == nil {
		return sso.Config{}, sso.ErrNotConfigured
	}

	return *f.config, nil
}

func (f *fakeConfigStore) SetConfig(_ context.Context, config sso.Config) error {
	stored := config
	f.saved = &stored

	return nil
}

type fakeStateStore struct {
	saved map[string]sso.AuthState
}

func newStateStore() *fakeStateStore { return &fakeStateStore{saved: map[string]sso.AuthState{}} }

func (f *fakeStateStore) Save(_ context.Context, state string, data sso.AuthState, _ time.Duration) error {
	f.saved[state] = data

	return nil
}

func (f *fakeStateStore) Consume(_ context.Context, state string) (sso.AuthState, error) {
	data, ok := f.saved[state]
	if !ok {
		return sso.AuthState{}, sso.ErrInvalidState
	}

	delete(f.saved, state)

	return data, nil
}

type fakeVerifier struct {
	claims sso.Claims
	err    error
}

func (f *fakeVerifier) Verify(context.Context, sso.Config, string, string) (sso.Claims, error) {
	return f.claims, f.err
}

type fakeIssuer struct {
	session  sso.Session
	err      error
	gotEmail string
	gotOrgID string
}

func (f *fakeIssuer) IssueFederatedSession(_ context.Context, email, organizationID string) (sso.Session, error) {
	f.gotEmail = email
	f.gotOrgID = organizationID

	return f.session, f.err
}

type fakeOrgResolver struct {
	err error
}

func (f *fakeOrgResolver) OrganizationIDBySlug(context.Context, string) (string, error) {
	if f.err != nil {
		return "", f.err
	}

	return orgID, nil
}

func enabledConfig() *sso.Config {
	return &sso.Config{
		OrganizationID:        orgID,
		Enabled:               true,
		Issuer:                "https://idp.example.com",
		ClientID:              "client-123",
		ClientSecret:          "secret",
		AuthorizationEndpoint: "https://idp.example.com/authorize",
		TokenEndpoint:         "https://idp.example.com/token",
		JWKSURI:               "https://idp.example.com/jwks",
		RedirectURI:           "https://app.example.com/callback",
	}
}

func TestStartBuildsAuthorizationURL(t *testing.T) {
	t.Parallel()

	states := newStateStore()
	svc := sso.NewService(
		&fakeConfigStore{config: enabledConfig()},
		states,
		&fakeVerifier{},
		&fakeIssuer{},
		&fakeOrgResolver{},
	)

	authURL, err := svc.Start(context.Background(), orgSlug)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}

	query := parsed.Query()
	if query.Get("client_id") != "client-123" || query.Get("response_type") != "code" {
		t.Fatalf("unexpected auth url query: %s", parsed.RawQuery)
	}

	state := query.Get("state")
	if state == "" {
		t.Fatalf("auth url missing state")
	}

	stored, ok := states.saved[state]
	if !ok || stored.OrganizationID != orgID || stored.Nonce != query.Get("nonce") {
		t.Fatalf("state not saved with matching nonce: %+v", stored)
	}
}

func TestStartNotConfigured(t *testing.T) {
	t.Parallel()

	svc := sso.NewService(&fakeConfigStore{}, newStateStore(), &fakeVerifier{}, &fakeIssuer{}, &fakeOrgResolver{})

	if _, err := svc.Start(context.Background(), orgSlug); !errors.Is(err, sso.ErrNotConfigured) {
		t.Fatalf("got %v, want ErrNotConfigured", err)
	}
}

func TestCallbackIssuesSession(t *testing.T) {
	t.Parallel()

	states := newStateStore()
	states.saved["state-1"] = sso.AuthState{OrganizationID: orgID, Nonce: "nonce-1"}
	issuer := &fakeIssuer{session: sso.Session{AccessToken: "at", TokenType: "Bearer"}}
	svc := sso.NewService(
		&fakeConfigStore{config: enabledConfig()},
		states,
		&fakeVerifier{claims: sso.Claims{Email: "jane@co.com", Subject: "sub"}},
		issuer,
		&fakeOrgResolver{},
	)

	session, err := svc.Callback(context.Background(), "state-1", "code-1")
	if err != nil {
		t.Fatalf("callback: %v", err)
	}

	if session.AccessToken != "at" {
		t.Fatalf("unexpected session: %+v", session)
	}

	if issuer.gotEmail != "jane@co.com" || issuer.gotOrgID != orgID {
		t.Fatalf("issuer called with wrong args: %s / %s", issuer.gotEmail, issuer.gotOrgID)
	}

	if _, stillThere := states.saved["state-1"]; stillThere {
		t.Fatalf("state should be consumed once")
	}
}

func TestCallbackRejectsBadState(t *testing.T) {
	t.Parallel()

	svc := sso.NewService(
		&fakeConfigStore{config: enabledConfig()},
		newStateStore(),
		&fakeVerifier{claims: sso.Claims{Email: "x@y.com"}},
		&fakeIssuer{},
		&fakeOrgResolver{},
	)

	if _, err := svc.Callback(context.Background(), "missing", "code"); !errors.Is(err, sso.ErrInvalidState) {
		t.Fatalf("got %v, want ErrInvalidState", err)
	}
}

func TestCallbackRejectsEmptyEmail(t *testing.T) {
	t.Parallel()

	states := newStateStore()
	states.saved["s"] = sso.AuthState{OrganizationID: orgID, Nonce: "n"}
	svc := sso.NewService(
		&fakeConfigStore{config: enabledConfig()},
		states,
		&fakeVerifier{claims: sso.Claims{Email: ""}},
		&fakeIssuer{},
		&fakeOrgResolver{},
	)

	if _, err := svc.Callback(context.Background(), "s", "code"); !errors.Is(err, sso.ErrVerification) {
		t.Fatalf("got %v, want ErrVerification", err)
	}
}

func TestSetConfigValidation(t *testing.T) {
	t.Parallel()

	svc := sso.NewService(&fakeConfigStore{}, newStateStore(), &fakeVerifier{}, &fakeIssuer{}, &fakeOrgResolver{})

	valid := sso.SetConfigInput{
		Enabled:               true,
		Issuer:                "https://idp.example.com",
		ClientID:              "c",
		ClientSecret:          "s",
		AuthorizationEndpoint: "https://idp.example.com/authorize",
		TokenEndpoint:         "https://idp.example.com/token",
		JWKSURI:               "https://idp.example.com/jwks",
		RedirectURI:           "https://app.example.com/callback",
	}
	if _, err := svc.SetConfig(context.Background(), orgID, valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	insecure := valid

	insecure.TokenEndpoint = "http://idp.example.com/token"
	if _, err := svc.SetConfig(context.Background(), orgID, insecure); !errors.Is(err, sso.ErrInvalidInput) {
		t.Fatalf("expected non-https endpoint rejected, got %v", err)
	}

	missingSecret := valid

	missingSecret.ClientSecret = ""
	if _, err := svc.SetConfig(context.Background(), orgID, missingSecret); !errors.Is(err, sso.ErrInvalidInput) {
		t.Fatalf("expected enabled-without-secret rejected, got %v", err)
	}
}

func (f *fakeConfigStore) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
