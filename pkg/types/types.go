package types

import "time"

// RequestSpec describes a single request executed by a protocol runner.
type RequestSpec struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
	Timeout time.Duration     `json:"timeout"`
}

// ErrorCategory groups failures into stable buckets for reporting.
type ErrorCategory string

const (
	ErrorCategoryNone       ErrorCategory = "none"
	ErrorCategoryTimeout    ErrorCategory = "timeout"
	ErrorCategoryNetwork    ErrorCategory = "network"
	ErrorCategoryHTTPStatus ErrorCategory = "http_status"
	ErrorCategoryUnknown    ErrorCategory = "unknown"
)

// ResponseMeta captures protocol-level response details.
type ResponseMeta struct {
	StatusCode   int           `json:"status_code"`
	BytesRead    int64         `json:"bytes_read"`
	Latency      time.Duration `json:"latency"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// RunResult is a single request outcome reported by a runner.
type RunResult struct {
	Success          bool            `json:"success"`
	Latency          time.Duration   `json:"latency"`
	StatusCode       int             `json:"status_code"`
	ErrorCategory    ErrorCategory   `json:"error_category"`
	ErrorMessage     string          `json:"error_message,omitempty"`
	BytesRead        int64           `json:"bytes_read"`
	TTFT             time.Duration   `json:"ttft,omitempty"`
	GenerationTime   time.Duration   `json:"generation_time,omitempty"`
	OutputTokens     int64           `json:"output_tokens,omitempty"`
	TokensPerSecond  float64         `json:"tokens_per_second,omitempty"`
	StreamingAborted bool            `json:"streaming_aborted,omitempty"`
	ITLSamples       []time.Duration `json:"-"`

	// Layer 2: semi-white-box metrics from x-ai-* response headers.
	UpstreamLatency time.Duration `json:"upstream_latency,omitempty"`
	InputTokens     int64         `json:"input_tokens,omitempty"`
	CacheHit        bool          `json:"cache_hit,omitempty"`
	CacheReported   bool          `json:"cache_reported,omitempty"`
	Provider        string        `json:"provider,omitempty"`
	Model           string        `json:"model,omitempty"`
}

// Percentiles holds latency percentile estimates from the histogram.
type Percentiles struct {
	P50      time.Duration `json:"p50"`
	P90      time.Duration `json:"p90"`
	P95      time.Duration `json:"p95"`
	P99      time.Duration `json:"p99"`
	P50Human string        `json:"p50_human"`
	P90Human string        `json:"p90_human"`
	P95Human string        `json:"p95_human"`
	P99Human string        `json:"p99_human"`
}

// HistogramSnapshot is a wire-safe representation of the latency histogram.
type HistogramSnapshot struct {
	BoundsMicros []int64 `json:"bounds_micros"`
	Counts       []int64 `json:"counts"`
}

// AggregateSnapshot is the mergeable stats payload exchanged between workers and master.
type AggregateSnapshot struct {
	StartTime                  time.Time               `json:"start_time"`
	EndTime                    time.Time               `json:"end_time"`
	TotalRequests              int64                   `json:"total_requests"`
	SuccessfulRequests         int64                   `json:"successful_requests"`
	FailedRequests             int64                   `json:"failed_requests"`
	TotalLatencyMicros         int64                   `json:"total_latency_micros"`
	MinLatencyMicros           int64                   `json:"min_latency_micros"`
	MaxLatencyMicros           int64                   `json:"max_latency_micros"`
	RequestsWithTTFT           int64                   `json:"requests_with_ttft"`
	TotalTTFTMicros            int64                   `json:"total_ttft_micros"`
	RequestsWithTokens         int64                   `json:"requests_with_tokens"`
	TotalOutputTokens          int64                   `json:"total_output_tokens"`
	TotalTokenRate             float64                 `json:"total_token_rate"`
	StreamingAborted           int64                   `json:"streaming_aborted"`
	StatusCodes                map[int]int64           `json:"status_codes,omitempty"`
	ErrorCategories            map[ErrorCategory]int64 `json:"error_categories,omitempty"`
	Histogram                  HistogramSnapshot       `json:"histogram"`
	TTFTHistogram              HistogramSnapshot       `json:"ttft_histogram,omitempty"`
	ITLHistogram               HistogramSnapshot       `json:"itl_histogram,omitempty"`
	TotalUpstreamLatencyMicros int64                   `json:"total_upstream_latency_micros,omitempty"`
	RequestsWithUpstream       int64                   `json:"requests_with_upstream,omitempty"`
	TotalInputTokens           int64                   `json:"total_input_tokens,omitempty"`
	CacheHits                  int64                   `json:"cache_hits,omitempty"`
	CacheRequests              int64                   `json:"cache_requests,omitempty"`
	Provider                   string                  `json:"provider,omitempty"`
	Model                      string                  `json:"model,omitempty"`
}

// Summary is the final aggregated report for a load generation run.
type Summary struct {
	StartTime          time.Time     `json:"start_time"`
	EndTime            time.Time     `json:"end_time"`
	Duration           time.Duration `json:"duration"`
	DurationHuman      string        `json:"duration_human"`
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	ErrorRate          float64       `json:"error_rate"`
	RequestsPerSecond  float64       `json:"requests_per_second"`
	AvgLatency         time.Duration `json:"avg_latency"`
	AvgLatencyHuman    string        `json:"avg_latency_human"`
	MinLatency         time.Duration `json:"min_latency"`
	MinLatencyHuman    string        `json:"min_latency_human"`
	MaxLatency         time.Duration `json:"max_latency"`
	MaxLatencyHuman    string        `json:"max_latency_human"`
	AvgTTFT            time.Duration `json:"avg_ttft"`
	AvgTTFTHuman       string        `json:"avg_ttft_human"`
	TotalOutputTokens  int64         `json:"total_output_tokens"`
	AvgTokensPerSecond float64       `json:"avg_tokens_per_second"`
	StreamingAborted   int64         `json:"streaming_aborted"`
	Percentiles        Percentiles   `json:"percentiles"`
	TTFTPercentiles    Percentiles   `json:"ttft_percentiles,omitempty"`
	ITLPercentiles     Percentiles   `json:"itl_percentiles,omitempty"`
	// Layer 2: derived AI metrics.
	AvgUpstreamLatency      time.Duration `json:"avg_upstream_latency,omitempty"`
	AvgUpstreamLatencyHuman string        `json:"avg_upstream_latency_human,omitempty"`
	AvgBackendOverhead      time.Duration `json:"avg_backend_overhead,omitempty"`
	AvgBackendOverheadHuman string        `json:"avg_backend_overhead_human,omitempty"`
	UpstreamLatencyRatio    float64       `json:"upstream_latency_ratio,omitempty"`
	TotalInputTokens        int64         `json:"total_input_tokens,omitempty"`
	CacheHitRate            float64       `json:"cache_hit_rate,omitempty"`
	EstimatedCost           float64       `json:"estimated_cost,omitempty"`
	Provider                string        `json:"provider,omitempty"`
	Model                   string        `json:"model,omitempty"`

	StatusCodes     map[int]int64           `json:"status_codes,omitempty"`
	ErrorCategories map[ErrorCategory]int64 `json:"error_categories,omitempty"`
}
