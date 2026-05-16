# Changelog

## v0.1.0 (2026-05-14)

Initial release.

### Protocols
- HTTP — shared connection pool, concurrency, QPS limiting, ramp-up
- HTTP Streaming — SSE / JSONL / raw line-delimited, TTFT, token, tok/s, streaming abort metrics
- gRPC Unary — server reflection + dynamicpb + protojson
- gRPC Server-Streaming — per-message ITL tracking, token field extraction
- WebSocket — zero-dependency RFC 6455, ws:// + wss:// (TLS), subprotocol support

### Execution Modes
- Fixed duration, fixed request count, QPS-limited, ramp-up
- Payload rotation from directory (`--payload-dir`)
- Config file (`--config`) with CLI overlay (only explicitly-set flags override)

### Distributed Mode
- Master/worker architecture
- One-shot mode: dispatch and wait
- Persistent mode: REST API job server (POST/GET/DELETE /api/jobs)
- HMAC-SHA256 shared-secret authentication
- Worker capacity slots (multi-job concurrency)
- Job cancellation (context propagation to workers)

### Observability
- Console summary with latency/TTFT/ITL percentile breakdown
- JSON report with human-readable duration fields
- Prometheus `/metrics` endpoint (30+ metrics)
- SSE real-time dashboard with polling fallback
- Three independent log-scale histograms: latency, TTFT, ITL

### Operations
- Multi-stage Docker build
- Kubernetes Helm chart (master Deployment + worker StatefulSet)
- Liveness/readiness probes
- Graceful shutdown (SIGTERM → drain → exit)

### Testing
- Unit tests for all internal packages
- Integration tests (HTTP unary, streaming, config file)
- Benchmarks for stats collector and engine
