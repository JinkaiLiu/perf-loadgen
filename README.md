# vibeready

A lightweight, model-aware load testing tool for AI API applications.
One command tells you whether your AI app is ready for real users.

## Who this is for

You built an AI app — a translation service, a chatbot, a document summarizer, and so on —
using Cursor, Codex, or Claude Code. It calls OpenAI, DeepSeek, or any LLM API.
You're about to share the link with friends, a small team, or even launch it publicly.
Before you do, here is what you actually need to know:

- 5 friends open it at once. Does anyone see an error?
- Someone pastes a long document. Does the request time out?
- The AI response feels slow. Is it your code, your VPS, or the model API?
- The streaming output looks choppy. Is that normal or is something broken?

vibeready answers them with one command.

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

vibeready measures all of these with zero configuration. If your backend adds
a few response headers (the `x-ai-*` convention), it also tells you model
provider, cache hit rate, input/output token counts, and estimated cost.

## Quick Start

### 1. Install

```bash
git clone https://github.com/JinkaiLiu/vibeready.git
cd vibeready
go build -o vibeready ./cmd/loadgen
```

Requires Go >= 1.25.

### 2. Run your first readiness check

```bash
./vibeready \
  --url http://localhost:3000/api/chat \
  --method POST \
  --headers "Content-Type:application/json" \
  --body '{"message":"Explain quantum computing in simple terms"}' \
  --concurrency 5 \
  --duration 10s \
  --timeout 45s \
  --output result.json \
  --agent-context agent-report.md
```

For most AI apps, you only need to change two fields:

```bash
--url   your AI backend API endpoint
--body  a valid JSON payload accepted by that endpoint
```

You may also want to adjust `--concurrency` (how many virtual users) and `--duration`
(how long to run). The defaults are conservative — bump them for a more realistic test.

### 3. What each flag means

| Flag | Description | Required? |
|------|-------------|-----------|
| `--url` | The backend API endpoint that receives user input and calls the LLM | Required |
| `--method` | HTTP method. Most AI app APIs use `POST` | Recommended |
| `--headers` | Request headers. JSON APIs usually need `Content-Type:application/json` | Usually needed |
| `--body` | The JSON payload sent on every request. Must match what your backend expects | Usually needed |
| `--concurrency` | Number of virtual users sending requests at once. Start with `5` | Recommended |
| `--duration` | How long the check runs. Start with `10s`, then try `30s` | Required (or `--requests`) |
| `--timeout` | Max time to wait for one request before counting it as failed | Recommended |
| `--output` | Writes machine-readable JSON results to a file | Optional |
| `--agent-context` | Writes a Markdown report you can paste into Cursor, Codex, or Claude Code | Strongly recommended |

### 4. Not sure what `--url` or `--body` should be?

If you built your app with Cursor, Codex, Claude Code, or another coding agent,
ask it to inspect your project:

> I want to check whether my AI app is ready for real users using vibeready.
> Please inspect this project and find the backend API route that receives user
> input and calls the LLM. Tell me the full local URL, HTTP method, required
> headers, and a valid JSON request body. Then give me a complete vibeready
> command using this shape:
>
> ```
> ./vibeready \
>   --url ... \
>   --method POST \
>   --headers "Content-Type:application/json" \
>   --body '...' \
>   --concurrency 5 \
>   --duration 10s \
>   --timeout 45s \
>   --output result.json \
>   --agent-context agent-report.md
> ```

### 5. Common request body examples

Use the field names your backend expects. Common names: `message`, `prompt`, `text`, `question`.

```json
{"message":"Explain quantum computing in simple terms"}
{"prompt":"Explain quantum computing in simple terms"}
{"text":"Explain quantum computing in simple terms","target":"zh"}
{"question":"What is quantum computing?"}
```

### 6. After the run

vibeready produces three outputs:

**Console** (always printed):

```text
Total: 285  Success: 278  Failed: 7  Error Rate: 2.46%  QPS: 9.50
Avg: 1.05s  P50: 1.02s  P95: 2.10s  P99: 3.80s
Avg TTFT: 320ms  TTFT P50: 280ms  TTFT P95: 650ms
Upstream: 780ms  Overhead: 270ms  Upstream %: 74.3%
Provider: deepseek  Model: deepseek-chat  Cache Hit: 12.0%  429: 3 (1.1%)
```

**`result.json`** — machine-readable results for scripts or future comparison.

**`agent-report.md`** — a Markdown report designed for coding agents, with
suggested actions. For example:

> **Likely model API bottleneck (74% of total time)** — Optimizing your backend
> code may have limited impact. Consider: faster model, prompt compression, or
> a provider with lower latency.

> **Rate limited (429)** — 3 requests hit rate limits. Consider: client-side
> throttling, exponential backoff, or a higher API tier.

### 7. Paste the report back to your coding agent

After the run, open `agent-report.md` and paste it into your coding agent:

> Here is my vibeready report. Please identify whether this AI app is ready for
> a small group of real users, find the main bottleneck, and suggest concrete
> code changes. Then give me a safer re-test command.

vibeready detects → coding agent fixes → vibeready re-tests.

## Semi-white-box AI metrics

If your backend adds these response headers, vibeready can tell you where the
time goes and whether caching helps:

```
x-ai-provider            → which model provider (openai, deepseek, etc.)
x-ai-model               → which model (gpt-4o, deepseek-chat, etc.)
x-ai-upstream-latency-ms → how long the model API took (separate from your backend)
x-ai-first-token-ms      → TTFT measured at the model side
x-ai-input-tokens        → prompt token count
x-ai-output-tokens       → completion token count
x-ai-cache-hit           → whether the response was served from cache
```

With these headers, vibeready computes: backend overhead (`total − upstream`),
upstream ratio (`upstream / total`), cache hit rate, and estimated cost (when
`--model-price` is set).

All headers are optional. vibeready degrades gracefully — without them you still
get latency, TTFT, error rates, and status codes.

See [docs/semi-white-box.md](docs/semi-white-box.md) for Python and Node.js
integration examples.

<details>
<summary><strong>Advanced capabilities</strong> (click to expand)</summary>

These exist and work, but are not required for the basic workflow:

| Area | Support |
|------|---------|
| gRPC (unary + server-streaming) | Via server reflection. `--protocol grpc` |
| WebSocket (`ws://` / `wss://`) | RFC 6455, TLS. `--protocol websocket` |
| Prometheus `/metrics` | `--metrics-port 9090` (binds 127.0.0.1) |
| Real-time SSE dashboard | Distributed mode only. |
| Docker | `docker build -t vibeready .` |
| Distributed master/worker | For throughput beyond a single machine |

See [docs/advanced/](docs/advanced/) for detailed guides.

</details>

## Why not just use k6?

k6 and wrk are excellent general-purpose load testers. vibeready focuses on a
narrower problem: AI app readiness checks with TTFT, token speed, and
upstream-vs-backend split — metrics traditional tools don't capture without
custom scripting. It also produces an agent-readable report for the fix→re-test
cycle.

## Documentation

- [CLI reference](docs/cli-reference.md) — every flag
- [Protocols](docs/protocols.md) — HTTP streaming, gRPC, WebSocket
- [Semi-white-box metrics](docs/semi-white-box.md) — `x-ai-*` header convention
- [Distributed mode](docs/advanced/distributed.md)
- [Docker & Kubernetes](docs/advanced/deployment.md)
- [Architecture](docs/architecture.md)
- [中文文档](README_zh.md)

## License

MIT — see [LICENSE](LICENSE).
