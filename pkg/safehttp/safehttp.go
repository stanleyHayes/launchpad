// Package safehttp provides an HTTP client hardened against SSRF. The client
// never follows redirects and refuses to connect to loopback, private,
// link-local, or unspecified addresses. The address check runs at DIAL time
// (after DNS resolution), so it also defeats DNS-rebinding and redirect-based
// bypasses — the right control for endpoints whose host is tenant-controlled
// (HRIS base URLs, SSO/OIDC issuer/JWKS/token endpoints).
package safehttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// ErrBlockedAddress indicates a dial to a disallowed (non-public) address was refused.
var ErrBlockedAddress = errors.New("blocked connection to a non-public address")

// Client returns an SSRF-hardened *http.Client with the given timeout.
func Client(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, Control: guardDial}

	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: refuseRedirect,
		Transport:     &http.Transport{DialContext: dialer.DialContext},
	}
}

func refuseRedirect(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

// guardDial rejects a dial to any non-public address. It runs after DNS
// resolution with the concrete IP the connection would use.
func guardDial(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse dial address: %w", err)
	}

	ip := net.ParseIP(host)
	if ip == nil || isBlocked(ip) {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, host)
	}

	return nil
}

// isBlocked reports whether ip is in a range an outbound tenant request must
// never reach: loopback (127/8, ::1), private (RFC1918 + ULA fc00::/7),
// link-local (169.254/16 incl. cloud metadata, fe80::/10), and unspecified.
func isBlocked(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}
