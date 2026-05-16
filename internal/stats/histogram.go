package stats

import (
	"math"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
)

// Histogram stores latency counts in logarithmic buckets.
type Histogram struct {
	bounds []time.Duration
	counts []int64
}

// NewHistogram builds a fixed log-scale histogram from 1us to 60s.
func NewHistogram() *Histogram {
	bounds := make([]time.Duration, 0, 93)
	current := 1.0
	maxValue := float64((60 * time.Second).Microseconds())
	for current < maxValue {
		bounds = append(bounds, time.Duration(current)*time.Microsecond)
		current *= 1.25
	}
	bounds = append(bounds, 60*time.Second)

	return &Histogram{
		bounds: bounds,
		counts: make([]int64, len(bounds)),
	}
}

// NewHistogramFromSnapshot reconstructs a histogram from a wire snapshot.
func NewHistogramFromSnapshot(snapshot types.HistogramSnapshot) *Histogram {
	bounds := make([]time.Duration, len(snapshot.BoundsMicros))
	for i, bound := range snapshot.BoundsMicros {
		bounds[i] = time.Duration(bound) * time.Microsecond
	}
	counts := append([]int64(nil), snapshot.Counts...)

	return &Histogram{
		bounds: bounds,
		counts: counts,
	}
}

// Snapshot returns a mergeable representation of the histogram.
func (h *Histogram) Snapshot() types.HistogramSnapshot {
	boundsMicros := make([]int64, len(h.bounds))
	for i, bound := range h.bounds {
		boundsMicros[i] = bound.Microseconds()
	}

	return types.HistogramSnapshot{
		BoundsMicros: boundsMicros,
		Counts:       append([]int64(nil), h.counts...),
	}
}

// Merge combines another histogram snapshot into this histogram.
func (h *Histogram) Merge(snapshot types.HistogramSnapshot) {
	if len(snapshot.Counts) == 0 {
		return
	}
	if len(h.counts) == 0 {
		merged := NewHistogramFromSnapshot(snapshot)
		h.bounds = merged.bounds
		h.counts = merged.counts
		return
	}
	if len(h.counts) != len(snapshot.Counts) {
		return
	}
	for i, count := range snapshot.Counts {
		h.counts[i] += count
	}
}

// Record adds a latency sample to the closest upper bucket.
func (h *Histogram) Record(latency time.Duration) {
	if latency < 0 {
		latency = 0
	}
	index := h.bucketIndex(latency)
	h.counts[index]++
}

// Quantile returns the approximate latency at the requested percentile.
func (h *Histogram) Quantile(q float64) time.Duration {
	total := h.Total()
	if total == 0 {
		return 0
	}
	if q <= 0 {
		return h.MinNonZero()
	}
	if q >= 1 {
		return h.MaxNonZero()
	}

	target := int64(math.Ceil(float64(total) * q))
	var cumulative int64
	for i, count := range h.counts {
		cumulative += count
		if cumulative >= target {
			return h.bounds[i]
		}
	}
	return h.bounds[len(h.bounds)-1]
}

// Total returns the number of samples recorded in the histogram.
func (h *Histogram) Total() int64 {
	var total int64
	for _, count := range h.counts {
		total += count
	}
	return total
}

func (h *Histogram) MinNonZero() time.Duration {
	for i, count := range h.counts {
		if count > 0 {
			return h.bounds[i]
		}
	}
	return 0
}

func (h *Histogram) MaxNonZero() time.Duration {
	for i := len(h.counts) - 1; i >= 0; i-- {
		if h.counts[i] > 0 {
			return h.bounds[i]
		}
	}
	return 0
}

func (h *Histogram) bucketIndex(latency time.Duration) int {
	for i, bound := range h.bounds {
		if latency <= bound {
			return i
		}
	}
	return len(h.bounds) - 1
}
