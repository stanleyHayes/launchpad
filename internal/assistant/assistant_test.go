package assistant_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"launchpad/internal/assistant"
	"launchpad/internal/knowledge"
)

const (
	orgID    = "org-1"
	userID   = "user-1"
	question = "How do I request VPN access?"
)

// fakeEmbedder returns a fixed vector per input; retrieval is driven by the
// fake vector store, so the vector content is irrelevant here.
type fakeEmbedder struct{}

func (fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i := range texts {
		out[i] = []float64{1, 0, 0}
	}

	return out, nil
}

// fakeVectorStore returns preset hits and records queries and upserts.
type fakeVectorStore struct {
	hits           []assistant.ScoredChunk
	recordedScopes []string
	upserted       []assistant.Chunk
}

func (f *fakeVectorStore) EnsureIndexes(context.Context) error { return nil }

func (f *fakeVectorStore) UpsertDocumentChunks(
	_ context.Context,
	_, _ string,
	chunks []assistant.Chunk,
) error {
	f.upserted = chunks

	return nil
}

func (f *fakeVectorStore) DeleteByDocument(context.Context, string, string) error { return nil }

func (f *fakeVectorStore) Search(
	_ context.Context,
	_ string,
	_ []float64,
	scopes []string,
	_ int,
) ([]assistant.ScoredChunk, error) {
	f.recordedScopes = scopes

	return f.hits, nil
}

// fakeGenerator returns a preset answer and records whether it was called.
type fakeGenerator struct {
	answer assistant.GeneratedAnswer
	called bool
}

func (f *fakeGenerator) Generate(context.Context, assistant.GenerateInput) (assistant.GeneratedAnswer, error) {
	f.called = true

	return f.answer, nil
}

// fakeRepo records saved interactions and feedback.
type fakeRepo struct {
	saved       []assistant.Interaction
	feedbackErr error
	feedbackSet bool
}

func (f *fakeRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeRepo) SaveInteraction(_ context.Context, interaction assistant.Interaction) error {
	f.saved = append(f.saved, interaction)

	return nil
}

func (f *fakeRepo) SetFeedback(context.Context, string, string, bool) error {
	f.feedbackSet = true

	return f.feedbackErr
}

func (f *fakeRepo) ListInteractions(_ context.Context, organizationID string) ([]assistant.Interaction, error) {
	interactions := make([]assistant.Interaction, 0, len(f.saved))
	for _, interaction := range f.saved {
		if interaction.OrganizationID == organizationID {
			interactions = append(interactions, interaction)
		}
	}

	return interactions, nil
}

func orgChunk(docID, title, text string) assistant.ScoredChunk {
	return assistant.ScoredChunk{
		Chunk: assistant.Chunk{
			DocumentID:    docID,
			DocumentTitle: title,
			Text:          text,
			AccessScope:   assistant.ScopeOrganization,
		},
		Score: 0.9,
	}
}

func newService(store *fakeVectorStore, gen *fakeGenerator, repo *fakeRepo) *assistant.Service {
	return assistant.NewService(fakeEmbedder{}, store, gen, repo)
}

func TestAskReturnsGroundedAnswerWithCitations(t *testing.T) {
	t.Parallel()

	store := &fakeVectorStore{hits: []assistant.ScoredChunk{
		orgChunk("d1", "VPN Policy", "Request VPN access through the IT portal."),
		orgChunk("d2", "IT Onboarding", "New hires get VPN on day one."),
	}}
	gen := &fakeGenerator{answer: assistant.GeneratedAnswer{Text: "Use the IT portal [1] on day one [2]."}}
	repo := &fakeRepo{}

	answer, err := newService(store, gen, repo).Ask(context.Background(), orgID, userID, question, false)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}

	if !answer.Grounded || answer.Refused {
		t.Fatalf("expected grounded answer, got %+v", answer)
	}

	if len(answer.Citations) != 2 {
		t.Fatalf("expected 2 citations, got %d", len(answer.Citations))
	}

	if answer.InteractionID == "" {
		t.Fatalf("expected interaction id to be set")
	}

	if len(repo.saved) != 1 || !repo.saved[0].Grounded {
		t.Fatalf("expected one grounded interaction saved, got %+v", repo.saved)
	}
}

func TestAskRefusesWhenNoRelevantSources(t *testing.T) {
	t.Parallel()

	weak := orgChunk("d1", "Unrelated", "Cafeteria hours are 8-5.")
	weak.Score = 0.02
	store := &fakeVectorStore{hits: []assistant.ScoredChunk{weak}}
	gen := &fakeGenerator{}
	repo := &fakeRepo{}

	answer, err := newService(store, gen, repo).Ask(context.Background(), orgID, userID, question, false)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}

	if !answer.Refused || answer.Grounded {
		t.Fatalf("expected refusal, got %+v", answer)
	}

	if len(answer.Citations) != 0 {
		t.Fatalf("expected no citations on refusal, got %d", len(answer.Citations))
	}

	if gen.called {
		t.Fatalf("generator must not be called when no source clears the threshold")
	}

	if len(repo.saved) != 1 || !repo.saved[0].Refused {
		t.Fatalf("expected one refused interaction saved, got %+v", repo.saved)
	}
}

func TestAskManagerScopeIncludesRestricted(t *testing.T) {
	t.Parallel()

	store := &fakeVectorStore{}
	if _, err := newService(store, &fakeGenerator{}, &fakeRepo{}).
		Ask(context.Background(), orgID, userID, question, true); err != nil {
		t.Fatalf("ask: %v", err)
	}

	if len(store.recordedScopes) != 2 {
		t.Fatalf("manager scopes = %v, want organization + restricted", store.recordedScopes)
	}
}

func TestAskMemberScopeExcludesRestricted(t *testing.T) {
	t.Parallel()

	store := &fakeVectorStore{}
	if _, err := newService(store, &fakeGenerator{}, &fakeRepo{}).
		Ask(context.Background(), orgID, userID, question, false); err != nil {
		t.Fatalf("ask: %v", err)
	}

	if len(store.recordedScopes) != 1 || store.recordedScopes[0] != assistant.ScopeOrganization {
		t.Fatalf("member scopes = %v, want organization only", store.recordedScopes)
	}
}

func TestIndexerChunksAndEmbedsDocument(t *testing.T) {
	t.Parallel()

	store := &fakeVectorStore{}
	indexer := assistant.NewIndexer(fakeEmbedder{}, store)

	err := indexer.Index(context.Background(), knowledge.Document{
		ID:             "doc-1",
		OrganizationID: orgID,
		Title:          "VPN Policy",
		Body:           "Request VPN access via the IT portal.\n\nApproval takes one day.",
		AccessScope:    knowledge.ScopeOrganization,
	})
	if err != nil {
		t.Fatalf("index: %v", err)
	}

	if len(store.upserted) == 0 {
		t.Fatalf("expected chunks to be upserted")
	}

	first := store.upserted[0]
	if first.DocumentID != "doc-1" || first.OrganizationID != orgID {
		t.Fatalf("chunk not scoped to document/tenant: %+v", first)
	}

	if !strings.Contains(first.Text, "VPN Policy") {
		t.Fatalf("first chunk should carry the title, got %q", first.Text)
	}

	if len(first.Embedding) == 0 {
		t.Fatalf("chunk should carry an embedding")
	}
}

func TestAskRefusesUncitedAnswer(t *testing.T) {
	t.Parallel()

	store := &fakeVectorStore{hits: []assistant.ScoredChunk{orgChunk("d1", "Doc", "Some text.")}}
	// The model answered without citing any source, so it must not be presented
	// as grounded even though relevant sources were found.
	gen := &fakeGenerator{answer: assistant.GeneratedAnswer{Text: "Just call the help desk."}}
	repo := &fakeRepo{}

	answer, err := newService(store, gen, repo).Ask(context.Background(), orgID, userID, question, false)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}

	if !answer.Refused || answer.Grounded || len(answer.Citations) != 0 {
		t.Fatalf("expected refusal for an uncited answer, got %+v", answer)
	}
}

func TestAskGeneratorRefusalYieldsRefusal(t *testing.T) {
	t.Parallel()

	store := &fakeVectorStore{hits: []assistant.ScoredChunk{orgChunk("d1", "Doc", "Some text.")}}
	gen := &fakeGenerator{answer: assistant.GeneratedAnswer{Refused: true}}
	repo := &fakeRepo{}

	answer, err := newService(store, gen, repo).Ask(context.Background(), orgID, userID, question, false)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}

	if !answer.Refused || len(answer.Citations) != 0 {
		t.Fatalf("expected refusal without citations, got %+v", answer)
	}
}

func TestAskRejectsEmptyQuestion(t *testing.T) {
	t.Parallel()

	_, err := newService(&fakeVectorStore{}, &fakeGenerator{}, &fakeRepo{}).
		Ask(context.Background(), orgID, userID, "   ", false)
	if !errors.Is(err, assistant.ErrInvalidInput) {
		t.Fatalf("got %v, want ErrInvalidInput", err)
	}
}

func TestFeedbackMapsNotFound(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{feedbackErr: assistant.ErrNotFound}
	svc := newService(&fakeVectorStore{}, &fakeGenerator{}, repo)

	err := svc.Feedback(context.Background(), orgID, "missing", true)
	if !errors.Is(err, assistant.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}

	if !repo.feedbackSet {
		t.Fatalf("expected feedback to be attempted")
	}
}

func (f *fakeVectorStore) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (f *fakeRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
