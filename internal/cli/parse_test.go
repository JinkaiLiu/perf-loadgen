package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseHeaders(t *testing.T) {
	t.Parallel()

	headers, err := parseHeaders("Content-Type: application/json, Authorization: Bearer token")
	if err != nil {
		t.Fatalf("parseHeaders returned error: %v", err)
	}
	if headers["Content-Type"] != "application/json" {
		t.Fatalf("unexpected content-type: %q", headers["Content-Type"])
	}
	if headers["Authorization"] != "Bearer token" {
		t.Fatalf("unexpected authorization: %q", headers["Authorization"])
	}
}

func TestParseRejectsBadHeader(t *testing.T) {
	t.Parallel()

	if _, err := parseHeaders("broken-header"); err == nil {
		t.Fatal("expected error for malformed header")
	}
}

func TestLoadBodyFromFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "payload.json")
	const expected = `{"prompt":"hello"}`
	if err := os.WriteFile(path, []byte(expected), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	body, text, payloads, err := loadPayloadSource("", path, "")
	if err != nil {
		t.Fatalf("loadPayloadSource returned error: %v", err)
	}
	if len(payloads) != 0 {
		t.Fatalf("expected no rotated payloads, got %d", len(payloads))
	}
	if string(body) != expected || text != expected {
		t.Fatalf("unexpected body payload: %q %q", string(body), text)
	}
}

func TestParseRejectsConflictingBodySources(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{
		"--url", "http://localhost:8080",
		"--method", "POST",
		"--body", "{}",
		"--body-file", "/tmp/payload.json",
		"--concurrency", "1",
		"--duration", "1s",
	})
	if err == nil {
		t.Fatal("expected conflict error for body and body-file")
	}
}

func TestParseStreamingFlags(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{
		"--url", "http://localhost:8080",
		"--method", "POST",
		"--body", "{}",
		"--stream",
		"--stream-format", "sse",
		"--concurrency", "1",
		"--duration", "1s",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !cfg.Streaming || cfg.StreamFormat != "sse" {
		t.Fatalf("unexpected streaming config: %#v", cfg)
	}
}

func TestParseStreamingSemanticFlags(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{
		"--url", "http://localhost:8080",
		"--method", "POST",
		"--body", "{}",
		"--stream",
		"--stream-done-marker", "<END>",
		"--stream-text-keys", "delta,message",
		"--stream-token-keys", "usage_tokens,completion_tokens",
		"--concurrency", "1",
		"--duration", "1s",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.StreamDoneMarker != "<END>" {
		t.Fatalf("unexpected done marker: %#v", cfg)
	}
	if len(cfg.StreamTextKeys) != 2 || cfg.StreamTextKeys[0] != "delta" || cfg.StreamTokenKeys[0] != "usage_tokens" {
		t.Fatalf("unexpected streaming key config: %#v", cfg)
	}
}

func TestParsePayloadDirAndRequests(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.json"), []byte(`{"prompt":"a"}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.json"), []byte(`{"prompt":"b"}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := Parse([]string{
		"--url", "http://localhost:8080",
		"--method", "POST",
		"--payload-dir", dir,
		"--requests", "7",
		"--concurrency", "2",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.Requests != 7 || cfg.PayloadDir != dir || len(cfg.PayloadFiles) != 2 {
		t.Fatalf("unexpected payload-dir config: %#v", cfg)
	}
}

func TestParseConfigEqualsForm(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	data := `{
		"url":"http://localhost:8080",
		"method":"POST",
		"body":"{}",
		"concurrency":1,
		"requests":1
	}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := Parse([]string{"--config=" + path, "--requests", "3"})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.URL != "http://localhost:8080" || cfg.Requests != 3 {
		t.Fatalf("unexpected config merge result: %#v", cfg)
	}
}

func TestParseConfigMissingValue(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"--config"})
	if err == nil {
		t.Fatal("expected missing config value error")
	}
}
