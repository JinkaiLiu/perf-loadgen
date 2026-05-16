package output

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/JinkaiLiu/vibeready/internal/util"
	"github.com/JinkaiLiu/vibeready/pkg/types"
)

type row struct {
	label string
	value string
}

// WriteConsoleSummary prints a concise human-readable summary.
func WriteConsoleSummary(w io.Writer, summary types.Summary) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	rows := []row{
		{"Total:", fmt.Sprintf("%d", summary.TotalRequests)},
		{"Success:", fmt.Sprintf("%d", summary.SuccessfulRequests)},
		{"Failed:", fmt.Sprintf("%d", summary.FailedRequests)},
		{"Error Rate:", fmt.Sprintf("%.2f%%", summary.ErrorRate*100)},
		{"QPS:", fmt.Sprintf("%.2f", summary.RequestsPerSecond)},
		{"Avg:", util.FormatDuration(summary.AvgLatency)},
		{"Min:", util.FormatDuration(summary.MinLatency)},
		{"Max:", util.FormatDuration(summary.MaxLatency)},
		{"Avg TTFT:", util.FormatDuration(summary.AvgTTFT)},
		{"Output Tokens:", formatTokens(summary.TotalOutputTokens, summary.TokensEstimated)},
		{"Avg tok/s:", formatTokS(summary.AvgTokensPerSecond, summary.TokensEstimated)},
		{"Stream Aborts:", fmt.Sprintf("%d", summary.StreamingAborted)},
		{"P50:", util.FormatDuration(summary.Percentiles.P50)},
		{"P90:", util.FormatDuration(summary.Percentiles.P90)},
		{"P95:", util.FormatDuration(summary.Percentiles.P95)},
		{"P99:", util.FormatDuration(summary.Percentiles.P99)},
	}

	if summary.TTFTPercentiles.P50 > 0 {
		rows = append(rows,
			row{"TTFT P50:", util.FormatDuration(summary.TTFTPercentiles.P50)},
			row{"TTFT P95:", util.FormatDuration(summary.TTFTPercentiles.P95)},
			row{"TTFT P99:", util.FormatDuration(summary.TTFTPercentiles.P99)},
		)
	}
	if summary.ITLPercentiles.P50 > 0 {
		rows = append(rows,
			row{"ITL P50:", util.FormatDuration(summary.ITLPercentiles.P50)},
			row{"ITL P95:", util.FormatDuration(summary.ITLPercentiles.P95)},
			row{"ITL P99:", util.FormatDuration(summary.ITLPercentiles.P99)},
		)
	}

	// Layer 2: AI metrics (only when upstream data is available).
	if summary.AvgUpstreamLatency > 0 {
		rows = append(rows,
			row{"", ""},
			row{"Upstream:", util.FormatDuration(summary.AvgUpstreamLatency)},
			row{"Overhead:", util.FormatDuration(summary.AvgBackendOverhead)},
			row{fmt.Sprintf("Upstream %%:"), fmt.Sprintf("%.1f%%", summary.UpstreamLatencyRatio*100)},
		)
		if summary.Provider != "" {
			rows = append(rows, row{"Provider:", summary.Provider})
		}
		if summary.Model != "" {
			rows = append(rows, row{"Model:", summary.Model})
		}
		if summary.TotalInputTokens > 0 {
			rows = append(rows, row{"Input Tokens:", fmt.Sprintf("%d", summary.TotalInputTokens)})
		}
		if summary.CacheHitRate > 0 || summary.Provider != "" {
			rows = append(rows, row{"Cache Hit %:", fmt.Sprintf("%.1f%%", summary.CacheHitRate*100)})
		}
		if summary.EstimatedCost > 0 {
			rows = append(rows, row{"Est. Cost:", fmt.Sprintf("$%.4f", summary.EstimatedCost)})
		}
	}

	// Status code highlights.
	if count, ok := summary.StatusCodes[429]; ok && count > 0 {
		pct := float64(count) / float64(summary.TotalRequests) * 100
		rows = append(rows, row{"429 Count:", fmt.Sprintf("%d (%.1f%%)", count, pct)})
	}

	for _, r := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", r.label, r.value); err != nil {
			return err
		}
	}

	return tw.Flush()
}

func formatTokens(n int64, estimated bool) string {
	if estimated {
		return fmt.Sprintf("%d (est.)", n)
	}
	return fmt.Sprintf("%d", n)
}

func formatTokS(n float64, estimated bool) string {
	if estimated {
		return fmt.Sprintf("%.2f (est.)", n)
	}
	return fmt.Sprintf("%.2f", n)
}
