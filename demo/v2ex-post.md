# 做了一个 AI 应用上线前的负载检查工具 — vibeready

## 起因

最近用 Cursor 写了个调 OpenAI API 的翻译应用。本地跑一切正常，分享链接给朋友前心里突然没底了：

- 5 个人同时用会不会报错？
- 那个"打字机效果"会不会卡顿？
- 慢到底是我的代码、我的 VPS、还是 OpenAI 的问题？

传统压测工具（wrk、k6、vegeta）只告诉你 p95 延迟和错误率，对 AI 应用关键指标——TTFT（首个 token 到达时间）、ITL（token 间隔延迟）、上游/后端耗时拆解——全测不了。

于是写了 vibeready。

## 一行命令

```bash
./vibeready \
  --url http://localhost:3000/api/chat \
  --method POST \
  --headers "Content-Type:application/json" \
  --body '{"message":"解释一下量子计算"}' \
  --concurrency 5 \
  --duration 30s \
  --timeout 45s \
  --output result.json \
  --agent-context agent-report.md
```

输出：

```
Total: 1423  Success: 1356  Failed: 67  Error Rate: 4.71%  QPS: 47.43
Avg TTFT: 320ms  TTFT P95: 650ms
Upstream: 780ms  Overhead: 270ms  Upstream %: 74.3%
Provider: openai  Model: gpt-4o  429: 12 (0.8%)
```

**`Upstream 74.3%` 一眼看出瓶颈在模型 API，优化后端代码没用。**

## 我觉得最有用的功能

不是控制台输出，是 `--agent-context` 生成的 agent-report.md：

把这份 Markdown 报告直接粘贴到 Cursor / Claude Code 里，Agent 会：

- 诊断瓶颈在哪里（上游 vs 后端 vs 限流）
- 给出具体的修复建议（加缓存、换模型、加指数退避）
- 生成一条复测命令，改完代码直接重跑

**vibeready 检测 → Agent 修复 → vibeready 复测。你只管发链接。**

## 半白盒指标

如果你的 AI 后端加几个 `x-ai-*` 响应头，vibeready 还能告诉你：

- 用了哪个模型、哪个 provider
- 上游模型延迟 vs 你的后端开销
- 输入/输出 token 数
- 缓存命中率
- 预估成本（设置 `--model-price`）

所有头都是可选的，不加也能用，只是指标少一点。

## 技术栈

Go 1.25，零运行时依赖（除了 gRPC/protobuf）。单二进制。

支持协议：
- HTTP（非流式 + SSE/JSONL/raw 流式）
- gRPC（Unary + Server Streaming，靠 server reflection，不用 .proto 文件）
- WebSocket（text/close/ping/pong，TLS）

还有分布式 master/worker 模式和一个 Helm chart，单机压不够可以扩。

## GitHub

https://github.com/JinkaiLiu/vibeready

MIT 开源。Star 欢迎，更欢迎提 issue 告诉我你还需要什么指标。

## 坦白说

目前还比较早期，WebSocket 覆盖了常用帧但不完整（缺 fragmentation/continuation），分布式持久模式还在打磨。如果你愿意试试然后给反馈，我会非常感激。
