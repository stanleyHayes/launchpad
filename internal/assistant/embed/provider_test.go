package embed_test

import (
	"context"
	"math"
	"testing"

	"launchpad/internal/assistant/embed"
)

func TestEmbedIsDeterministic(t *testing.T) {
	t.Parallel()

	provider := embed.NewProvider()

	first, err := provider.Embed(context.Background(), []string{"reset your VPN password"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	second, err := provider.Embed(context.Background(), []string{"reset your VPN password"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	if len(first[0]) != embed.Dimensions {
		t.Fatalf("dimension = %d, want %d", len(first[0]), embed.Dimensions)
	}

	for i := range first[0] {
		if first[0][i] != second[0][i] {
			t.Fatalf("embedding is not deterministic at index %d", i)
		}
	}
}

func TestEmbedRewardsSharedVocabulary(t *testing.T) {
	t.Parallel()

	provider := embed.NewProvider()

	vecs, err := provider.Embed(context.Background(), []string{
		"how do I reset my VPN password",
		"reset the VPN password from the portal",
		"the cafeteria serves lunch at noon",
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	related := dot(vecs[0], vecs[1])
	unrelated := dot(vecs[0], vecs[2])

	if related <= unrelated {
		t.Fatalf("related similarity %.3f should exceed unrelated %.3f", related, unrelated)
	}
}

func TestEmbedIsUnitLength(t *testing.T) {
	t.Parallel()

	vecs, err := embed.NewProvider().Embed(context.Background(), []string{"onboarding checklist"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	norm := math.Sqrt(dot(vecs[0], vecs[0]))
	if math.Abs(norm-1.0) > 1e-9 {
		t.Fatalf("norm = %.6f, want ~1.0", norm)
	}
}

func dot(vecA, vecB []float64) float64 {
	var sum float64
	for i := range vecA {
		sum += vecA[i] * vecB[i]
	}

	return sum
}
