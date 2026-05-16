# vibeready

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

vibeready 零配置即可测量以上所有指标。如果你的后端加几个响应头
（`x-ai-*` 规范），还能告诉你模型提供商、缓存命中率、输入/输出 token 数量和成本估算。

## 快速开始

### 1. 安装

```bash
git clone https://github.com/JinkaiLiu/vibeready.git
cd vibeready
go build -o vibeready ./cmd/loadgen
```

需要 Go >= 1.25。

### 2. 首次上线检查

```bash
./vibeready \
  --url http://localhost:3000/api/chat \
  --method POST \
  --headers "Content-Type:application/json" \
  --body '{"message":"用简单的话解释量子计算"}' \
  --concurrency 5 \
  --duration 10s \
  --timeout 45s \
  --output result.json \
  --agent-context agent-report.md
```

大多数 AI 应用只需要改两个参数：

```bash
--url   你的 AI 后端 API 地址
--body  你的接口接受的 JSON 请求体
```

### 3. 各参数含义

| 参数 | 填什么 | 必需？ |
|------|--------|--------|
| `--url` | 接收用户输入并调用 LLM 的后端 API 地址 | 必需 |
| `--method` | HTTP 方法。大多数 AI 应用使用 `POST` | 推荐 |
| `--headers` | 请求头。JSON API 通常需要 `Content-Type:application/json` | 通常需要 |
| `--body` | 每次请求发送的 JSON。必须与你的后端代码匹配 | 通常需要 |
| `--concurrency` | 同时发送请求的虚拟用户数。从 `5` 开始 | 推荐 |
| `--duration` | 检查持续多久。先试 `10s`，再试 `30s` | 必需（或用 `--requests`） |
| `--timeout` | 单个请求最长等待时间，超时算失败 | 推荐 |
| `--output` | 输出机器可读的 JSON 结果到文件 | 可选 |
| `--agent-context` | 输出 Markdown 报告，可粘贴到 Cursor、Codex 等 agent | 强烈推荐 |

### 4. 不确定 `--url` 或 `--body` 填什么？

如果你用 Cursor、Codex、Claude Code 等工具生成了这个项目，直接让你的 coding agent 检查：

> 我想用 vibeready 检查我的 AI 应用是否准备好上线了。请检查这个项目，
> 找到接收用户输入并调用 LLM 的后端 API 路由。告诉我完整的本地 URL、
> HTTP 方法、必需的请求头和有效的 JSON 请求体。然后给我一条完整的
> vibeready 命令，格式如下：
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

### 5. 常见请求体示例

使用你的后端代码中定义好的字段名。常见的字段名：`message`、`prompt`、`text`、`question`。

```json
{"message":"用简单的话解释量子计算"}
{"prompt":"用简单的话解释量子计算"}
{"text":"用简单的话解释量子计算","target":"en"}
{"question":"什么是量子计算？"}
```

### 6. 运行之后

vibeready 输出三样东西：

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

**`result.json`** — 机器可读的结构化结果，用于脚本或日后对比。

**`agent-report.md`** — 面向 coding agent 的 Markdown 报告，含诊断建议。例如：

> **Likely model API bottleneck (74% of total time)** — 优化后端代码效果有限。
> 考虑更快模型、prompt 压缩或延迟更低的提供商。

> **Rate limited (429)** — 3 个请求触发限流。考虑客户端节流、指数退避或更高 API 套餐。

### 7. 把报告交给 coding agent

运行后打开 `agent-report.md`，粘贴给你的 coding agent：

> 这是我的 vibeready 报告。请判断这个 AI 应用是否准备好了小范围上线，
> 找出主要瓶颈，建议具体代码修改，并给出更安全的复测命令。

vibeready 检测 → coding agent 修复 → vibeready 复测。

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

如果你的后端返回这些响应头，vibeready 会计算上游延迟、后端开销、缓存命中率：

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

<details>
<summary><strong>高级能力</strong>（点击展开）</summary>

这些功能可用，但不是基础使用所必需的：

| 领域 | 支持情况 |
|------|---------|
| gRPC（unary + server-streaming） | 通过 server reflection。`--protocol grpc` |
| WebSocket（`ws://` / `wss://`） | RFC 6455，TLS。`--protocol websocket` |
| Prometheus `/metrics` | `--metrics-port 9090`（绑定 127.0.0.1） |
| SSE 实时 Dashboard | 仅分布式模式 |
| Docker | `docker build -t vibeready .` |
| 分布式 master/worker | 单机吞吐量不够时 |

详见 [docs/advanced/](docs/advanced/)（英文）。

</details>

## 和 k6 的区别

你可以让 coding agent 帮你写 k6 脚本，对通用 HTTP 压测来说这没毛病。但：

- Agent 生成的 k6 脚本测 HTTP 延迟，不测 TTFT、token 速率。
- 它们不会告诉你瓶颈在后端还是模型 API。
- 每次生成的脚本不同，无法形成标准化的"修完 → 复测 → 对比"流程。

vibeready 不替代 k6。它聚焦一个更窄的问题：AI 应用上线的标准化体检，
产出模型感知的指标和 coding agent 可直接消化的报告。

## 文档

- [CLI 参考](docs/cli-reference.md)（英文）
- [协议详解](docs/protocols.md)（英文）
- [半白盒指标](docs/semi-white-box.md)（英文）
- [分布式模式](docs/advanced/distributed.md)（英文）
- [Docker & Kubernetes](docs/advanced/deployment.md)（英文）
- [架构](docs/architecture.md)（英文）

## License

MIT — 详见 [LICENSE](LICENSE)。
