package assistant

import "context"

// EmbeddingProvider turns text into vectors. Implementations may be a local
// deterministic embedder (default) or a hosted model behind an HTTP adapter.
type EmbeddingProvider interface {
	// Embed returns one vector per input string, in order.
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

// VectorStore persists and searches chunk embeddings. Every method is
// tenant-scoped by organizationId so retrieval can never cross tenants.
type VectorStore interface {
	EnsureIndexes(ctx context.Context) error
	// UpsertDocumentChunks replaces all chunks for a document with the given set.
	UpsertDocumentChunks(ctx context.Context, organizationID, documentID string, chunks []Chunk) error
	// DeleteByDocument removes all chunks belonging to a document.
	DeleteByDocument(ctx context.Context, organizationID, documentID string) error
	// Search returns the most similar chunks within a tenant, limited to the
	// given access scopes and ranked by descending similarity.
	Search(ctx context.Context, organizationID string, query []float64, scopes []string, limit int) ([]ScoredChunk, error)
	// DeleteForOrganization removes every chunk of the organization and
	// returns the number deleted. Called only by the platform GDPR tenant
	// purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// AnswerGenerator produces a grounded natural-language answer from source
// passages. Implementations must not introduce facts absent from the passages.
type AnswerGenerator interface {
	Generate(ctx context.Context, in GenerateInput) (GeneratedAnswer, error)
}

// ConversationRepository persists interactions and their feedback.
type ConversationRepository interface {
	EnsureIndexes(ctx context.Context) error
	SaveInteraction(ctx context.Context, interaction Interaction) error
	SetFeedback(ctx context.Context, organizationID, interactionID string, helpful bool) error
	// ListInteractions returns all interactions for a tenant, newest first.
	// Read-only; used by analytics reporting.
	ListInteractions(ctx context.Context, organizationID string) ([]Interaction, error)
	// DeleteForOrganization removes every interaction of the organization
	// and returns the number deleted. Called only by the platform GDPR
	// tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}
