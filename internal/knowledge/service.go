package knowledge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/entitlements"
	"launchpad/internal/notifications"
)

type PlanReader interface {
	PlanCode(ctx context.Context, organizationID string) (string, error)
}

// Service implements knowledge document use cases.
type Service struct {
	repo      Repository
	indexer   Indexer
	connector Connector
	notifier  Notifier
	plans     PlanReader
}

func (s *Service) WithPlanLimits(plans PlanReader) *Service {
	s.plans = plans
	return s
}

func (s *Service) WithConnector(connector Connector) *Service {
	s.connector = connector
	return s
}

func (s *Service) WithNotifier(notifier Notifier) *Service {
	s.notifier = notifier
	return s
}

// NewService constructs a Service.
func NewService(repo Repository, indexer Indexer) *Service {
	return &Service{repo: repo, indexer: indexer}
}

// Create registers a new draft knowledge document for a tenant.
func (s *Service) Create(
	ctx context.Context,
	organizationID, createdByUserID string,
	in CreateInput,
) (Document, error) {
	title := strings.TrimSpace(in.Title)
	if organizationID == "" || createdByUserID == "" || title == "" {
		return Document{}, ErrInvalidInput
	}
	if s.plans != nil {
		planCode, err := s.plans.PlanCode(ctx, organizationID)
		if err != nil {
			return Document{}, fmt.Errorf("read organization plan: %w", err)
		}
		items, err := s.repo.List(ctx, organizationID)
		if err != nil {
			return Document{}, fmt.Errorf("count knowledge documents for plan limit: %w", err)
		}
		if err := entitlements.Check(planCode, entitlements.ResourceKnowledgeDocuments, len(items)); err != nil {
			return Document{}, err
		}
	}

	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = sourceManual
	}

	if !isValidSource(source) {
		return Document{}, ErrInvalidInput
	}

	scope := strings.TrimSpace(in.AccessScope)
	if scope == "" {
		scope = ScopeOrganization
	}

	if !isValidScope(scope) {
		return Document{}, ErrInvalidInput
	}
	if in.RetentionDays < 0 {
		return Document{}, ErrInvalidInput
	}

	owner := strings.TrimSpace(in.OwnerUserID)
	if owner == "" {
		owner = createdByUserID
	}

	now := time.Now().UTC()

	doc := Document{
		ID:              uuid.NewString(),
		OrganizationID:  organizationID,
		Title:           title,
		Source:          source,
		URI:             strings.TrimSpace(in.URI),
		Body:            in.Body,
		Tags:            in.Tags,
		AccessScope:     scope,
		Status:          StatusDraft,
		Version:         1,
		OwnerUserID:     owner,
		ReviewDate:      in.ReviewDate,
		RetentionDays:   in.RetentionDays,
		History:         []VersionSnapshot{},
		CreatedByUserID: createdByUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repo.Create(ctx, doc); err != nil {
		return Document{}, fmt.Errorf("create knowledge document: %w", err)
	}

	return doc, nil
}

// List returns a tenant's documents. Restricted documents are omitted unless
// includeRestricted is set (managers only).
func (s *Service) List(
	ctx context.Context,
	organizationID string,
	includeRestricted bool,
) ([]Document, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.repo.List(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list knowledge documents: %w", err)
	}

	if includeRestricted {
		return items, nil
	}

	visible := make([]Document, 0, len(items))
	for _, doc := range items {
		if doc.AccessScope != ScopeRestricted {
			visible = append(visible, doc)
		}
	}

	return visible, nil
}

// Get returns one document scoped to the tenant. Restricted documents are
// hidden from non-managers by returning ErrNotFound so their existence is not
// disclosed.
func (s *Service) Get(
	ctx context.Context,
	organizationID, documentID string,
	includeRestricted bool,
) (Document, error) {
	doc, err := s.load(ctx, organizationID, documentID)
	if err != nil {
		return Document{}, err
	}

	if doc.AccessScope == ScopeRestricted && !includeRestricted {
		return Document{}, ErrNotFound
	}

	return doc, nil
}

// Update mutates editable fields of a draft document.
func (s *Service) Update(
	ctx context.Context,
	organizationID, documentID string,
	in UpdateInput,
) (Document, error) {
	doc, err := s.load(ctx, organizationID, documentID)
	if err != nil {
		return Document{}, err
	}

	if doc.Status != StatusDraft {
		return Document{}, ErrInvalidState
	}

	doc.History = append(doc.History, snapshotDocument(doc))
	doc, err = applyUpdate(doc, in)
	if err != nil {
		return Document{}, err
	}
	doc.Version++

	doc.UpdatedAt = time.Now().UTC()
	doc.StaleNotifiedAt = nil
	if err := s.repo.Update(ctx, doc); err != nil {
		return Document{}, fmt.Errorf("update knowledge document: %w", err)
	}

	return doc, nil
}

// Sync refreshes a connector-backed document. Changed content always returns
// to draft so a human must approve it before the new version is indexed.
func (s *Service) Sync(ctx context.Context, organizationID, documentID string) (Document, error) {
	doc, err := s.load(ctx, organizationID, documentID)
	if err != nil {
		return Document{}, err
	}
	if s.connector == nil || doc.URI == "" || doc.Source == sourceManual || doc.Source == sourceUpload {
		return Document{}, ErrInvalidState
	}

	body, err := s.connector.Fetch(ctx, doc.Source, doc.URI)
	now := time.Now().UTC()
	if err != nil {
		doc.SyncError = err.Error()
		doc.UpdatedAt = now
		_ = s.repo.Update(ctx, doc)
		return Document{}, fmt.Errorf("sync knowledge connector: %w", err)
	}
	if body != doc.Body {
		if doc.Status == StatusIndexed {
			if err := s.indexer.Remove(ctx, organizationID, doc.ID); err != nil {
				return Document{}, fmt.Errorf("remove changed knowledge document from index: %w", err)
			}
		}
		doc.History = append(doc.History, snapshotDocument(doc))
		doc.Version++
		doc.Body = body
		doc.Status = StatusDraft
		doc.ApprovedByUserID = ""
		doc.ApprovedAt, doc.IndexedAt = nil, nil
	}
	doc.LastSyncedAt = &now
	doc.StaleNotifiedAt = nil
	doc.SyncError = ""
	doc.UpdatedAt = now
	if err := s.repo.Update(ctx, doc); err != nil {
		return Document{}, fmt.Errorf("persist knowledge sync: %w", err)
	}
	return doc, nil
}

// NotifyStale sends one reminder per stale period to each document owner.
func (s *Service) NotifyStale(ctx context.Context) error {
	if s.notifier == nil {
		return nil
	}
	now := time.Now().UTC()
	docs, err := s.repo.ListStale(ctx, now)
	if err != nil {
		return fmt.Errorf("list stale knowledge documents: %w", err)
	}
	for _, doc := range docs {
		if doc.StaleNotifiedAt != nil || doc.OwnerUserID == "" || doc.Status == StatusArchived {
			continue
		}
		if _, err := s.notifier.Create(ctx, doc.OrganizationID, notifications.CreateInput{
			UserID: doc.OwnerUserID,
			Type:   notifications.TypeSystem,
			Title:  "Knowledge review due",
			Body:   fmt.Sprintf("%s is due for review or synchronization.", doc.Title),
			Link:   "/knowledge",
		}); err != nil {
			return fmt.Errorf("notify stale knowledge owner: %w", err)
		}
		doc.StaleNotifiedAt = &now
		if err := s.repo.Update(ctx, doc); err != nil {
			return fmt.Errorf("mark stale knowledge notification: %w", err)
		}
	}
	return nil
}

func (s *Service) History(
	ctx context.Context,
	organizationID, documentID string,
) ([]VersionSnapshot, error) {
	doc, err := s.load(ctx, organizationID, documentID)
	if err != nil {
		return nil, err
	}
	items := append([]VersionSnapshot(nil), doc.History...)
	items = append(items, snapshotDocument(doc))
	return items, nil
}

func (s *Service) CreateNewVersion(
	ctx context.Context,
	organizationID, documentID string,
) (Document, error) {
	doc, err := s.load(ctx, organizationID, documentID)
	if err != nil {
		return Document{}, err
	}
	if doc.Status == StatusDraft {
		return Document{}, ErrInvalidState
	}
	if doc.Status == StatusIndexed {
		if err := s.indexer.Remove(ctx, organizationID, doc.ID); err != nil {
			return Document{}, fmt.Errorf("remove prior knowledge version from index: %w", err)
		}
	}
	doc.History = append(doc.History, snapshotDocument(doc))
	doc.Version++
	doc.Status = StatusDraft
	doc.ApprovedAt, doc.IndexedAt = nil, nil
	doc.ApprovedByUserID = ""
	doc.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, doc); err != nil {
		return Document{}, fmt.Errorf("create knowledge version: %w", err)
	}
	return doc, nil
}

func snapshotDocument(doc Document) VersionSnapshot {
	return VersionSnapshot{
		Version: doc.Version, Title: doc.Title, Body: doc.Body, URI: doc.URI,
		Tags: append([]string(nil), doc.Tags...), AccessScope: doc.AccessScope,
		SavedAt: doc.UpdatedAt,
	}
}

// applyUpdate overlays the non-nil fields of in onto doc, validating each.
func applyUpdate(doc Document, in UpdateInput) (Document, error) {
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			return Document{}, ErrInvalidInput
		}

		doc.Title = title
	}

	if in.URI != nil {
		doc.URI = strings.TrimSpace(*in.URI)
	}

	if in.Body != nil {
		doc.Body = *in.Body
	}

	if in.Tags != nil {
		doc.Tags = *in.Tags
	}

	if in.AccessScope != nil {
		scope := strings.TrimSpace(*in.AccessScope)
		if !isValidScope(scope) {
			return Document{}, ErrInvalidInput
		}

		doc.AccessScope = scope
	}

	if in.OwnerUserID != nil {
		owner := strings.TrimSpace(*in.OwnerUserID)
		if owner == "" {
			return Document{}, ErrInvalidInput
		}

		doc.OwnerUserID = owner
	}

	if in.ReviewDate != nil {
		doc.ReviewDate = in.ReviewDate
	}
	if in.RetentionDays != nil {
		if *in.RetentionDays < 0 {
			return Document{}, ErrInvalidInput
		}
		doc.RetentionDays = *in.RetentionDays
	}

	return doc, nil
}

// Approve moves a draft document to the approved state, recording the approver.
// Approval is the human gate that must precede AI indexing.
func (s *Service) Approve(
	ctx context.Context,
	organizationID, documentID, approverUserID string,
) (Document, error) {
	if approverUserID == "" {
		return Document{}, ErrInvalidInput
	}

	doc, err := s.load(ctx, organizationID, documentID)
	if err != nil {
		return Document{}, err
	}

	if doc.Status != StatusDraft {
		return Document{}, ErrInvalidState
	}

	now := time.Now().UTC()

	doc.Status = StatusApproved
	doc.ApprovedByUserID = approverUserID
	doc.ApprovedAt = &now
	doc.UpdatedAt = now

	if err := s.repo.Update(ctx, doc); err != nil {
		return Document{}, fmt.Errorf("approve knowledge document: %w", err)
	}

	return doc, nil
}

// Index hands an approved document to the AI indexer and marks it indexed.
func (s *Service) Index(ctx context.Context, organizationID, documentID string) (Document, error) {
	doc, err := s.load(ctx, organizationID, documentID)
	if err != nil {
		return Document{}, err
	}

	if doc.Status != StatusApproved {
		return Document{}, ErrInvalidState
	}

	if err := s.indexer.Index(ctx, doc); err != nil {
		return Document{}, fmt.Errorf("index knowledge document: %w", err)
	}

	now := time.Now().UTC()

	doc.Status = StatusIndexed
	doc.IndexedAt = &now
	doc.UpdatedAt = now

	if err := s.repo.Update(ctx, doc); err != nil {
		return Document{}, fmt.Errorf("persist indexed knowledge document: %w", err)
	}

	return doc, nil
}

// Archive withdraws a document from the AI index and marks it archived.
func (s *Service) Archive(ctx context.Context, organizationID, documentID string) (Document, error) {
	doc, err := s.load(ctx, organizationID, documentID)
	if err != nil {
		return Document{}, err
	}

	if doc.Status == StatusArchived {
		return Document{}, ErrInvalidState
	}

	if err := s.indexer.Remove(ctx, organizationID, documentID); err != nil {
		return Document{}, fmt.Errorf("remove knowledge document from index: %w", err)
	}

	doc.Status = StatusArchived
	doc.IndexedAt = nil
	doc.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, doc); err != nil {
		return Document{}, fmt.Errorf("archive knowledge document: %w", err)
	}

	return doc, nil
}

func (s *Service) load(ctx context.Context, organizationID, documentID string) (Document, error) {
	if organizationID == "" || documentID == "" {
		return Document{}, ErrInvalidInput
	}

	doc, err := s.repo.GetByID(ctx, organizationID, documentID)
	if err != nil {
		return Document{}, fmt.Errorf("get knowledge document: %w", err)
	}

	return doc, nil
}
