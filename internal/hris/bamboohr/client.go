// Package bamboohr is an HRIS Provider adapter for BambooHR's employee
// directory API. It implements hris.Provider over stdlib net/http.
package bamboohr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"launchpad/internal/hris"
	"launchpad/pkg/safehttp"
)

const (
	// Provider is the config provider name this adapter handles.
	Provider = "bamboohr"

	defaultBaseURL = "https://api.bamboohr.com"
	defaultTimeout = 15 * time.Second
	maxBodyBytes   = 8 << 20 // 8 MiB directory payloads
)

var (
	_ hris.Provider = (*Client)(nil)

	errUnexpectedStatus = errors.New("bamboohr returned an error status")
)

// Client fetches directories from BambooHR.
type Client struct {
	httpClient *http.Client
}

// NewClient constructs a Client. When client is nil it applies an SSRF-hardened
// default (no redirects; refuses non-public addresses) because the BaseURL is
// tenant-controlled.
func NewClient(client *http.Client) *Client {
	if client == nil {
		client = safehttp.Client(defaultTimeout)
	}

	return &Client{httpClient: client}
}

type directoryResponse struct {
	Employees []employee `json:"employees"`
}

type employee struct {
	ID         string `json:"id"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	WorkEmail  string `json:"workEmail"`
	JobTitle   string `json:"jobTitle"`
	Department string `json:"department"`
}

// FetchDirectory returns the tenant's active employee directory.
func (c *Client) FetchDirectory(ctx context.Context, config hris.Config) ([]hris.DirectoryEntry, error) {
	endpoint := directoryURL(config)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build directory request: %w", err)
	}

	req.SetBasicAuth(config.APIToken, "x")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch directory: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read directory response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
	}

	var decoded directoryResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode directory response: %w", err)
	}

	entries := make([]hris.DirectoryEntry, 0, len(decoded.Employees))
	for _, emp := range decoded.Employees {
		entries = append(entries, hris.DirectoryEntry{
			ExternalID: emp.ID,
			FirstName:  emp.FirstName,
			LastName:   emp.LastName,
			Email:      strings.ToLower(strings.TrimSpace(emp.WorkEmail)),
			JobTitle:   emp.JobTitle,
			Department: emp.Department,
			Active:     true,
		})
	}

	return entries, nil
}

func directoryURL(config hris.Config) string {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return strings.TrimRight(baseURL, "/") + "/api/gateway.php/" + config.Subdomain + "/v1/employees/directory"
}
