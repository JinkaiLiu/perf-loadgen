package tokenizer

import "strings"

// Tokenizer estimates token counts for text.
type Tokenizer interface {
	Count(text string) int64
}

// WordCount is a heuristic tokenizer that splits on whitespace.
type WordCount struct{}

// Count approximates token count as the number of whitespace-delimited words.
func (WordCount) Count(text string) int64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return int64(len(strings.Fields(text)))
}

// New returns a tokenizer for the given spec. Currently only "word" is supported.
func New(spec string) Tokenizer {
	switch {
	case spec == "" || spec == "word":
		return WordCount{}
	default:
		return WordCount{}
	}
}
