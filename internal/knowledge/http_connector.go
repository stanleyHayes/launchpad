package knowledge

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const maxConnectorBytes = 2 << 20

// HTTPConnector synchronizes URL-like sources while rejecting private-network
// destinations. Provider-specific export URLs can therefore be used without
// allowing the API to become an SSRF proxy.
type HTTPConnector struct {
	client *http.Client
}

func NewHTTPConnector() *HTTPConnector {
	return &HTTPConnector{client: &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return validateConnectorURL(req.Context(), req.URL)
		},
	}}
}

func (c *HTTPConnector) Fetch(ctx context.Context, _ string, rawURI string) (string, error) {
	target, err := url.Parse(rawURI)
	if err != nil {
		return "", fmt.Errorf("parse connector URL: %w", err)
	}
	if err := validateConnectorURL(ctx, target); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create connector request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch connector: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("connector returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxConnectorBytes+1))
	if err != nil {
		return "", fmt.Errorf("read connector response: %w", err)
	}
	if len(body) > maxConnectorBytes {
		return "", fmt.Errorf("connector response exceeds %d bytes", maxConnectorBytes)
	}
	return string(body), nil
}

func validateConnectorURL(ctx context.Context, target *url.URL) error {
	if target.Scheme != "https" || target.Hostname() == "" || target.User != nil {
		return fmt.Errorf("%w: connector URI must be a public HTTPS URL without credentials", ErrInvalidInput)
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, target.Hostname())
	if err != nil || len(addresses) == 0 {
		return fmt.Errorf("%w: connector host cannot be resolved", ErrInvalidInput)
	}
	for _, address := range addresses {
		ip := address.IP
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("%w: connector host must be public", ErrInvalidInput)
		}
	}
	return nil
}
