package httpstream

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strings"
	"time"

	"github.com/JinkaiLiu/vibeready/internal/protocol/httputil"
	"github.com/JinkaiLiu/vibeready/pkg/types"
)

// Options defines how the streaming runner should interpret streamed chunks.
type Options struct {
	Timeout      time.Duration
	StreamFormat string
	DoneMarker   string
	TextKeys     []string
	TokenKeys    []string
}

// Runner executes HTTP requests and extracts metrics from streaming responses.
type Runner struct {
	client       *stdhttp.Client
	streamFormat string
	doneMarker   string
	textKeys     []string
	tokenKeys    []string
}

// NewRunner constructs a streaming-aware HTTP runner.
func NewRunner(opts Options) *Runner {
	if opts.StreamFormat == "" {
		opts.StreamFormat = "auto"
	}
	if opts.DoneMarker == "" {
		opts.DoneMarker = "[DONE]"
	}
	if len(opts.TextKeys) == 0 {
		opts.TextKeys = []string{"content", "text", "token", "output_text", "delta"}
	}
	if len(opts.TokenKeys) == 0 {
		opts.TokenKeys = []string{"output_tokens", "completion_tokens", "generated_tokens"}
	}

	return &Runner{
		client: &stdhttp.Client{
			Timeout:   opts.Timeout,
			Transport: httputil.NewTransport(),
		},
		streamFormat: opts.StreamFormat,
		doneMarker:   opts.DoneMarker,
		textKeys:     append([]string(nil), opts.TextKeys...),
		tokenKeys:    append([]string(nil), opts.TokenKeys...),
	}
}

// Run performs one request and parses the streaming response body.
func (r *Runner) Run(ctx context.Context, req types.RequestSpec) (types.RunResult, error) {
	start := time.Now()

	httpReq, err := stdhttp.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return httputil.FailedResult(types.ErrorCategoryUnknown, err, time.Since(start)), err
	}
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := r.client.Do(httpReq)
	if err != nil {
		category := httputil.ClassifyRequestError(err)
		return httputil.FailedResult(category, err, time.Since(start)), err
	}
	defer resp.Body.Close()

	result, readErr := r.consumeStream(resp, start)
	if readErr != nil {
		return result, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		result.Success = false
		result.ErrorCategory = types.ErrorCategoryHTTPStatus
		result.ErrorMessage = resp.Status
		return result, errors.New(resp.Status)
	}
	return result, nil
}

func (r *Runner) consumeStream(resp *stdhttp.Response, start time.Time) (types.RunResult, error) {
	result := types.RunResult{
		Success:       true,
		StatusCode:    resp.StatusCode,
		ErrorCategory: types.ErrorCategoryNone,
	}

	format := r.detectFormat(resp.Header.Get("Content-Type"))
	reader := bufio.NewReader(resp.Body)
	var (
		bytesRead      int64
		firstChunkTime time.Time
		lastChunkTime  time.Time
		prevChunkTime  time.Time
		textBuilder    strings.Builder
		sawDone        bool
		itlSamples     []time.Duration
	)

	switch format {
	case "sse":
		var eventLines []string
		for {
			line, err := reader.ReadString('\n')
			bytesRead += int64(len(line))
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				if len(eventLines) > 0 {
					chunk := r.extractSSEChunk(eventLines)
					if chunk.Done {
						sawDone = true
					}
					if chunk.Text != "" {
						now := time.Now()
						if firstChunkTime.IsZero() {
							firstChunkTime = now
						} else {
							itlSamples = append(itlSamples, now.Sub(prevChunkTime))
						}
						prevChunkTime = now
						lastChunkTime = now
						textBuilder.WriteString(chunk.Text)
						textBuilder.WriteByte(' ')
					}
					if chunk.Tokens > 0 {
						result.OutputTokens += chunk.Tokens
					}
					eventLines = eventLines[:0]
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					return failedReadResult(resp.StatusCode, bytesRead, start, err), err
				}
				continue
			}
			eventLines = append(eventLines, line)
			if err == io.EOF {
				break
			}
			if err != nil {
				return failedReadResult(resp.StatusCode, bytesRead, start, err), err
			}
		}
	case "jsonl", "raw":
		for {
			line, err := reader.ReadString('\n')
			bytesRead += int64(len(line))
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				chunk := r.extractChunk(trimmed)
				if chunk.Done {
					sawDone = true
				}
				if chunk.Text != "" {
					now := time.Now()
					if firstChunkTime.IsZero() {
						firstChunkTime = now
					} else {
						itlSamples = append(itlSamples, now.Sub(prevChunkTime))
					}
					prevChunkTime = now
					lastChunkTime = now
					textBuilder.WriteString(chunk.Text)
					textBuilder.WriteByte(' ')
				}
				if chunk.Tokens > 0 {
					result.OutputTokens += chunk.Tokens
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return failedReadResult(resp.StatusCode, bytesRead, start, err), err
			}
		}
	default:
		return failedReadResult(resp.StatusCode, bytesRead, start, errors.New("unsupported streaming format")), errors.New("unsupported streaming format")
	}

	result.BytesRead = bytesRead
	result.Latency = time.Since(start)
	enrichInferenceMetrics(resp, &result)
	collectedText := strings.TrimSpace(textBuilder.String())
	if !firstChunkTime.IsZero() {
		result.TTFT = firstChunkTime.Sub(start)
	}
	if !lastChunkTime.IsZero() && !firstChunkTime.IsZero() {
		result.GenerationTime = lastChunkTime.Sub(firstChunkTime)
	}
	if result.OutputTokens == 0 && collectedText != "" {
		result.OutputTokens = int64(len(strings.Fields(collectedText)))
		result.TokensEstimated = true
	}
	if result.TokensPerSecond == 0 && result.OutputTokens > 0 && result.GenerationTime > 0 {
		result.TokensPerSecond = float64(result.OutputTokens) / result.GenerationTime.Seconds()
	}
	if !sawDone {
		result.StreamingAborted = true
	}
	result.ITLSamples = itlSamples
	httputil.EnrichAIHeaders(resp.Header, &result, "", "")

	return result, nil
}

type parsedChunk struct {
	Text   string
	Tokens int64
	Done   bool
}

func (r *Runner) extractSSEChunk(lines []string) parsedChunk {
	var payload strings.Builder
	for _, line := range lines {
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if value == r.doneMarker {
			return parsedChunk{Done: true}
		}
		payload.WriteString(value)
	}
	raw := payload.String()
	if raw == "" {
		return parsedChunk{}
	}
	return r.extractChunk(raw)
}

func (r *Runner) extractChunk(raw string) parsedChunk {
	if raw == r.doneMarker {
		return parsedChunk{Done: true}
	}

	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return parsedChunk{}
		}
		return parsedChunk{
			Text: trimmed,
		}
	}

	var texts []string
	collectPreferredText(payload, &texts, normalizeKeys(r.textKeys))
	return parsedChunk{
		Text:   strings.Join(texts, " "),
		Tokens: collectPreferredTokens(payload, normalizeKeys(r.tokenKeys)),
		Done:   detectDone(payload, r.doneMarker),
	}
}

func (r *Runner) detectFormat(contentType string) string {
	switch r.streamFormat {
	case "sse", "jsonl", "raw":
		return r.streamFormat
	}
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return "sse"
	}
	if strings.Contains(strings.ToLower(contentType), "jsonl") || strings.Contains(strings.ToLower(contentType), "x-ndjson") || strings.Contains(strings.ToLower(contentType), "ndjson") {
		return "jsonl"
	}
	return "raw"
}

func normalizeKeys(keys []string) map[string]struct{} {
	out := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		out[strings.ToLower(key)] = struct{}{}
	}
	return out
}

func collectPreferredText(node any, out *[]string, preferred map[string]struct{}) {
	switch value := node.(type) {
	case map[string]any:
		for key, nested := range value {
			lowerKey := strings.ToLower(key)
			if _, ok := preferred[lowerKey]; ok {
				if str, ok := nested.(string); ok && strings.TrimSpace(str) != "" {
					*out = append(*out, str)
					continue
				}
			}
			collectPreferredText(nested, out, preferred)
		}
	case []any:
		for _, nested := range value {
			collectPreferredText(nested, out, preferred)
		}
	}
}

func collectPreferredTokens(node any, preferred map[string]struct{}) int64 {
	switch value := node.(type) {
	case map[string]any:
		for key, nested := range value {
			lowerKey := strings.ToLower(key)
			if _, ok := preferred[lowerKey]; ok {
				switch typed := nested.(type) {
				case float64:
					return int64(typed)
				case int64:
					return typed
				}
			}
			if nestedValue := collectPreferredTokens(nested, preferred); nestedValue > 0 {
				return nestedValue
			}
		}
	case []any:
		for _, nested := range value {
			if nestedValue := collectPreferredTokens(nested, preferred); nestedValue > 0 {
				return nestedValue
			}
		}
	}
	return 0
}

func detectDone(node any, doneMarker string) bool {
	switch value := node.(type) {
	case map[string]any:
		for key, nested := range value {
			lowerKey := strings.ToLower(key)
			if lowerKey == "done" {
				if flag, ok := nested.(bool); ok && flag {
					return true
				}
			}
			if lowerKey == "finish_reason" || lowerKey == "stop_reason" {
				if str, ok := nested.(string); ok && strings.TrimSpace(str) != "" {
					return true
				}
			}
			if lowerKey == "event" {
				if str, ok := nested.(string); ok && strings.EqualFold(str, doneMarker) {
					return true
				}
			}
			if detectDone(nested, doneMarker) {
				return true
			}
		}
	case []any:
		for _, nested := range value {
			if detectDone(nested, doneMarker) {
				return true
			}
		}
	}
	return false
}

func enrichInferenceMetrics(resp *stdhttp.Response, result *types.RunResult) {
	if result.TTFT == 0 {
		result.TTFT = httputil.ParseDurationHeader(resp.Header, "X-Loadgen-TTFT-Ms")
	}
	if result.GenerationTime == 0 {
		result.GenerationTime = httputil.ParseDurationHeader(resp.Header, "X-Loadgen-Generation-Ms")
	}
	if result.OutputTokens == 0 {
		result.OutputTokens = httputil.ParseIntHeader(resp.Header, "X-Loadgen-Output-Tokens")
	}
	if result.TokensPerSecond == 0 {
		if tokenRate := httputil.ParseFloatHeader(resp.Header, "X-Loadgen-Tokens-Per-Second"); tokenRate > 0 {
			result.TokensPerSecond = tokenRate
		} else if result.OutputTokens > 0 && result.GenerationTime > 0 {
			result.TokensPerSecond = float64(result.OutputTokens) / result.GenerationTime.Seconds()
		}
	}
	if !result.StreamingAborted {
		result.StreamingAborted = httputil.ParseBoolHeader(resp.Header, "X-Loadgen-Streaming-Aborted")
	}

	// Layer 2: semi-white-box AI metrics (x-ai-* headers).
	if result.UpstreamLatency == 0 {
		result.UpstreamLatency = httputil.ParseDurationHeader(resp.Header, httputil.HeaderAIUpstreamLatency)
	}
	if result.InputTokens == 0 {
		result.InputTokens = httputil.ParseIntHeader(resp.Header, httputil.HeaderAIInputTokens)
	}
	if resp.Header.Get(httputil.HeaderAICacheHit) != "" {
		result.CacheReported = true
		result.CacheHit = httputil.ParseBoolHeader(resp.Header, httputil.HeaderAICacheHit)
	}
	if result.Provider == "" {
		result.Provider = resp.Header.Get(httputil.HeaderAIProvider)
	}
	if result.Model == "" {
		result.Model = resp.Header.Get(httputil.HeaderAIModel)
	}
}

func failedReadResult(statusCode int, bytesRead int64, start time.Time, err error) types.RunResult {
	return types.RunResult{
		Success:       false,
		StatusCode:    statusCode,
		Latency:       time.Since(start),
		BytesRead:     bytesRead,
		ErrorCategory: httputil.ClassifyRequestError(err),
		ErrorMessage:  err.Error(),
	}
}
