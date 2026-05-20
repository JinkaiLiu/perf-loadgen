package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestBuildRetestCommandBasic(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		URL:         "http://localhost:8080/api/chat",
		Method:      "POST",
		Body:        []byte(`{"message":"hello"}`),
		Concurrency: 5,
		Duration:    30 * time.Second,
		Timeout:     45 * time.Second,
	}

	cmd := buildRetestCommand(cfg)
	for _, want := range []string{
		"./vibeready",
		"--url 'http://localhost:8080/api/chat'",
		"--method POST",
		"--body '{\"message\":\"hello\"}'",
		"--concurrency 5",
		"--duration 30s",
		"--timeout 45s",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("expected command to contain %q\ngot: %s", want, cmd)
		}
	}
}

func TestBuildRetestCommandHeadersShellQuote(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		URL:    "http://localhost/api",
		Method: "POST",
		Headers: map[string]string{
			"Authorization": "Bearer test'token",
			"Content-Type":  "application/json",
		},
		Concurrency: 1,
		Timeout:     30 * time.Second,
	}

	cmd := buildRetestCommand(cfg)
	if !strings.Contains(cmd, `--headers 'Authorization:Bearer test'token,Content-Type:application/json'`) {
		t.Errorf("unexpected headers quoting\ngot: %s", cmd)
	}
}

func TestBuildRetestCommandGRPC(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		URL:            "localhost:9090",
		Method:         "POST",
		Protocol:       "grpc",
		GRPCService:    "inference.v1.InferenceService",
		GRPCMethod:     "Generate",
		GRPCTLS:        true,
		GRPCTokenField: "total_tokens",
		Body:           []byte(`{"prompt":"hello"}`),
		Concurrency:    3,
		Requests:       100,
		Timeout:        60 * time.Second,
	}

	cmd := buildRetestCommand(cfg)
	for _, want := range []string{
		"--protocol grpc",
		"--proto-service inference.v1.InferenceService",
		"--proto-method Generate",
		"--grpc-tls",
		"--grpc-token-field total_tokens",
		"--requests 100",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("expected %q in gRPC command\ngot: %s", want, cmd)
		}
	}
}

func TestBuildRetestCommandWebSocket(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		URL:           "ws://localhost:8080/stream",
		Protocol:      "websocket",
		WSSubprotocol: "chat",
		Body:          []byte(`{"msg":"hi"}`),
		Concurrency:   2,
		Duration:      15 * time.Second,
		Timeout:       30 * time.Second,
	}

	cmd := buildRetestCommand(cfg)
	for _, want := range []string{
		"--protocol websocket",
		"--ws-subprotocol chat",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("expected %q in WebSocket command\ngot: %s", want, cmd)
		}
	}
}

func TestBuildRetestCommandStreamingFull(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		URL:              "http://localhost/stream",
		Method:           "POST",
		Body:             []byte(`{"prompt":"hi"}`),
		Streaming:        true,
		StreamFormat:     "sse",
		StreamDoneMarker: "<END>",
		StreamTextKeys:   []string{"delta", "output"},
		StreamTokenKeys:  []string{"usage"},
		Concurrency:      5,
		Duration:         10 * time.Second,
		Timeout:          30 * time.Second,
	}

	cmd := buildRetestCommand(cfg)
	for _, want := range []string{
		"--stream",
		"--stream-format sse",
		"--stream-done-marker '<END>'",
		"--stream-text-keys 'delta,output'",
		"--stream-token-keys 'usage'",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("expected %q in streaming command\ngot: %s", want, cmd)
		}
	}
	// Default stream keys should NOT appear.
	if strings.Contains(cmd, "content,text,token,output_text,delta") {
		t.Error("default stream-text-keys should not appear")
	}
}

func TestBuildRetestCommandBodyFileAndPayloadDir(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		URL:         "http://localhost/api",
		Method:      "POST",
		BodyFile:    "/path/to/body.json",
		Concurrency: 1,
		Timeout:     30 * time.Second,
	}

	cmd := buildRetestCommand(cfg)
	if !strings.Contains(cmd, "--body-file '/path/to/body.json'") {
		t.Errorf("expected body-file\ngot: %s", cmd)
	}

	cfg2 := config.Config{
		URL:         "http://localhost/api",
		Method:      "POST",
		PayloadDir:  "/path/to/payloads",
		Concurrency: 1,
		Timeout:     30 * time.Second,
	}
	cmd2 := buildRetestCommand(cfg2)
	if !strings.Contains(cmd2, "--payload-dir '/path/to/payloads'") {
		t.Errorf("expected payload-dir\ngot: %s", cmd2)
	}
}

func TestBuildRetestCommandLongBodyTruncation(t *testing.T) {
	t.Parallel()

	longBody := `{"message":"` + strings.Repeat("x", 4100) + `"}`
	cfg := config.Config{
		URL:         "http://localhost/api",
		Method:      "POST",
		Body:        []byte(longBody),
		Concurrency: 1,
		Timeout:     30 * time.Second,
	}

	cmd := buildRetestCommand(cfg)
	if !strings.Contains(cmd, "...") {
		t.Error("long body should be truncated")
	}
	if len(cmd) > 4500 {
		t.Errorf("command too long: %d chars", len(cmd))
	}
}

func TestBuildRetestCommandURLShellQuote(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		URL:         "http://localhost/api?key=val'ue",
		Method:      "GET",
		Concurrency: 1,
		Timeout:     30 * time.Second,
	}

	cmd := buildRetestCommand(cfg)
	if !strings.Contains(cmd, `--url 'http://localhost/api?key=val'\''ue'`) {
		t.Errorf("URL not shell-quoted\ngot: %s", cmd)
	}
}

func TestBuildRetestCommandAllOptionalFields(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		URL:             "http://localhost/api",
		Method:          "POST",
		Body:            []byte(`{}`),
		ModelPricePer1K: 0.002,
		Concurrency:     10,
		Duration:        30 * time.Second,
		Timeout:         45 * time.Second,
		Output:          "result.json",
		AgentContext:    "agent-report.md",
		QPS:             50,
		RampUp:          5 * time.Second,
		MetricsPort:     9090,
	}

	cmd := buildRetestCommand(cfg)
	for _, want := range []string{
		"--model-price 0.0020",
		"--output 'result.json'",
		"--agent-context 'agent-report.md'",
		"--qps 50.0",
		"--ramp-up 5s",
		"--metrics-port 9090",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("expected %q\ngot: %s", want, cmd)
		}
	}
}
