# Distributed Mode

For tests that need more throughput than a single machine can generate.

## One-shot

```bash
./loadgen-worker --id worker-a --listen :8081 --capacity 3 &
./loadgen-worker --id worker-b --listen :8082 --capacity 3 &

./loadgen-master \
  --workers http://127.0.0.1:8081,http://127.0.0.1:8082 \
  --dashboard-addr :7070 \
  --url http://target:8080/infer --method POST \
  --body '{"prompt":"hello"}' --requests 1000 --concurrency 20
```

Master dispatches to workers, waits, aggregates results, prints summary, exits.
Open `http://127.0.0.1:7070` for the real-time dashboard.

## Persistent (job server)

```bash
./loadgen-master --persistent --dashboard-addr 127.0.0.1:7070 --auth-secret mysecret

# Submit (with Bearer token)
curl -X POST http://127.0.0.1:7070/api/jobs \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer mysecret' \
  -d '{"config":{...},"workers":["http://127.0.0.1:8081"]}'

# List   → GET /api/jobs
# Status → GET /api/jobs/{id}
# Cancel → DELETE /api/jobs/{id} (requires Authorization header)
```

## Authentication

Set `--auth-secret` on master and all workers. Master signs each dispatch with
HMAC-SHA256(jobID, secret). Workers verify before executing.

Additionally, when `--auth-secret` is set on the master, write endpoints
(POST/DELETE /api/jobs) require an `Authorization: Bearer <secret>` header.
GET endpoints and the dashboard remain public.

Always bind the dashboard to `127.0.0.1` unless you have a reverse proxy
handling auth. `/health` is always unauthenticated.

## Capacity slots

Workers accept multiple concurrent jobs via `--capacity N`. Master checks
`active_jobs < capacity` before dispatching. Full workers return 409.
