# Architecture

```
CLI (parse.go + --config) → Config → Engine (pacer, payload rotation)
                                          │
                    ┌─────────────────────┼─────────────────────┐
                    ▼                     ▼                     ▼
              Registry           Stats Collector           Output
         ┌─────┼─────┐       ┌──────┼──────┐       ┌──────┼──────┐
         │     │     │       │      │      │       │      │      │
        http grpc  ws    latency  TTFT   ITL   console  json  prometheus
       stream                                           agent_context
```

## Package responsibilities

| Package | Role |
|---------|------|
| `cmd/loadgen` | Single-node entry. CLI → Engine → Runner → Output. |
| `cmd/loadgen-master` | Distributed master. One-shot or persistent, dispatches to workers. |
| `cmd/loadgen-worker` | Distributed worker. `/run`, `/health`, `/cancel/{id}`, `/status/{id}`. |
| `cmd/mockserver` | Mock server for development and testing. |
| `pkg/types` | Shared types: `RequestSpec`, `RunResult`, `Summary`, `AggregateSnapshot`. |
| `internal/cli` | Flag parsing, `--config` loading, explicit-flag tracking. |
| `internal/config` | `Config` struct, validation, file loading, CLI merge. |
| `internal/engine` | Worker lifecycle, QPS pacer, payload rotation. |
| `internal/runner` | `Runner` interface: `Run(ctx, RequestSpec) (RunResult, error)`. |
| `internal/protocol/registry` | Map-based runner registry with `init()` registration. |
| `internal/protocol/http` | HTTP runner. Shared transport, header parsing, error classification. |
| `internal/protocol/httpstream` | Streaming runner (SSE/JSONL/raw). TTFT, ITL, token extraction. |
| `internal/protocol/grpc` | gRPC runner. Server reflection + dynamicpb + protojson. |
| `internal/protocol/websocket` | WebSocket runner (text/close/ping/pong frames, 10 MiB limit). |
| `internal/protocol/httputil` | Shared HTTP transport, `x-ai-*` header constants, error helpers. |
| `internal/stats` | Three log-scale histograms (latency, TTFT, ITL) with mergeable snapshots. |
| `internal/distributed` | Master API, worker, job queue, HMAC auth, SSE dashboard. |
| `internal/output` | Console, JSON, Prometheus, agent context Markdown. |
| `internal/tokenizer` | Tokenizer interface (word-count default). |

## Key design decisions

**Three independent histograms.** Latency, TTFT, and ITL share identical log-scale
bucket boundaries (1µs–60s, 1.25× factor, ~93 buckets). Snapshots merge without
raw sample replay — each worker ships a HistogramSnapshot, master merges by
adding bucket counts.

**Runner registry.** `map[string]Factory` + `init()`. Adding a protocol requires
a new package + blank import in `factory.go`. Zero changes to existing code.

**SSE dashboard.** `/api/stream` pushes state on every worker result update.
Falls back to `GET /api/latest` polling when EventSource is unavailable.

**Dynamic gRPC.** Uses server reflection to discover method signatures at
runtime. No `.proto` files or code generation needed. JSON body is converted via
`protojson.Unmarshal` + `dynamicpb.NewMessage`.

**Zero-dependency WebSocket.** Pure standard library: TCP dial →
HTTP upgrade → manual frame mask/unmask → text/close/ping/pong frame handling, 10 MiB frame limit.
