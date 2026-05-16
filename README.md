# perf-loadgen

A lightweight, model-aware load testing tool for AI API applications.
One command tells you whether your AI app is ready for real users.

## Who this is for

You built an AI app — a translation service, a chatbot, a document summarizer —
using Cursor, Codex, or Claude Code. It calls OpenAI, DeepSeek, or any LLM API.
You're about to share the link with friends or a small team. You need to know:
will it hold up?

## Why this exists

General-purpose load testers (wrk, k6, vegeta) measure HTTP performance.
They tell you p95 latency and error rate. That is useful, but for AI apps it is
not enough.

AI app users do not care about HTTP response time. They care about:

| Metric | What it means for your users |
|--------|------------------------------|
| **TTFT** (time to first token) | How long before they see the first word. Determines whether streaming feels instant. |
| **ITL** (inter-token latency) | Whether tokens arrive smoothly or in bursts. Determines the "typewriter effect" quality. |
| **tok/s** | Model generation speed. Low → users wait. |
| **Streaming aborts** | Streams that break before completion. Users see blank or partial answers. |
| **429 / rate limit** | Hitting the model API limit. Your app returns errors even if your code is fine. |
| **Upstream vs backend** | Is the bottleneck your code, your VPS, or the model API? Without this split, you optimize the wrong thing. |

perf-loadgen measures all of these with zero configuration. If your backend adds
a few response headers (the `x-ai-*` convention), it also tells you model
provider, cache hit rate, input/output token counts, and estimated cost.

## Quick Start

```bash
# Install (Go >= 1.25)
git clone https://github.com/JinkaiLiu/perf-loadgen.git
cd perf-loadgen && go build -o loadgen ./cmd/loadgen

# Run against your AI endpoint
./loadgen \
  --url https://your-app.com/api/translate \
  --method POST \
  --headers "Content-Type:application/json" \
  --body '{"text":"Explain quantum computing in simple terms","target":"zh"}' \
  --concurrency 10 \
  --duration 30s \
  --timeout 45s \
  --output result.json \
  --agent-context agent-report.md
```

That is it. One command, two output files. `result.json` has the raw data.
`agent-report.md` is a structured report you can paste directly into your
coding agent for follow-up fixes.

You can also try it locally with the bundled mock server:

```bash
go run ./cmd/mockserver --port 8080 &
./loadgen --url http://127.0.0.1:8080/infer --method POST \
  --body '{"prompt":"hello"}' --concurrency 5 --duration 10s
```

## What the output looks like

**Console** (always printed):

```text
Total:          285
Success:        278
Failed:         7
Error Rate:     2.46%
QPS:            9.50
Avg:            1.05s     P50: 1.02s     P95: 2.10s     P99: 3.80s
Avg TTFT:       320ms     TTFT P50: 280ms    TTFT P95: 650ms
Upstream:       780ms     Overhead: 270ms    Upstream %: 74.3%
Provider:       deepseek     Model: deepseek-chat
Cache Hit %:    12.0%
429 Count:      3 (1.1%)
```

**Agent report** (`--agent-context agent-report.md`):

A structured Markdown report with test configuration, latency percentiles,
error breakdown, upstream analysis, and **suggested actions** — for example:

> **Bottleneck: Upstream Model API** — 74% of total latency is model API time.
> Optimizing your backend code will have limited impact. Consider prompt
> compression, a smaller model, or a provider with lower latency.

> **Rate Limiting Detected (429)** — 3 requests returned 429. Add request
> throttling with `--qps`, exponential backoff retry, or upgrade your API tier.

## Features

### Core (always available)

- HTTP and HTTP streaming (SSE / JSONL / raw)
- TTFT, ITL, token throughput, streaming abort rate
- Latency percentiles (P50/P90/P95/P99) with human-readable output
- Status code and error category breakdown (timeout / network / HTTP / 429)
- Payload rotation from directory (`--payload-dir`)
- QPS limiting and ramp-up (`--qps`, `--ramp-up`)
- JSON report (`--output`) and agent-friendly Markdown report (`--agent-context`)

### Semi-white-box AI metrics

If your backend returns these response headers, perf-loadgen computes upstream
latency, backend overhead, cache hit rate, and per-request token counts:

```
x-ai-provider: deepseek
x-ai-model: deepseek-chat
x-ai-upstream-latency-ms: 780
x-ai-first-token-ms: 280
x-ai-input-tokens: 50
x-ai-output-tokens: 120
x-ai-cache-hit: false
```

See [docs/semi-white-box.md](docs/semi-white-box.md) for integration instructions.

### Advanced capabilities

These exist and work, but are not required for the basic workflow:

| Area | Support |
|------|---------|
| gRPC (unary + server-streaming) | Via server reflection. `--protocol grpc` |
| WebSocket (`ws://` / `wss://`) | RFC 6455, TLS. `--protocol websocket` |
| Prometheus `/metrics` | `--metrics-port 9090` (binds 127.0.0.1) |
| Real-time SSE dashboard | Distributed mode only. |
| Docker | `docker build -t perf-loadgen .` |
| Distributed master/worker | For throughput beyond a single machine |

See [docs/advanced/](docs/advanced/) for detailed guides.

## Why not just use k6?

You can ask a coding agent to generate a k6 script, and that works for general
HTTP load testing. But:

- Agent-generated k6 scripts measure HTTP latency, not TTFT or token speed.
- They do not tell you whether the bottleneck is your code or the model API.
- Each generated script is different — no standardized re-test workflow.

perf-loadgen is not a k6 replacement. It focuses on a narrower problem:
standardized AI app readiness checks with model-aware metrics and agent-friendly
reports that stay comparable across runs.

## Documentation

- [CLI reference](docs/cli-reference.md) — every flag
- [Protocols](docs/protocols.md) — HTTP streaming, gRPC, WebSocket
- [Semi-white-box metrics](docs/semi-white-box.md) — `x-ai-*` header convention
- [Distributed mode](docs/advanced/distributed.md)
- [Docker & Kubernetes](docs/advanced/deployment.md)
- [Architecture](docs/architecture.md)
- [中文文档](README_zh.md)

## Roadmap

- [ ] Tiktoken-based tokenizer (currently heuristic word-count)
- [ ] Persistent job queue with disk/RDB backend
- [ ] k6 script export (`perf-loadgen export k6`)
- [ ] OpenTelemetry trace export

## License

MIT — see [LICENSE](LICENSE).
