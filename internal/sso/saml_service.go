package sso

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"

	"launchpad/pkg/security"
)

const defaultSAMLEmailAttribute = "email"

// SAMLService implements tenant-configured, SP-initiated SAML Web SSO.
type SAMLService struct {
	configs SAMLConfigStore
	states  StateStore
	issuer  SessionIssuer
	orgs    OrgResolver
	baseURL string
	key     crypto.Signer
	cert    *x509.Certificate
}

func NewSAMLService(
	configs SAMLConfigStore,
	states StateStore,
	issuer SessionIssuer,
	orgs OrgResolver,
	baseURL, privateKeyPEM, certificatePEM string,
) *SAMLService {
	service := &SAMLService{
		configs: configs, states: states, issuer: issuer, orgs: orgs,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
	pair, err := tls.X509KeyPair([]byte(certificatePEM), []byte(privateKeyPEM))
	if err == nil && len(pair.Certificate) > 0 {
		service.key, _ = pair.PrivateKey.(crypto.Signer)
		service.cert, _ = x509.ParseCertificate(pair.Certificate[0])
	}

	return service
}

func (s *SAMLService) GetConfig(ctx context.Context, organizationID, orgSlug string) (SAMLConfigResponse, error) {
	config, err := s.configs.GetSAMLByOrganization(ctx, organizationID)
	if err != nil {
		if !errors.Is(err, ErrNotConfigured) {
			return SAMLConfigResponse{}, fmt.Errorf("get saml config: %w", err)
		}
		config = SAMLConfig{OrganizationID: organizationID}
	}

	return s.response(config, orgSlug), nil
}

func (s *SAMLService) SetConfig(
	ctx context.Context,
	organizationID, orgSlug string,
	input SetSAMLConfigInput,
) (SAMLConfigResponse, error) {
	metadataXML := strings.TrimSpace(input.IDPMetadataXML)
	if metadataXML == "" {
		existing, err := s.configs.GetSAMLByOrganization(ctx, organizationID)
		if err == nil {
			metadataXML = existing.IDPMetadataXML
		} else if !errors.Is(err, ErrNotConfigured) {
			return SAMLConfigResponse{}, fmt.Errorf("get existing saml config: %w", err)
		}
	}
	emailAttribute := strings.TrimSpace(input.EmailAttribute)
	if emailAttribute == "" {
		emailAttribute = defaultSAMLEmailAttribute
	}
	if organizationID == "" || orgSlug == "" || (input.Enabled && metadataXML == "") {
		return SAMLConfigResponse{}, ErrInvalidInput
	}
	if metadataXML != "" {
		if _, err := samlsp.ParseMetadata([]byte(metadataXML)); err != nil {
			return SAMLConfigResponse{}, ErrInvalidInput
		}
	}
	if input.Enabled && (s.key == nil || s.cert == nil) {
		return SAMLConfigResponse{}, ErrNotConfigured
	}
	config := SAMLConfig{
		OrganizationID: organizationID, Enabled: input.Enabled,
		IDPMetadataXML: metadataXML, EmailAttribute: emailAttribute, UpdatedAt: time.Now().UTC(),
	}
	if err := s.configs.SetSAMLConfig(ctx, config); err != nil {
		return SAMLConfigResponse{}, fmt.Errorf("set saml config: %w", err)
	}

	return s.response(config, orgSlug), nil
}

func (s *SAMLService) Start(ctx context.Context, orgSlug string) (string, error) {
	organizationID, config, sp, err := s.provider(ctx, orgSlug)
	if err != nil {
		return "", err
	}
	request, err := sp.MakeAuthenticationRequest(
		sp.GetSSOBindingLocation(saml.HTTPRedirectBinding),
		saml.HTTPRedirectBinding,
		saml.HTTPPostBinding,
	)
	if err != nil {
		return "", fmt.Errorf("%w: create saml request: %w", ErrVerification, err)
	}
	state, err := security.NewRefreshToken()
	if err != nil {
		return "", fmt.Errorf("generate saml state: %w", err)
	}
	if err := s.states.Save(ctx, state, AuthState{
		OrganizationID: organizationID, Nonce: request.ID, Provider: "saml",
	}, stateTTL); err != nil {
		return "", fmt.Errorf("save saml state: %w", err)
	}
	destination, err := request.Redirect(state, sp)
	if err != nil {
		return "", fmt.Errorf("%w: encode saml request: %w", ErrVerification, err)
	}
	_ = config

	return destination.String(), nil
}

func (s *SAMLService) Callback(r *http.Request, orgSlug string) (Session, error) {
	if err := r.ParseForm(); err != nil {
		return Session{}, ErrInvalidInput
	}
	stateValue := r.Form.Get("RelayState")
	if stateValue == "" {
		return Session{}, ErrInvalidState
	}
	authState, err := s.states.Consume(r.Context(), stateValue)
	if err != nil || authState.Provider != "saml" {
		return Session{}, ErrInvalidState
	}
	organizationID, config, sp, err := s.provider(r.Context(), orgSlug)
	if err != nil || organizationID != authState.OrganizationID {
		return Session{}, ErrInvalidState
	}
	assertion, err := sp.ParseResponse(r, []string{authState.Nonce})
	if err != nil {
		return Session{}, fmt.Errorf("%w: parse saml assertion: %w", ErrVerification, err)
	}
	email := samlEmail(assertion, config.EmailAttribute)
	if email == "" {
		return Session{}, ErrVerification
	}
	session, err := s.issuer.IssueFederatedSession(r.Context(), email, organizationID)
	if err != nil {
		return Session{}, fmt.Errorf("issue saml session: %w", err)
	}

	return session, nil
}

func (s *SAMLService) Metadata(ctx context.Context, orgSlug string) ([]byte, error) {
	if s.key == nil || s.cert == nil {
		return nil, ErrNotConfigured
	}
	if _, err := s.orgs.OrganizationIDBySlug(ctx, strings.TrimSpace(orgSlug)); err != nil {
		return nil, ErrNotConfigured
	}
	sp := s.baseProvider(orgSlug, nil)
	document, err := xml.MarshalIndent(sp.Metadata(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal saml metadata: %w", err)
	}

	return append([]byte(xml.Header), document...), nil
}

func (s *SAMLService) provider(
	ctx context.Context,
	orgSlug string,
) (string, SAMLConfig, *saml.ServiceProvider, error) {
	if s.key == nil || s.cert == nil {
		return "", SAMLConfig{}, nil, ErrNotConfigured
	}
	organizationID, err := s.orgs.OrganizationIDBySlug(ctx, strings.TrimSpace(orgSlug))
	if err != nil {
		return "", SAMLConfig{}, nil, ErrNotConfigured
	}
	config, err := s.configs.GetSAMLByOrganization(ctx, organizationID)
	if err != nil || !config.Enabled {
		return "", SAMLConfig{}, nil, ErrNotConfigured
	}
	metadata, err := samlsp.ParseMetadata([]byte(config.IDPMetadataXML))
	if err != nil {
		return "", SAMLConfig{}, nil, ErrNotConfigured
	}
	sp := s.baseProvider(orgSlug, metadata)

	return organizationID, config, sp, nil
}

func (s *SAMLService) baseProvider(orgSlug string, metadata *saml.EntityDescriptor) *saml.ServiceProvider {
	metadataURL, _ := url.Parse(s.metadataURL(orgSlug))
	acsURL, _ := url.Parse(s.acsURL(orgSlug))

	return &saml.ServiceProvider{
		EntityID: metadataURL.String(), Key: s.key, Certificate: s.cert,
		MetadataURL: *metadataURL, AcsURL: *acsURL, IDPMetadata: metadata,
		AuthnNameIDFormat: saml.EmailAddressNameIDFormat,
	}
}

func (s *SAMLService) response(config SAMLConfig, orgSlug string) SAMLConfigResponse {
	return SAMLConfigResponse{
		OrganizationID: config.OrganizationID, Enabled: config.Enabled,
		Configured: config.IDPMetadataXML != "", EmailAttribute: config.EmailAttribute,
		MetadataURL: s.metadataURL(orgSlug), ACSURL: s.acsURL(orgSlug),
		EntityID: s.metadataURL(orgSlug), UpdatedAt: config.UpdatedAt,
	}
}

func (s *SAMLService) metadataURL(slug string) string {
	return s.baseURL + "/api/v1/auth/saml/" + url.PathEscape(slug) + "/metadata"
}

func (s *SAMLService) acsURL(slug string) string {
	return s.baseURL + "/api/v1/auth/saml/" + url.PathEscape(slug) + "/acs"
}

func samlEmail(assertion *saml.Assertion, attributeName string) string {
	for _, statement := range assertion.AttributeStatements {
		for _, attribute := range statement.Attributes {
			if attribute.Name != attributeName && attribute.FriendlyName != attributeName {
				continue
			}
			for _, value := range attribute.Values {
				if email := strings.TrimSpace(value.Value); email != "" {
					return strings.ToLower(email)
				}
			}
		}
	}
	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		return strings.ToLower(strings.TrimSpace(assertion.Subject.NameID.Value))
	}

	return ""
}
