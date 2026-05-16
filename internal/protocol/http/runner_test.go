package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JinkaiLiu/vibeready/pkg/types"
)

func TestRunnerSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if got := r.Header.Get("X-Test"); got != "1" {
			t.Fatalf("unexpected header %q", got)
		}
		w.Header().Set("X-Loadgen-TTFT-Ms", "15")
		w.Header().Set("X-Loadgen-Generation-Ms", "100")
		w.Header().Set("X-Loadgen-Output-Tokens", "42")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	runner := NewRunner(2 * time.Second)
	result, err := runner.Run(context.Background(), types.RequestSpec{
		URL:     server.URL,
		Method:  http.MethodPost,
		Headers: map[string]string{"X-Test": "1"},
		Body:    []byte(`{"hello":"world"}`),
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success result, got %#v", result)
	}
	if result.BytesRead == 0 {
		t.Fatal("expected bytes read to be tracked")
	}
	if result.TTFT != 15*time.Millisecond {
		t.Fatalf("expected TTFT to be parsed, got %s", result.TTFT)
	}
	if result.OutputTokens != 42 {
		t.Fatalf("expected output tokens to be parsed, got %d", result.OutputTokens)
	}
	if result.TokensPerSecond <= 0 {
		t.Fatalf("expected tokens per second to be derived, got %f", result.TokensPerSecond)
	}
}

func TestRunnerHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	runner := NewRunner(time.Second)
	result, err := runner.Run(context.Background(), types.RequestSpec{
		URL:    server.URL,
		Method: http.MethodGet,
	})
	if err == nil {
		t.Fatal("expected error for non-success status")
	}
	if result.ErrorCategory != types.ErrorCategoryHTTPStatus {
		t.Fatalf("unexpected error category %q", result.ErrorCategory)
	}
	if result.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unexpected status code %d", result.StatusCode)
	}
}

func TestRunnerTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	runner := NewRunner(20 * time.Millisecond)
	result, err := runner.Run(context.Background(), types.RequestSpec{
		URL:    server.URL,
		Method: http.MethodGet,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if result.ErrorCategory != types.ErrorCategoryTimeout {
		t.Fatalf("unexpected error category %q", result.ErrorCategory)
	}
}

func TestRunnerNetworkFailure(t *testing.T) {
	t.Parallel()

	runner := NewRunner(100 * time.Millisecond)
	result, err := runner.Run(context.Background(), types.RequestSpec{
		URL:    "http://127.0.0.1:1",
		Method: http.MethodGet,
	})
	if err == nil {
		t.Fatal("expected network error")
	}
	if result.ErrorCategory != types.ErrorCategoryNetwork {
		t.Fatalf("unexpected error category %q", result.ErrorCategory)
	}
}
