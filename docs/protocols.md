# Protocols

## HTTP

Standard request/response. Extracts inference metrics from `X-Loadgen-*` and
`x-ai-*` response headers when present.

```bash
./vibeready --url http://localhost:8080/infer --method POST \
  --body '{"prompt":"hello"}' --concurrency 20 --duration 30s
```

## HTTP Streaming

SSE, JSONL, or raw line-delimited responses. The runner parses each chunk for
text content, token counts, and stream completion signals.

**SSE**:
```bash
./vibeready --url http://localhost:8080/v1/chat/completions --method POST \
  --body '{"model":"llama3","messages":[{"role":"user","content":"hello"}],"stream":true}' \
  --stream --stream-format sse --stream-text-keys content,delta \
  --stream-token-keys usage.completion_tokens \
  --concurrency 20 --duration 30s --timeout 15s
```

**JSONL**:
```bash
./vibeready --url http://localhost:8080/stream --method POST \
  --body '{"prompt":"hello"}' --stream --stream-format jsonl \
  --stream-text-keys text,content --stream-token-keys tokens \
  --concurrency 10 --requests 500
```

Internally: each line is parsed as JSON. `--stream-text-keys` are searched
case-insensitively and recursively for text. `--stream-token-keys` are searched
for token counts. Done is detected via `--stream-done-marker`, `done:true`,
`finish_reason`, or `stop_reason` fields. TTFT is time to first text chunk.
ITL is time between consecutive text chunks.

## gRPC

Requires server reflection enabled on the target. No `.proto` files needed.

**Unary**:
```bash
./vibeready --url localhost:50051 --protocol grpc \
  --proto-service inference.GRPCInferenceService --proto-method ModelInfer \
  --body '{"model_name":"llama3","inputs":[{"name":"prompt","contents":"hello"}]}' \
  --concurrency 50 --duration 30s --timeout 10s
```

**Server-streaming**:
```bash
./vibeready --url localhost:50051 --protocol grpc-stream \
  --proto-service inference.GRPCInferenceService --proto-method ModelStreamInfer \
  --body '{"model_name":"llama3","inputs":[{"name":"prompt","contents":"hello"}]}' \
  --grpc-token-field output_tokens --concurrency 20 --duration 30s
```

**TLS**: add `--grpc-tls`.

Internally: uses `grpc_reflection_v1` to discover method descriptors at runtime,
`dynamicpb` + `protojson` to convert JSON body to protobuf messages.
For streaming, TTFT = first `RecvMsg`, ITL = between consecutive `RecvMsg`.

## WebSocket

WebSocket client (text/close/ping/pong frames, 10 MiB frame limit). Each request is a full lifecycle:
TCP/TLS dial → HTTP upgrade → send text frame → receive frames → close.

TLS is auto-detected from the URL scheme: `ws://` uses plain TCP (port 80),
`wss://` uses TLS (port 443). A custom `tls.Config` can be set via `SetTLSConfig`.

```bash
# Plain
./vibeready --url ws://localhost:8080/graphql --protocol websocket \
  --ws-subprotocol graphql-transport-ws \
  --body '{"type":"subscribe","payload":{"query":"subscription { message }"}}' \
  --concurrency 50 --duration 30s --timeout 10s

# TLS
./vibeready --url wss://api.example.com/graphql --protocol websocket \
  --ws-subprotocol graphql-transport-ws \
  --body '{"type":"subscribe","payload":{"query":"subscription { message }"}}' \
  --concurrency 50 --duration 30s --timeout 10s
```
