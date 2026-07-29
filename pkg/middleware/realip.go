package middleware

import (
	"net/http"
	"strings"
)

// RealIP rewrites r.RemoteAddr to the first X-Forwarded-For value, else
// X-Real-IP, so downstream middleware (the rate limiter) keys on the real
// client IP. It replaces chi's deprecated RealIP, which is vulnerable to IP
// spoofing (GHSA-3fxj-6jh8-hvhx): only deploy this behind a trusted load
// balancer that overwrites those headers, never directly internet-exposed.
func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			if ip := strings.TrimSpace(first); ip != "" {
				r.RemoteAddr = ip
			}
		} else if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			r.RemoteAddr = xri
		}

		next.ServeHTTP(w, r)
	})
}
