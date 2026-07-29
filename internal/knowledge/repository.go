package knowledge

import (
	"context"
	"time"

	"launchpad/internal/notifications"
)

// Repository persists knowledge documents. Every method is tenant-scoped by
// organizationId so no query can cross tenant boundaries.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, doc Document) error
	GetByID(ctx context.Context, organizationID, documentID string) (Document, error)
	List(ctx context.Context, organizationID string) ([]Document, error)
	ListStale(ctx context.Context, now time.Time) ([]Document, error)
	Update(ctx context.Context, doc Document) error
	// DeleteForOrganization removes every knowledge document of the
	// organization and returns the number deleted. Called only by the
	// platform GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// Connector retrieves the current text of a configured external source.
type Connector interface {
	Fetch(ctx context.Context, source, uri string) (string, error)
}

type Notifier interface {
	Create(ctx context.Context, organizationID string, in notifications.CreateInput) (notifications.Notification, error)
}

// Indexer hands approved document content to the AI retrieval layer.
//
// The concrete adapter (built with the AI assistant module) chunks Body,
// generates embeddings with Claude, and upserts them into MongoDB Atlas Vector
// Search keyed by organizationId. It is defined as a port here so document
// management stays independent of the retrieval stack; NoopIndexer lets the
// module run before that adapter exists.
type Indexer interface {
	// Index makes a document's content retrievable by the assistant.
	Index(ctx context.Context, doc Document) error
	// Remove withdraws a document's content from the retrieval index.
	Remove(ctx context.Context, organizationID, documentID string) error
}

// NoopIndexer satisfies Indexer without performing any indexing. It is the
// default until the Claude + Mongo vector adapter lands with the assistant.
type NoopIndexer struct{}

// Index does nothing and reports success.
func (NoopIndexer) Index(context.Context, Document) error { return nil }

// Remove does nothing and reports success.
func (NoopIndexer) Remove(context.Context, string, string) error { return nil }
