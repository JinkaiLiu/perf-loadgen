# CLI Reference

## Universal flags

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | (required) | Target. HTTP: `http://host/path`, gRPC: `host:port`, WebSocket: `ws://host/path` |
| `--method` | `GET` | HTTP method (ignored for gRPC/WebSocket) |
| `--headers` | — | Comma-separated `Key:Value` pairs |
| `--body` | — | Inline request body. JSON for gRPC. |
| `--body-file` | — | Read body from a file |
| `--payload-dir` | — | Rotate payload files from a directory |
| `--config` | — | JSON config file. CLI flags only override explicitly-set values. |
| `--protocol` | `http` | `http`, `grpc`, `grpc-stream`, `websocket` |
| `--concurrency` | `1` | Concurrent workers |
| `--duration` | `0` | Run duration. `0` = use `--requests`. |
| `--requests` | `0` | Total requests. `0` = use `--duration`. |
| `--qps` | `0` | Global requests/sec cap. `0` = unlimited. |
| `--ramp-up` | `0` | Linear worker activation period |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | — | JSON report path |
| `--agent-context` | — | Agent-friendly Markdown report path |
| `--model-price` | `0` | Model price per 1K output tokens (for cost estimation) |
| `--metrics-port` | `0` | Prometheus port on `127.0.0.1`. `0` = disabled. |

## HTTP Streaming

| Flag | Default | Description |
|------|---------|-------------|
| `--stream` | `false` | Enable streaming |
| `--stream-format` | `auto` | `auto`, `sse`, `jsonl`, `raw` |
| `--stream-done-marker` | `[DONE]` | Completion marker |
| `--stream-text-keys` | — | JSON keys for text (`content,delta`) |
| `--stream-token-keys` | — | JSON keys for tokens (`output_tokens`) |

## gRPC

| Flag | Default | Description |
|------|---------|-------------|
| `--proto-service` | — | Fully-qualified service name |
| `--proto-method` | — | Method name |
| `--grpc-tls` | `false` | Enable TLS |
| `--grpc-token-field` | — | Response field for token count |

Target must have server reflection enabled.

## WebSocket

| Flag | Default | Description |
|------|---------|-------------|
| `--ws-subprotocol` | — | Subprotocol (`graphql-ws`) |

`ws://` and `wss://` (TLS) are both supported. TLS is auto-detected from the URL scheme.

## Master (`loadgen-master`)

| Flag | Default | Description |
|------|---------|-------------|
| `--workers` | — | Comma-separated worker URLs |
| `--dashboard-addr` | — | Dashboard + API address (`:7070`) |
| `--persistent` | `false` | Run as persistent job server |
| `--max-concurrent-jobs` | `10` | Max concurrent jobs |
| `--auth-secret` | — | HMAC shared secret |

## Worker (`loadgen-worker`)

| Flag | Default | Description |
|------|---------|-------------|
| `--id` | `worker-1` | Unique worker identifier |
| `--listen` | `:8081` | Listen address |
| `--capacity` | `1` | Max concurrent jobs |
| `--auth-secret` | — | HMAC shared secret (must match master) |
