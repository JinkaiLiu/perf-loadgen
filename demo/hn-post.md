# Show HN: VibeReady — one command to check if your AI app is ready for real users

**TL;DR:** You built an AI app with Cursor. It works fine for you. But will it hold up when 10 friends use it at once? `vibeready` answers that with one command.

## The problem

General-purpose load testers (k6, wrk, vegeta) tell you p95 latency and error rate. That's useful for standard HTTP services but not enough for AI apps. Your users don't care about HTTP response time — they care about:

- **Time to first token (TTFT):** Does streaming feel instant or is there a pause before the first word?
- **Inter-token latency (ITL):** Do tokens arrive smoothly or in bursts?
- **tok/s:** Is the model generating fast enough?
- **Where is the bottleneck:** Is it your backend code, your VPS, or the model API?
- **429 rate limits:** Are you hitting the model API rate limit?

None of these show up in traditional tools.

## What vibeready does

One command:

```bash
./vibeready \
  --url http://localhost:3000/api/chat \
  --method POST \
  --headers "Content-Type:application/json" \
  --body '{"message":"Explain quantum computing"}' \
  --concurrency 5 \
  --duration 30s \
  --timeout 45s \
  --output result.json \
  --agent-context agent-report.md
```

You get:

```
Total: 1423  Success: 1356  Failed: 67  Error Rate: 4.71%  QPS: 47.43
Avg: 1.05s  P50: 1.02s  P95: 2.10s  P99: 3.80s
Avg TTFT: 320ms  TTFT P50: 280ms  TTFT P95: 650ms
Upstream: 780ms  Overhead: 270ms  Upstream %: 74.3%
Provider: openai  Model: gpt-4o  429: 12 (0.8%)
```

But the real killer feature is the agent report. `vibeready` generates `agent-report.md` — a Markdown file with diagnostic interpretation, not just raw numbers. Paste it into Cursor, Claude Code, or any coding agent and it'll suggest *concrete* fixes:

- *"74% of latency is upstream. Optimizing your backend won't help — consider a faster model or prompt caching."*
- *"12 requests hit 429. Add exponential backoff or upgrade your API tier."*

**The loop:** vibeready detects → agent fixes → vibeready re-tests. You just ship the link.

## Semi-white-box metrics

If your backend adds a few `x-ai-*` response headers, vibeready gives you even more:

```
x-ai-provider            → openai / deepseek / ...
x-ai-model               → gpt-4o / deepseek-chat / ...
x-ai-upstream-latency-ms → backend vs. model API split
x-ai-first-token-ms      → TTFT measured on the model side
x-ai-input-tokens        → prompt token count
x-ai-output-tokens       → completion token count
x-ai-cache-hit           → was this served from cache?
```

With these, vibeready computes backend overhead, upstream ratio, cache hit rate, and estimated cost.

## Tech

Go, zero runtime dependencies beyond gRPC/protobuf. HTTP, HTTP streaming (SSE/JSONL/raw), gRPC (unary + server-streaming via reflection), and WebSocket (text/close/ping/pong, TLS). Single binary.

Distributed master/worker mode, Docker image, and Helm chart for throughput beyond one machine.

## Links

- GitHub: https://github.com/JinkaiLiu/vibeready
- Docs: https://github.com/JinkaiLiu/vibeready/tree/main/docs

## Limitations (honest)

- WebSocket supports text/close/ping/pong frames — not full RFC 6455 (no continuation/fragmentation/binary)
- Distributed persistent mode still early
- Works best when your backend adds the `x-ai-*` headers (otherwise you get Layer 1 metrics only, which is still useful)

Built this because I needed it myself. Would love feedback — especially on what other AI-specific metrics would be useful to measure.
