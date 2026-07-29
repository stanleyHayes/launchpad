package sso

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/auth"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes SSO HTTP endpoints.
type Handler struct {
	svc           *Service
	saml          *SAMLService
	orgSlugs      OrgSlugResolver
	samlReturnURL string
	// refreshTTL bounds the refresh cookie lifetime and secureCookies marks
	// session cookies Secure; both are wired from config via
	// WithSessionCookies. When unset (zero TTL) no cookies are written.
	refreshTTL    time.Duration
	secureCookies bool
}

func (h *Handler) WithSAML(service *SAMLService, orgSlugs OrgSlugResolver, returnURL string) *Handler {
	h.saml = service
	h.orgSlugs = orgSlugs
	h.samlReturnURL = returnURL

	return h
}

func (h *Handler) HandleGetSAMLConfig(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	slug, err := h.orgSlugs.OrganizationSlugByID(r.Context(), principal.OrganizationID)
	if err != nil {
		writeSSOError(w, r, err)
		return
	}
	config, err := h.saml.GetConfig(r.Context(), principal.OrganizationID, slug)
	if err != nil {
		writeSSOError(w, r, err)
		return
	}
	writeJSON(w, r, config)
}

func (h *Handler) HandleSetSAMLConfig(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	var body struct {
		Enabled        bool   `json:"enabled"`
		IDPMetadataXML string `json:"idpMetadataXml"`
		EmailAttribute string `json:"emailAttribute"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
		return
	}
	slug, err := h.orgSlugs.OrganizationSlugByID(r.Context(), principal.OrganizationID)
	if err != nil {
		writeSSOError(w, r, err)
		return
	}
	config, err := h.saml.SetConfig(r.Context(), principal.OrganizationID, slug, SetSAMLConfigInput{
		Enabled: body.Enabled, IDPMetadataXML: body.IDPMetadataXML, EmailAttribute: body.EmailAttribute,
	})
	if err != nil {
		writeSSOError(w, r, err)
		return
	}
	writeJSON(w, r, config)
}

func (h *Handler) HandleSAMLStart(w http.ResponseWriter, r *http.Request) {
	authURL, err := h.saml.Start(r.Context(), chi.URLParam(r, "orgSlug"))
	if err != nil {
		writeSSOError(w, r, err)
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *Handler) HandleSAMLMetadata(w http.ResponseWriter, r *http.Request) {
	metadata, err := h.saml.Metadata(r.Context(), chi.URLParam(r, "orgSlug"))
	if err != nil {
		writeSSOError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(metadata); err != nil {
		slog.ErrorContext(r.Context(), "write saml metadata", "error", err)
	}
}

func (h *Handler) HandleSAMLACS(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	session, err := h.saml.Callback(r, chi.URLParam(r, "orgSlug"))
	if err != nil {
		writeSSOError(w, r, err)
		return
	}
	if h.refreshTTL > 0 {
		auth.SetSessionCookies(w, auth.TokenPair{
			AccessToken: session.AccessToken, RefreshToken: session.RefreshToken,
			TokenType: session.TokenType, ExpiresIn: session.ExpiresIn,
		}, h.refreshTTL, h.secureCookies)
	}
	target := h.samlReturnURL
	if target == "" {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// NewHandler constructs an SSO Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, refreshTTL: 0, secureCookies: false}
}

// WithSessionCookies enables writing the issued session as HttpOnly cookies
// on callback and returns the handler for chaining.
func (h *Handler) WithSessionCookies(refreshTTL time.Duration, secure bool) *Handler {
	h.refreshTTL = refreshTTL
	h.secureCookies = secure

	return h
}

// HandleGetConfig returns the tenant's OIDC configuration (secret omitted).
func (h *Handler) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	config, err := h.svc.GetConfig(r.Context(), principal.OrganizationID)
	if err != nil {
		writeSSOError(w, r, err)

		return
	}

	writeJSON(w, r, config)
}

// HandleSetConfig stores the tenant's OIDC configuration.
func (h *Handler) HandleSetConfig(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Enabled               bool   `json:"enabled"`
		Issuer                string `json:"issuer"`
		ClientID              string `json:"clientId"`
		ClientSecret          string `json:"clientSecret"`
		AuthorizationEndpoint string `json:"authorizationEndpoint"`
		TokenEndpoint         string `json:"tokenEndpoint"`
		JWKSURI               string `json:"jwksUri"`
		RedirectURI           string `json:"redirectUri"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	config, err := h.svc.SetConfig(r.Context(), principal.OrganizationID, SetConfigInput{
		Enabled:               body.Enabled,
		Issuer:                body.Issuer,
		ClientID:              body.ClientID,
		ClientSecret:          body.ClientSecret,
		AuthorizationEndpoint: body.AuthorizationEndpoint,
		TokenEndpoint:         body.TokenEndpoint,
		JWKSURI:               body.JWKSURI,
		RedirectURI:           body.RedirectURI,
	})
	if err != nil {
		writeSSOError(w, r, err)

		return
	}

	writeJSON(w, r, config)
}

// HandleStart redirects the browser to the tenant IdP's authorization endpoint.
func (h *Handler) HandleStart(w http.ResponseWriter, r *http.Request) {
	authURL, err := h.svc.Start(r.Context(), chi.URLParam(r, "orgSlug"))
	if err != nil {
		writeSSOError(w, r, err)

		return
	}

	// authURL is built from the tenant's admin-configured, https-validated IdP
	// authorization endpoint — not from user input — so this is not an open redirect.
	http.Redirect(w, r, authURL, http.StatusFound) //nolint:gosec // trusted per-tenant IdP endpoint
}

// HandleCallback completes the login and returns the issued session.
func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if idpErr := query.Get("error"); idpErr != "" {
		writeError(w, r, http.StatusUnauthorized, "SSO_DENIED", "Identity provider denied the request")

		return
	}

	session, err := h.svc.Callback(r.Context(), query.Get("state"), query.Get("code"))
	if err != nil {
		writeSSOError(w, r, err)

		return
	}

	if h.refreshTTL > 0 {
		// Browser clients get the same HttpOnly cookie session as password
		// login; the token pair stays in the JSON body for non-browser clients.
		auth.SetSessionCookies(w, auth.TokenPair{
			AccessToken:  session.AccessToken,
			RefreshToken: session.RefreshToken,
			TokenType:    session.TokenType,
			ExpiresIn:    session.ExpiresIn,
		}, h.refreshTTL, h.secureCookies)
	}

	writeJSON(w, r, session)
}

// requirePrincipal authenticates the caller; authorization is handled by the
// route middleware (integrations.manage on the SSO config routes).
func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func writeSSOError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid SSO request")
	case errors.Is(err, ErrInvalidState):
		writeError(w, r, http.StatusBadRequest, "INVALID_STATE", "Invalid or expired login state")
	case errors.Is(err, ErrNotConfigured):
		writeError(w, r, http.StatusNotFound, "SSO_NOT_CONFIGURED", "SSO is not configured")
	case errors.Is(err, ErrNotProvisioned):
		writeError(w, r, http.StatusForbidden, "NOT_PROVISIONED", "No account for this organization")
	case errors.Is(err, ErrVerification):
		writeError(w, r, http.StatusUnauthorized, "SSO_VERIFICATION_FAILED", "Could not verify the identity provider")
	default:
		slog.ErrorContext(r.Context(), "sso handler error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected error")
	}
}

func writeJSON(w http.ResponseWriter, r *http.Request, data any) {
	if err := httpx.WriteJSON(w, http.StatusOK, data); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write error response", "error", err)
	}
}
