package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JinkaiLiu/vibeready/internal/config"
	"github.com/JinkaiLiu/vibeready/pkg/types"
)

func TestWriteConsoleSummary(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	summary := types.Summary{
		TotalRequests:      10,
		SuccessfulRequests: 9,
		FailedRequests:     1,
		ErrorRate:          0.1,
		RequestsPerSecond:  100,
		AvgLatency:         10 * time.Millisecond,
		MinLatency:         5 * time.Millisecond,
		MaxLatency:         50 * time.Millisecond,
		AvgTTFT:            7 * time.Millisecond,
		TotalOutputTokens:  512,
		AvgTokensPerSecond: 123.45,
		StreamingAborted:   2,
		Percentiles:        types.Percentiles{P50: 10 * time.Millisecond, P90: 40 * time.Millisecond, P95: 45 * time.Millisecond, P99: 50 * time.Millisecond},
	}

	if err := WriteConsoleSummary(&buf, summary); err != nil {
		t.Fatalf("WriteConsoleSummary returned error: %v", err)
	}
	output := buf.String()
	for _, fragment := range []string{"Total:", "Success:", "Failed:", "P95:", "QPS:", "Avg TTFT:", "Output Tokens:", "Avg tok/s:"} {
		if !bytes.Contains([]byte(output), []byte(fragment)) {
			t.Fatalf("expected output to contain %q, got %q", fragment, output)
		}
	}
}

func TestWriteJSONReport(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "report.json")
	cfg := config.Config{
		URL:              "http://localhost",
		Method:           "POST",
		BodyText:         `{"prompt":"hello"}`,
		Streaming:        true,
		StreamFormat:     "sse",
		StreamDoneMarker: "<END>",
		StreamTextKeys:   []string{"delta"},
		StreamTokenKeys:  []string{"usage_tokens"},
		Concurrency:      10,
		Duration:         time.Second,
		Timeout:          2 * time.Second,
		Output:           path,
	}
	summary := types.Summary{TotalRequests: 1}

	if err := WriteJSONReport(path, cfg, summary); err != nil {
		t.Fatalf("WriteJSONReport returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}

	var report JSONReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if report.Config.URL != cfg.URL || report.Summary.TotalRequests != 1 {
		t.Fatalf("unexpected report contents: %#v", report)
	}
	if !report.Config.Streaming || report.Config.StreamDoneMarker != "<END>" {
		t.Fatalf("unexpected streaming config contents: %#v", report.Config)
	}
}
