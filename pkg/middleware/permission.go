package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"launchpad/pkg/security"
)

// PermissionResolver resolves the effective permission set of a membership
// role within an organization. internal/roles.Service satisfies it; the app
// layer injects it so this package stays domain-free.
type PermissionResolver interface {
	ResolvePermissions(ctx context.Context, organizationID, roleCode string) (map[string]struct{}, error)
}

// RequirePermission returns middleware that allows the request only when the
// authenticated principal's role grants permission (PRD 6.3
// `resource.action`). Permissions are resolved once per request — there is no
// global cache — and every failure mode fails closed: no principal is 401, a
// resolver error is 500, and a missing permission is 403 FORBIDDEN.
//
// Platform support impersonation principals (PRD 5.2.2) bypass the resolver:
// they hold exactly the fixed read-only security.ImpersonatorPermissions set,
// regardless of the tenant role the token carries, so an impersonation token
// can never mutate tenant state, change billing, or manage members.
func RequirePermission(resolver PermissionResolver, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := security.PrincipalFromContext(r.Context())
			if !ok {
				writeAuthFailure(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

				return
			}

			if principal.Impersonator {
				if _, granted := security.ImpersonatorPermissions()[permission]; !granted {
					writeAuthFailure(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")

					return
				}

				next.ServeHTTP(w, r)

				return
			}

			permissions, err := resolver.ResolvePermissions(r.Context(), principal.OrganizationID, principal.RoleCode)
			if err != nil {
				slog.ErrorContext(r.Context(), "resolve permissions failed", "error", err)
				writeAuthFailure(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to resolve permissions")

				return
			}

			if _, granted := permissions[permission]; !granted {
				writeAuthFailure(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
