// Package embed is a deterministic, dependency-free EmbeddingProvider.
//
// It uses signed feature hashing over word tokens to map text into a fixed
// unit-length vector, so cosine similarity reduces to a dot product. It needs
// no network or API key, which keeps the assistant runnable and its tests
// reproducible offline. A hosted embedding model (e.g. Voyage, Anthropic's
// recommended partner) can replace it behind the same assistant.EmbeddingProvider
// port without touching retrieval logic.
package embed

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"unicode"

	"launchpad/internal/assistant"
)

// Dimensions is the fixed embedding width.
const Dimensions = 256

const minTokenLen = 2

var _ assistant.EmbeddingProvider = (*Provider)(nil)

// Provider is a deterministic local embedding provider.
type Provider struct {
	dims uint32
}

// NewProvider constructs a Provider.
func NewProvider() *Provider {
	return &Provider{dims: Dimensions}
}

// Embed returns one unit-length vector per input string.
func (p *Provider) Embed(_ context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i, text := range texts {
		out[i] = p.vectorize(text)
	}

	return out, nil
}

func (p *Provider) vectorize(text string) []float64 {
	vec := make([]float64, p.dims)

	for _, token := range tokenize(text) {
		bucket, sign := hashToken(token, p.dims)
		vec[bucket] += sign
	}

	normalize(vec)

	return vec
}

func hashToken(token string, dims uint32) (int, float64) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(token)) // hash.Write never returns an error.
	sum := h.Sum32()

	// sum%dims is in [0, dims), so the conversion to int cannot overflow.
	bucket := int(sum % dims)

	sign := 1.0
	if sum&1 == 1 {
		sign = -1.0
	}

	return bucket, sign
}

func normalize(vec []float64) {
	var sumSquares float64
	for _, v := range vec {
		sumSquares += v * v
	}

	if sumSquares == 0 {
		return
	}

	norm := math.Sqrt(sumSquares)
	for i := range vec {
		vec[i] /= norm
	}
}

func tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) >= minTokenLen {
			tokens = append(tokens, field)
		}
	}

	return tokens
}
