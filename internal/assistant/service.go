package assistant

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	errEmbeddingCount = errors.New("unexpected embedding count for question")
	citationMarker    = regexp.MustCompile(`\[(\d+)\]`)
)

const (
	defaultTopK      = 6
	defaultMinScore  = 0.15
	maxQuestionChars = 2000
	snippetChars     = 240

	refusalText = "I don't have a reliable source in your organization's knowledge base to answer that. " +
		"Try rephrasing, or ask an administrator to add and approve a document that covers it."
)

// Service implements the AI onboarding assistant use cases.
type Service struct {
	embeddings EmbeddingProvider
	vectors    VectorStore
	generator  AnswerGenerator
	repo       ConversationRepository
	topK       int
	minScore   float64
}

// NewService constructs a Service with default retrieval tuning.
func NewService(
	embeddings EmbeddingProvider,
	vectors VectorStore,
	generator AnswerGenerator,
	repo ConversationRepository,
) *Service {
	return &Service{
		embeddings: embeddings,
		vectors:    vectors,
		generator:  generator,
		repo:       repo,
		topK:       defaultTopK,
		minScore:   defaultMinScore,
	}
}

// Ask answers a question grounded in the tenant's indexed knowledge. Retrieval
// is filtered by organization and by the caller's access scope; when no source
// clears the relevance threshold the assistant refuses instead of guessing.
func (s *Service) Ask(
	ctx context.Context,
	organizationID, userID, question string,
	canManage bool,
) (Answer, error) {
	question = strings.TrimSpace(question)
	if !validAskInput(organizationID, userID, question) {
		return Answer{}, ErrInvalidInput
	}

	vectors, err := s.embeddings.Embed(ctx, []string{question})
	if err != nil {
		return Answer{}, fmt.Errorf("embed question: %w", err)
	}

	if len(vectors) != 1 {
		return Answer{}, fmt.Errorf("%w: got %d want 1", errEmbeddingCount, len(vectors))
	}

	hits, err := s.vectors.Search(ctx, organizationID, vectors[0], allowedScopes(canManage), s.topK)
	if err != nil {
		return Answer{}, fmt.Errorf("search knowledge index: %w", err)
	}

	// One passage per source document keeps inline [n] markers aligned 1:1 with
	// the returned citations.
	relevant := dedupeByDocument(s.aboveThreshold(hits))
	if len(relevant) == 0 {
		return s.recordAndReturn(ctx, organizationID, userID, question, refusedAnswer(), nil)
	}

	generated, err := s.generator.Generate(ctx, GenerateInput{
		Question: question,
		Passages: buildPassages(relevant),
	})
	if err != nil {
		return Answer{}, fmt.Errorf("generate answer: %w", err)
	}

	answerText := strings.TrimSpace(generated.Text)

	// An answer that cites no source is treated as ungrounded and refused, so a
	// model that ignores the sources cannot be presented as source-backed.
	cited := citedIndices(answerText, len(relevant))
	if generated.Refused || answerText == "" || len(cited) == 0 {
		return s.recordAndReturn(ctx, organizationID, userID, question, refusedAnswer(), nil)
	}

	citations := citationsFor(relevant, cited)

	return s.recordAndReturn(ctx, organizationID, userID, question, Answer{
		Text:      answerText,
		Citations: citations,
		Grounded:  true,
		Refused:   false,
	}, citations)
}

// Feedback records whether an answer was helpful (PRD §16.2 answer feedback).
func (s *Service) Feedback(ctx context.Context, organizationID, interactionID string, helpful bool) error {
	if organizationID == "" || interactionID == "" {
		return ErrInvalidInput
	}

	if err := s.repo.SetFeedback(ctx, organizationID, interactionID, helpful); err != nil {
		return fmt.Errorf("record answer feedback: %w", err)
	}

	return nil
}

// ListInteractions returns all interactions for a tenant, newest first.
// Read-only; used by analytics reporting.
func (s *Service) ListInteractions(ctx context.Context, organizationID string) ([]Interaction, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	interactions, err := s.repo.ListInteractions(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list interactions: %w", err)
	}

	return interactions, nil
}

func (s *Service) recordAndReturn(
	ctx context.Context,
	organizationID, userID, question string,
	answer Answer,
	citations []Citation,
) (Answer, error) {
	interaction := Interaction{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		UserID:         userID,
		Question:       question,
		Answer:         answer.Text,
		Citations:      citations,
		Grounded:       answer.Grounded,
		Refused:        answer.Refused,
		Helpful:        nil,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.repo.SaveInteraction(ctx, interaction); err != nil {
		return Answer{}, fmt.Errorf("save interaction: %w", err)
	}

	answer.InteractionID = interaction.ID

	return answer, nil
}

func (s *Service) aboveThreshold(hits []ScoredChunk) []ScoredChunk {
	relevant := make([]ScoredChunk, 0, len(hits))
	for _, hit := range hits {
		if hit.Score >= s.minScore {
			relevant = append(relevant, hit)
		}
	}

	return relevant
}

func validAskInput(organizationID, userID, question string) bool {
	return organizationID != "" && userID != "" && question != "" && len(question) <= maxQuestionChars
}

func allowedScopes(canManage bool) []string {
	if canManage {
		return []string{ScopeOrganization, ScopeRestricted}
	}

	return []string{ScopeOrganization}
}

func buildPassages(hits []ScoredChunk) []Passage {
	passages := make([]Passage, len(hits))
	for i, hit := range hits {
		passages[i] = Passage{
			Index:         i + 1,
			DocumentTitle: hit.Chunk.DocumentTitle,
			Text:          hit.Chunk.Text,
		}
	}

	return passages
}

func refusedAnswer() Answer {
	return Answer{Text: refusalText, Grounded: false, Refused: true}
}

// dedupeByDocument keeps the highest-ranked chunk per source document. Hits
// arrive ranked by descending score, so the first occurrence of each document
// is its best chunk.
func dedupeByDocument(hits []ScoredChunk) []ScoredChunk {
	seen := make(map[string]struct{}, len(hits))

	out := make([]ScoredChunk, 0, len(hits))
	for _, hit := range hits {
		if _, ok := seen[hit.Chunk.DocumentID]; ok {
			continue
		}

		seen[hit.Chunk.DocumentID] = struct{}{}
		out = append(out, hit)
	}

	return out
}

// citedIndices returns the distinct 1-based passage indices the answer cites via
// [n] markers, ignoring out-of-range references.
func citedIndices(text string, count int) []int {
	seen := make(map[int]struct{})
	indices := make([]int, 0)

	for _, match := range citationMarker.FindAllStringSubmatch(text, -1) {
		n, err := strconv.Atoi(match[1])
		if err != nil || n < 1 || n > count {
			continue
		}

		if _, ok := seen[n]; ok {
			continue
		}

		seen[n] = struct{}{}

		indices = append(indices, n)
	}

	sort.Ints(indices)

	return indices
}

// citationsFor builds citations for the passages the answer actually cited, so
// the citation list reflects the sources the model used.
func citationsFor(hits []ScoredChunk, indices []int) []Citation {
	citations := make([]Citation, 0, len(indices))
	for _, idx := range indices {
		chunk := hits[idx-1].Chunk
		citations = append(citations, Citation{
			DocumentID:    chunk.DocumentID,
			DocumentTitle: chunk.DocumentTitle,
			DocumentURI:   chunk.DocumentURI,
			Snippet:       snippet(chunk.Text),
		})
	}

	return citations
}

func snippet(text string) string {
	text = strings.TrimSpace(text)

	runes := []rune(text)
	if len(runes) <= snippetChars {
		return text
	}

	return strings.TrimSpace(string(runes[:snippetChars])) + "…"
}
