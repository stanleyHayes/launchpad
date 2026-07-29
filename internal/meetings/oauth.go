package meetings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const oauthResponseLimit = 1 << 20

// OAuthConfig contains the server-side registration for one calendar provider.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Tenant       string
}

// OAuthToken is the credential returned by an OAuth token endpoint.
type OAuthToken struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    *time.Time
}

// OAuthClient implements the authorization-code and refresh-token exchanges
// against the fixed Google and Microsoft endpoints.
type OAuthClient struct {
	provider string
	config   OAuthConfig
	client   *http.Client
}

func NewOAuthClient(provider string, config OAuthConfig, client *http.Client) *OAuthClient {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if strings.TrimSpace(config.Tenant) == "" {
		config.Tenant = "common"
	}

	return &OAuthClient{provider: provider, config: config, client: client}
}

func (c *OAuthClient) Configured() bool {
	return strings.TrimSpace(c.config.ClientID) != "" &&
		strings.TrimSpace(c.config.ClientSecret) != "" &&
		strings.TrimSpace(c.config.RedirectURI) != "" &&
		validProvider(c.provider)
}

func (c *OAuthClient) AuthorizationURL(state string) (string, error) {
	if !c.Configured() || strings.TrimSpace(state) == "" {
		return "", ErrInvalidState
	}

	endpoint, scope := c.endpoints()
	parsed, err := url.Parse(endpoint.authorization)
	if err != nil {
		return "", fmt.Errorf("parse oauth authorization endpoint: %w", err)
	}
	query := parsed.Query()
	query.Set("client_id", c.config.ClientID)
	query.Set("redirect_uri", c.config.RedirectURI)
	query.Set("response_type", "code")
	query.Set("scope", scope)
	query.Set("state", state)
	if c.provider == ProviderGoogle {
		query.Set("access_type", "offline")
		query.Set("prompt", "consent")
	}
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code string) (OAuthToken, error) {
	if strings.TrimSpace(code) == "" {
		return OAuthToken{}, ErrInvalidInput
	}

	return c.tokenRequest(ctx, url.Values{
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.config.RedirectURI},
	})
}

func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (OAuthToken, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return OAuthToken{}, ErrInvalidCredential
	}

	values := url.Values{
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}
	if c.provider == ProviderMicrosoft {
		_, scope := c.endpoints()
		values.Set("scope", scope)
	}

	token, err := c.tokenRequest(ctx, values)
	if err == nil && token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}

	return token, err
}

func (c *OAuthClient) tokenRequest(ctx context.Context, values url.Values) (OAuthToken, error) {
	if !c.Configured() {
		return OAuthToken{}, ErrInvalidState
	}
	endpoint, _ := c.endpoints()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.token, strings.NewReader(values.Encode()))
	if err != nil {
		return OAuthToken{}, fmt.Errorf("build oauth token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return OAuthToken{}, fmt.Errorf("%w: token exchange: %w", ErrCalendarProvider, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, oauthResponseLimit))
	if err != nil {
		return OAuthToken{}, fmt.Errorf("%w: read token response: %w", ErrCalendarProvider, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return OAuthToken{}, fmt.Errorf("%w: token endpoint status %d", ErrInvalidCredential, resp.StatusCode)
	}

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return OAuthToken{}, fmt.Errorf("%w: decode token response: %w", ErrCalendarProvider, err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return OAuthToken{}, errors.Join(ErrInvalidCredential, errors.New("missing access token"))
	}

	var expiresAt *time.Time
	if payload.ExpiresIn > 0 {
		value := time.Now().UTC().Add(time.Duration(payload.ExpiresIn) * time.Second)
		expiresAt = &value
	}

	return OAuthToken{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

type oauthEndpoints struct {
	authorization string
	token         string
}

func (c *OAuthClient) endpoints() (oauthEndpoints, string) {
	if c.provider == ProviderMicrosoft {
		tenant := url.PathEscape(c.config.Tenant)

		return oauthEndpoints{
			authorization: "https://login.microsoftonline.com/" + tenant + "/oauth2/v2.0/authorize",
			token:         "https://login.microsoftonline.com/" + tenant + "/oauth2/v2.0/token",
		}, "offline_access Calendars.ReadWrite"
	}

	return oauthEndpoints{
		authorization: "https://accounts.google.com/o/oauth2/v2/auth",
		token:         "https://oauth2.googleapis.com/token",
	}, "https://www.googleapis.com/auth/calendar"
}
