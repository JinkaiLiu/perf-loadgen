package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

func main() {
	port := flag.Int("port", 8080, "Listen port")
	latency := flag.Duration("latency", 10*time.Millisecond, "Base response latency")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/infer", handleInfer(*latency))
	mux.HandleFunc("/stream", handleStream(*latency))
	mux.HandleFunc("/health", handleHealth)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("mock inference server listening on %s (latency=%s)", addr, *latency)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func handleInfer(baseLatency time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		delay := baseLatency + time.Duration(rand.Int63n(int64(baseLatency)))
		time.Sleep(delay)

		outputTokens := int64(10 + rand.Intn(50))
		ttftMs := float64(delay.Milliseconds())
		genMs := float64(outputTokens) * 15.0

		w.Header().Set("Content-Type", "application/json")
		// Layer 2: x-ai-* headers (proposed lightweight standard for AI API observability).
		w.Header().Set("x-ai-provider", "mock")
		w.Header().Set("x-ai-model", "mock-model-v1")
		w.Header().Set("x-ai-upstream-latency-ms", strconv.FormatFloat(ttftMs*0.8, 'f', 1, 64))
		w.Header().Set("x-ai-first-token-ms", strconv.FormatFloat(ttftMs*0.6, 'f', 1, 64))
		w.Header().Set("x-ai-input-tokens", "50")
		w.Header().Set("x-ai-output-tokens", strconv.FormatInt(outputTokens, 10))
		w.Header().Set("x-ai-cache-hit", strconv.FormatBool(rand.Intn(3) == 0))
		// Layer 1: legacy X-Loadgen-* headers.
		w.Header().Set("X-Loadgen-TTFT-Ms", strconv.FormatFloat(ttftMs, 'f', 1, 64))
		w.Header().Set("X-Loadgen-Generation-Ms", strconv.FormatFloat(genMs, 'f', 1, 64))
		w.Header().Set("X-Loadgen-Output-Tokens", strconv.FormatInt(outputTokens, 10))
		w.Header().Set("X-Loadgen-Tokens-Per-Second", strconv.FormatFloat(float64(outputTokens)/(genMs/1000), 'f', 1, 64))
		w.WriteHeader(http.StatusOK)

		payload := fmt.Sprintf(`{"text":"Generated response with %d tokens","output_tokens":%d}`, outputTokens, outputTokens)
		_, _ = w.Write([]byte(payload))
	}
}

func handleStream(baseLatency time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		words := []string{"Hello", "world", "this", "is", "a", "streaming", "response", "for", "load", "testing"}
		for i, word := range words {
			chunkLatency := baseLatency + time.Duration(rand.Int63n(int64(baseLatency/2)))
			time.Sleep(chunkLatency)

			data := fmt.Sprintf(`{"id":"%d","content":"%s","output_tokens":%d}`, i, word, i+1)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
