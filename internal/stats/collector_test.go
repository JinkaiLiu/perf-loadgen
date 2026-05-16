package stats

import (
	"testing"
	"time"

	"github.com/JinkaiLiu/vibeready/pkg/types"
)

func TestCollectorSummary(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	start := time.Now()
	collector.SetWindow(start, start.Add(2*time.Second))

	collector.Add(types.RunResult{Success: true, Latency: 10 * time.Millisecond, StatusCode: 200})
	collector.Add(types.RunResult{Success: true, Latency: 20 * time.Millisecond, StatusCode: 200, TTFT: 5 * time.Millisecond, OutputTokens: 20, TokensPerSecond: 40})
	collector.Add(types.RunResult{Success: false, Latency: 300 * time.Millisecond, StatusCode: 500, ErrorCategory: types.ErrorCategoryHTTPStatus})

	summary := collector.Summary()
	if summary.TotalRequests != 3 {
		t.Fatalf("unexpected total requests %d", summary.TotalRequests)
	}
	if summary.SuccessfulRequests != 2 || summary.FailedRequests != 1 {
		t.Fatalf("unexpected success/failure counts: %#v", summary)
	}
	if summary.AvgLatency <= 0 {
		t.Fatalf("expected positive average latency, got %s", summary.AvgLatency)
	}
	if summary.Percentiles.P95 < summary.Percentiles.P50 {
		t.Fatalf("expected ordered percentiles, got %#v", summary.Percentiles)
	}
	if summary.ErrorCategories[types.ErrorCategoryHTTPStatus] != 1 {
		t.Fatalf("unexpected error category counts: %#v", summary.ErrorCategories)
	}
	if summary.AvgTTFT != 5*time.Millisecond {
		t.Fatalf("unexpected avg ttft: %s", summary.AvgTTFT)
	}
	if summary.TotalOutputTokens != 20 || summary.AvgTokensPerSecond != 40 {
		t.Fatalf("unexpected token metrics: %#v", summary)
	}
}

func TestCollectorEmpty(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	summary := collector.Summary()
	if summary.TotalRequests != 0 {
		t.Fatalf("expected empty summary, got %#v", summary)
	}
}

func TestHistogramExtremeTail(t *testing.T) {
	t.Parallel()

	h := NewHistogram()
	h.Record(1 * time.Millisecond)
	h.Record(2 * time.Millisecond)
	h.Record(10 * time.Second)

	if got := h.Quantile(0.99); got < time.Second {
		t.Fatalf("expected tail quantile to capture long latency, got %s", got)
	}
}

func TestCollectorMergeSnapshot(t *testing.T) {
	t.Parallel()

	left := NewCollector()
	start := time.Now()
	left.SetWindow(start, start.Add(time.Second))
	left.Add(types.RunResult{Success: true, Latency: 10 * time.Millisecond, StatusCode: 200})

	right := NewCollector()
	right.SetWindow(start.Add(500*time.Millisecond), start.Add(2*time.Second))
	right.Add(types.RunResult{Success: false, Latency: 250 * time.Millisecond, StatusCode: 500, ErrorCategory: types.ErrorCategoryHTTPStatus, TTFT: 12 * time.Millisecond, OutputTokens: 50, TokensPerSecond: 25, StreamingAborted: true})

	left.MergeSnapshot(right.Snapshot())
	summary := left.Summary()
	if summary.TotalRequests != 2 {
		t.Fatalf("expected merged total requests to be 2, got %d", summary.TotalRequests)
	}
	if summary.Percentiles.P95 < 200*time.Millisecond {
		t.Fatalf("expected merged histogram to preserve tail latency, got %s", summary.Percentiles.P95)
	}
	if summary.StatusCodes[500] != 1 {
		t.Fatalf("expected merged status code count, got %#v", summary.StatusCodes)
	}
	if summary.AvgTTFT != 12*time.Millisecond || summary.TotalOutputTokens != 50 || summary.StreamingAborted != 1 {
		t.Fatalf("expected merged inference metrics, got %#v", summary)
	}
}

func TestCollectorCountsExplicitCacheMiss(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	start := time.Now()
	collector.SetWindow(start, start.Add(time.Second))
	collector.Add(types.RunResult{Success: true, Latency: time.Millisecond, CacheReported: true, CacheHit: false})

	summary := collector.Summary()
	if summary.CacheHitRate != 0 {
		t.Fatalf("expected explicit cache miss to produce zero hit rate, got %f", summary.CacheHitRate)
	}
	if snapshot := collector.Snapshot(); snapshot.CacheRequests != 1 || snapshot.CacheHits != 0 {
		t.Fatalf("expected cache miss in snapshot, got %#v", snapshot)
	}
}
