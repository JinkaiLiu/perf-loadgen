# Semi-White-Box AI Metrics

If your backend adds standardized response headers, vibeready computes
richer metrics: upstream latency breakdown, cache efficiency, per-request token
counts, and provider/model identification.

## Header convention

Add these headers to your AI backend responses:

| Header | Type | Example | Description |
|--------|------|---------|-------------|
| `x-ai-provider` | string | `deepseek` | Model provider name |
| `x-ai-model` | string | `deepseek-chat` | Model identifier |
| `x-ai-upstream-latency-ms` | float | `780.0` | Model API round-trip time (ms) |
| `x-ai-first-token-ms` | float | `280.0` | Time from API call to first token (ms) |
| `x-ai-input-tokens` | int | `50` | Input token count |
| `x-ai-output-tokens` | int | `120` | Output token count |
| `x-ai-cache-hit` | bool | `false` | Whether this response was served from cache |

All are optional. vibeready degrades gracefully when headers are absent.

## What this unlocks

With these headers, vibeready computes:

```text
backend_overhead  = total_latency - upstream_latency
upstream_ratio    = upstream_latency / total_latency
cache_hit_rate    = cache_hits / total_requests_with_headers
tokens_per_second = output_tokens / generation_time
```

And the agent report (`--agent-context`) adds targeted suggestions:

- upstream_ratio > 70% → bottleneck is the model API
- upstream_ratio < 30% → bottleneck is your backend code
- cache_hit_rate == 0 → consider semantic caching
- 429 count > 0 → hitting rate limits, add throttling

## Python (FastAPI) example

```python
from fastapi import FastAPI
from fastapi.responses import JSONResponse
import time

app = FastAPI()

@app.post("/translate")
async def translate(text: str, target: str):
    t0 = time.time()
    result = await call_deepseek(text, target)  # your model call
    upstream_ms = (time.time() - t0) * 1000

    return JSONResponse(
        content={"translated": result.text},
        headers={
            "x-ai-provider": "deepseek",
            "x-ai-model": "deepseek-chat",
            "x-ai-upstream-latency-ms": f"{upstream_ms:.1f}",
            "x-ai-first-token-ms": f"{result.ttft_ms:.1f}",
            "x-ai-input-tokens": str(result.input_tokens),
            "x-ai-output-tokens": str(result.output_tokens),
            "x-ai-cache-hit": str(result.from_cache).lower(),
        },
    )
```

## Node.js (Express) example

```javascript
app.post('/translate', async (req, res) => {
  const t0 = Date.now();
  const result = await callDeepSeek(req.body.text, req.body.target);
  const upstreamMs = Date.now() - t0;

  res.set({
    'x-ai-provider': 'deepseek',
    'x-ai-model': 'deepseek-chat',
    'x-ai-upstream-latency-ms': String(upstreamMs),
    'x-ai-first-token-ms': String(result.ttftMs),
    'x-ai-input-tokens': String(result.inputTokens),
    'x-ai-output-tokens': String(result.outputTokens),
    'x-ai-cache-hit': String(result.fromCache),
  });
  res.json({ translated: result.text });
});
```
