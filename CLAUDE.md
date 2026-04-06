# CLAUDE.md — NetPilot

> LLM-Agent 驱动的智能网络代理客户端。sing-box 内核 + Go 后端 + iOS 优先。

## 项目概述

NetPilot 是一个能听懂用户意图、实时感知流量状态、自动编排复杂链路的智能网络代理客户端。核心差异：Agent 深度驱动，不只是代理 GUI。

## 技术栈

- **网络内核**: sing-box（Clash API HTTP 控制，Phase 3 切 libbox gRPC）
- **后端语言**: Go
- **LLM**: DeepSeek V3.2 via 硅基流动（开发阶段）；生产环境 LLM Router 多模型切换
- **目标平台**: iOS 优先（NEPacketTunnelProvider + SwiftUI）
- **架构模式**: 借鉴 Claude Code 的 tool-use 闭环 + Agent 角色分工

## 核心架构（四层）

```
UI Layer (SwiftUI)
    ↓
Intent Router → 95% 走 Local Engine（零 LLM 成本）
              → 5% 走 LLM Agent（复杂编排）
    ↓
Tool Pipeline (Pre-Hook → Snapshot → Execute → Post-Hook → Telemetry)
    ↓
EngineAdapter interface → SingBoxAdapter (Clash API / libbox gRPC)
    ↓
sing-box 内核
```

## 设计原则

1. **Agent 不是黑箱**: 每次操作可审计、可解释、可回滚
2. **用户配置不可侵犯**: Agent 只在 `_agent:` 前缀的 Overlay 区域操作
3. **失败安全**: 异常 → 自动回滚 → 上报，不继续尝试
4. **本地优先**: 能本地处理的绝不调 LLM
5. **内核可替换**: 所有 sing-box 依赖收敛到 EngineAdapter 接口

## 当前进度

**Phase 1 — CLI 原型（进行中）**

已完成:
- [x] 任务 1: EngineAdapter + SingBoxAdapter (Clash API HTTP) + CLI REPL
- [x] 任务 2: Intent Router (中文关键词匹配) + Local Engine (7 个 Action)
- [x] 任务 3: Tool Pipeline + Hook + Snapshot/Rollback + Telemetry
- [x] 任务 4: LLM Agent (DeepSeek via 硅基流动, tool-use 闭环)
- [x] 任务 5: Config Overlay + 路由规则修改 + 模板系统 (4 个内置模板)
- [x] 任务 6: 修复 Agent 二轮调用 bug (纯文本消息格式绕过硅基流动校验)
- [x] 任务 7: Agent 角色分工 (Diagnose/Configure/Verify Orchestrator)
- [x] 任务 8: 对话历史 + 多轮上下文 (摘要注入 system prompt)
- [ ] 任务 9: 真实节点测试
- [ ] 任务 10: 自动化测试

验证里程碑:
- [x] sing-box Clash API 端到端可控 ✅
- [x] libbox iOS framework 编译成功 (gomobile bind) ✅
- [x] 中文自然语言 → 本地执行 ✅
- [x] 写操作自动快照 + 失败回滚 ✅
- [x] LLM Agent tool-use 闭环 ✅
- [x] Agent 三角色分工 + Orchestrator 编排 ✅
- [x] Config Overlay 路由规则增删 + 模板 ✅
- [x] 多轮对话上下文 + /clear /history 命令 ✅

## 详细设计文档

以下文档包含各模块的完整实现方案，按需引用：

| 文档 | 内容 | 何时引用 |
|------|------|---------|
| `docs/architecture.md` | 四层架构详细设计、Intent Router/Local Engine 实现代码 | 修改架构时 |
| `docs/agent-runtime.md` | Agent 运行时系统：动态 Prompt 编排、Tool Pipeline 8 步管道、Hook 实现、Agent 角色分工（Diagnose/Configure/Verify）、Orchestrator | **任务 4/5 必读** |
| `docs/ios-platform.md` | iOS NE 实现、进程架构、IPC 四通道、内存管理、Entitlement 配置（含 Hiddify 源码验证数据） | Phase 3 时 |
| `docs/intent-pipeline.md` | 自然语言 → sing-box 配置的四层转译、Intent Schema 定义 | 做意图解析时 |
| `docs/traffic-engine.md` | 流量感知引擎、Sniffing metadata、主动汇报触发器 | Phase 2 时 |
| `docs/engine-evolution.md` | 内核演进路线、EngineAdapter 完整接口、SingBoxAdapter 实现、三阶段替换计划 | 修改 EngineAdapter 时 |
| `docs/business-model.md` | LLM 成本估算、收费模型（双轨并行）、商业模型 | 产品决策时 |
| `docs/roadmap.md` | 完整开发路线图（Phase 0-4）、各阶段痛点与对策 | 规划时 |

## 代码规范

- Go 标准项目布局：`cmd/` 入口、`internal/` 业务逻辑
- 所有 sing-box 交互通过 EngineAdapter 接口，禁止直接调用 Clash API
- 写操作必须经过 Tool Pipeline（Pre-Hook → Snapshot → Execute → Post-Hook）
- Agent 生成的配置资源使用 `_agent:` 前缀
- 错误处理：不 panic，返回友好错误信息
- 中文注释/英文代码

## 参考资源

- [sing-box 官方文档](https://sing-box.sagernet.org/)
- [sing-box Clash API](https://sing-box.sagernet.org/configuration/experimental/clash-api/)
- [Anthropic Tool Use 文档](https://docs.anthropic.com/en/docs/build-with-claude/tool-use)
- `~/References/claude-code-sourcemap/` — Claude Code 还原源码（Agent 循环、Tool 调度参考）
- `~/References/hiddify-app/` — Hiddify 源码（iOS sing-box 集成参考）
