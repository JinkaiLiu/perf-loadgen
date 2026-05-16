package stats

import (
	"testing"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
)

func BenchmarkCollectorAdd(b *testing.B) {
	result := types.RunResult{Success: true, Latency: 25 * time.Millisecond, StatusCode: 200}
	b.ResetTimer()
	for b.Loop() {
		collector := NewCollector()
		for range 1024 {
			collector.Add(result)
		}
	}
}

func BenchmarkHistogramQuantile(b *testing.B) {
	h := NewHistogram()
	for i := 0; i < 100000; i++ {
		h.Record(time.Duration((i%1000)+1) * time.Microsecond)
	}

	b.ResetTimer()
	for b.Loop() {
		_ = h.Quantile(0.99)
	}
}
