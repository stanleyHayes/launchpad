// Package mongo is the MongoDB persistence adapter for the assistant domain.
//
// VectorStore keeps chunk embeddings in a standard MongoDB collection and ranks
// them with an in-process cosine similarity, so it runs on any MongoDB
// deployment (including the local docker-compose instance). For large corpora,
// swap Search for a MongoDB Atlas Vector Search `$vectorSearch` aggregation
// behind the same assistant.VectorStore port — no change to callers.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"launchpad/internal/assistant"
)

const (
	fieldID             = "_id"
	fieldOrganizationID = "organizationId"
	fieldDocumentID     = "documentId"
	fieldAccessScope    = "accessScope"
	fieldUserID         = "userId"
	fieldHelpful        = "helpful"
	fieldCreatedAt      = "createdAt"
)

var _ assistant.VectorStore = (*VectorStore)(nil)

// VectorStore persists and searches chunk embeddings for a tenant.
type VectorStore struct {
	chunks *drivermongo.Collection
}

// NewVectorStore constructs a VectorStore.
func NewVectorStore(db *drivermongo.Database) *VectorStore {
	return &VectorStore{chunks: db.Collection("assistant_chunks")}
}

// EnsureIndexes creates assistant chunk indexes.
func (s *VectorStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.chunks.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldDocumentID, Value: 1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldAccessScope, Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure assistant chunk indexes: %w", err)
	}

	return nil
}

// UpsertDocumentChunks replaces all chunks for a document. The new chunks are
// inserted first and the document's older chunks deleted afterwards, so a failed
// insert never leaves the document with zero indexed chunks (it keeps serving
// the previous generation until the next successful re-index). This avoids the
// empty-window a delete-then-insert would open without needing a transaction,
// so it works on standalone MongoDB as well as an Atlas replica set.
func (s *VectorStore) UpsertDocumentChunks(
	ctx context.Context,
	organizationID, documentID string,
	chunks []assistant.Chunk,
) error {
	if len(chunks) == 0 {
		return s.DeleteByDocument(ctx, organizationID, documentID)
	}

	docs := make([]any, len(chunks))
	newIDs := make([]string, len(chunks))

	for i, chunk := range chunks {
		docs[i] = chunk
		newIDs[i] = chunk.ID
	}

	if _, err := s.chunks.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("insert assistant chunks: %w", err)
	}

	_, err := s.chunks.DeleteMany(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldDocumentID:     documentID,
		fieldID:             bson.M{"$nin": newIDs},
	})
	if err != nil {
		return fmt.Errorf("delete superseded assistant chunks: %w", err)
	}

	return nil
}

// DeleteByDocument removes all chunks belonging to a document.
func (s *VectorStore) DeleteByDocument(ctx context.Context, organizationID, documentID string) error {
	_, err := s.chunks.DeleteMany(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldDocumentID:     documentID,
	})
	if err != nil {
		return fmt.Errorf("delete assistant chunks: %w", err)
	}

	return nil
}

// Search returns the most similar chunks within a tenant and access scopes.
func (s *VectorStore) Search(
	ctx context.Context,
	organizationID string,
	query []float64,
	scopes []string,
	limit int,
) ([]assistant.ScoredChunk, error) {
	if len(query) == 0 || limit <= 0 || len(scopes) == 0 {
		return nil, nil
	}

	cursor, err := s.chunks.Find(ctx, bson.M{
		fieldOrganizationID: organizationID,
		fieldAccessScope:    bson.M{"$in": scopes},
	})
	if err != nil {
		return nil, fmt.Errorf("find assistant chunks: %w", err)
	}

	items := make([]assistant.Chunk, 0)
	if err := drainCursor(ctx, cursor, &items); err != nil {
		return nil, err
	}

	return rank(query, items, limit), nil
}

func rank(query []float64, chunks []assistant.Chunk, limit int) []assistant.ScoredChunk {
	scored := make([]assistant.ScoredChunk, 0, len(chunks))
	for _, chunk := range chunks {
		scored = append(scored, assistant.ScoredChunk{Chunk: chunk, Score: cosine(query, chunk.Embedding)})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	return scored
}

func cosine(vecA, vecB []float64) float64 {
	if len(vecA) != len(vecB) || len(vecA) == 0 {
		return 0
	}

	var dot, normA, normB float64

	for i := range vecA {
		dot += vecA[i] * vecB[i]
		normA += vecA[i] * vecA[i]
		normB += vecB[i] * vecB[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

var _ assistant.ConversationRepository = (*ConversationStore)(nil)

// ConversationStore persists assistant interactions and feedback.
type ConversationStore struct {
	interactions *drivermongo.Collection
}

// NewConversationStore constructs a ConversationStore.
func NewConversationStore(db *drivermongo.Database) *ConversationStore {
	return &ConversationStore{interactions: db.Collection("assistant_interactions")}
}

// EnsureIndexes creates assistant interaction indexes.
func (s *ConversationStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.interactions.Indexes().CreateMany(ctx, []drivermongo.IndexModel{
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldCreatedAt, Value: -1}}},
		{Keys: bson.D{{Key: fieldOrganizationID, Value: 1}, {Key: fieldUserID, Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("ensure assistant interaction indexes: %w", err)
	}

	return nil
}

// SaveInteraction inserts an interaction.
func (s *ConversationStore) SaveInteraction(ctx context.Context, interaction assistant.Interaction) error {
	if _, err := s.interactions.InsertOne(ctx, interaction); err != nil {
		return fmt.Errorf("insert assistant interaction: %w", err)
	}

	return nil
}

// SetFeedback records whether an interaction's answer was helpful, scoped to the tenant.
func (s *ConversationStore) SetFeedback(
	ctx context.Context,
	organizationID, interactionID string,
	helpful bool,
) error {
	res, err := s.interactions.UpdateOne(
		ctx,
		bson.M{fieldID: interactionID, fieldOrganizationID: organizationID},
		bson.M{"$set": bson.M{fieldHelpful: helpful}},
	)
	if err != nil {
		return fmt.Errorf("update assistant feedback: %w", err)
	}

	if res.MatchedCount == 0 {
		return assistant.ErrNotFound
	}

	return nil
}

// ListInteractions returns all interactions for a tenant, newest first.
func (s *ConversationStore) ListInteractions(
	ctx context.Context,
	organizationID string,
) ([]assistant.Interaction, error) {
	cursor, err := s.interactions.Find(
		ctx,
		bson.M{fieldOrganizationID: organizationID},
		options.Find().SetSort(bson.D{{Key: fieldCreatedAt, Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("list assistant interactions: %w", err)
	}

	interactions := make([]assistant.Interaction, 0)
	if err := cursor.All(ctx, &interactions); err != nil {
		return nil, fmt.Errorf("decode assistant interactions: %w", err)
	}

	if err := cursor.Close(ctx); err != nil {
		return nil, fmt.Errorf("close assistant interactions cursor: %w", err)
	}

	return interactions, nil
}

func drainCursor(ctx context.Context, cursor *drivermongo.Cursor, out *[]assistant.Chunk) error {
	decodeErr := cursor.All(ctx, out)
	closeErr := cursor.Close(ctx)

	if decodeErr != nil && closeErr != nil {
		return errors.Join(
			fmt.Errorf("decode assistant chunks: %w", decodeErr),
			fmt.Errorf("close assistant chunks cursor: %w", closeErr),
		)
	}

	if decodeErr != nil {
		return fmt.Errorf("decode assistant chunks: %w", decodeErr)
	}

	if closeErr != nil {
		return fmt.Errorf("close assistant chunks cursor: %w", closeErr)
	}

	return nil
}

// DeleteForOrganization removes every chunk of the organization and returns
// the number deleted. It serves only the platform GDPR tenant purge
// (PRD 7.4).
func (s *VectorStore) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.chunks.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete assistant chunks for organization: %w", err)
	}

	return res.DeletedCount, nil
}

// DeleteForOrganization removes every interaction of the organization and
// returns the number deleted. It serves only the platform GDPR tenant purge
// (PRD 7.4).
func (s *ConversationStore) DeleteForOrganization(ctx context.Context, organizationID string) (int64, error) {
	res, err := s.interactions.DeleteMany(ctx, bson.M{fieldOrganizationID: organizationID})
	if err != nil {
		return 0, fmt.Errorf("delete assistant interactions for organization: %w", err)
	}

	return res.DeletedCount, nil
}
