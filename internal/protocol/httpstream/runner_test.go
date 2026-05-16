package httpstream

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
)

func TestRunnerParsesSSEStream(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		flusher.Flush()
		time.Sleep(20 * time.Millisecond)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"world again\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	runner := NewRunner(Options{Timeout: 2 * time.Second, StreamFormat: "sse"})
	result, err := runner.Run(context.Background(), types.RequestSpec{
		URL:    server.URL,
		Method: http.MethodGet,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %#v", result)
	}
	if result.TTFT <= 0 {
		t.Fatalf("expected positive TTFT, got %s", result.TTFT)
	}
	if result.GenerationTime <= 0 {
		t.Fatalf("expected positive generation time, got %s", result.GenerationTime)
	}
	if result.OutputTokens != 3 {
		t.Fatalf("expected 3 output tokens, got %d", result.OutputTokens)
	}
	if result.TokensPerSecond <= 0 {
		t.Fatalf("expected positive token rate, got %f", result.TokensPerSecond)
	}
	if result.StreamingAborted {
		t.Fatal("expected completed SSE stream")
	}
}

func TestRunnerMarksAbortedSSEWithoutDone(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
	}))
	defer server.Close()

	runner := NewRunner(Options{Timeout: time.Second, StreamFormat: "auto"})
	result, err := runner.Run(context.Background(), types.RequestSpec{
		URL:    server.URL,
		Method: http.MethodGet,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.StreamingAborted {
		t.Fatal("expected aborted stream when [DONE] is missing")
	}
}

func TestRunnerParsesJSONLStream(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"text":"alpha beta"}`)
		fmt.Fprintln(w, `{"text":"gamma"}`)
	}))
	defer server.Close()

	runner := NewRunner(Options{Timeout: time.Second, StreamFormat: "auto"})
	result, err := runner.Run(context.Background(), types.RequestSpec{
		URL:    server.URL,
		Method: http.MethodGet,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.OutputTokens != 3 {
		t.Fatalf("expected 3 tokens, got %d", result.OutputTokens)
	}
}

func TestRunnerUsesCustomDoneMarkerAndTokenKey(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"message\":\"alpha beta\",\"usage_tokens\":2}\n\n")
		fmt.Fprint(w, "data: {\"finish_reason\":\"stop\"}\n\n")
		fmt.Fprint(w, "data: <END>\n\n")
	}))
	defer server.Close()

	runner := NewRunner(Options{
		Timeout:      time.Second,
		StreamFormat: "sse",
		DoneMarker:   "<END>",
		TextKeys:     []string{"message"},
		TokenKeys:    []string{"usage_tokens"},
	})
	result, err := runner.Run(context.Background(), types.RequestSpec{
		URL:    server.URL,
		Method: http.MethodGet,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.OutputTokens != 2 {
		t.Fatalf("expected token count from custom usage field, got %d", result.OutputTokens)
	}
	if result.StreamingAborted {
		t.Fatal("expected custom done marker to terminate stream cleanly")
	}
}
