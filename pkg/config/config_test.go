package config

import "testing"

func TestHTTPAddrPrefersExplicitAddress(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("PORT", "10000")

	if got := httpAddr(); got != ":9090" {
		t.Fatalf("httpAddr() = %q, want :9090", got)
	}
}

func TestHTTPAddrUsesPlatformPort(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PORT", "10000")

	if got := httpAddr(); got != ":10000" {
		t.Fatalf("httpAddr() = %q, want :10000", got)
	}
}

func TestHTTPAddrDefaultsLocally(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PORT", "")

	if got := httpAddr(); got != defaultHTTPAddr {
		t.Fatalf("httpAddr() = %q, want %q", got, defaultHTTPAddr)
	}
}
