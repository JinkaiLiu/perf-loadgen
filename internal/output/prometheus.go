package output

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/internal/stats"
	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
)

// PrometheusExporter exposes live run metrics in Prometheus text format.
type PrometheusExporter struct {
	server    *http.Server
	startTime time.Time

	mu        sync.RWMutex
	collector *stats.Collector
	final     *types.Summary
	isFinal   bool
}

// NewPrometheusExporter constructs an exporter bound to the given port.
func NewPrometheusExporter(port int) *PrometheusExporter {
	exporter := &PrometheusExporter{
		startTime: time.Now(),
		collector: stats.NewCollector(),
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", exporter)
	exporter.server = &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return exporter
}

// Start begins serving the Prometheus endpoint in a background goroutine.
func (e *PrometheusExporter) Start() {
	go func() {
		if err := e.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("prometheus exporter stopped: %v\n", err)
		}
	}()
}

// Shutdown gracefully stops the metrics server.
func (e *PrometheusExporter) Shutdown(ctx context.Context) error {
	return e.server.Shutdown(ctx)
}

// Observe records one request result.
func (e *PrometheusExporter) Observe(result types.RunResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.collector.Add(result)
}

// Finalize freezes the final summary once the run completes.
func (e *PrometheusExporter) Finalize(summary types.Summary) {
	e.mu.Lock()
	defer e.mu.Unlock()
	snapshot := summary
	e.final = &snapshot
	e.isFinal = true
}

// ServeHTTP renders Prometheus text exposition.
func (e *PrometheusExporter) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	e.mu.Lock()
	if !e.isFinal {
		e.collector.SetWindow(e.startTime, time.Now())
	}
	summary := e.currentSummaryLocked()
	e.mu.Unlock()

	var builder strings.Builder
	writePrometheusLine(&builder, "# HELP loadgen_requests_total Total number of executed requests.")
	writePrometheusLine(&builder, "# TYPE loadgen_requests_total counter")
	writePrometheusLine(&builder, fmt.Sprintf("loadgen_requests_total %d", summary.TotalRequests))
	writePrometheusLine(&builder, "# HELP loadgen_requests_success_total Total number of successful requests.")
	writePrometheusLine(&builder, "# TYPE loadgen_requests_success_total counter")
	writePrometheusLine(&builder, fmt.Sprintf("loadgen_requests_success_total %d", summary.SuccessfulRequests))
	writePrometheusLine(&builder, "# HELP loadgen_requests_failed_total Total number of failed requests.")
	writePrometheusLine(&builder, "# TYPE loadgen_requests_failed_total counter")
	writePrometheusLine(&builder, fmt.Sprintf("loadgen_requests_failed_total %d", summary.FailedRequests))
	writePrometheusLine(&builder, "# HELP loadgen_error_rate Ratio of failed requests.")
	writePrometheusLine(&builder, "# TYPE loadgen_error_rate gauge")
	writePrometheusLine(&builder, fmt.Sprintf("loadgen_error_rate %.6f", summary.ErrorRate))
	writePrometheusLine(&builder, "# HELP loadgen_qps Observed requests per second.")
	writePrometheusLine(&builder, "# TYPE loadgen_qps gauge")
	writePrometheusLine(&builder, fmt.Sprintf("loadgen_qps %.6f", summary.RequestsPerSecond))

	writeDurationGauge(&builder, "loadgen_latency_avg_seconds", "Average request latency.", summary.AvgLatency)
	writeDurationGauge(&builder, "loadgen_latency_min_seconds", "Minimum request latency.", summary.MinLatency)
	writeDurationGauge(&builder, "loadgen_latency_max_seconds", "Maximum request latency.", summary.MaxLatency)
	writeDurationGauge(&builder, "loadgen_ttft_avg_seconds", "Average time to first token.", summary.AvgTTFT)
	writeDurationGauge(&builder, "loadgen_latency_p50_seconds", "Estimated p50 request latency.", summary.Percentiles.P50)
	writeDurationGauge(&builder, "loadgen_latency_p90_seconds", "Estimated p90 request latency.", summary.Percentiles.P90)
	writeDurationGauge(&builder, "loadgen_latency_p95_seconds", "Estimated p95 request latency.", summary.Percentiles.P95)
	writeDurationGauge(&builder, "loadgen_latency_p99_seconds", "Estimated p99 request latency.", summary.Percentiles.P99)

	if summary.TTFTPercentiles.P50 > 0 {
		writeDurationGauge(&builder, "loadgen_ttft_p50_seconds", "Estimated p50 TTFT.", summary.TTFTPercentiles.P50)
		writeDurationGauge(&builder, "loadgen_ttft_p95_seconds", "Estimated p95 TTFT.", summary.TTFTPercentiles.P95)
		writeDurationGauge(&builder, "loadgen_ttft_p99_seconds", "Estimated p99 TTFT.", summary.TTFTPercentiles.P99)
	}
	if summary.ITLPercentiles.P50 > 0 {
		writeDurationGauge(&builder, "loadgen_itl_p50_seconds", "Estimated p50 inter-token latency.", summary.ITLPercentiles.P50)
		writeDurationGauge(&builder, "loadgen_itl_p95_seconds", "Estimated p95 inter-token latency.", summary.ITLPercentiles.P95)
		writeDurationGauge(&builder, "loadgen_itl_p99_seconds", "Estimated p99 inter-token latency.", summary.ITLPercentiles.P99)
	}

	writePrometheusLine(&builder, "# HELP loadgen_output_tokens_total Total output tokens across all requests.")
	writePrometheusLine(&builder, "# TYPE loadgen_output_tokens_total counter")
	writePrometheusLine(&builder, fmt.Sprintf("loadgen_output_tokens_total %d", summary.TotalOutputTokens))
	writePrometheusLine(&builder, "# HELP loadgen_tokens_per_second_avg Average generated tokens per second across token-bearing requests.")
	writePrometheusLine(&builder, "# TYPE loadgen_tokens_per_second_avg gauge")
	writePrometheusLine(&builder, fmt.Sprintf("loadgen_tokens_per_second_avg %.6f", summary.AvgTokensPerSecond))
	writePrometheusLine(&builder, "# HELP loadgen_streaming_aborted_total Total requests marked as streaming aborted.")
	writePrometheusLine(&builder, "# TYPE loadgen_streaming_aborted_total counter")
	writePrometheusLine(&builder, fmt.Sprintf("loadgen_streaming_aborted_total %d", summary.StreamingAborted))

	if len(summary.StatusCodes) > 0 {
		writePrometheusLine(&builder, "# HELP loadgen_status_code_total Total responses by status code.")
		writePrometheusLine(&builder, "# TYPE loadgen_status_code_total counter")
		codes := make([]int, 0, len(summary.StatusCodes))
		for code := range summary.StatusCodes {
			codes = append(codes, code)
		}
		sort.Ints(codes)
		for _, code := range codes {
			writePrometheusLine(&builder, fmt.Sprintf("loadgen_status_code_total{code=%q} %d", strconv.Itoa(code), summary.StatusCodes[code]))
		}
	}

	if len(summary.ErrorCategories) > 0 {
		writePrometheusLine(&builder, "# HELP loadgen_error_category_total Total failures by error category.")
		writePrometheusLine(&builder, "# TYPE loadgen_error_category_total counter")
		categories := make([]string, 0, len(summary.ErrorCategories))
		for category := range summary.ErrorCategories {
			categories = append(categories, string(category))
		}
		sort.Strings(categories)
		for _, category := range categories {
			writePrometheusLine(&builder, fmt.Sprintf("loadgen_error_category_total{category=%q} %d", category, summary.ErrorCategories[types.ErrorCategory(category)]))
		}
	}

	_, _ = w.Write([]byte(builder.String()))
}

func (e *PrometheusExporter) currentSummaryLocked() types.Summary {
	if e.final != nil {
		return *e.final
	}
	return e.collector.Summary()
}

func writeDurationGauge(builder *strings.Builder, name, help string, value time.Duration) {
	writePrometheusLine(builder, fmt.Sprintf("# HELP %s %s", name, help))
	writePrometheusLine(builder, fmt.Sprintf("# TYPE %s gauge", name))
	writePrometheusLine(builder, fmt.Sprintf("%s %.9f", name, value.Seconds()))
}

func writePrometheusLine(builder *strings.Builder, line string) {
	builder.WriteString(line)
	builder.WriteByte('\n')
}
