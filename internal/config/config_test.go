package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	valid := Config{
		URL:         "http://localhost:8080/infer",
		Method:      "POST",
		Concurrency: 10,
		Duration:    2 * time.Second,
		Timeout:     5 * time.Second,
	}

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{name: "valid", cfg: valid},
		{name: "requests only valid", cfg: Config{URL: valid.URL, Method: "POST", Concurrency: 2, Requests: 10, Timeout: time.Second}},
		{name: "missing url", cfg: Config{Method: "GET", Concurrency: 1, Duration: time.Second, Timeout: time.Second}, wantErr: "url is required"},
		{name: "invalid url", cfg: Config{URL: "://bad", Method: "GET", Concurrency: 1, Duration: time.Second, Timeout: time.Second}, wantErr: "invalid url"},
		{name: "invalid method", cfg: Config{URL: valid.URL, Method: "FETCH", Concurrency: 1, Duration: time.Second, Timeout: time.Second}, wantErr: "invalid method"},
		{name: "bad concurrency", cfg: Config{URL: valid.URL, Method: "GET", Concurrency: 0, Duration: time.Second, Timeout: time.Second}, wantErr: "concurrency must be greater than 0"},
		{name: "missing duration and requests", cfg: Config{URL: valid.URL, Method: "GET", Concurrency: 1, Timeout: time.Second}, wantErr: "either duration or requests must be greater than 0"},
		{name: "bad timeout", cfg: Config{URL: valid.URL, Method: "GET", Concurrency: 1, Duration: time.Second, Timeout: 0}, wantErr: "timeout must be greater than 0"},
		{name: "negative requests", cfg: Config{URL: valid.URL, Method: "GET", Concurrency: 1, Requests: -1, Timeout: time.Second}, wantErr: "requests must be greater than or equal to 0"},
		{name: "negative qps", cfg: Config{URL: valid.URL, Method: "GET", Concurrency: 1, Duration: time.Second, Timeout: time.Second, QPS: -1}, wantErr: "qps must be greater than or equal to 0"},
		{name: "bad ramp-up", cfg: Config{URL: valid.URL, Method: "GET", Concurrency: 1, Duration: time.Second, Timeout: time.Second, RampUp: 2 * time.Second}, wantErr: "ramp-up must not exceed duration"},
		{name: "bad metrics port", cfg: Config{URL: valid.URL, Method: "GET", Concurrency: 1, Duration: time.Second, Timeout: time.Second, MetricsPort: 70000}, wantErr: "metrics-port must be between 0 and 65535"},
		{name: "bad stream format", cfg: Config{URL: valid.URL, Method: "GET", Concurrency: 1, Duration: time.Second, Timeout: time.Second, StreamFormat: "weird"}, wantErr: "stream-format must be one of auto, sse, jsonl, raw"},
		{name: "empty payload dir", cfg: Config{URL: valid.URL, Method: "GET", Concurrency: 1, Requests: 1, Timeout: time.Second, PayloadDir: "/tmp/none"}, wantErr: "payload-dir did not contain any readable payload files"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if got := err.Error(); !contains(got, tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, got)
				}
			}
		})
	}
}

func TestConfigGRPCURL(t *testing.T) {
	t.Parallel()

	// gRPC allows host:port format.
	cfg := Config{
		URL:         "localhost:50051",
		Protocol:    "grpc",
		GRPCService: "test.Svc",
		GRPCMethod:  "Call",
		Concurrency: 1,
		Duration:    time.Second,
		Timeout:     time.Second,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("gRPC host:port should be valid, got: %v", err)
	}

	// gRPC-stream also allows host:port.
	cfg.Protocol = "grpc-stream"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("gRPC-stream host:port should be valid, got: %v", err)
	}

	// HTTP still requires scheme.
	cfg.Protocol = "http"
	cfg.URL = "localhost:8080"
	if err := cfg.Validate(); err == nil {
		t.Fatal("HTTP without scheme should fail")
	}
}

func TestLoadFileBodyFile(t *testing.T) {
	dir := t.TempDir()
	bodyPath := dir + "/body.json"
	if err := osWrite(bodyPath, `{"prompt":"hello"}`); err != nil {
		t.Fatal(err)
	}

	configPath := dir + "/config.json"
	data := `{"url":"http://localhost:8080","method":"POST","body_file":"` + bodyPath + `","concurrency":1,"duration":"1s"}`
	if err := osWrite(configPath, data); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if string(cfg.Body) != `{"prompt":"hello"}` {
		t.Fatalf("expected body from body_file, got %q", cfg.Body)
	}
}

func TestLoadFilePayloadDir(t *testing.T) {
	dir := t.TempDir()
	payloadDir := dir + "/payloads"
	if err := osMkdir(payloadDir); err != nil {
		t.Fatal(err)
	}
	osWrite(payloadDir+"/a.json", `{"a":1}`)
	osWrite(payloadDir+"/b.json", `{"b":2}`)

	configPath := dir + "/config.json"
	data := `{"url":"http://localhost:8080","method":"POST","payload_dir":"` + payloadDir + `","concurrency":1,"requests":1}`
	if err := osWrite(configPath, data); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(cfg.PayloadFiles) != 2 {
		t.Fatalf("expected 2 payload files, got %d", len(cfg.PayloadFiles))
	}
}

func TestLoadFileNewFields(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config.json"
	data := `{
		"url":"ws://localhost:8080",
		"protocol":"websocket",
		"ws_subprotocol":"graphql-ws",
		"model_price_per_1k": 0.002,
		"agent_context":"report.md",
		"concurrency":1,
		"duration":"1s"
	}`
	if err := osWrite(configPath, data); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if cfg.WSSubprotocol != "graphql-ws" {
		t.Fatalf("expected ws_subprotocol, got %q", cfg.WSSubprotocol)
	}
	if cfg.ModelPricePer1K != 0.002 {
		t.Fatalf("expected model_price_per_1k, got %f", cfg.ModelPricePer1K)
	}
	if cfg.AgentContext != "report.md" {
		t.Fatalf("expected agent_context, got %q", cfg.AgentContext)
	}
}

func TestMergePayloadSourceOverridesAreExclusive(t *testing.T) {
	t.Parallel()

	base := Config{
		URL:          "http://localhost:8080",
		Method:       "POST",
		Body:         []byte(`{"from":"file"}`),
		BodyText:     `{"from":"file"}`,
		BodyFile:     "body.json",
		PayloadDir:   "payloads",
		PayloadFiles: []string{"payloads/a.json"},
		PayloadCount: 1,
		Concurrency:  1,
		Requests:     1,
		Timeout:      time.Second,
	}

	bodyOverlay := Config{
		Body:     []byte(`{"from":"cli-body"}`),
		BodyText: `{"from":"cli-body"}`,
	}
	bodyMerged := Merge(base, bodyOverlay, map[string]bool{"body": true})
	if bodyMerged.BodyText != `{"from":"cli-body"}` {
		t.Fatalf("expected CLI body to win, got %q", bodyMerged.BodyText)
	}
	if bodyMerged.BodyFile != "" || bodyMerged.PayloadDir != "" || len(bodyMerged.PayloadFiles) != 0 || bodyMerged.PayloadCount != 0 {
		t.Fatalf("expected CLI body to clear other payload sources, got %#v", bodyMerged)
	}

	bodyFileOverlay := Config{
		Body:     []byte(`{"from":"cli-file"}`),
		BodyText: `{"from":"cli-file"}`,
		BodyFile: "cli-body.json",
	}
	bodyFileMerged := Merge(base, bodyFileOverlay, map[string]bool{"body-file": true})
	if bodyFileMerged.BodyFile != "cli-body.json" || bodyFileMerged.PayloadDir != "" || len(bodyFileMerged.PayloadFiles) != 0 {
		t.Fatalf("expected CLI body-file to clear payload dir, got %#v", bodyFileMerged)
	}

	payloadOverlay := Config{
		PayloadDir:   "cli-payloads",
		PayloadFiles: []string{"cli-payloads/a.json", "cli-payloads/b.json"},
		PayloadCount: 2,
	}
	payloadMerged := Merge(base, payloadOverlay, map[string]bool{"payload-dir": true})
	if payloadMerged.PayloadDir != "cli-payloads" || payloadMerged.PayloadCount != 2 {
		t.Fatalf("expected CLI payload-dir to win, got %#v", payloadMerged)
	}
	if len(payloadMerged.Body) != 0 || payloadMerged.BodyText != "" || payloadMerged.BodyFile != "" {
		t.Fatalf("expected CLI payload-dir to clear body sources, got %#v", payloadMerged)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func osWrite(path, data string) error {
	return os.WriteFile(path, []byte(data), 0o644)
}

func osMkdir(path string) error {
	return os.MkdirAll(path, 0o755)
}
