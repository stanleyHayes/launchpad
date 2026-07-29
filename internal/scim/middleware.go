package scim

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const orgContextKey contextKey = "scimOrganizationID"

// Authenticate validates the SCIM provisioning bearer token and injects the
// resolved tenant into the request context. Every SCIM resource route sits
// behind this middleware, so a token can only ever act within its own tenant.
func (h *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

			return
		}

		organizationID, err := h.svc.ResolveOrganization(r.Context(), strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			h.writeError(w, r, http.StatusUnauthorized, "Invalid provisioning token")

			return
		}

		ctx := context.WithValue(r.Context(), orgContextKey, organizationID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func organizationFromContext(ctx context.Context) (string, bool) {
	organizationID, ok := ctx.Value(orgContextKey).(string)

	return organizationID, ok && organizationID != ""
}
