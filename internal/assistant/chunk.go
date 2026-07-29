package assistant

import "strings"

const (
	// chunkTargetChars is the approximate size of each chunk.
	chunkTargetChars = 700
	// chunkMaxChars is the hard cap before a chunk is flushed mid-paragraph.
	chunkMaxChars = 1000
)

// chunkText splits document text into overlapping, paragraph-aware chunks.
// A leading title line is prepended to the text so it participates in
// retrieval. The split is deterministic: identical input yields identical
// chunks, which keeps re-indexing idempotent.
func chunkText(title, body string) []string {
	combined := strings.TrimSpace(body)
	if title = strings.TrimSpace(title); title != "" {
		combined = title + "\n\n" + combined
	}

	if combined == "" {
		return nil
	}

	paragraphs := splitParagraphs(combined)

	chunks := make([]string, 0, len(paragraphs))

	var current strings.Builder

	flush := func() {
		if current.Len() == 0 {
			return
		}

		chunks = append(chunks, strings.TrimSpace(current.String()))
		current.Reset()
	}

	for _, para := range paragraphs {
		if current.Len() > 0 && current.Len()+len(para) > chunkMaxChars {
			flush()
		}

		if len(para) > chunkMaxChars {
			flush()

			chunks = append(chunks, hardSplit(para)...)

			continue
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}

		current.WriteString(para)

		if current.Len() >= chunkTargetChars {
			flush()
		}
	}

	flush()

	return chunks
}

func splitParagraphs(text string) []string {
	raw := strings.Split(text, "\n\n")

	paragraphs := make([]string, 0, len(raw))
	for _, para := range raw {
		if trimmed := strings.TrimSpace(para); trimmed != "" {
			paragraphs = append(paragraphs, trimmed)
		}
	}

	return paragraphs
}

// hardSplit breaks an oversized paragraph into fixed-size windows on rune
// boundaries so multibyte characters are never split.
func hardSplit(para string) []string {
	runes := []rune(para)

	windows := make([]string, 0, len(runes)/chunkMaxChars+1)
	for start := 0; start < len(runes); start += chunkMaxChars {
		end := min(start+chunkMaxChars, len(runes))

		windows = append(windows, strings.TrimSpace(string(runes[start:end])))
	}

	return windows
}
