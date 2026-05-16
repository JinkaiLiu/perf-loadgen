package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	stdhttp "net/http"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/internal/protocol/httputil"
	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
)

// Runner executes single HTTP requests using a shared client.
type Runner struct {
	client *stdhttp.Client
}

// NewRunner creates an HTTP runner with a reusable transport.
func NewRunner(timeout time.Duration) *Runner {
	return &Runner{
		client: &stdhttp.Client{
			Timeout:   timeout,
			Transport: httputil.NewTransport(),
		},
	}
}

// Run performs one complete request/response cycle and consumes the full body.
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

	bytesRead, readErr := io.Copy(io.Discard, resp.Body)
	latency := time.Since(start)
	if readErr != nil {
		category := httputil.ClassifyRequestError(readErr)
		return types.RunResult{
			Success:       false,
			Latency:       latency,
			StatusCode:    resp.StatusCode,
			ErrorCategory: category,
			ErrorMessage:  readErr.Error(),
			BytesRead:     bytesRead,
		}, readErr
	}

	success := resp.StatusCode >= 200 && resp.StatusCode < 400
	result := types.RunResult{
		Success:       success,
		Latency:       latency,
		StatusCode:    resp.StatusCode,
		ErrorCategory: types.ErrorCategoryNone,
		BytesRead:     bytesRead,
	}
	enrichInferenceMetrics(resp, &result)
	if !success {
		result.ErrorCategory = types.ErrorCategoryHTTPStatus
		result.ErrorMessage = resp.Status
		return result, errors.New(resp.Status)
	}

	return result, nil
}

func enrichInferenceMetrics(resp *stdhttp.Response, result *types.RunResult) {
	// Layer 1: inference timing (X-Loadgen-* legacy headers).
	result.TTFT = orDuration(result.TTFT, httputil.ParseDurationHeader(resp.Header, "X-Loadgen-TTFT-Ms"))
	result.GenerationTime = orDuration(result.GenerationTime, httputil.ParseDurationHeader(resp.Header, "X-Loadgen-Generation-Ms"))
	result.OutputTokens = orInt(result.OutputTokens, httputil.ParseIntHeader(resp.Header, "X-Loadgen-Output-Tokens"))
	if result.TokensPerSecond == 0 {
		if tokenRate := httputil.ParseFloatHeader(resp.Header, "X-Loadgen-Tokens-Per-Second"); tokenRate > 0 {
			result.TokensPerSecond = tokenRate
		} else if result.OutputTokens > 0 && result.GenerationTime > 0 {
			result.TokensPerSecond = float64(result.OutputTokens) / result.GenerationTime.Seconds()
		}
	}
	result.StreamingAborted = result.StreamingAborted || httputil.ParseBoolHeader(resp.Header, "X-Loadgen-Streaming-Aborted")

	// Layer 2: semi-white-box AI metrics (x-ai-* headers).
	if result.UpstreamLatency == 0 {
		result.UpstreamLatency = httputil.ParseDurationHeader(resp.Header, httputil.HeaderAIUpstreamLatency)
	}
	if result.TTFT == 0 {
		result.TTFT = httputil.ParseDurationHeader(resp.Header, httputil.HeaderAIFirstToken)
	}
	result.InputTokens = orInt(result.InputTokens, httputil.ParseIntHeader(resp.Header, httputil.HeaderAIInputTokens))
	result.OutputTokens = orInt(result.OutputTokens, httputil.ParseIntHeader(resp.Header, httputil.HeaderAIOutputTokens))
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

func orDuration(a, b time.Duration) time.Duration {
	if a > 0 {
		return a
	}
	return b
}

func orInt(a, b int64) int64 {
	if a > 0 {
		return a
	}
	return b
}
