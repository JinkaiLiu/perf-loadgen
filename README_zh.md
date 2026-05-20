# vibeready

面向 AI API 应用的轻量级、模型感知压测工具。
一条命令告诉你：你的 AI 应用是否准备好上线了。

## 适合谁

你用 Cursor、Codex、Claude Code 等工具做了一个 AI 应用——翻译服务、聊天机器人、文档总结等等。它调用 OpenAI、DeepSeek 或任何 LLM API。你准备把链接发给朋友、微信群或者小团队试用，甚至正式上线。链接发出去之前，下面这些才是你真正需要知道的：

- 5 个朋友同时打开。会有人看到报错吗？
- 有人粘贴了一篇长文档。请求会不会超时？
- AI 回复感觉很慢。是你代码的问题、VPS 的问题，还是模型 API 的问题？
- 流式输出看着一顿一顿的。这是正常的还是哪里坏了？

vibeready 一条命令回答它们。

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

你也可以调整 `--concurrency`（模拟多少用户）和 `--duration`（持续多久）。
默认值比较保守——想要更真实的测试结果，把它们调大。

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
Total: 285  Success: 278  Failed: 7  Error Rate: 2.46%  QPS: 9.50
Avg: 1.05s  P50: 1.02s  P95: 2.10s  P99: 3.80s
Avg TTFT: 320ms  TTFT P50: 280ms  TTFT P95: 650ms
Upstream: 780ms  Overhead: 270ms  Upstream %: 74.3%
Provider: deepseek  Model: deepseek-chat  Cache Hit: 12.0%  429: 3 (1.1%)
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

## 半白盒 AI 指标

如果你的后端加上这些响应头，vibeready 能告诉你时间花在哪、缓存有没有用：

```
x-ai-provider            → 模型提供商（openai、deepseek 等）
x-ai-model               → 模型名称（gpt-4o、deepseek-chat 等）
x-ai-upstream-latency-ms → 模型 API 耗时（与你的后端开销分开）
x-ai-first-token-ms      → 模型侧测量的 TTFT
x-ai-input-tokens        → prompt token 数量
x-ai-output-tokens       → 输出 token 数量
x-ai-cache-hit           → 是否命中缓存
```

有了这些 header，vibeready 可以算出：后端开销（`total − upstream`）、上游占比
（`upstream / total`）、缓存命中率，以及成本估算（设置 `--model-price` 后）。

所有 header 都是可选的。不加也能拿到延迟、TTFT、错误率和状态码。

集成示例见 [docs/semi-white-box.md](docs/semi-white-box.md)（Python / Node.js）。

<details>
<summary><strong>高级能力</strong>（点击展开）</summary>

这些功能可用，但不是基础使用所必需的：

| 领域 | 支持情况 |
|------|---------|
| gRPC（unary + server-streaming） | 通过 server reflection。`--protocol grpc` |
| WebSocket（`ws://` / `wss://`） | Text 帧、ping/pong、TLS。`--protocol websocket` |
| Prometheus `/metrics` | `--metrics-port 9090`（绑定 127.0.0.1） |
| SSE 实时 Dashboard | 仅分布式模式 |
| Docker | `docker build -t vibeready .` |
| 分布式 master/worker | 单机吞吐量不够时 |

详见 [docs/advanced/](docs/advanced/)（英文）。

</details>

## 和 k6 的区别

k6 和 wrk 是优秀的通用压测工具。vibeready 聚焦一个更窄的问题：AI 应用上线检查，
关注 TTFT、token 速率和上游/后端延迟拆分——这些是传统工具不写脚本测不到的。
同时输出 coding agent 可直接消费的诊断报告。

## 文档

- [CLI 参考](docs/cli-reference.md)（英文）
- [协议详解](docs/protocols.md)（英文）
- [半白盒指标](docs/semi-white-box.md)（英文）
- [分布式模式](docs/advanced/distributed.md)（英文）
- [Docker & Kubernetes](docs/advanced/deployment.md)（英文）
- [架构](docs/architecture.md)（英文）

## License

MIT — 详见 [LICENSE](LICENSE)。
