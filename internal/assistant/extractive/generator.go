// Package extractive is a no-LLM AnswerGenerator fallback.
//
// When no answering model is configured (e.g. no ANTHROPIC_API_KEY), it returns
// the top retrieved passage verbatim rather than fabricating prose. This keeps
// the assistant useful and grounded offline; the service still supplies the
// citations, so the [1] marker aligns with the first citation.
package extractive

import (
	"context"
	"strings"

	"launchpad/internal/assistant"
)

var _ assistant.AnswerGenerator = (*Generator)(nil)

// Generator returns an extractive answer built from the top source passage.
type Generator struct{}

// NewGenerator constructs a Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate returns the highest-ranked passage, attributed to source [1].
func (Generator) Generate(_ context.Context, in assistant.GenerateInput) (assistant.GeneratedAnswer, error) {
	if len(in.Passages) == 0 {
		return assistant.GeneratedAnswer{Refused: true}, nil
	}

	top := strings.TrimSpace(in.Passages[0].Text)
	if top == "" {
		return assistant.GeneratedAnswer{Refused: true}, nil
	}

	return assistant.GeneratedAnswer{
		Text: "Based on your organization's documents: " + top + " [1]",
	}, nil
}
