package integrations

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/entitlements"
)

type PlanReader interface {
	PlanCode(ctx context.Context, organizationID string) (string, error)
}

// Service implements the provider-connection use cases.
type Service struct {
	repo    Repository
	audit   AuditRecorder
	clients map[string]providerClient
	plans   PlanReader
}

func (s *Service) WithPlanLimits(plans PlanReader) *Service {
	s.plans = plans
	return s
}

// NewService constructs a Service with the credential validators for each
// supported provider.
func NewService(repo Repository, audit AuditRecorder, github, jira providerClient) *Service {
	return &Service{
		repo:    repo,
		audit:   audit,
		clients: map[string]providerClient{ProviderGitHub: github, ProviderJira: jira},
	}
}

// Connect validates the credential against the provider BEFORE persisting
// anything, then upserts the tenant's connection and records an audit event.
// Reconnecting replaces the existing connection (idempotent upsert).
func (s *Service) Connect(
	ctx context.Context,
	organizationID, actorUserID, providerKey string,
	in ConnectInput,
) (ConnectionResponse, error) {
	info, err := providerInfoFor(providerKey)
	if err != nil {
		return ConnectionResponse{}, err
	}

	if err := validateConnectInput(info, organizationID, in); err != nil {
		return ConnectionResponse{}, err
	}
	if s.plans != nil {
		if _, err := s.repo.Get(ctx, organizationID, providerKey); errors.Is(err, ErrNotFound) {
			planCode, planErr := s.plans.PlanCode(ctx, organizationID)
			if planErr != nil {
				return ConnectionResponse{}, fmt.Errorf("read organization plan: %w", planErr)
			}
			items, listErr := s.repo.List(ctx, organizationID)
			if listErr != nil {
				return ConnectionResponse{}, fmt.Errorf("count integrations for plan limit: %w", listErr)
			}
			if limitErr := entitlements.Check(planCode, entitlements.ResourceIntegrations, len(items)); limitErr != nil {
				return ConnectionResponse{}, limitErr
			}
		} else if err != nil {
			return ConnectionResponse{}, fmt.Errorf("check existing integration: %w", err)
		}
	}

	handle, err := s.validateCredential(ctx, providerKey, in)
	if err != nil {
		return ConnectionResponse{}, err
	}

	conn, err := s.buildConnection(ctx, organizationID, actorUserID, providerKey, in, handle)
	if err != nil {
		return ConnectionResponse{}, err
	}

	if err := s.repo.Upsert(ctx, conn); err != nil {
		return ConnectionResponse{}, fmt.Errorf("upsert integration connection: %w", err)
	}

	if err := s.recordAudit(ctx, organizationID, actorUserID, "integration.connected", providerKey, map[string]any{
		"provider":      providerKey,
		"accountHandle": handle,
		"connectionId":  conn.ID,
	}); err != nil {
		return ConnectionResponse{}, fmt.Errorf("record integration connected audit: %w", err)
	}

	return conn.ToResponse(), nil
}

// Disconnect removes the tenant's connection for a provider (org-scoped) and
// records an audit event.
func (s *Service) Disconnect(ctx context.Context, organizationID, actorUserID, providerKey string) error {
	if organizationID == "" {
		return ErrInvalidInput
	}

	if _, err := providerInfoFor(providerKey); err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, organizationID, providerKey); err != nil {
		return err
	}

	if err := s.recordAudit(ctx, organizationID, actorUserID, "integration.disconnected", providerKey, map[string]any{
		"provider": providerKey,
	}); err != nil {
		return fmt.Errorf("record integration disconnected audit: %w", err)
	}

	return nil
}

// List returns the tenant's connections as masked DTOs (no credential fields).
func (s *Service) List(ctx context.Context, organizationID string) ([]ConnectionResponse, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	connections, err := s.repo.List(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list integration connections: %w", err)
	}

	return ToResponses(connections), nil
}

// Health re-validates the stored credential against the provider and records
// the outcome on the connection: status connected/error, lastError, and
// lastSyncAt are always updated. A failed validation returns the error after
// the status has been persisted.
func (s *Service) Health(ctx context.Context, organizationID, providerKey string) (ConnectionResponse, error) {
	if organizationID == "" {
		return ConnectionResponse{}, ErrInvalidInput
	}

	if _, err := providerInfoFor(providerKey); err != nil {
		return ConnectionResponse{}, err
	}

	conn, err := s.repo.Get(ctx, organizationID, providerKey)
	if err != nil {
		return ConnectionResponse{}, err
	}

	handle, validateErr := s.validateCredential(ctx, providerKey, ConnectInput{
		Token:   conn.Token,
		BaseURL: conn.BaseURL,
		Email:   conn.Email,
	})

	now := time.Now().UTC()
	conn.LastSyncAt = &now
	conn.UpdatedAt = now

	if validateErr != nil {
		conn.Status = StatusError
		conn.LastError = healthErrorMessage(validateErr)
	} else {
		conn.Status = StatusConnected
		conn.LastError = ""
		conn.AccountHandle = handle
	}

	if err := s.repo.Upsert(ctx, conn); err != nil {
		return ConnectionResponse{}, fmt.Errorf("update connection health: %w", err)
	}

	if validateErr != nil {
		return ConnectionResponse{}, fmt.Errorf("integration health check: %w", validateErr)
	}

	return conn.ToResponse(), nil
}

// buildConnection constructs the connection to persist, preserving identity
// fields (id, createdBy, createdAt) when reconnecting an existing provider.
func (s *Service) buildConnection(
	ctx context.Context,
	organizationID, actorUserID, providerKey string,
	in ConnectInput,
	handle string,
) (Connection, error) {
	now := time.Now().UTC()
	conn := Connection{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		Provider:       providerKey,
		Status:         StatusConnected,
		BaseURL:        strings.TrimSpace(in.BaseURL),
		Email:          strings.TrimSpace(in.Email),
		AccountHandle:  handle,
		Token:          in.Token,
		LastSyncAt:     &now,
		LastError:      "",
		CreatedBy:      actorUserID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	existing, err := s.repo.Get(ctx, organizationID, providerKey)
	switch {
	case err == nil:
		conn.ID = existing.ID
		conn.CreatedBy = existing.CreatedBy
		conn.CreatedAt = existing.CreatedAt
	case errors.Is(err, ErrNotFound):
	default:
		return Connection{}, fmt.Errorf("load existing connection: %w", err)
	}

	return conn, nil
}

// validateCredential calls the provider's validator. Validation happens before
// any persistence so a rejected credential is never stored.
func (s *Service) validateCredential(ctx context.Context, providerKey string, in ConnectInput) (string, error) {
	client, ok := s.clients[providerKey]
	if !ok || client == nil {
		return "", ErrUnknownProvider
	}

	handle, err := client.Validate(ctx, in)
	if err != nil {
		return "", fmt.Errorf("validate provider credential: %w", err)
	}

	return handle, nil
}

func (s *Service) recordAudit(
	ctx context.Context,
	organizationID, actorUserID, action, providerKey string,
	metadata map[string]any,
) error {
	orgID := organizationID
	if err := s.audit.Record(ctx, &orgID, actorUserID, action, "integration", providerKey, metadata); err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}

	return nil
}

func providerInfoFor(key string) (ProviderInfo, error) {
	for _, info := range Providers() {
		if info.Key == key {
			return info, nil
		}
	}

	return ProviderInfo{}, ErrUnknownProvider
}

func validateConnectInput(info ProviderInfo, organizationID string, in ConnectInput) error {
	if organizationID == "" || strings.TrimSpace(in.Token) == "" {
		return ErrInvalidInput
	}

	if info.RequiresBaseURL && strings.TrimSpace(in.BaseURL) == "" {
		return ErrInvalidInput
	}

	if info.RequiresEmail && strings.TrimSpace(in.Email) == "" {
		return ErrInvalidInput
	}

	if baseURL := strings.TrimSpace(in.BaseURL); baseURL != "" && !isHTTPSURL(baseURL) {
		return ErrInvalidInput
	}

	return nil
}

// healthErrorMessage reduces a validation failure to a generic, secret-free
// message safe to store and return to clients.
func healthErrorMessage(err error) string {
	if errors.Is(err, ErrInvalidCredential) {
		return "credential rejected by provider"
	}

	return "provider validation failed"
}

func isHTTPSURL(raw string) bool {
	parsed, err := url.Parse(raw)

	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}
