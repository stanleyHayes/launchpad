package sso

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/crewjam/saml"
)

const testIDPMetadata = `<?xml version="1.0"?>
<md:EntityDescriptor entityID="https://idp.example.test" xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata">
  <md:IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.test/sso"/>
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`

type testSAMLStore struct {
	config SAMLConfig
}

func (s *testSAMLStore) GetSAMLByOrganization(_ context.Context, organizationID string) (SAMLConfig, error) {
	if s.config.OrganizationID != organizationID {
		return SAMLConfig{}, ErrNotConfigured
	}
	return s.config, nil
}

func (s *testSAMLStore) SetSAMLConfig(_ context.Context, config SAMLConfig) error {
	s.config = config
	return nil
}

type testSAMLStates struct {
	key   string
	value AuthState
}

func (s *testSAMLStates) Save(_ context.Context, key string, value AuthState, _ time.Duration) error {
	s.key, s.value = key, value
	return nil
}

func (s *testSAMLStates) Consume(_ context.Context, key string) (AuthState, error) {
	if key != s.key {
		return AuthState{}, errors.New("missing state")
	}
	return s.value, nil
}

type testSAMLOrg struct{}

func (testSAMLOrg) OrganizationIDBySlug(_ context.Context, slug string) (string, error) {
	if slug != "acme" {
		return "", errors.New("not found")
	}
	return "org-1", nil
}

type testSAMLIssuer struct{}

func (testSAMLIssuer) IssueFederatedSession(context.Context, string, string) (Session, error) {
	return Session{}, nil
}

func TestSAMLStartProducesBoundRequest(t *testing.T) {
	t.Parallel()

	keyPEM, certPEM := testSAMLKeyPair(t)
	store := &testSAMLStore{config: SAMLConfig{
		OrganizationID: "org-1", Enabled: true, IDPMetadataXML: testIDPMetadata,
		EmailAttribute: "email",
	}}
	states := &testSAMLStates{}
	service := NewSAMLService(
		store, states, testSAMLIssuer{}, testSAMLOrg{},
		"https://api.example.test", keyPEM, certPEM,
	)

	destination, err := service.Start(context.Background(), "acme")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	parsed, err := url.Parse(destination)
	if err != nil {
		t.Fatalf("parse destination: %v", err)
	}
	if parsed.Host != "idp.example.test" || parsed.Query().Get("SAMLRequest") == "" {
		t.Fatalf("unexpected SAML destination: %s", destination)
	}
	if parsed.Query().Get("RelayState") != states.key || states.value.OrganizationID != "org-1" ||
		states.value.Provider != "saml" || states.value.Nonce == "" {
		t.Fatalf("state was not bound to request: %+v", states.value)
	}

	metadata, err := service.Metadata(context.Background(), "acme")
	if err != nil || len(metadata) == 0 {
		t.Fatalf("Metadata: len=%d err=%v", len(metadata), err)
	}
}

func TestSAMLConfigRejectsMalformedMetadata(t *testing.T) {
	t.Parallel()

	service := NewSAMLService(
		&testSAMLStore{}, &testSAMLStates{}, testSAMLIssuer{}, testSAMLOrg{},
		"https://api.example.test", "", "",
	)
	_, err := service.SetConfig(context.Background(), "org-1", "acme", SetSAMLConfigInput{
		IDPMetadataXML: "<not-metadata>", EmailAttribute: "mail",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("SetConfig error = %v, want ErrInvalidInput", err)
	}
}

func TestSAMLEmailUsesConfiguredAttributeThenNameID(t *testing.T) {
	t.Parallel()

	assertion := &saml.Assertion{
		AttributeStatements: []saml.AttributeStatement{{
			Attributes: []saml.Attribute{{
				Name:   "mail",
				Values: []saml.AttributeValue{{Value: " Person@Example.COM "}},
			}},
		}},
		Subject: &saml.Subject{NameID: &saml.NameID{Value: "fallback@example.com"}},
	}
	if got := samlEmail(assertion, "mail"); got != "person@example.com" {
		t.Fatalf("samlEmail attribute = %q", got)
	}
	if got := samlEmail(assertion, "unknown"); got != "fallback@example.com" {
		t.Fatalf("samlEmail fallback = %q", got)
	}
}

func testSAMLKeyPair(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "api.example.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})),
		string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}
