package engine

import (
	"context"
	"testing"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/internal/config"
	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
)

type benchmarkRunner struct{}

func (benchmarkRunner) Run(ctx context.Context, req types.RequestSpec) (types.RunResult, error) {
	return types.RunResult{Success: true, Latency: time.Millisecond, StatusCode: 200}, nil
}

func BenchmarkEngineRun(b *testing.B) {
	cfg := config.Config{
		URL:         "http://localhost",
		Method:      "GET",
		Concurrency: 1000,
		Duration:    10 * time.Millisecond,
		Timeout:     time.Second,
	}

	engine := New(benchmarkRunner{})

	b.ResetTimer()
	for b.Loop() {
		if _, err := engine.Run(context.Background(), cfg); err != nil {
			b.Fatalf("Run returned error: %v", err)
		}
	}
}
