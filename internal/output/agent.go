package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/JinkaiLiu/vibeready/internal/config"
	"github.com/JinkaiLiu/vibeready/internal/util"
	"github.com/JinkaiLiu/vibeready/pkg/types"
)

// WriteAgentReport generates a Markdown report designed to be pasted into a coding agent.
// It includes diagnostic interpretation of the metrics — not just raw numbers.
func WriteAgentReport(path string, cfg config.Config, summary types.Summary) error {
	var b strings.Builder

	b.WriteString("# VibeReady Report\n\n")

	// Context.
	b.WriteString("## Test Configuration\n\n")
	b.WriteString(fmt.Sprintf("- **Target**: `%s %s`\n", cfg.Method, cfg.URL))
	b.WriteString(fmt.Sprintf("- **Concurrency**: %d\n", cfg.Concurrency))
	if cfg.Duration > 0 {
		b.WriteString(fmt.Sprintf("- **Duration**: %s\n", cfg.Duration))
	}
	if cfg.Requests > 0 {
		b.WriteString(fmt.Sprintf("- **Requests**: %d\n", cfg.Requests))
	}
	b.WriteString(fmt.Sprintf("- **Timeout**: %s\n", cfg.Timeout))
	if cfg.Protocol != "" && cfg.Protocol != "http" {
		b.WriteString(fmt.Sprintf("- **Protocol**: %s\n", cfg.Protocol))
	}
	b.WriteString("\n")

	// Summary.
	b.WriteString("## Results\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|--------|-------|\n")
	b.WriteString(fmt.Sprintf("| Total Requests | %d |\n", summary.TotalRequests))
	b.WriteString(fmt.Sprintf("| Successful | %d |\n", summary.SuccessfulRequests))
	b.WriteString(fmt.Sprintf("| Failed | %d |\n", summary.FailedRequests))
	b.WriteString(fmt.Sprintf("| Error Rate | %.2f%% |\n", summary.ErrorRate*100))
	b.WriteString(fmt.Sprintf("| QPS | %.2f |\n", summary.RequestsPerSecond))
	b.WriteString(fmt.Sprintf("| Avg Latency | %s |\n", util.FormatDuration(summary.AvgLatency)))
	b.WriteString(fmt.Sprintf("| P50 | %s |\n", util.FormatDuration(summary.Percentiles.P50)))
	b.WriteString(fmt.Sprintf("| P95 | %s |\n", util.FormatDuration(summary.Percentiles.P95)))
	b.WriteString(fmt.Sprintf("| P99 | %s |\n", util.FormatDuration(summary.Percentiles.P99)))

	if summary.AvgTTFT > 0 {
		b.WriteString(fmt.Sprintf("| Avg TTFT | %s |\n", util.FormatDuration(summary.AvgTTFT)))
	}
	if summary.TotalOutputTokens > 0 {
		b.WriteString(fmt.Sprintf("| Output Tokens | %d |\n", summary.TotalOutputTokens))
		b.WriteString(fmt.Sprintf("| Avg tok/s | %.2f |\n", summary.AvgTokensPerSecond))
	}
	if summary.StreamingAborted > 0 {
		b.WriteString(fmt.Sprintf("| Stream Aborts | %d |\n", summary.StreamingAborted))
	}
	b.WriteString("\n")

	// AI-specific diagnostics (Layer 2).
	if summary.AvgUpstreamLatency > 0 {
		b.WriteString("## AI Model Diagnostics\n\n")
		b.WriteString("| Metric | Value |\n")
		b.WriteString("|--------|-------|\n")
		b.WriteString(fmt.Sprintf("| Upstream (model) Latency | %s |\n", util.FormatDuration(summary.AvgUpstreamLatency)))
		b.WriteString(fmt.Sprintf("| Backend Overhead | %s |\n", util.FormatDuration(summary.AvgBackendOverhead)))
		b.WriteString(fmt.Sprintf("| Upstream Ratio | %.1f%% |\n", summary.UpstreamLatencyRatio*100))
		if summary.Provider != "" {
			b.WriteString(fmt.Sprintf("| Provider | %s |\n", summary.Provider))
		}
		if summary.Model != "" {
			b.WriteString(fmt.Sprintf("| Model | %s |\n", summary.Model))
		}
		if summary.TotalInputTokens > 0 {
			b.WriteString(fmt.Sprintf("| Input Tokens | %d |\n", summary.TotalInputTokens))
		}
		if summary.CacheHitRate > 0 || summary.Provider != "" {
			b.WriteString(fmt.Sprintf("| Cache Hit Rate | %.1f%% |\n", summary.CacheHitRate*100))
		}
		if summary.EstimatedCost > 0 {
			b.WriteString(fmt.Sprintf("| Estimated Cost | $%.4f |\n", summary.EstimatedCost))
		}
		b.WriteString("\n")
	}

	// Error breakdown.
	if len(summary.StatusCodes) > 0 {
		b.WriteString("## Status Code Distribution\n\n")
		b.WriteString("| Code | Count |\n")
		b.WriteString("|------|-------|\n")
		for code, count := range summary.StatusCodes {
			b.WriteString(fmt.Sprintf("| %d | %d |\n", code, count))
		}
		b.WriteString("\n")
	}

	if len(summary.ErrorCategories) > 0 {
		b.WriteString("## Error Categories\n\n")
		b.WriteString("| Category | Count |\n")
		b.WriteString("|----------|-------|\n")
		for cat, count := range summary.ErrorCategories {
			b.WriteString(fmt.Sprintf("| %s | %d |\n", cat, count))
		}
		b.WriteString("\n")
	}

	// Diagnostic interpretation.
	b.WriteString("## Diagnostic Notes\n\n")
	writeDiagnostics(&b, summary)

	// Re-test command.
	b.WriteString("## Re-test Command\n\n")
	b.WriteString("Run this after making fixes to verify improvement:\n\n")
	b.WriteString("```bash\n")
	b.WriteString(buildRetestCommand(cfg))
	b.WriteString("\n```\n")

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeDiagnostics(b *strings.Builder, s types.Summary) {
	b.WriteString("*Diagnostic notes are heuristics based on observed metrics. They are not definitive — use them as starting points for investigation, not final conclusions.*\n\n")
	issues := 0

	if s.ErrorRate > 0.02 {
		b.WriteString(fmt.Sprintf("- **Elevated error rate (%.2f%%)**: Common causes: timeouts, rate limiting, or upstream server errors. ", s.ErrorRate*100))
		if s.TotalRequests < 100 {
			b.WriteString("Small sample size — consider running a longer test for a more reliable signal.\n")
		}
		issues++
	}
	if s.Percentiles.P99 > s.AvgLatency*5 {
		b.WriteString(fmt.Sprintf("- **Long tail latency**: P99 (%s) is %.1fx the average. ", util.FormatDuration(s.Percentiles.P99), float64(s.Percentiles.P99)/float64(s.AvgLatency)))
		b.WriteString("May indicate resource contention, GC pauses, or queuing under load. If P99 is acceptable for your use case, this can be ignored.\n")
		issues++
	}
	if s.UpstreamLatencyRatio > 0 && s.UpstreamLatencyRatio > 0.85 {
		b.WriteString(fmt.Sprintf("- **High upstream latency ratio (%.0f%% of total time)**: ", s.UpstreamLatencyRatio*100))
		b.WriteString("The model API dominates response time. If this ratio is accurate (depends on correct `x-ai-upstream-latency-ms` header from your backend), optimizing backend code will have limited impact. Consider:\n")
		b.WriteString("  - A faster model or provider\n")
		b.WriteString("  - Enabling streaming if not already used\n")
		b.WriteString("  - Adding caching for repeated prompts\n")
		issues++
	}
	if s.UpstreamLatencyRatio > 0 && s.UpstreamLatencyRatio < 0.2 {
		b.WriteString(fmt.Sprintf("- **Low upstream latency ratio (%.0f%% overhead in backend)**: ", (1-s.UpstreamLatencyRatio)*100))
		b.WriteString("The model responds quickly but your backend adds noticeable latency. If the upstream ratio is accurate, common causes include:\n")
		b.WriteString("  - Connection pooling and keep-alive settings\n")
		b.WriteString("  - Synchronous processing that could be parallelized\n")
		b.WriteString("  - Serialization/deserialization overhead\n")
		issues++
	}
	if s.TotalRequests > 0 && float64(s.StreamingAborted)/float64(s.TotalRequests) > 0.01 {
		b.WriteString(fmt.Sprintf("- **%d streams aborted (%.1f%% of requests)**: ", s.StreamingAborted, float64(s.StreamingAborted)/float64(s.TotalRequests)*100))
		b.WriteString("May indicate timeout settings, upstream connection drops, or a mismatch between the configured done marker and what the server sends.\n")
		issues++
	}
	has429 := s.StatusCodes[429] > 0
	if has429 {
		rate := float64(s.StatusCodes[429]) / float64(s.TotalRequests) * 100
		b.WriteString(fmt.Sprintf("- **Rate limited (429)**: %d requests (%.1f%%) hit rate limits. ", s.StatusCodes[429], rate))
		b.WriteString("Consider: client-side throttling (`--qps`), exponential backoff, or a higher API tier. If the rate is very low (< 0.5%%), this may be acceptable.\n")
		issues++
	}
	if s.CacheHitRate == 0 && s.Provider != "" {
		b.WriteString("- **No cache hits detected**: This may be normal if you don't have a caching layer. If your prompts have repetition, adding semantic caching could reduce latency and cost.\n")
		issues++
	}

	if issues == 0 {
		b.WriteString("No significant patterns detected. The service performed within the observed range.\n")
	}
	b.WriteString("\n")
}

func buildRetestCommand(cfg config.Config) string {
	var b strings.Builder
	b.WriteString("./vibeready")
	b.WriteString(" --url '")
	b.WriteString(shellQuote(cfg.URL))
	b.WriteString("'")
	if cfg.Method != "" && cfg.Method != "GET" {
		b.WriteString(" --method ")
		b.WriteString(cfg.Method)
	}
	if len(cfg.Headers) > 0 {
		parts := make([]string, 0, len(cfg.Headers))
		for k, v := range cfg.Headers {
			parts = append(parts, k+":"+shellQuote(v))
		}
		b.WriteString(" --headers '")
		b.WriteString(strings.Join(parts, ","))
		b.WriteString("'")
	}
	if len(cfg.Body) > 0 {
		body := string(cfg.Body)
		if len(body) > 4000 {
			body = body[:3997] + "..."
		}
		b.WriteString(" --body '")
		b.WriteString(strings.ReplaceAll(body, "'", "'\\''"))
		b.WriteString("'")
	} else if cfg.BodyFile != "" {
		b.WriteString(" --body-file '")
		b.WriteString(shellQuote(cfg.BodyFile))
		b.WriteString("'")
	} else if cfg.PayloadDir != "" {
		b.WriteString(" --payload-dir '")
		b.WriteString(shellQuote(cfg.PayloadDir))
		b.WriteString("'")
	}
	if cfg.Protocol != "" && cfg.Protocol != "http" {
		b.WriteString(" --protocol ")
		b.WriteString(cfg.Protocol)
	}
	if cfg.Protocol == "grpc" || cfg.Protocol == "grpc-stream" {
		b.WriteString(" --proto-service ")
		b.WriteString(cfg.GRPCService)
		b.WriteString(" --proto-method ")
		b.WriteString(cfg.GRPCMethod)
		if cfg.GRPCTLS {
			b.WriteString(" --grpc-tls")
		}
		if cfg.GRPCTokenField != "" {
			b.WriteString(" --grpc-token-field ")
			b.WriteString(cfg.GRPCTokenField)
		}
	}
	if cfg.Protocol == "websocket" && cfg.WSSubprotocol != "" {
		b.WriteString(" --ws-subprotocol ")
		b.WriteString(cfg.WSSubprotocol)
	}
	if cfg.Streaming {
		b.WriteString(" --stream")
		if cfg.StreamFormat != "" && cfg.StreamFormat != "auto" {
			b.WriteString(" --stream-format ")
			b.WriteString(cfg.StreamFormat)
		}
		if cfg.StreamDoneMarker != "" && cfg.StreamDoneMarker != "[DONE]" {
			b.WriteString(" --stream-done-marker '")
			b.WriteString(shellQuote(cfg.StreamDoneMarker))
			b.WriteString("'")
		}
		if len(cfg.StreamTextKeys) > 0 && !streamKeysEqual(cfg.StreamTextKeys, []string{"content", "text", "token", "output_text", "delta"}) {
			b.WriteString(" --stream-text-keys '")
			b.WriteString(strings.Join(cfg.StreamTextKeys, ","))
			b.WriteString("'")
		}
		if len(cfg.StreamTokenKeys) > 0 && !streamKeysEqual(cfg.StreamTokenKeys, []string{"output_tokens", "completion_tokens", "generated_tokens"}) {
			b.WriteString(" --stream-token-keys '")
			b.WriteString(strings.Join(cfg.StreamTokenKeys, ","))
			b.WriteString("'")
		}
	}
	if cfg.ModelPricePer1K > 0 {
		b.WriteString(fmt.Sprintf(" --model-price %.4f", cfg.ModelPricePer1K))
	}
	b.WriteString(fmt.Sprintf(" --concurrency %d", cfg.Concurrency))
	if cfg.Duration > 0 {
		b.WriteString(" --duration ")
		b.WriteString(cfg.Duration.String())
	}
	if cfg.Requests > 0 {
		b.WriteString(fmt.Sprintf(" --requests %d", cfg.Requests))
	}
	b.WriteString(" --timeout ")
	b.WriteString(cfg.Timeout.String())
	if cfg.Output != "" {
		b.WriteString(" --output '")
		b.WriteString(shellQuote(cfg.Output))
		b.WriteString("'")
	}
	if cfg.AgentContext != "" {
		b.WriteString(" --agent-context '")
		b.WriteString(shellQuote(cfg.AgentContext))
		b.WriteString("'")
	}
	if cfg.MetricsPort > 0 {
		b.WriteString(fmt.Sprintf(" --metrics-port %d", cfg.MetricsPort))
	}
	if cfg.QPS > 0 {
		b.WriteString(fmt.Sprintf(" --qps %.1f", cfg.QPS))
	}
	if cfg.RampUp > 0 {
		b.WriteString(" --ramp-up ")
		b.WriteString(cfg.RampUp.String())
	}
	return b.String()
}

func shellQuote(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}

func streamKeysEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}
