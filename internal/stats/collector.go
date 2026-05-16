package stats

import (
	"time"

	"github.com/JinkaiLiu/vibeready/internal/util"
	"github.com/JinkaiLiu/vibeready/pkg/types"
)

// Collector aggregates request results without storing unbounded raw samples.
type Collector struct {
	startTime          time.Time
	endTime            time.Time
	totalRequests      int64
	successRequests    int64
	failedRequests     int64
	totalLatency       time.Duration
	minLatency         time.Duration
	maxLatency         time.Duration
	requestsWithTTFT   int64
	totalTTFT          time.Duration
	minTTFT            time.Duration
	maxTTFT            time.Duration
	requestsWithTokens int64
	totalOutputTokens  int64
	tokensEstimated    bool
	totalTokenRate     float64
	streamingAborted   int64
	statusCodes        map[int]int64
	errorCategories    map[types.ErrorCategory]int64
	histogram          *Histogram
	ttftHist           *Histogram
	itlHist            *Histogram

	// Layer 2: semi-white-box AI metrics.
	totalUpstreamLatency time.Duration
	requestsWithUpstream int64
	totalInputTokens     int64
	cacheHits            int64
	cacheRequests        int64
	provider             string
	model                string
	modelPricePer1K      float64
}

// SetPricing configures the model price for cost estimation ($/1K output tokens).
func (c *Collector) SetPricing(pricePer1K float64) {
	c.modelPricePer1K = pricePer1K
}

// NewCollector creates an empty metrics collector.
func NewCollector() *Collector {
	return &Collector{
		statusCodes:     make(map[int]int64),
		errorCategories: make(map[types.ErrorCategory]int64),
		histogram:       NewHistogram(),
		ttftHist:        NewHistogram(),
		itlHist:         NewHistogram(),
	}
}

// NewCollectorFromSnapshot reconstructs a collector from a mergeable snapshot.
func NewCollectorFromSnapshot(snapshot types.AggregateSnapshot) *Collector {
	c := NewCollector()
	c.MergeSnapshot(snapshot)
	return c
}

// SetWindow records the run time window for throughput and reporting.
func (c *Collector) SetWindow(start, end time.Time) {
	c.startTime = start
	c.endTime = end
}

// Add records one request result.
func (c *Collector) Add(result types.RunResult) {
	if c.totalRequests == 0 || result.Latency < c.minLatency {
		c.minLatency = result.Latency
	}
	if result.Latency > c.maxLatency {
		c.maxLatency = result.Latency
	}

	c.totalRequests++
	c.totalLatency += result.Latency
	c.histogram.Record(result.Latency)

	if result.TTFT > 0 {
		c.requestsWithTTFT++
		c.totalTTFT += result.TTFT
		c.ttftHist.Record(result.TTFT)
		if c.minTTFT == 0 || result.TTFT < c.minTTFT {
			c.minTTFT = result.TTFT
		}
		if result.TTFT > c.maxTTFT {
			c.maxTTFT = result.TTFT
		}
	}

	for _, sample := range result.ITLSamples {
		if sample > 0 {
			c.itlHist.Record(sample)
		}
	}

	if result.OutputTokens > 0 {
		c.requestsWithTokens++
		c.totalOutputTokens += result.OutputTokens
		if result.TokensEstimated {
			c.tokensEstimated = true
		}
	}
	if result.TokensPerSecond > 0 {
		c.totalTokenRate += result.TokensPerSecond
	}
	if result.StreamingAborted {
		c.streamingAborted++
	}

	if result.Success {
		c.successRequests++
	} else {
		c.failedRequests++
	}

	if result.StatusCode > 0 {
		c.statusCodes[result.StatusCode]++
	}
	if result.ErrorCategory != types.ErrorCategoryNone {
		c.errorCategories[result.ErrorCategory]++
	}

	// Layer 2: semi-white-box AI metrics.
	if result.UpstreamLatency > 0 {
		c.requestsWithUpstream++
		c.totalUpstreamLatency += result.UpstreamLatency
	}
	if result.InputTokens > 0 {
		c.totalInputTokens += result.InputTokens
	}
	// Provider/Model: record from the first response that sets them.
	if result.Provider != "" && c.provider == "" {
		c.provider = result.Provider
	}
	if result.Model != "" && c.model == "" {
		c.model = result.Model
	}
	// Cache hit: only count requests where the server explicitly reported cache status.
	if result.CacheReported || result.CacheHit {
		c.cacheRequests++
		if result.CacheHit {
			c.cacheHits++
		}
	}
}

// Snapshot returns a mergeable collector snapshot for distributed aggregation.
func (c *Collector) Snapshot() types.AggregateSnapshot {
	return types.AggregateSnapshot{
		StartTime:                  c.startTime,
		EndTime:                    c.endTime,
		TotalRequests:              c.totalRequests,
		SuccessfulRequests:         c.successRequests,
		FailedRequests:             c.failedRequests,
		TotalLatencyMicros:         c.totalLatency.Microseconds(),
		MinLatencyMicros:           c.minLatency.Microseconds(),
		MaxLatencyMicros:           c.maxLatency.Microseconds(),
		RequestsWithTTFT:           c.requestsWithTTFT,
		TotalTTFTMicros:            c.totalTTFT.Microseconds(),
		RequestsWithTokens:         c.requestsWithTokens,
		TotalOutputTokens:          c.totalOutputTokens,
		TotalTokenRate:             c.totalTokenRate,
		StreamingAborted:           c.streamingAborted,
		StatusCodes:                cloneStatusMap(c.statusCodes),
		ErrorCategories:            cloneErrorMap(c.errorCategories),
		Histogram:                  c.histogram.Snapshot(),
		TTFTHistogram:              c.ttftHist.Snapshot(),
		ITLHistogram:               c.itlHist.Snapshot(),
		TotalUpstreamLatencyMicros: c.totalUpstreamLatency.Microseconds(),
		RequestsWithUpstream:       c.requestsWithUpstream,
		TotalInputTokens:           c.totalInputTokens,
		CacheHits:                  c.cacheHits,
		CacheRequests:              c.cacheRequests,
		Provider:                   c.provider,
		Model:                      c.model,
	}
}

// MergeSnapshot combines a worker aggregate snapshot into this collector.
func (c *Collector) MergeSnapshot(snapshot types.AggregateSnapshot) {
	if !snapshot.StartTime.IsZero() && (c.startTime.IsZero() || snapshot.StartTime.Before(c.startTime)) {
		c.startTime = snapshot.StartTime
	}
	if snapshot.EndTime.After(c.endTime) {
		c.endTime = snapshot.EndTime
	}
	if snapshot.TotalRequests == 0 {
		return
	}

	incomingMin := time.Duration(snapshot.MinLatencyMicros) * time.Microsecond
	incomingMax := time.Duration(snapshot.MaxLatencyMicros) * time.Microsecond
	if c.totalRequests == 0 || (incomingMin > 0 && incomingMin < c.minLatency) {
		c.minLatency = incomingMin
	}
	if incomingMax > c.maxLatency {
		c.maxLatency = incomingMax
	}

	c.totalRequests += snapshot.TotalRequests
	c.successRequests += snapshot.SuccessfulRequests
	c.failedRequests += snapshot.FailedRequests
	c.totalLatency += time.Duration(snapshot.TotalLatencyMicros) * time.Microsecond
	c.requestsWithTTFT += snapshot.RequestsWithTTFT
	c.totalTTFT += time.Duration(snapshot.TotalTTFTMicros) * time.Microsecond
	c.requestsWithTokens += snapshot.RequestsWithTokens
	c.totalOutputTokens += snapshot.TotalOutputTokens
	c.totalTokenRate += snapshot.TotalTokenRate
	c.streamingAborted += snapshot.StreamingAborted
	c.histogram.Merge(snapshot.Histogram)
	c.ttftHist.Merge(snapshot.TTFTHistogram)
	c.itlHist.Merge(snapshot.ITLHistogram)

	// Layer 2 merge.
	c.totalUpstreamLatency += time.Duration(snapshot.TotalUpstreamLatencyMicros) * time.Microsecond
	c.requestsWithUpstream += snapshot.RequestsWithUpstream
	c.totalInputTokens += snapshot.TotalInputTokens
	c.cacheHits += snapshot.CacheHits
	c.cacheRequests += snapshot.CacheRequests
	if snapshot.Provider != "" && c.provider == "" {
		c.provider = snapshot.Provider
	}
	if snapshot.Model != "" && c.model == "" {
		c.model = snapshot.Model
	}

	for code, count := range snapshot.StatusCodes {
		c.statusCodes[code] += count
	}
	for category, count := range snapshot.ErrorCategories {
		c.errorCategories[category] += count
	}
}

// Summary returns the aggregated run report.
func (c *Collector) Summary() types.Summary {
	summary := types.Summary{
		StartTime:       c.startTime,
		EndTime:         c.endTime,
		Duration:        c.endTime.Sub(c.startTime),
		TotalRequests:   c.totalRequests,
		StatusCodes:     cloneStatusMap(c.statusCodes),
		ErrorCategories: cloneErrorMap(c.errorCategories),
	}
	if c.totalRequests == 0 {
		return summary
	}

	summary.SuccessfulRequests = c.successRequests
	summary.FailedRequests = c.failedRequests
	summary.ErrorRate = float64(c.failedRequests) / float64(c.totalRequests)
	summary.AvgLatency = c.totalLatency / time.Duration(c.totalRequests)
	summary.AvgLatencyHuman = util.FormatDuration(summary.AvgLatency)
	summary.MinLatency = c.minLatency
	summary.MinLatencyHuman = util.FormatDuration(c.minLatency)
	summary.MaxLatency = c.maxLatency
	summary.MaxLatencyHuman = util.FormatDuration(c.maxLatency)

	if c.requestsWithTTFT > 0 {
		summary.AvgTTFT = c.totalTTFT / time.Duration(c.requestsWithTTFT)
		summary.AvgTTFTHuman = util.FormatDuration(summary.AvgTTFT)
		summary.TTFTPercentiles = types.Percentiles{
			P50:      c.ttftHist.Quantile(0.50),
			P90:      c.ttftHist.Quantile(0.90),
			P95:      c.ttftHist.Quantile(0.95),
			P99:      c.ttftHist.Quantile(0.99),
			P50Human: util.FormatDuration(c.ttftHist.Quantile(0.50)),
			P90Human: util.FormatDuration(c.ttftHist.Quantile(0.90)),
			P95Human: util.FormatDuration(c.ttftHist.Quantile(0.95)),
			P99Human: util.FormatDuration(c.ttftHist.Quantile(0.99)),
		}
	}

	if c.itlHist.Total() > 0 {
		summary.ITLPercentiles = types.Percentiles{
			P50:      c.itlHist.Quantile(0.50),
			P90:      c.itlHist.Quantile(0.90),
			P95:      c.itlHist.Quantile(0.95),
			P99:      c.itlHist.Quantile(0.99),
			P50Human: util.FormatDuration(c.itlHist.Quantile(0.50)),
			P90Human: util.FormatDuration(c.itlHist.Quantile(0.90)),
			P95Human: util.FormatDuration(c.itlHist.Quantile(0.95)),
			P99Human: util.FormatDuration(c.itlHist.Quantile(0.99)),
		}
	}

	summary.TotalOutputTokens = c.totalOutputTokens
	summary.TokensEstimated = c.tokensEstimated
	if c.requestsWithTokens > 0 {
		summary.AvgTokensPerSecond = c.totalTokenRate / float64(c.requestsWithTokens)
	}
	summary.StreamingAborted = c.streamingAborted

	// Layer 2: derived AI metrics.
	if c.requestsWithUpstream > 0 {
		summary.AvgUpstreamLatency = c.totalUpstreamLatency / time.Duration(c.requestsWithUpstream)
		summary.AvgUpstreamLatencyHuman = util.FormatDuration(summary.AvgUpstreamLatency)
	}
	if summary.AvgLatency > 0 && summary.AvgUpstreamLatency > 0 {
		summary.AvgBackendOverhead = summary.AvgLatency - summary.AvgUpstreamLatency
		summary.AvgBackendOverheadHuman = util.FormatDuration(summary.AvgBackendOverhead)
		summary.UpstreamLatencyRatio = float64(summary.AvgUpstreamLatency) / float64(summary.AvgLatency)
	}
	summary.TotalInputTokens = c.totalInputTokens
	if c.cacheRequests > 0 {
		summary.CacheHitRate = float64(c.cacheHits) / float64(c.cacheRequests)
	}
	summary.Provider = c.provider
	summary.Model = c.model
	if c.modelPricePer1K > 0 && c.totalOutputTokens > 0 {
		summary.EstimatedCost = float64(c.totalOutputTokens) * c.modelPricePer1K / 1000.0
	}

	summary.Percentiles = types.Percentiles{
		P50:      c.histogram.Quantile(0.50),
		P90:      c.histogram.Quantile(0.90),
		P95:      c.histogram.Quantile(0.95),
		P99:      c.histogram.Quantile(0.99),
		P50Human: util.FormatDuration(c.histogram.Quantile(0.50)),
		P90Human: util.FormatDuration(c.histogram.Quantile(0.90)),
		P95Human: util.FormatDuration(c.histogram.Quantile(0.95)),
		P99Human: util.FormatDuration(c.histogram.Quantile(0.99)),
	}

	durationSeconds := summary.Duration.Seconds()
	if durationSeconds > 0 {
		summary.RequestsPerSecond = float64(c.totalRequests) / durationSeconds
	}
	summary.DurationHuman = util.FormatDuration(summary.Duration)

	return summary
}

func cloneStatusMap(src map[int]int64) map[int]int64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[int]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneErrorMap(src map[types.ErrorCategory]int64) map[types.ErrorCategory]int64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[types.ErrorCategory]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
