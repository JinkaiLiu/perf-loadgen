# perf-loadgen

面向 AI API 应用的轻量级、模型感知压测工具。
一条命令告诉你：你的 AI 应用是否准备好上线了。

## 适合谁

你用 Cursor、Codex、Claude Code 等工具做了一个 AI 应用——翻译服务、聊天机器人、文档总结。它调用 OpenAI、DeepSeek 或任何 LLM API。你准备把链接发给朋友、微信群或者小团队试用。你需要确认：扛得住吗？

## 为什么需要它

通用压测工具（wrk、k6、vegeta）测的是 HTTP 性能。它们告诉你 p95 延迟和错误率。这对 AI 应用来说不够。

AI 应用的用户不关心 HTTP 响应时间，他们关心：

| 指标 | 对用户意味着什么 |
|------|-----------------|
| **TTFT**（首 token 时间） | 多久看到第一个字。决定流式体验是"秒出"还是"干等"。 |
| **ITL**（token 间延迟） | token 输出是否平滑。决定"打字机效果"是否流畅。 |
| **tok/s** | 模型生成速度。低 → 用户等。 |
| **流式中断** | 流未正常完成。用户看到空白或半截回答。 |
| **429 / 限流** | 撞上模型 API 频率限制。你代码没问题，但用户拿错误。 |
| **上游 vs 后端** | 瓶颈在你的代码、你的 VPS，还是模型 API？不知道就没法优化对的方向。 |

perf-loadgen 零配置即可测量以上所有指标。如果你的后端加几个响应头
（`x-ai-*` 规范），还能告诉你模型提供商、缓存命中率、输入/输出 token 数量和成本估算。

## 快速开始

```bash
# 安装（Go >= 1.25）
git clone https://github.com/JinkaiLiu/perf-loadgen.git
cd perf-loadgen && go build -o loadgen ./cmd/loadgen

# 测你的 AI 接口
./loadgen \
  --url https://your-app.com/api/translate \
  --method POST \
  --headers "Content-Type:application/json" \
  --body '{"text":"用简单的话解释量子计算","target":"en"}' \
  --concurrency 10 \
  --duration 30s \
  --timeout 45s \
  --output result.json \
  --agent-context agent-report.md
```

一条命令，两个输出文件。`result.json` 是原始数据，`agent-report.md`
是结构化报告，可以直接交给 coding agent 做后续修复。

## 输出长什么样

**Console**（始终输出）：

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

**Agent 报告**（`--agent-context agent-report.md`）：

结构化 Markdown，含测试配置、延迟百分位、错误分类、上游分析、**自动诊断建议**。例如：

> **瓶颈在模型 API** — 74% 的延迟来自上游模型。优化你的后端代码效果有限。
> 考虑 prompt 压缩、更小的模型或延迟更低的提供商。

> **检测到限流 (429)** — 3 个请求返回 429。添加 `--qps` 限速、指数退避重试或升级 API 套餐。

## 功能

### 核心（始终可用）

- HTTP 和 HTTP streaming（SSE / JSONL / raw）
- TTFT、ITL、token 吞吐率、流式中断率
- 延迟百分位（P50/P90/P95/P99），人类可读
- 状态码和错误类别分类（timeout / network / HTTP / 429）
- Payload 目录轮转（`--payload-dir`）
- QPS 限制和渐进加压（`--qps`、`--ramp-up`）
- JSON 报告（`--output`）和 Agent 友好的 Markdown 报告（`--agent-context`）

### 半白盒 AI 指标

如果你的后端返回这些响应头，perf-loadgen 会计算上游延迟、后端开销、缓存命中率：

```
x-ai-provider: deepseek
x-ai-model: deepseek-chat
x-ai-upstream-latency-ms: 780
x-ai-first-token-ms: 280
x-ai-input-tokens: 50
x-ai-output-tokens: 120
x-ai-cache-hit: false
```

详见 [docs/semi-white-box.md](docs/semi-white-box.md)（英文）。

### 高级能力

这些功能可用，但不是基础使用所必需的：

| 领域 | 支持情况 |
|------|---------|
| gRPC（unary + server-streaming） | 通过 server reflection。`--protocol grpc` |
| WebSocket（`ws://` / `wss://`） | RFC 6455，TLS。`--protocol websocket` |
| Prometheus `/metrics` | `--metrics-port 9090`（绑定 127.0.0.1） |
| SSE 实时 Dashboard | 仅分布式模式 |
| Docker | `docker build -t perf-loadgen .` |
| 分布式 master/worker | 单机吞吐量不够时 |

详见 [docs/advanced/](docs/advanced/)（英文）。

## 和 k6 的区别

你可以让 coding agent 帮你写 k6 脚本，对通用 HTTP 压测来说这没毛病。但：

- Agent 生成的 k6 脚本测 HTTP 延迟，不测 TTFT、token 速率。
- 它们不会告诉你瓶颈在后端还是模型 API。
- 每次生成的脚本不同，无法形成标准化的"修完 → 复测 → 对比"流程。

perf-loadgen 不替代 k6。它聚焦一个更窄的问题：AI 应用上线的标准化体检，
产出模型感知的指标和 coding agent 可直接消化的报告。

## 文档

- [CLI 参考](docs/cli-reference.md)（英文）
- [协议详解](docs/protocols.md)（英文）
- [半白盒指标](docs/semi-white-box.md)（英文）
- [分布式模式](docs/advanced/distributed.md)（英文）
- [Docker & Kubernetes](docs/advanced/deployment.md)（英文）
- [架构](docs/architecture.md)（英文）

## Roadmap

- [ ] 基于 tiktoken 的精确 tokenizer（目前是启发式词数统计）
- [ ] 持久化任务队列（磁盘/RDB）
- [ ] k6 脚本导出（`perf-loadgen export k6`）
- [ ] OpenTelemetry trace 导出

## License

MIT — 详见 [LICENSE](LICENSE)。
