package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JinkaiLiu/vibeready/internal/config"
	"github.com/JinkaiLiu/vibeready/pkg/types"
)

type fakeRunner struct {
	running int64
	maxSeen int64
	delay   time.Duration
	result  types.RunResult
	count   int64
	mu      sync.Mutex
	bodies  []string
}

func (f *fakeRunner) Run(ctx context.Context, req types.RequestSpec) (types.RunResult, error) {
	current := atomic.AddInt64(&f.running, 1)
	for {
		maxSeen := atomic.LoadInt64(&f.maxSeen)
		if current <= maxSeen || atomic.CompareAndSwapInt64(&f.maxSeen, maxSeen, current) {
			break
		}
	}
	defer atomic.AddInt64(&f.running, -1)
	atomic.AddInt64(&f.count, 1)
	f.mu.Lock()
	f.bodies = append(f.bodies, string(req.Body))
	f.mu.Unlock()

	select {
	case <-ctx.Done():
		return types.RunResult{Success: false, ErrorCategory: types.ErrorCategoryTimeout}, ctx.Err()
	case <-time.After(f.delay):
		return f.result, nil
	}
}

func (f *fakeRunner) totalCalls() int64 {
	return atomic.LoadInt64(&f.count)
}

func (f *fakeRunner) seenBodies() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.bodies))
	copy(out, f.bodies)
	return out
}

func TestEngineRun(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		delay:  5 * time.Millisecond,
		result: types.RunResult{Success: true, Latency: 5 * time.Millisecond, StatusCode: 200},
	}

	cfg := config.Config{
		URL:         "http://localhost",
		Method:      "GET",
		Concurrency: 4,
		Duration:    50 * time.Millisecond,
		Timeout:     time.Second,
	}

	summary, err := New(runner).Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if summary.TotalRequests == 0 {
		t.Fatal("expected at least one request")
	}
	if summary.TotalRequests != summary.SuccessfulRequests+summary.FailedRequests {
		t.Fatalf("summary counts do not add up: %#v", summary)
	}
	if atomic.LoadInt64(&runner.maxSeen) > int64(cfg.Concurrency) {
		t.Fatalf("observed concurrency %d exceeds configured %d", runner.maxSeen, cfg.Concurrency)
	}
}

func TestEngineStopsSlowWorkers(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		delay:  200 * time.Millisecond,
		result: types.RunResult{Success: true, Latency: 200 * time.Millisecond, StatusCode: 200},
	}

	cfg := config.Config{
		URL:         "http://localhost",
		Method:      "GET",
		Concurrency: 2,
		Duration:    30 * time.Millisecond,
		Timeout:     time.Second,
	}

	start := time.Now()
	summary, err := New(runner).Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if time.Since(start) > 300*time.Millisecond {
		t.Fatalf("engine took too long to stop slow workers")
	}
	if summary.TotalRequests != summary.SuccessfulRequests+summary.FailedRequests {
		t.Fatalf("summary counts do not add up: %#v", summary)
	}
}

func TestEngineQPSLimit(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		result: types.RunResult{Success: true, Latency: time.Millisecond, StatusCode: 200},
	}
	cfg := config.Config{
		URL:         "http://localhost",
		Method:      "GET",
		Concurrency: 8,
		Duration:    300 * time.Millisecond,
		Timeout:     time.Second,
		QPS:         10,
	}

	summary, err := New(runner).Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if summary.TotalRequests > 5 {
		t.Fatalf("expected qps limiter to constrain requests, got %d", summary.TotalRequests)
	}
}

func TestEngineRampUpReducesEarlyBurst(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		delay:  10 * time.Millisecond,
		result: types.RunResult{Success: true, Latency: 10 * time.Millisecond, StatusCode: 200},
	}

	noRampCfg := config.Config{
		URL:         "http://localhost",
		Method:      "GET",
		Concurrency: 6,
		Duration:    120 * time.Millisecond,
		Timeout:     time.Second,
	}
	rampCfg := noRampCfg
	rampCfg.RampUp = 90 * time.Millisecond

	noRampSummary, err := New(runner).Run(context.Background(), noRampCfg)
	if err != nil {
		t.Fatalf("no-ramp run returned error: %v", err)
	}

	runnerWithRamp := &fakeRunner{
		delay:  10 * time.Millisecond,
		result: types.RunResult{Success: true, Latency: 10 * time.Millisecond, StatusCode: 200},
	}
	rampSummary, err := New(runnerWithRamp).Run(context.Background(), rampCfg)
	if err != nil {
		t.Fatalf("ramp run returned error: %v", err)
	}

	if rampSummary.TotalRequests >= noRampSummary.TotalRequests {
		t.Fatalf("expected ramp-up to reduce early load, got ramp=%d no-ramp=%d", rampSummary.TotalRequests, noRampSummary.TotalRequests)
	}
}

func TestEngineStopsAtRequestCount(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		result: types.RunResult{Success: true, Latency: time.Millisecond, StatusCode: 200},
	}
	cfg := config.Config{
		URL:         "http://localhost",
		Method:      "GET",
		Concurrency: 4,
		Requests:    11,
		Timeout:     time.Second,
	}

	summary, err := New(runner).Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if summary.TotalRequests != 11 {
		t.Fatalf("expected exactly 11 requests, got %d", summary.TotalRequests)
	}
}

func TestEngineRotatesPayloads(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		result: types.RunResult{Success: true, Latency: time.Millisecond, StatusCode: 200},
	}
	cfg := config.Config{
		URL:         "http://localhost",
		Method:      "POST",
		Concurrency: 1,
		Requests:    4,
		Timeout:     time.Second,
		Payloads: [][]byte{
			[]byte(`{"prompt":"a"}`),
			[]byte(`{"prompt":"b"}`),
		},
	}

	_, err := New(runner).Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	bodies := runner.seenBodies()
	if len(bodies) != 4 {
		t.Fatalf("expected 4 bodies, got %d", len(bodies))
	}
	expected := []string{`{"prompt":"a"}`, `{"prompt":"b"}`, `{"prompt":"a"}`, `{"prompt":"b"}`}
	for i, body := range bodies {
		if body != expected[i] {
			t.Fatalf("unexpected payload rotation at %d: got %q want %q", i, body, expected[i])
		}
	}
}
