package output

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
)

func TestPrometheusExporterServesMetrics(t *testing.T) {
	t.Parallel()

	exporter := NewPrometheusExporter(9099)
	exporter.Observe(types.RunResult{Success: true, Latency: 20 * time.Millisecond, StatusCode: 200, TTFT: 4 * time.Millisecond, OutputTokens: 20, TokensPerSecond: 100})
	exporter.Observe(types.RunResult{Success: false, Latency: 50 * time.Millisecond, StatusCode: 500, ErrorCategory: types.ErrorCategoryHTTPStatus, StreamingAborted: true})

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	exporter.ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, fragment := range []string{
		"loadgen_requests_total 2",
		`loadgen_status_code_total{code="200"} 1`,
		`loadgen_error_category_total{category="http_status"} 1`,
		"loadgen_latency_p95_seconds",
		"loadgen_ttft_avg_seconds",
		"loadgen_output_tokens_total 20",
		"loadgen_streaming_aborted_total 1",
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected metrics output to contain %q, got %q", fragment, body)
		}
	}
}
