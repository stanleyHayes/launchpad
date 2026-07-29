package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"launchpad/pkg/safehttp"
)

const (
	// providerTimeout bounds a single credential-validation call.
	providerTimeout = 10 * time.Second

	githubBaseURL = "https://api.github.com"

	maxProviderBodyBytes = 1 << 20 // 1 MiB; identity payloads are tiny
)

// providerClient validates tenant credentials against a provider API and
// returns the provider-side account handle (GitHub login, Jira display name).
type providerClient interface {
	Validate(ctx context.Context, in ConnectInput) (string, error)
}

// GitHubClient validates GitHub personal access tokens via GET /user.
type GitHubClient struct {
	httpClient *http.Client
}

// NewGitHubClient constructs a GitHubClient. When client is nil it applies an
// SSRF-hardened default; tests inject a stubbed transport instead.
func NewGitHubClient(client *http.Client) *GitHubClient {
	if client == nil {
		client = safehttp.Client(providerTimeout)
	}

	return &GitHubClient{httpClient: client}
}

// Validate calls the GitHub API with the token and returns the user's login.
// A non-2xx response means the credential was rejected: ErrInvalidCredential.
func (c *GitHubClient) Validate(ctx context.Context, in ConnectInput) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubBaseURL+"/user", nil)
	if err != nil {
		return "", fmt.Errorf("build github user request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+in.Token)
	req.Header.Set("Accept", "application/vnd.github+json")

	var payload struct {
		Login string `json:"login"`
	}
	if err := doProviderJSON(c.httpClient, req, &payload); err != nil {
		return "", err
	}

	if payload.Login == "" {
		return "", ErrInvalidCredential
	}

	return payload.Login, nil
}

// JiraClient validates Jira API tokens via GET {baseURL}/rest/api/3/myself.
type JiraClient struct {
	httpClient *http.Client
}

// NewJiraClient constructs a JiraClient. When client is nil it applies an
// SSRF-hardened default (the base URL is tenant-controlled); tests inject a
// stubbed transport instead.
func NewJiraClient(client *http.Client) *JiraClient {
	if client == nil {
		client = safehttp.Client(providerTimeout)
	}

	return &JiraClient{httpClient: client}
}

// Validate calls the Jira API with email:token basic auth and returns the
// user's display name (falling back to their email address). A non-2xx
// response means the credential was rejected: ErrInvalidCredential.
func (c *JiraClient) Validate(ctx context.Context, in ConnectInput) (string, error) {
	endpoint, err := jiraMyselfURL(in.BaseURL)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("build jira myself request: %w", err)
	}

	req.SetBasicAuth(in.Email, in.Token)
	req.Header.Set("Accept", "application/json")

	var payload struct {
		DisplayName  string `json:"displayName"`
		EmailAddress string `json:"emailAddress"`
	}
	if err := doProviderJSON(c.httpClient, req, &payload); err != nil {
		return "", err
	}

	handle := payload.DisplayName
	if handle == "" {
		handle = payload.EmailAddress
	}

	if handle == "" {
		return "", ErrInvalidCredential
	}

	return handle, nil
}

// jiraMyselfURL enforces an https-only base URL before building the endpoint,
// so a token is never sent over plaintext.
func jiraMyselfURL(rawBaseURL string) (string, error) {
	baseURL := strings.TrimSpace(rawBaseURL)

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse jira base url: %w", err)
	}

	if parsed.Scheme != "https" || parsed.Host == "" {
		return "", ErrInvalidInput
	}

	return strings.TrimRight(baseURL, "/") + "/rest/api/3/myself", nil
}

// doProviderJSON executes req and decodes a 2xx JSON body into dst. Any
// non-2xx status maps to ErrInvalidCredential; the response body is never
// included in errors so provider messages cannot leak into logs or clients.
func doProviderJSON(client *http.Client, req *http.Request, dst any) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send provider request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderBodyBytes))
	if err != nil {
		return fmt.Errorf("read provider response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ErrInvalidCredential
	}

	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode provider response: %w", err)
	}

	return nil
}
