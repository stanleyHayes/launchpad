// Package assistant implements the AI onboarding assistant: grounded,
// citation-backed retrieval over a tenant's approved knowledge documents
// (PRD §5.4.3, §16.2). Answers are drawn only from indexed sources; when no
// reliable source is found the assistant refuses rather than inventing content.
package assistant

import (
	"errors"
	"time"
)

var (
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid assistant input")
	// ErrNotFound indicates the referenced record does not exist for the tenant.
	ErrNotFound = errors.New("assistant record not found")
)

// Scope access levels mirror the knowledge module's document scopes.
const (
	ScopeOrganization = "organization"
	ScopeRestricted   = "restricted"
)

// Chunk is a fragment of a knowledge document with its embedding vector.
// It carries enough source metadata to build a citation without a second read.
type Chunk struct {
	ID             string    `bson:"_id"            json:"id"`
	OrganizationID string    `bson:"organizationId" json:"organizationId"`
	DocumentID     string    `bson:"documentId"     json:"documentId"`
	DocumentTitle  string    `bson:"documentTitle"  json:"documentTitle"`
	DocumentURI    string    `bson:"documentUri"    json:"documentUri"`
	AccessScope    string    `bson:"accessScope"    json:"accessScope"`
	Ordinal        int       `bson:"ordinal"        json:"ordinal"`
	Text           string    `bson:"text"           json:"text"`
	Embedding      []float64 `bson:"embedding"      json:"-"`
	CreatedAt      time.Time `bson:"createdAt"      json:"createdAt"`
}

// ScoredChunk is a chunk with its similarity score against a query.
type ScoredChunk struct {
	Chunk Chunk
	Score float64
}

// Citation references the source a passage of the answer was drawn from.
type Citation struct {
	DocumentID    string `bson:"documentId"            json:"documentId"`
	DocumentTitle string `bson:"documentTitle"         json:"documentTitle"`
	DocumentURI   string `bson:"documentUri,omitempty" json:"documentUri,omitempty"`
	Snippet       string `bson:"snippet"               json:"snippet"`
}

// Answer is the assistant's response to a question.
type Answer struct {
	InteractionID string     `json:"interactionId"`
	Text          string     `json:"text"`
	Citations     []Citation `json:"citations"`
	Grounded      bool       `json:"grounded"`
	Refused       bool       `json:"refused"`
}

// Interaction is a persisted question/answer exchange with optional feedback.
type Interaction struct {
	ID             string     `bson:"_id"                 json:"id"`
	OrganizationID string     `bson:"organizationId"      json:"organizationId"`
	UserID         string     `bson:"userId"              json:"userId"`
	Question       string     `bson:"question"            json:"question"`
	Answer         string     `bson:"answer"              json:"answer"`
	Citations      []Citation `bson:"citations,omitempty" json:"citations,omitempty"`
	Grounded       bool       `bson:"grounded"            json:"grounded"`
	Refused        bool       `bson:"refused"             json:"refused"`
	Helpful        *bool      `bson:"helpful,omitempty"   json:"helpful,omitempty"`
	CreatedAt      time.Time  `bson:"createdAt"           json:"createdAt"`
}

// Passage is a candidate source passage handed to the answer generator.
type Passage struct {
	Index         int
	DocumentTitle string
	Text          string
}

// GenerateInput is the grounded-generation request.
type GenerateInput struct {
	Question string
	Passages []Passage
}

// GeneratedAnswer is the answer generator's output.
type GeneratedAnswer struct {
	Text    string
	Refused bool
}
