# Deployment

## Docker

```bash
docker build -t perf-loadgen .
docker run perf-loadgen \
  --url http://target:8080/infer --method POST \
  --body '{"prompt":"hello"}' --concurrency 10 --duration 30s

# Different entry points
docker run --entrypoint /usr/local/bin/loadgen-master vibeready --persistent --dashboard-addr :7070
docker run --entrypoint /usr/local/bin/loadgen-worker vibeready --capacity 3
```

The image contains all four binaries: `loadgen`, `loadgen-master`, `loadgen-worker`, `mockserver`.

## Kubernetes (advanced)

A Helm chart is provided under `deploy/helm/vibeready/`. This is an advanced
deployment option. For most users, the single-binary CLI or Docker workflow is
the recommended path.

```bash
helm install loadgen ./deploy/helm/perf-loadgen \
  --set config.targetURL=http://inference-service:8080/infer \
  --set worker.replicas=3
```

Resources deployed:

| Resource | Kind | Purpose |
|----------|------|---------|
| `{release}-master` | Deployment (1 replica) | Job server + dashboard + API |
| `{release}-worker` | StatefulSet (N replicas) | Worker pool |
| `{release}-master` | Service (ClusterIP) | Dashboard port |
| `{release}-worker` | Service (headless) | Stable DNS per pod |
| `{release}-config` | ConfigMap | Reference job config template (mounted at `/etc/vibeready/job-config.json`; not auto-executed — submit jobs via API) |
