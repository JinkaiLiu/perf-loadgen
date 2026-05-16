package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/JinkaiLiu/vibeready/internal/cli"
	"github.com/JinkaiLiu/vibeready/internal/engine"
	"github.com/JinkaiLiu/vibeready/internal/protocol"
)

func startMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/infer", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Loadgen-TTFT-Ms", "50.0")
		w.Header().Set("X-Loadgen-Generation-Ms", "200.0")
		w.Header().Set("X-Loadgen-Output-Tokens", "15")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"hello world","output_tokens":15}`))
	})
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 5; i++ {
			_, _ = w.Write([]byte("data: {\"content\":\"token\",\"output_tokens\":3}\n\n"))
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	})
	return httptest.NewServer(mux)
}

func TestIntegrationHTTPUnary(t *testing.T) {
	t.Parallel()
	server := startMockServer(t)
	defer server.Close()

	cfg, err := cli.Parse([]string{
		"--url", server.URL + "/infer",
		"--method", "POST",
		"--body", `{"prompt":"hello"}`,
		"--concurrency", "2",
		"--requests", "10",
		"--timeout", "2s",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	runner, err := protocol.BuildRunner(cfg); if err != nil { t.Fatalf("BuildRunner returned error: %v", err) }
	summary, err := engine.New(runner).Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if summary.TotalRequests != 10 {
		t.Fatalf("expected 10 total requests, got %d", summary.TotalRequests)
	}
	if summary.SuccessfulRequests != 10 {
		t.Fatalf("expected 10 successful, got %d", summary.SuccessfulRequests)
	}
	if summary.MinLatency == 0 || summary.MaxLatency == 0 {
		t.Fatal("expected non-zero latencies")
	}
	if summary.TotalOutputTokens == 0 {
		t.Fatal("expected non-zero output tokens from inference headers")
	}
	if summary.AvgTTFT == 0 {
		t.Fatal("expected non-zero avg TTFT from inference headers")
	}
}

func TestIntegrationHTTPStreaming(t *testing.T) {
	t.Parallel()
	server := startMockServer(t)
	defer server.Close()

	cfg, err := cli.Parse([]string{
		"--url", server.URL + "/stream",
		"--method", "POST",
		"--body", `{"prompt":"hello"}`,
		"--stream",
		"--stream-format", "sse",
		"--stream-text-keys", "content",
		"--stream-token-keys", "output_tokens",
		"--concurrency", "2",
		"--requests", "10",
		"--timeout", "3s",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	runner, err := protocol.BuildRunner(cfg); if err != nil { t.Fatalf("BuildRunner returned error: %v", err) }
	summary, err := engine.New(runner).Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if summary.TotalRequests != 10 {
		t.Fatalf("expected 10 total, got %d", summary.TotalRequests)
	}
	if summary.TotalOutputTokens == 0 {
		t.Fatal("expected non-zero output tokens from streaming")
	}
}

func TestIntegrationConfigFile(t *testing.T) {
	t.Parallel()
	server := startMockServer(t)
	defer server.Close()

	dir := t.TempDir()
	configPath := dir + "/config.json"
	configData := `{"url":"` + server.URL + `/infer","method":"POST","body":"{\"prompt\":\"from-file\"}","concurrency":4,"duration":"100ms","timeout":"2s"}`
	if err := os.WriteFile(configPath, []byte(configData), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := cli.Parse([]string{
		"--config", configPath,
		"--concurrency", "2",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.Concurrency != 2 {
		t.Fatalf("expected CLI to override concurrency to 2, got %d", cfg.Concurrency)
	}
	if cfg.URL != server.URL+"/infer" {
		t.Fatalf("expected URL from config file, got %s", cfg.URL)
	}

	runner, err := protocol.BuildRunner(cfg); if err != nil { t.Fatalf("BuildRunner returned error: %v", err) }
	summary, err := engine.New(runner).Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if summary.TotalRequests == 0 {
		t.Fatal("expected requests from config-file-driven run")
	}
}
