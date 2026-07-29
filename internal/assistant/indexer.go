package assistant

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/knowledge"
)

var errEmbeddingMismatch = errors.New("embedding count does not match chunk count")

// Indexer implements knowledge.Indexer by chunking approved documents,
// embedding each chunk, and upserting the vectors into the tenant-scoped store.
type Indexer struct {
	embeddings EmbeddingProvider
	vectors    VectorStore
}

var _ knowledge.Indexer = (*Indexer)(nil)

// NewIndexer constructs an Indexer.
func NewIndexer(embeddings EmbeddingProvider, vectors VectorStore) *Indexer {
	return &Indexer{embeddings: embeddings, vectors: vectors}
}

// Index makes a document's content retrievable by the assistant. Re-indexing is
// idempotent: the document's existing chunks are replaced wholesale.
func (i *Indexer) Index(ctx context.Context, doc knowledge.Document) error {
	texts := chunkText(doc.Title, doc.Body)
	if len(texts) == 0 {
		// Nothing to index; ensure any stale chunks are cleared.
		if err := i.vectors.DeleteByDocument(ctx, doc.OrganizationID, doc.ID); err != nil {
			return fmt.Errorf("clear empty document chunks: %w", err)
		}

		return nil
	}

	vectors, err := i.embeddings.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed document chunks: %w", err)
	}

	if len(vectors) != len(texts) {
		return fmt.Errorf("%w: %d vs %d", errEmbeddingMismatch, len(vectors), len(texts))
	}

	now := time.Now().UTC()

	chunks := make([]Chunk, len(texts))
	for ordinal, text := range texts {
		chunks[ordinal] = Chunk{
			ID:             uuid.NewString(),
			OrganizationID: doc.OrganizationID,
			DocumentID:     doc.ID,
			DocumentTitle:  doc.Title,
			DocumentURI:    doc.URI,
			AccessScope:    doc.AccessScope,
			Ordinal:        ordinal,
			Text:           text,
			Embedding:      vectors[ordinal],
			CreatedAt:      now,
		}
	}

	if err := i.vectors.UpsertDocumentChunks(ctx, doc.OrganizationID, doc.ID, chunks); err != nil {
		return fmt.Errorf("upsert document chunks: %w", err)
	}

	return nil
}

// Remove withdraws a document's content from the retrieval index.
func (i *Indexer) Remove(ctx context.Context, organizationID, documentID string) error {
	if err := i.vectors.DeleteByDocument(ctx, organizationID, documentID); err != nil {
		return fmt.Errorf("remove document from index: %w", err)
	}

	return nil
}
