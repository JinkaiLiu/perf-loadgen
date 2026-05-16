package httputil

import (
	"net/http"
	"strconv"
	"time"

	"github.com/JinkaiLiu/vibeready/pkg/types"
)

// x-ai-* response header names (proposed lightweight standard for AI API observability).
const (
	HeaderAIProvider        = "x-ai-provider"
	HeaderAIModel           = "x-ai-model"
	HeaderAIUpstreamLatency = "x-ai-upstream-latency-ms"
	HeaderAIFirstToken      = "x-ai-first-token-ms"
	HeaderAIInputTokens     = "x-ai-input-tokens"
	HeaderAIOutputTokens    = "x-ai-output-tokens"
	HeaderAICacheHit        = "x-ai-cache-hit"
)

// ParseDurationHeader reads a response header interpreted as milliseconds.
func ParseDurationHeader(headers http.Header, key string) time.Duration {
	raw := headers.Get(key)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 {
		return 0
	}
	return time.Duration(value * float64(time.Millisecond))
}

// ParseIntHeader reads a response header as an int64.
func ParseIntHeader(headers http.Header, key string) int64 {
	raw := headers.Get(key)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

// ParseFloatHeader reads a response header as a float64.
func ParseFloatHeader(headers http.Header, key string) float64 {
	raw := headers.Get(key)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

// ParseBoolHeader reads a response header as a bool.
func ParseBoolHeader(headers http.Header, key string) bool {
	raw := headers.Get(key)
	if raw == "" {
		return false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return value
}

// EnrichAIHeaders populates Layer 2 AI metrics from x-ai-* response headers.
// When headers are absent, result fields remain zero — backward compatible.
func EnrichAIHeaders(headers http.Header, result *types.RunResult, rspProvider, rspModel string) {
	if rspProvider != "" {
		result.Provider = rspProvider
	} else {
		result.Provider = headers.Get(HeaderAIProvider)
	}
	if rspModel != "" {
		result.Model = rspModel
	} else {
		result.Model = headers.Get(HeaderAIModel)
	}
	result.UpstreamLatency = ParseDurationHeader(headers, HeaderAIUpstreamLatency)
	if result.TTFT == 0 {
		result.TTFT = ParseDurationHeader(headers, HeaderAIFirstToken)
	}
	if result.OutputTokens == 0 {
		result.OutputTokens = ParseIntHeader(headers, HeaderAIOutputTokens)
	}
	if result.InputTokens == 0 {
		result.InputTokens = ParseIntHeader(headers, HeaderAIInputTokens)
	}
	if headers.Get(HeaderAICacheHit) != "" {
		result.CacheReported = true
		result.CacheHit = ParseBoolHeader(headers, HeaderAICacheHit)
	}
}

// ParseStringHeader reads a response header as a string.
func ParseStringHeader(headers http.Header, key string) string {
	return headers.Get(key)
}
