# CLAUDE.md — NetPilot (工作代号)

> **LLM-Agent 驱动的智能网络代理客户端**
> 介于 Surge 的极致观测性与 NekoBox 的全能协议支持之间，核心差异：Agent 深度驱动。

---

## 一、项目定位

NetPilot 不是又一个代理 GUI。它是一个能"听懂"用户意图、实时"感知"流量状态、并自动编排复杂链路的**智能网络管家**。

| 维度 | Surge | NekoBox/SFA | **NetPilot** |
|------|-------|-------------|-------------|
| 协议支持 | 有限（闭源） | 全能（sing-box） | 全能（sing-box） |
| 可观测性 | 极致 | 基础 | Agent 增强的语义可观测 |
| 链路编排 | 手动规则 | 手动配置 | **自然语言 → 自动编排** |
| 智能决策 | 无 | 无 | **Agent 闭环控制** |

---

## 二、技术栈决策

### 已锁定 ✅

| 组件 | 选型 | 理由 |
|------|------|------|
| **网络内核** | sing-box | 原生 detour 链式代理 + JSON 配置对 Agent 友好 + RESTful Clash API |
| **LLM** | Claude (Anthropic API) | Tool-use 能力成熟，支持结构化输出 |
| **目标平台** | **iOS 优先** | 移动端是代理工具主战场；iOS 用户付费意愿更高 |
| **Agent 架构模式** | 借鉴 Claude Code 的 tool-use 闭环 | 感知-决策-执行-验证循环 |

### 已锁定（第二轮）✅

| 组件 | 选型 | 理由 |
|------|------|------|
| **Agent 后端语言** | **Go**（首选）/ Rust（备选） | 性能优先；Go 与 sing-box 同生态，可直接 import libbox；Rust 备选用于极端性能场景 |
| **Agent-内核通信** | Unix domain socket（目标）/ HTTP localhost（原型） | UDS 延迟 <0.1ms 远优于 HTTP |

### 待定 ⏳

| 组件 | 候选 | 决策时机 |
|------|------|---------|
| **前端框架** | Jetpack Compose (Android native) / Flutter | 确定是否需要跨平台后决定 |
| **项目正式名** | 待定 | 品牌确定时 |

---

## 三、核心架构

### 3.1 四层架构（修订版：本地智能为主，LLM 为增强层）

```
┌─────────────────────────────────────────┐
│            UI Layer (展示层)              │
│   SwiftUI (iOS) — 只做渲染 + 输入采集    │
└──────────────────┬──────────────────────┘
                   │
          ┌────────▼────────┐
          │  Intent Router   │ ← 纯本地，<1ms，零成本
          │  (意图分流器)     │    判断本地能否处理
          └───┬──────────┬──┘
              │          │
       匹配成功      匹配失败/复杂意图
              │          │
   ┌──────────▼──┐  ┌───▼─────────────┐
   │ Local Engine │  │  LLM Agent      │
   │ (本地智能层)  │  │  (增强层, 按需)  │
   │ 零 API 成本  │  │  Claude/DeepSeek │
   │ 覆盖 95%交互 │  │  覆盖 5%复杂交互  │
   └──────────┬──┘  └────────┬────────┘
              │              │
              └──────┬───────┘
                     │
          ┌──────────▼──────────┐
          │    Tool Layer        │
          │  Config Overlay      │
          │  Conflict Resolver   │
          └──────────┬──────────┘
                     │ (Clash API / IPC)
          ┌──────────▼──────────┐
          │   Engine Layer       │
          │   sing-box (NE 进程) │
          └─────────────────────┘
```

**关键设计变更：LLM 从"核心引擎"降级为"增强层"。**

Local Engine 是主引擎，功能完整，覆盖绝大多数交互，零 API 成本。
LLM Agent 是增强层，只在复杂推理场景按需调用。

### 3.1.1 Intent Router（意图分流器）

**本质：关键词匹配 + 打分，没有任何机器学习。** 纯本地运行，<1ms。

```go
// 实现：分词 → 关键词命中打分 → 超过阈值则本地处理
func (r *IntentRouter) Route(input string) RoutingResult {
    // 第一步：预处理（去标点、转小写、中文分词）
    // 中文分词用 jieba Go 移植版，或更简单——直接子串匹配
    // "帮我换个快点的节点" → 包含 "换" + "节点" → 命中 SwitchNode
    tokens := tokenize(input)

    // 第二步：关键词命中打分
    bestMatch, score := matchKeywords(tokens)
    if score >= threshold {
        return RoutingResult{Handler: "local", Action: bestMatch}
    }

    // 第三步（可选优化）：iOS 上用 Apple NLClassifier 做模糊匹配
    // 200-500 条训练样本，模型 <5MB，推理 <10ms
    // 起步阶段不需要，关键词匹配已覆盖绝大多数场景
    if r.classifier != nil {
        intent, confidence := r.classifier.Classify(input)
        if confidence > 0.85 {
            return RoutingResult{Handler: "local", Action: r.templates[intent]}
        }
    }

    // 第四步：兜底交给 LLM
    return RoutingResult{Handler: "llm"}
}
```

### 3.1.2 Local Engine（本地执行引擎）

**本质：一组写死的 Go 函数 + 预置配置模板。没有模型，没有推理，全部是确定性逻辑。**

核心 Action 函数（每个 ~100-200 行 Go）：

```go
// Action: SwitchBestNode — "换个节点"、"这个不行了"
func SwitchBestNode(ctx *EngineContext) Result {
    nodes, _ := ctx.ClashAPI.GetProxies()
    // 并发测延迟
    results := parallelLatencyTest(nodes, ctx.ClashAPI)
    sort.Slice(results, func(i, j int) bool {
        return results[i].Latency < results[j].Latency
    })
    best := results[0]
    ctx.ClashAPI.SetProxy(ctx.CurrentGroup, best.Tag)
    return Result{
        Message: fmt.Sprintf("已切换到 %s（延迟 %dms）", best.Tag, best.Latency),
    }
}

// Action: ApplyTemplate — "帮我配 Netflix 分流"
func ApplyTemplate(ctx *EngineContext, templateID string) Result {
    tmpl := ctx.Templates.Get(templateID) // 读取预置 JSON 模板
    patch := tmpl.ToJSONPatch()           // 转为 Config Overlay 补丁
    // 走和 LLM Agent 完全相同的 Overlay 流程
    if conflict := ctx.ConflictResolver.Check(patch); conflict != nil {
        return Result{Message: "规则冲突：" + conflict.Detail, NeedConfirm: true}
    }
    ctx.ConfigOverlay.Apply(patch)
    return Result{Message: "Netflix 分流规则已启用"}
}

// Action: AutoFailover — 后台定时任务，非用户触发
func AutoFailover(ctx *EngineContext) {
    current := ctx.GetCurrentNode()
    // 滑动窗口：连续 3 次失败才判定故障
    failures := ctx.HealthTracker.GetRecentFailures(current.Tag, 3*time.Minute)
    if len(failures) < 3 {
        return
    }
    // 在同组找可用节点
    for _, candidate := range ctx.GetNodeGroup(current.GroupTag).SortedByLatency() {
        if candidate.Tag != current.Tag && ctx.ClashAPI.TestLatency(candidate.Tag) > 0 {
            ctx.ClashAPI.SetProxy(current.GroupTag, candidate.Tag)
            ctx.Notify(NotifyImmediate, fmt.Sprintf(
                "节点 %s 不可用，已切换到 %s（延迟 %dms）",
                current.Tag, candidate.Tag, candidate.Latency))
            return
        }
    }
    ctx.Notify(NotifyImmediate, "所有节点不可用，请检查网络或更新订阅")
}
```

完整 Action 列表：

| Action | 触发词示例 | 实现复杂度 |
|--------|-----------|-----------|
| `SwitchBestNode` | "换节点"、"不行了"、"太慢了" | 并发 ping + 排序 |
| `LatencyTestAll` | "测速"、"哪个快" | 并发 URL test |
| `ApplyTemplate` | "Netflix 规则"、"YouTube 分流" | 读模板 + Config Overlay |
| `SetMode` | "全局模式"、"直连"、"规则模式" | 切换 sing-box outbound |
| `AutoFailover` | (后台自动) | 滑动窗口 + 自动切换 |
| `ImportSubscription` | "导入订阅"、"添加节点" | 解析 Base64/JSON |
| `UpdateSubscription` | "更新订阅" | HTTP fetch + 解析 |
| `FilterNodes` | "日本节点"、"香港的" | 按 tag/地区过滤 |

### 3.1.3 预置模板库

**本质：一堆手写（或 LLM 离线批量生成后人工审核）的 JSON 文件。**

```
templates/
├── routing/
│   ├── streaming_netflix.json    # Netflix 域名 + 自动选节点
│   ├── streaming_youtube.json
│   ├── streaming_disney.json
│   ├── ai_services.json          # ChatGPT, Claude, Midjourney
│   ├── social_media.json         # Telegram, Twitter, Instagram
│   ├── gaming_low_latency.json
│   ├── dev_tools.json            # GitHub, npm, Docker Hub
│   └── china_direct.json         # 国内域名直连
├── chains/
│   ├── residential_banking.json  # 银行场景链式代理
│   └── double_hop_generic.json   # 通用双跳
└── dns/
    ├── leak_protection.json
    └── china_optimized.json
```

模板文件示例 (`streaming_netflix.json`)：

```json
{
  "name": "Netflix 流媒体解锁",
  "keywords": ["netflix", "奈飞", "网飞", "看剧", "流媒体"],
  "route_rules": [
    {
      "domain_suffix": [".netflix.com", ".nflxvideo.net", ".nflxso.net"],
      "outbound": "_tmpl:streaming-netflix"
    }
  ],
  "outbounds": [
    {
      "tag": "_tmpl:streaming-netflix",
      "type": "urltest",
      "outbounds": "__ALL_USER_NODES__",
      "url": "https://www.netflix.com/title/80018499",
      "interval": "10m"
    }
  ]
}
```

`__ALL_USER_NODES__` 是占位符，Apply 时替换为用户实际的节点列表。
`keywords` 字段直接供 Intent Router 做匹配。
模板跟 App 版本打包发布，或通过 App Group 共享目录做热更新。

### 3.1.4 代码量估算

```
Intent Router:      ~200-300 行 Go (关键词表 + 分词 + 打分)
Local Engine:       ~800-1200 行 Go (5-8 个 Action 函数)
Health Tracker:     ~200-300 行 Go (滑动窗口 + 故障计数 + 冷却期)
模板库:             ~20-30 个 JSON 文件 (手写或 LLM 离线生成)
─────────────────────────────────────────
总计: ~1500-2000 行 Go + JSON 模板
无模型、无训练、无推理。全部确定性逻辑。
```

### 3.1.5 LLM 成本控制机制

- **结果缓存**: 同类意图的 LLM 输出缓存复用（语义签名去重）
- **预生成模板**: 离线用 LLM 生成常见场景配置，内置到 App（定期更新）
- **批量推理**: 5-10 秒指令窗口攒批提交，减少 API roundtrip
- **模型分级**: 简单意图用便宜模型 (DeepSeek)，复杂编排用强模型 (Claude)

### 3.2 Config Overlay 模型（核心安全机制）

**原则：Agent 永远不直接修改用户配置，只生成增量补丁。**

```
最终生效配置 = Base Config (用户手动/订阅导入)
             ⊕ Agent Overlay (Agent 生成的增量补丁)
             经过 Conflict Resolver 合并
```

#### 配置分区规则

- **用户区 (User Zone)**：用户手动配置或订阅导入的 outbound、route rules、DNS 设置
- **Agent 区 (Agent Zone)**：Agent 创建的所有资源，tag 统一使用 `_agent:` 前缀
- **系统区 (System Zone)**：内核参数、TUN 配置、日志级别等，Agent 只读不写

#### 冲突检测三维度

1. **规则遮蔽 (Rule Shadowing)**：新规则与已有规则的匹配条件重叠 → 上报用户确认
2. **Tag 命名冲突**：`_agent:` 前缀强制隔离
3. **DNS 一致性**：Agent 修改路由时必须联动检查 DNS 规则，防止 DNS 泄露

#### 变更流程

```
Agent 生成 JSON Patch (RFC 6902)
  → Conflict Resolver 校验
    → 通过 → 保存快照 → Apply → Health Check
      → 健康 → Commit
      → 异常 → Rollback 到快照 → 上报原因
    → 冲突 → 阻断 → 向用户呈报冲突详情
```

---

## 四、Agent 运行时系统（借鉴 Claude Code 源码架构）

> 以下设计借鉴 tvytlx/ai-agent-deep-dive 对 Claude Code 源码的逆向研究。
> 核心洞察：成熟 Agent 的价值不是一段 prompt，而是把 prompt、tool、permission、
> agent、hook、context 统一起来的 **Agent Operating System**。

### 4.1 动态 System Prompt 编排（不是静态文本）

Claude Code 的 `getSystemPrompt()` 分为**静态前缀**（可缓存，降低 token 成本）和**动态后缀**（按会话条件注入）。NetPilot 的 Agent system prompt 也必须动态组装。

**实现：Go 中的 Prompt Assembly Engine**

```go
// PromptAssembler 按当前状态动态拼装 system prompt
type PromptAssembler struct {
    staticPrefix  string          // 编译时固定，可被 LLM API cache
    templates     *TemplateStore
    configStore   *ConfigStore
}

// Assemble 生成当前会话的完整 system prompt
func (pa *PromptAssembler) Assemble(ctx *SessionContext) string {
    var sections []string

    // ── 静态前缀（身份 + 基础规则，适合 prompt caching）──
    sections = append(sections, pa.staticPrefix)
    // 内容示例：
    // "你是 NetPilot 的网络管理 Agent。你通过 Tool 操作 sing-box 内核。
    //  你绝不直接修改用户配置，只在 Agent Overlay 区域操作。
    //  每次写操作前必须 snapshot，失败必须 rollback。"

    // ── 动态后缀（按会话实时注入）──

    // 1. 当前网络状态摘要
    sections = append(sections, pa.buildNetworkStateSection(ctx))
    // 内容示例：
    // "当前状态：代理已连接，活跃节点 hk-01（延迟 89ms），
    //  12 个节点可用，3 个超时。无异常。"

    // 2. 用户配置摘要
    sections = append(sections, pa.buildUserConfigSection(ctx))
    // 内容示例：
    // "用户配置：5 条自定义路由规则，2 条 Agent 生成规则。
    //  Agent Overlay 有 3 个活跃 outbound。"

    // 3. 当前可用 Tool 列表（按上下文裁剪）
    sections = append(sections, pa.buildAvailableToolsSection(ctx))
    // 如果代理未连接，移除 test_node_latency 等依赖出站的工具

    // 4. 活跃 Agent 角色约束
    sections = append(sections, pa.buildAgentRoleSection(ctx))
    // 如果当前是 DiagnoseAgent，注入 "你只负责诊断，不要修改配置"

    // 5. 降级状态提示
    if ctx.LLMTier != TierClaude {
        sections = append(sections, fmt.Sprintf(
            "注意：当前使用 %s 作为推理后端，能力可能有限。"+
            "复杂链式代理编排请提示用户切换到完整模式。", ctx.LLMTier))
    }

    return strings.Join(sections, "\n\n")
}

// buildNetworkStateSection 把 TrafficSnapshot 压缩为 prompt 友好的文本
func (pa *PromptAssembler) buildNetworkStateSection(ctx *SessionContext) string {
    snap := ctx.LatestSnapshot
    return fmt.Sprintf(`## 当前网络状态
- 代理状态: %s
- 活跃节点: %s (延迟 %dms)
- 可用节点: %d / 总节点: %d
- 异常: %s
- 活跃 Agent 规则: %d 条`,
        snap.ProxyStatus, snap.ActiveNode, snap.ActiveLatency,
        snap.AvailableNodes, snap.TotalNodes,
        snap.AnomalySummary(), snap.AgentRuleCount)
}
```

**为什么这样设计：**
- 静态前缀约占 prompt 的 60%，LLM API 的 prompt caching 可以大幅降低费用
- 动态后缀让 Agent 不用每次都调 `get_connections` 等 Tool 来"了解状况"——状态直接在 prompt 里
- 按角色裁剪 Tool 列表，防止 DiagnoseAgent 误调写操作

### 4.2 Tool Runtime Pipeline（工具不是裸调）

Claude Code 的工具调用走完整的 runtime pipeline，不是模型直接裸调函数。NetPilot 的每个 Tool 调用也必须经过这个管道：

```
LLM 输出 tool_use
    │
    ▼
┌─────────────────────┐
│ 1. Schema 校验       │  验证 input 参数符合 JSON Schema
│    validateInput()   │  类型错误 → 直接拒绝，返回错误给 LLM
└──────────┬──────────┘
           │
    ┌──────▼──────────┐
    │ 2. Pre-Tool Hook │  写操作专用：冲突检测 + 权限检查
    │                  │  - Conflict Resolver 扫描规则遮蔽
    │                  │  - 检查是否需要用户确认（渐进式信任）
    │                  │  - 注入额外上下文（如当前快照 ID）
    │                  │  返回值: ALLOW / DENY / ASK_USER
    └──────────┬──────┘
               │
        ┌──────▼──────┐
        │ 3. Permission│  如果 Pre-Hook 返回 ASK_USER
        │    Decision  │  → 暂停，向用户展示变更预览
        │              │  → 用户确认后继续 / 拒绝后中止
        └──────┬──────┘
               │
        ┌──────▼──────┐
        │ 4. Snapshot  │  写操作前自动保存配置快照
        │    (写操作)   │  快照 ID 记录到操作日志
        └──────┬──────┘
               │
        ┌──────▼──────┐
        │ 5. Tool 执行  │  实际调用 sing-box Clash API
        │    execute() │  HTTP PUT/GET/DELETE localhost
        └──────┬──────┘
               │
        ┌──────▼──────────┐
        │ 6. Post-Tool Hook│  执行后验证
        │                  │  - 写操作：自动触发 health check
        │                  │  - 检测 sing-box 是否报错
        │                  │  - 如果 health check 失败 → 自动 rollback
        └──────┬──────────┘
               │
        ┌──────▼──────┐
        │ 7. Telemetry │  记录操作日志
        │              │  - Tool 名、参数、结果、耗时
        │              │  - Agent 的推理原因（从 LLM 输出中提取）
        │              │  - 快照 ID（用于回滚追溯）
        └──────┬──────┘
               │
        ┌──────▼──────────┐
        │ 8. Failure Hook  │  如果 Tool 执行抛异常
        │    (异常专用)     │  - 自动 rollback 到最近快照
        │                  │  - 解析 sing-box 错误日志
        │                  │  - 构造结构化错误信息返回给 LLM
        └─────────────────┘
```

**Go 实现核心结构：**

```go
// ToolPipeline 管理所有 Tool 的执行管道
type ToolPipeline struct {
    tools       map[string]Tool          // 注册的 Tool 实现
    hooks       *HookEngine              // Hook 引擎
    permissions *PermissionManager       // 权限管理
    snapshots   *SnapshotStore           // 快照存储
    telemetry   *TelemetryLogger         // 操作日志
}

// Execute 是所有 Tool 调用的统一入口
func (p *ToolPipeline) Execute(ctx context.Context, call ToolCall) ToolResult {
    tool, ok := p.tools[call.Name]
    if !ok {
        return ToolResult{Error: fmt.Sprintf("unknown tool: %s", call.Name)}
    }

    // Step 1: Schema 校验
    if err := tool.ValidateInput(call.Input); err != nil {
        return ToolResult{Error: fmt.Sprintf("invalid input: %v", err)}
    }

    // Step 2: Pre-Tool Hook
    hookDecision := p.hooks.RunPreHooks(call)
    switch hookDecision.Action {
    case HookDeny:
        return ToolResult{Error: hookDecision.Reason}
    case HookAskUser:
        // 暂停执行，等待用户确认
        confirmed := p.permissions.RequestUserConfirmation(ctx, hookDecision.Preview)
        if !confirmed {
            return ToolResult{Error: "用户拒绝了此操作"}
        }
    }

    // Step 3: 写操作前自动快照
    var snapshotID string
    if tool.IsWriteOp() {
        snapshotID = p.snapshots.Save()
    }

    // Step 4: 执行
    result, err := tool.Execute(ctx, call.Input)

    // Step 5: 异常处理
    if err != nil {
        if snapshotID != "" {
            p.snapshots.Rollback(snapshotID)
        }
        // Failure Hook: 解析错误，构造结构化信息
        errInfo := p.hooks.RunFailureHooks(call, err)
        p.telemetry.Log(call, nil, err, snapshotID)
        return ToolResult{Error: errInfo.UserFriendlyMessage}
    }

    // Step 6: Post-Tool Hook (写操作自动 health check)
    if tool.IsWriteOp() {
        health := p.hooks.RunPostHooks(call, result)
        if !health.OK {
            p.snapshots.Rollback(snapshotID)
            p.telemetry.Log(call, result, health.Error, snapshotID)
            return ToolResult{
                Error: fmt.Sprintf("操作已执行但验证失败，已自动回滚。原因: %s", health.Reason),
            }
        }
    }

    // Step 7: 记录遥测
    p.telemetry.Log(call, result, nil, snapshotID)
    return *result
}
```

**Hook 的具体实现：**

```go
// HookEngine 管理 Pre/Post/Failure 三类 Hook
type HookEngine struct {
    preHooks     []PreToolHook
    postHooks    []PostToolHook
    failureHooks []FailureHook
}

// ConflictDetectionHook 是最重要的 Pre-Hook
type ConflictDetectionHook struct {
    resolver *ConflictResolver
}

func (h *ConflictDetectionHook) Run(call ToolCall) HookDecision {
    // 只对写操作生效
    if call.Name != "patch_outbound" && call.Name != "patch_route_rule" && call.Name != "patch_dns_rule" {
        return HookDecision{Action: HookAllow}
    }

    // 将 Tool input 转为 JSON Patch
    patch, _ := call.ToPatch()

    // 三维冲突检测
    conflicts := h.resolver.Check(patch)
    if len(conflicts) == 0 {
        return HookDecision{Action: HookAllow}
    }

    // 有冲突 → 构造预览让用户确认
    return HookDecision{
        Action:  HookAskUser,
        Reason:  fmt.Sprintf("检测到 %d 个潜在冲突", len(conflicts)),
        Preview: h.resolver.FormatConflictPreview(conflicts, patch),
    }
}

// HealthCheckHook 是最重要的 Post-Hook
type HealthCheckHook struct {
    clashAPI *ClashAPIClient
}

func (h *HealthCheckHook) Run(call ToolCall, result *ToolResult) HealthStatus {
    // 写操作后等 500ms 让 sing-box 重载
    time.Sleep(500 * time.Millisecond)

    // 检查 sing-box 是否有新的错误日志
    logs, _ := h.clashAPI.GetLogs("error", 5)
    if len(logs) > 0 {
        return HealthStatus{OK: false, Reason: "sing-box 报告错误: " + logs[0].Message}
    }

    // 如果是节点相关操作，测试延迟
    if call.Name == "patch_outbound" {
        tag := call.Input["tag"].(string)
        latency, err := h.clashAPI.TestLatency("_agent:"+tag, testURL)
        if err != nil || latency <= 0 {
            return HealthStatus{OK: false, Reason: "新节点延迟测试失败"}
        }
    }

    return HealthStatus{OK: true}
}
```

### 4.3 Agent 角色分工系统

Claude Code 内建了 Explore/Plan/Verification 等专用角色。NetPilot 也不应该让一个 Agent 既诊断又配置又验证。

**三个专用 Agent 角色：**

```go
// AgentRole 定义了 Agent 的角色约束
type AgentRole struct {
    Name            string
    SystemPromptExt string      // 注入到 system prompt 的角色约束
    AllowedTools    []string    // 该角色可以使用的 Tool 白名单
    Model           string      // 推荐使用的模型级别
}

var (
    // DiagnoseAgent: 只读，分析问题，不修改配置
    RoleDiagnose = AgentRole{
        Name: "diagnose",
        SystemPromptExt: `## 你的角色：诊断专家
你只负责分析网络问题，绝不修改任何配置。
你的输出是一份结构化诊断报告，包含：
- 问题描述
- 可能原因（按概率排序）
- 建议的修复方案（交给 ConfigureAgent 执行）
你可以读取日志、测试延迟、查看连接，但不能 patch 任何东西。`,
        AllowedTools: []string{
            "get_connections", "get_node_pool", "test_node_latency",
            "get_logs", "get_traffic_stats",
        },
        Model: "cheap", // 诊断用便宜模型即可
    }

    // ConfigureAgent: 可写，根据诊断结果或用户意图修改配置
    RoleConfigure = AgentRole{
        Name: "configure",
        SystemPromptExt: `## 你的角色：配置工程师
你根据诊断报告或用户意图来修改网络配置。
每次修改前必须 snapshot。
你的输出必须包含：你做了什么、为什么做、如何验证。
修改后交给 VerifyAgent 验证，你不要自己验证。`,
        AllowedTools: []string{
            "patch_outbound", "patch_route_rule", "patch_dns_rule",
            "snapshot", "rollback",
            "get_node_pool",  // 需要知道有哪些节点可选
        },
        Model: "strong", // 配置编排需要强推理
    }

    // VerifyAgent: 只读，验证配置变更是否生效且健康
    RoleVerify = AgentRole{
        Name: "verify",
        SystemPromptExt: `## 你的角色：验证专家
你的任务是验证刚才的配置变更是否成功。
检查项：
1. 目标链路是否连通（test_node_latency）
2. 流量是否已按预期路由（get_connections）
3. 有无新增错误日志（get_logs）
4. DNS 是否存在泄露风险
如果验证失败，输出失败原因并建议 rollback。
你绝不修改配置，只报告验证结果。`,
        AllowedTools: []string{
            "test_node_latency", "get_connections", "get_logs",
            "get_traffic_stats",
        },
        Model: "cheap", // 验证用便宜模型即可
    }
)
```

**Agent Orchestrator 实现：**

```go
// Orchestrator 协调多个 Agent 角色完成复杂任务
type Orchestrator struct {
    llmRouter   *LLMRouter
    pipeline    *ToolPipeline
    assembler   *PromptAssembler
}

// HandleComplexTask 处理需要 LLM 的复杂任务
// 流程: Diagnose → Configure → Verify → 报告
func (o *Orchestrator) HandleComplexTask(ctx context.Context, userMessage string) string {
    sessionCtx := o.buildSessionContext()

    // 第一步：判断是否需要先诊断
    // 如果用户主动要求配置（"帮我配链式代理"），跳过诊断
    // 如果用户描述问题（"连不上了"），先诊断
    needDiagnose := o.classifyNeedDiagnose(userMessage)

    var diagnosis string
    if needDiagnose {
        // ── Diagnose Phase ──
        diagnosis = o.runAgentRole(ctx, RoleDiagnose, sessionCtx,
            fmt.Sprintf("用户反馈: %s\n请分析问题并给出诊断报告。", userMessage))

        // 如果诊断结论是"不需要改配置"（如节点本身就是挂的），直接返回
        if o.diagnosisNeedsNoAction(diagnosis) {
            return diagnosis
        }
    }

    // ── Configure Phase ──
    configPrompt := userMessage
    if diagnosis != "" {
        configPrompt = fmt.Sprintf("用户请求: %s\n\n诊断报告:\n%s\n\n请根据以上信息执行配置变更。", 
            userMessage, diagnosis)
    }
    configResult := o.runAgentRole(ctx, RoleConfigure, sessionCtx, configPrompt)

    // ── Verify Phase ──
    verifyResult := o.runAgentRole(ctx, RoleVerify, sessionCtx,
        fmt.Sprintf("刚才执行了以下配置变更:\n%s\n\n请验证变更是否成功。", configResult))

    // 如果验证失败，自动 rollback
    if o.verificationFailed(verifyResult) {
        o.pipeline.Execute(ctx, ToolCall{Name: "rollback"})
        return fmt.Sprintf("配置变更验证失败，已自动回滚。\n诊断: %s\n验证报告: %s", 
            diagnosis, verifyResult)
    }

    return fmt.Sprintf("操作完成。\n%s", verifyResult)
}

// runAgentRole 用指定角色约束运行 Agent
func (o *Orchestrator) runAgentRole(
    ctx context.Context, role AgentRole, 
    sessionCtx *SessionContext, prompt string,
) string {
    // 动态组装 prompt，注入角色约束
    sessionCtx.ActiveRole = &role
    systemPrompt := o.assembler.Assemble(sessionCtx)

    // 构建受限的 Tool 列表（只暴露该角色允许的 Tool）
    tools := o.pipeline.GetToolSchemas(role.AllowedTools)

    // 选择模型
    model := o.llmRouter.SelectModel(role.Model)

    // LLM tool-use 循环
    messages := []Message{
        {Role: "user", Content: prompt},
    }

    for i := 0; i < MaxIterationsPerRole; i++ {
        resp := model.Complete(ctx, systemPrompt, messages, tools)

        if resp.StopReason == "end_turn" {
            return resp.TextContent()
        }

        // 处理 tool calls
        for _, toolCall := range resp.ToolUseCalls() {
            // 通过 Pipeline 执行（走完整的 hook/permission/telemetry）
            result := o.pipeline.Execute(ctx, toolCall)
            messages = append(messages, 
                Message{Role: "assistant", Content: resp.Raw},
                Message{Role: "tool", ToolCallID: toolCall.ID, Content: result.ToJSON()},
            )
        }
    }
    return "达到最大迭代次数，Agent 已停止。"
}
```

**为什么角色分工比单一 Agent 更好：**
- DiagnoseAgent 只有读权限，不可能误操作
- ConfigureAgent 的输出被 VerifyAgent 独立验证，形成制衡
- 每个角色的 prompt 更精简，LLM 推理更聚焦
- 可以对不同角色用不同模型：诊断和验证用便宜模型，配置编排用强模型

### 4.4 Tool 清单

| Tool | 类型 | 可用角色 | 作用 |
|------|------|---------|------|
| `patch_outbound` | 写 | Configure | 添加/修改 outbound 节点（自动加 `_agent:` 前缀） |
| `patch_route_rule` | 写 | Configure | 插入路由规则到 Agent 区域（含遮蔽检测） |
| `patch_dns_rule` | 写 | Configure | 插入 DNS 规则（与路由规则联动） |
| `get_connections` | 读 | Diagnose, Verify | 获取当前活跃连接（含 sniffing metadata） |
| `get_node_pool` | 读 | Diagnose, Configure | 获取所有可用节点及其状态 |
| `test_node_latency` | 读 | Diagnose, Verify | 对指定节点做延迟探测 |
| `get_logs` | 读 | Diagnose, Verify | 读取 sing-box 日志（可按级别过滤） |
| `get_traffic_stats` | 读 | Diagnose, Verify | 获取实时流量统计 |
| `rollback` | 写 | Configure | 回滚到上一次稳定配置快照 |
| `snapshot` | 写 | Configure | 主动保存当前配置快照 |

### 4.5 Agent 闭环全流程（三角色协作版）

```
用户输入 ("连不上了" / "帮我配链式代理")
    │
    ├── Intent Router 判定: 复杂意图 → 转交 LLM Orchestrator
    │
    ▼
┌─ DiagnoseAgent (只读) ──────────────────────────────┐
│  Tools: get_connections, get_logs, test_node_latency │
│  输出: 结构化诊断报告                                  │
│  "节点 jp-03 超时，连续失败 5 次。                     │
│   备选节点 jp-07 延迟 92ms 可用。                      │
│   建议: 切换到 jp-07 并更新路由规则。"                  │
└───────────────────────────────┬──────────────────────┘
                                │
                                ▼
┌─ ConfigureAgent (可写) ────────────────────────────────┐
│  Tools: patch_outbound, patch_route_rule, snapshot      │
│  执行: snapshot → patch_outbound(jp-07) →               │
│        patch_route_rule(银行域名→新链路)                  │
│  输出: "已将 exit 节点从 jp-03 切换到 jp-07,             │
│        银行域名路由已更新。快照 ID: snap-20260403-001"    │
└───────────────────────────────┬────────────────────────┘
                                │
                                ▼
┌─ VerifyAgent (只读) ──────────────────────────────────┐
│  Tools: test_node_latency, get_connections, get_logs   │
│  检查: 链路延迟 92ms ✓, 流量已切换 ✓, 无新错误 ✓        │
│  输出: "验证通过。新链路健康，延迟 92ms。"               │
│  (如果失败 → Orchestrator 自动 rollback)                │
└───────────────────────────────┬────────────────────────┘
                                │
                                ▼
                     返回给用户: 操作完成 ✅

---

## 五、意图转译流水线

### 自然语言 → sing-box 配置的四层转译

```
Layer 1: NLU → Intent Schema
  输入: "帮我找个干净的 IP 登录银行，入口要快"
  输出: { intent: "secure_browsing",
          constraints: { exit_ip: "residential", entry: "low_latency",
                        security: { dns_leak_protect: true } },
          scope: { target: "domain_group:banking" } }

Layer 2: Intent → Abstract Topology
  输出: User → [Entry: low_latency, nearby] → [Exit: residential, low_risk] → Target

Layer 3: Topology → sing-box JSON Patch
  输出: outbound 定义 + detour 关系 + route rules + dns rules

Layer 4: Patch → Validate → Apply → Verify
  输出: 生效的配置 + 健康检查结果
```

### Intent Schema 标准字段

```json
{
  "intent": "secure_browsing | streaming | download | gaming | general",
  "constraints": {
    "exit_ip": {
      "type": "residential | datacenter | any",
      "geo": "JP | US | auto_match_account",
      "risk_score": "low | medium | any"
    },
    "entry": {
      "optimize": "latency | bandwidth | stability"
    },
    "security": {
      "dns_leak_protect": true,
      "strict_routing": true
    }
  },
  "scope": {
    "target": "domain_group:banking | domain:example.com | process:app_name | all",
    "duration": "permanent | session | minutes:30"
  }
}
```

---

## 六、iOS 平台特殊考量

### 6.1 进程架构 (iOS — 基于 Hiddify 源码验证)

```
┌──────────────────────────────────────────────────┐
│             主进程 (Main App)                      │
│                                                    │
│  SwiftUI                                          │
│    ↕                                              │
│  Agent Core (Go HTTP server in-process)           │
│    ├── LLM Router                                 │
│    ├── Intent Router + Local Engine               │
│    ├── Tool Pipeline                              │
│    └── Traffic Semantics Engine                   │
│    ↕                                              │
│  LibboxNewStandaloneCommandClient                 │
│    (gRPC client → 连接 NE 进程的 gRPC server)     │
│    获取: stats, proxy groups, logs 实时数据        │
│                                                    │
│  MobileSetup(mode=4, port=17078) ← 主进程也初始化  │
└────────┬───────────────────────────────────────────┘
         │  IPC 四通道:
         │  1. startVPNTunnel(options:) — 启动时传配置
         │  2. sendProviderMessage()   — 运行时查流量
         │  3. App Group 共享文件系统   — 配置/日志/错误
         │  4. gRPC (127.0.0.1:17079)  — 实时 stats/groups/logs
         ↓
┌──────────────────────────────────────────────────┐
│          NE 进程 (PacketTunnel Extension)          │
│                                                    │
│  PacketTunnelProvider : NEPacketTunnelProvider      │
│                                                    │
│  startTunnel():                                    │
│    1. MobileSetup(opts, platformInterface)         │
│    2. LibboxSetMemoryLimit(true) — Go 软限 ~30MB   │
│    3. MobileStart(config, "")   — 启动 sing-box    │
│                                                    │
│  ExtensionPlatformInterface:                       │
│    • openTun() → setTunnelNetworkSettings          │
│    • 获取 TUN fd → 传给 libbox                     │
│    • clearDNSCache()                               │
│                                                    │
│  stopTunnel():                                     │
│    MobileClose(4)                                  │
│                                                    │
│  wake(): MobileWake()                              │
│  sleep(): 当前无操作 (MobilePause 已注释)          │
└──────────────────────────────────────────────────┘
```

### 6.2 iOS 特殊挑战与对策（基于 Hiddify 实测更新）

| 挑战 | Hiddify 的做法 | NetPilot 的策略 |
|------|---------------|----------------|
| NE 内存限制 50MB | `LibboxSetMemoryLimit(true)` Go 软限 ~30MB；禁用本地 GeoIP/GeoSite | 同样启用；远程 rule-set 替代本地文件 |
| 主进程 ↔ NE 通信 | gRPC + sendProviderMessage + App Group | 同方案；gRPC 用于实时数据，App Group 用于配置 |
| 网络切换 (Wi-Fi↔4G) | **直接躺平** — NWPathMonitor 代码已写但被禁用，依赖 sing-box 内核自身重连 | Phase 3 先同样依赖内核；Phase 4 加 Agent 主动检测 |
| 后台存活 | NE 常驻；主进程 wake() 调 MobileWake() | 同方案；Agent 逻辑只在前台活跃 |
| LLM API 不可达 | N/A (Hiddify 没有 AI) | **四级降级策略**（见 6.3） |
| App Store 审核 | 已通过，iOS 15.0+，Team ID 3JFTY5BP58 | 需要自己申请 NE entitlement |
| libbox 编译 | 使用私有 fork hiddify-core，输出 HiddifyCore.xcframework | 使用官方 sing-box gomobile bind，输出 Libbox.xcframework |

### 6.2.1 Entitlement 配置（参照 Hiddify 验证）

**主 App 需要：**
- `com.apple.developer.networking.networkextension`: packet-tunnel-provider, app-proxy-provider, dns-proxy
- `com.apple.developer.networking.vpn.api`: allow-vpn
- `com.apple.security.application-groups`: group.$(BUNDLE_ID)
- `com.apple.security.network.client`: true
- `com.apple.security.network.server`: true

**NE Extension 额外需要：**
- `content-filter-provider`（比主 App 多一个）

### 6.2.2 EngineAdapter 调整：gRPC 替代 Clash API

**关键发现：Hiddify 不用 Clash API HTTP，而是用 libbox 原生的 gRPC command client。**

libbox 提供了 `LibboxNewStandaloneCommandClient` 可以直接通过 gRPC 获取 stats、proxy groups、logs。这比我们之前设计的 Clash API HTTP 方案更好：不需要在 NE 内额外启动 HTTP server，减少内存开销；gRPC 是二进制协议，比 JSON over HTTP 更高效。

EngineAdapter 应支持两种后端模式：

```go
// Phase 1 CLI 原型：用 Clash API HTTP（简单、curl 可调试）
type ClashAPIAdapter struct { ... }

// Phase 3 iOS 集成：用 libbox 原生 gRPC（性能好、不占额外端口）
type LibboxGRPCAdapter struct { ... }

// 两者都实现 EngineAdapter interface，上层代码无需修改
```

### 6.3 中国大陆 LLM 可达性问题（致命 — 四级降级策略）

**核心矛盾：Agent 需要 LLM 来编排代理，但 LLM API 本身需要代理才能访问。**

sing-box 的 direct 规则和 NEPacketTunnelProvider 绕过在此场景下均无效——`api.anthropic.com` 在中国大陆被 GFW 阻断，直连根本不通。

**四级降级策略：**

```
Tier 0: Claude API via 代理 (完整能力)
  代理已通时的默认路径。走 sing-box 出站。
  ↓ 代理不可用时

Tier 1: 国内可达 LLM API (接近完整能力)
  方案 A: 自建国内中继 → 转发到 Anthropic API
  方案 B: DeepSeek / Qwen / GLM API (原生国内直连)
  要求: 必须支持 function calling / tool use
  注意: 需要针对不同模型适配 system prompt
  ↓ 网络完全不可用时

Tier 2: 本地规则引擎 (零 LLM 依赖)
  预编译意图模板库，覆盖高频操作:
  - 切换节点 / 测速 / 全局模式 / 直连模式
  - 导入订阅 / 手动添加节点
  本质上就是一个"传统代理客户端"的完整能力
  ↓ 用户需要复杂自然语言交互时

Tier 3: 本地意图分类 (轻量级)
  Apple NLClassifier 或 Core ML 模型 (<10MB)
  能力: 将自然语言映射到 Tier 2 的模板 ID
  不做复杂推理，不做 tool use
  复杂请求提示用户:"连接代理后可解锁完整 AI 能力"
```

**为什么不内置大参数本地模型：**
- App 体积膨胀 1GB+，App Store 下载意愿骤降
- 0.5B 模型的 tool use 可靠性极低，生成错误配置反而危险
- iOS 对内存极苛刻，推理时内存峰值容易触发 OOM
- 维护两套 prompt 适配（云端 + 本地模型）工程量翻倍

**LLM Router 实现：**

```go
type LLMRouter struct {
    tiers []LLMTier  // 按优先级排序
}

type LLMTier struct {
    Name        string        // "claude_proxy", "claude_relay", "deepseek", "local"
    Provider    LLMProvider
    HealthCheck func() bool   // 可达性探测
    Cooldown    time.Duration // 探测间隔
}

func (r *LLMRouter) Route(ctx context.Context, req ToolUseRequest) Response {
    for _, tier := range r.tiers {
        if tier.HealthCheck() {
            resp, err := tier.Provider.Complete(ctx, req)
            if err == nil {
                return resp
            }
        }
    }
    return r.localFallback(req)  // Tier 2 模板匹配
}
```

**冷启动用户流程（中国大陆）：**

```
用户首次打开 App
  ├── Agent 检测: 无代理, 海外 LLM 不可达
  ├── 自动进入 Tier 2 (本地规则引擎)
  │
  ▼
引导界面 (零网络依赖):
  ├── "粘贴订阅链接" (但订阅 URL 可能也被墙)
  ├── "从文件导入" (用户通过其他渠道获取配置)
  ├── "手动输入节点" (ss:// vmess:// trojan:// 链接)
  ├── "扫描二维码"
  ├── "剪贴板自动检测"
  │
  ▼
节点导入成功 → Tier 2 自动测速 → 选最快节点 → 连接建立 ✅
  │
  ├── 代理已通 → 尝试 Tier 0 (Claude API)
  │   └── 成功 → 完整 Agent 模式上线
  │
  └── Agent: "网络已连通。检测到 12 个节点，已按延迟排序。
              需要我帮您配置智能分流规则吗？"
```

### 6.4 NE 内存预算管理（基于 Hiddify 实测）

**Hiddify 的实际策略：**
1. `LibboxSetMemoryLimit(true)` — 硬编码启用，Go `debug.SetMemoryLimit()` 约 30MB 软限
2. 本地 GeoIP/GeoSite **已全部注释掉** — 改用远程 rule-set URL 按需加载
3. 默认 gVisor TUN 栈 — 内存占用较高 (~10-20MB 额外) 但兼容性好
4. **没有** `didReceiveMemoryWarning` 处理或内存监控逻辑
5. NE 进程大量功能被注释/禁用，保持尽量精简

**NetPilot 内存预算（优化版）：**

```
Go runtime + libbox 基础:      ~15-20MB
远程 rule-set 缓存:            ~2-5MB (按需加载，非全量)
活跃连接状态:                   ~3-5MB
TUN 栈 (mixed 模式):           ~5-10MB (比 gVisor 省 5-10MB)
─────────────────────────────
合计:                          ~25-40MB ✅ 在 50MB 限制内
```

**对策：**
- 必须调用 `debug.SetMemoryLimit(30MB)` — Go 侧
- 使用 mixed TUN 栈（TCP 走 system，UDP 走 gVisor）替代纯 gVisor
- 远程 rule-set 替代本地 geoip.db/geosite.db（一个 db 约 5-10MB）
- MTU 从默认 9000 降到 1500，减少缓冲区内存
- 添加 `os_proc_available_memory()` 监控，接近阈值时主动降级
- NE 进程只运行 sing-box 核心，所有 Agent/UI 逻辑在主进程

### 6.5 主进程 ↔ NE 的 IPC 方案（基于 Hiddify 实测更新）

**Hiddify 实际使用四通道：**

| 通道 | 用途 | 方向 | 时机 |
|------|------|------|------|
| `startVPNTunnel(options:)` | 传入 sing-box JSON 配置 + gRPC 端口 | 主→NE | 仅启动时一次 |
| `sendProviderMessage()` | 查询流量统计 (upload,download) | 主→NE→主 | 运行时按需 |
| App Group 共享文件系统 | 配置文件、日志、错误文件、UserDefaults | 双向 | 持续 |
| **gRPC (127.0.0.1:17079)** | **实时 stats、proxy groups、logs 流** | 主→NE | **核心数据通道** |

**未使用：** Darwin Notification、XPC Service、Clash API HTTP

**关键结论：gRPC 是 Hiddify 的核心数据通道。** libbox 原生提供 `LibboxNewStandaloneCommandClient`，主进程通过它订阅 NE 进程的实时数据，无需额外开 HTTP server。NetPilot 的 EngineAdapter 在 iOS 阶段应从 Clash API HTTP 切换到 libbox gRPC。

### 6.6 vs Surge / Shadowrocket 差距评估

**劣势（不可弥补但非核心）：**
- 无 MitM (HTTPS 解密) 能力 — sing-box 在 iOS 上不支持；但 Agent 场景不需要
- 无 HTTP Rewrite — 非核心需求
- 内存效率：Go runtime 固有开销比 Surge 的 C/ObjC 栈多 ~10-15MB

**劣势（可优化）：**
- 首包延迟 ~5-15ms vs Surge ~2-5ms — 用户无感知

**优势（核心差异化）：**
- 全能协议支持（sing-box 生态）
- Agent 自动编排链式代理（Surge 只能手动配）
- 自然语言交互
- Agent 自动故障检测 + 修复
- 内核开源可审计

**赛道判断：不是同一赛道。** Surge 卖极致性能和精细控制，面向懂网络的技术用户。NetPilot 卖智能和省心，面向不想手动配规则的用户。

### 6.7 关键工程风险

| 风险 | 严重度 | 状态 | 对策 |
|------|--------|------|------|
| NE Entitlement 申请 | 🔴 阻断级 | 待申请 | **立即申请**，审批周期不确定（1天-2周） |
| App Store 审核 | 🔴 阻断级 | 待评估 | 考虑非中国区上架 / TestFlight / 企业签名分发 |
| libbox iOS 成熟度 | 🟡 高风险 | 待验证 | 官方 SFI 已归档，需自行处理 gomobile 编译 + NE 生命周期 |
| Go ↔ Swift 桥接 | 🟡 中风险 | 已有方案 | 起步用 Go HTTP server + Swift HTTP client；稳定后切 gomobile bind |

---

## 七、流量感知与语义化引擎

### 7.1 sing-box Sniffing 可用数据

| 字段 | 来源 | Agent 用途 |
|------|------|-----------|
| `destination` | DNS 查询 / TLS SNI / HTTP Host | 知道用户在访问什么 |
| `protocol` | 协议识别 (HTTP/TLS/QUIC/BitTorrent...) | 判断流量类型 |
| `process_name` | 进程路径（Android 需权限） | 知道哪个 App 发起 |
| `upload/download` | 连接级字节计数 | 判断带宽消耗模式 |
| `start_time + duration` | 连接时间戳 | 长连接 vs 短请求 |
| `chain` | 实际经过的 outbound 链 | 验证链路是否按预期走 |

### 7.2 Traffic Semantics Engine（Go 实现，非 LLM）

**关键设计：语义化聚合在 Go 层完成，不调 LLM。LLM 只在异常或用户提问时介入。**

```
原始连接数据 (数百条/秒)
       │
       ▼
Traffic Semantics Engine (Go)
  ├── 按 process 分组
  ├── 按 destination domain 聚类
  ├── 按协议类型分类
  ├── 带宽 Top-N 排序
  ├── 异常检测 (连接失败率突增、延迟突变)
  │
  输出: TrafficSnapshot (每 N 秒一份)
  {
    "dominant_activity": "video_streaming",
    "top_bandwidth_app": "com.youtube",
    "active_chains": [{ "tag": "_agent:xx", "health": "ok" }],
    "anomalies": [{ "type": "high_fail_rate", "node": "jp-03", "rate": 0.34 }],
    "dns_leak_risk": false
  }
```

### 7.3 Agent 主动汇报触发器

```
硬触发 (IMMEDIATE → 立即通知 + 自动修复):
  - 当前使用中的节点不可用
  - 所有链路失败
  - 检测到 DNS 泄露

软触发 (DEFERRED → 下次打开 App 时汇报):
  - 大带宽场景切换 (开始/结束)
  - 发现显著更优节点
  - exit IP 风险评分升高

SILENT (仅记录日志):
  - 其他一切
```

**反 flapping 机制：**
- 冷却期：同一节点 5 分钟内最多触发一次切换
- 滑动窗口：连续 3 次探测失败才判定故障（非单次）
- 区分"降级"（延迟升高但可用）和"故障"（超时/不可达）

---

## 八、开发路线图（修订版）

> **原则：先在桌面 CLI 验证核心逻辑，再移植 iOS。不要被移动端工程问题淹没核心架构验证。**

### Phase 0: 概念验证 ← 当前
- [ ] 架构文档定稿 (本文档)
- [ ] 核心议题讨论完成 (4 个议题)
- [x] 确定语言栈 → Go
- [x] 确定目标平台 → iOS 优先
- [x] 确定 LLM 降级策略 → 四级降级
- [ ] **⚠️ 立即：申请 Apple Network Extension Entitlement**（阻断级前置依赖）
- [ ] **⚠️ 立即：确认 App 分发策略**（中国区 App Store / 非中国区 / TestFlight / 企业签名）

### Phase 1: 桌面 CLI 原型 (第 1-2 周)
- [ ] sing-box 跑在 Linux/Mac（直接二进制）
- [ ] Go 实现 Tool 层（封装 Clash API）
- [ ] Go 实现 Conflict Resolver + Config Overlay
- [ ] Go 实现 Intent Router（关键词匹配 + 模板库）
- [ ] Go 实现 Local Engine（节点管理、分流模板、自动测速）
- [ ] LLM Agent 主循环（Claude API tool-use 闭环，Intent Router 兜底时触发）
- [ ] CLI 交互端到端验证
- **验证标准 1：** "换个节点" → Local Engine 直接处理，零 LLM 调用
- **验证标准 2：** "帮我串联 HK+JP 节点访问银行" → Intent Router 判定为复杂意图 → LLM Agent 处理 → 配置生效

**Phase 1 核心痛点：LLM 响应延迟**
- 一次闭环 2-4 次 API roundtrip，每次 1-3s，总延迟 5-12s
- 对策：读操作并行注入上下文；Agent 单次输出完整 action plan；常见场景预构建模板直接匹配

### Phase 2: 流量感知层 (第 3-4 周)
- [ ] Traffic Semantics Engine 接入真实流量
- [ ] 异常检测逻辑（节点故障、DNS 泄露）
- [ ] Agent 主动修复闭环验证
- [ ] 汇报触发器调优
- **验证标准：** 手动断掉一个节点 → Agent 5 秒内检测并自动切换 + 输出解释

**Phase 2 核心痛点：异常检测误报 (flapping)**
- 节点延迟波动是常态，不能每次都触发切换
- 对策：冷却期 + 滑动窗口 + 降级/故障分级

### Phase 3: iOS 移植 (第 5-8 周)
- [ ] libbox 编译为 iOS framework（gomobile / 官方 apple library）— **建议 Phase 1 期间先做可行性验证**
- [ ] NEPacketTunnelProvider (Network Extension) 封装
- [ ] 主进程 ↔ NE 三通道 IPC (App Group + Clash API + sendProviderMessage)
- [ ] Go Agent Core 桥接方案：起步 Go HTTP server + Swift URLSession 调用
- [ ] LLM Router 四级降级策略实现
- [ ] 最小 UI (SwiftUI)：一个输入框 + 状态卡片
- [ ] 冷启动引导流程 (零网络依赖)
- [ ] NE 内存预算测试与规则集精简

**Phase 3 核心痛点：中国大陆 LLM 不可达**
- api.anthropic.com 被 GFW 阻断，direct 规则无效
- 对策：四级降级（代理直通 → 国内中继/国内模型 → 本地规则引擎 → 本地意图分类）

**Phase 3 额外痛点：Network Extension 内存限制**
- iOS NE 进程硬限约 50MB，sing-box + 大量规则可能逼近上限
- 对策：精简配置；Agent 逻辑全部留在主进程，NE 只跑内核

**Phase 3 额外痛点：App Store 审核**
- VPN 类 App 需要 Network Extension entitlement
- 需提前申请；审核周期不可控

### Phase 4: UI 观测性 (第 9+ 周)
- [ ] 流量可视化（Surge 级连接列表）
- [ ] Agent 操作历史时间线
- [ ] 节点地图 / 链路拓扑图
- [ ] 设置界面（信任级别、通知偏好）
- [ ] 节点质量评分系统（IP 风险、延迟历史）
- [ ] 用户行为学习（常用场景自动识别）

**Phase 4 核心痛点：实时渲染性能**
- 高流量时数百条连接/秒更新，直接绑定 SwiftUI 会卡死
- 对策：后端 500ms throttle 快照；UI 用 LazyVStack 虚拟滚动；详情按需加载

---

## 九、设计原则

1. **Agent 不是黑箱**：每次自动操作都必须可审计、可解释、可回滚
2. **用户配置神圣不可侵犯**：Agent 只在自己的 Overlay 区域操作
3. **失败安全 (Fail-safe)**：任何异常 → 回滚 → 上报，而不是继续尝试
4. **渐进式信任**：初期 Agent 的写操作需用户确认；随着信任建立可授权自动执行
5. **内核可替换性**：所有 sing-box 依赖收敛到 EngineAdapter 接口层，上层零直接引用（详见第十一章）
6. **本地优先 (Local-first)**：能在本地处理的绝不调 LLM；LLM 是增强层不是依赖项
7. **成本可控**：LLM 调用必须经过 Intent Router 过滤，每次调用都要有明确的用户价值

---

## 十、商业模型

### LLM 成本估算

| LLM 后端 | 每用户每月成本 | 说明 |
|----------|--------------|------|
| Claude Sonnet | ~$4.2 (¥30) | 日均 25K tokens，高成本 |
| DeepSeek | ~$0.4 (¥3) | 同等使用量，成本降一个量级 |
| 本地 Engine | $0 | 覆盖 95% 交互 |

### 收费模型（双轨并行）

```
轨道 A: 托管服务 (面向普通用户)
  ├── 基础版: 免费
  │   Local Engine 全部功能 + 每月 20 次 LLM 调用体验额度
  │
  ├── LLM 调用包: ¥9.9 / 100 次
  │   成本 (DeepSeek): ~¥1-2 → 毛利 ~80%
  │
  └── 无限 Agent: ¥29.9/月
      不限次数 (fair use 日均 50 次上限)
      成本 (DeepSeek): ~¥5-8/月 → 毛利 ~75%

轨道 B: 自带 API Key (面向技术用户)
  ├── App 免费或买断 ¥68
  │   Local Engine 全部功能
  │
  └── LLM: 用户填自己的 Key
      支持 Claude / DeepSeek / Qwen / OpenAI compatible
      你的成本: 零
```

---

## 十一、内核演进路线与 EngineAdapter 抽象层

### 11.1 为什么不能永远依赖 sing-box

| 风险 | 说明 | 严重度 |
|------|------|--------|
| 维护者风险 | sing-box 核心由 nekohasekai 一人维护，Clash 删库事件是前车之鉴 | 🔴 高 |
| iOS 集成深度不足 | 官方 SFI 已归档，libbox iOS 端成熟度低于 Android | 🟡 中 |
| 协议演进被动 | 新协议（如 AnyTLS）的支持速度取决于上游社区 | 🟡 中 |
| GPL-3.0 许可证 | 分发含 sing-box 的二进制理论上需要开源，企业化/被收购时可能成为法务问题 | 🟡 中 |
| Go runtime 开销 | GC 停顿造成延迟抖动；NE 50MB 内存限制下余量紧张 | 🟡 中 |

### 11.2 渐进式替换路线（不是推倒重写）

```
阶段 1 (现在 ~ 第 1-2 年): 100% sing-box
  ├── 全部能力依赖 sing-box + Clash API
  ├── 通过 EngineAdapter 接口隔离依赖
  └── 专注 Agent 智能层，不碰内核

阶段 2 (第 2-3 年，用户量起来后): sing-box + 自研路由引擎
  ├── sing-box 仍做协议层（SS/VMess/Trojan/VLESS...）
  ├── 路由匹配引擎用 Rust 重写
  │   原因：路由是 Agent 操作最频繁的模块，
  │   自研可原生支持 Overlay 分区、冲突检测、热更新
  │   （这些你现在是在 sing-box 外面包了一层）
  └── 替换为 RustRouterAdapter，sing-box 降级为协议执行器

阶段 3 (第 3-5 年，规模化后): 自研内核
  ├── 用 Rust 重写核心数据路径（packet 转发）
  │   Rust 无 GC → 延迟抖动消除 + iOS NE 内存从容
  ├── 协议实现逐个替换（从最常用的开始）
  ├── 仍兼容 sing-box 配置格式（用户零迁移成本）
  └── 完全自主可控，不受上游社区节奏制约
```

### 11.3 EngineAdapter 接口定义（当前就要实现）

**这是确保未来可替换的关键抽象层。所有上层代码（Tool Layer、Local Engine、Agent）只通过此接口与内核交互，禁止直接调用 libbox 或 Clash API。**

```go
// EngineAdapter 是网络内核的抽象接口
// 当前实现: SingBoxAdapter (libbox + Clash API)
// 将来替换: RustCoreAdapter (自研内核)
type EngineAdapter interface {
    // ── 生命周期 ──
    Start(configPath string) error
    Stop() error
    Reload() error          // 重载配置，不中断连接
    IsRunning() bool

    // ── 配置管理 ──
    GetCurrentConfig() ([]byte, error)
    PatchConfig(patch JSONPatch) error      // 增量修改
    ReplaceConfig(config []byte) error      // 全量替换（慎用）
    ValidateConfig(config []byte) error     // 只校验不应用

    // ── 节点操作 ──
    GetProxies() ([]ProxyInfo, error)
    GetProxyGroup(groupTag string) (*ProxyGroup, error)
    SetActiveProxy(groupTag, proxyTag string) error
    TestLatency(proxyTag string, url string, timeout time.Duration) (ms int, err error)
    TestLatencyBatch(tags []string, url string, timeout time.Duration) ([]LatencyResult, error)

    // ── 连接与流量 ──
    GetConnections() ([]ConnectionInfo, error)
    CloseConnection(id string) error
    GetTrafficStats() (*TrafficStats, error)

    // ── 日志 ──
    GetLogs(level string, lines int) ([]LogEntry, error)
    SubscribeLogs(level string) (<-chan LogEntry, func())  // 流式日志 + 取消函数

    // ── DNS ──
    QueryDNS(domain string) (*DNSResult, error)

    // ── 网络事件 ──
    OnNetworkChanged()      // 通知内核网络环境变化（Wi-Fi↔4G）
}

// ProxyInfo 节点信息
type ProxyInfo struct {
    Tag      string `json:"tag"`
    Type     string `json:"type"`      // shadowsocks, vmess, trojan...
    Server   string `json:"server"`
    Port     int    `json:"port"`
    Alive    bool   `json:"alive"`
    Latency  int    `json:"latency"`   // ms, -1 = 未测试, 0 = 超时
    GroupTag string `json:"group_tag"` // 所属分组
}

// ConnectionInfo 连接信息（含 sniffing metadata）
type ConnectionInfo struct {
    ID          string    `json:"id"`
    Destination string    `json:"destination"`   // 嗅探到的域名或 IP
    Protocol    string    `json:"protocol"`      // HTTP, TLS, QUIC, BitTorrent...
    ProcessName string    `json:"process_name"`  // 发起进程（需权限）
    Upload      int64     `json:"upload"`         // bytes
    Download    int64     `json:"download"`       // bytes
    StartTime   time.Time `json:"start_time"`
    Chain       []string  `json:"chain"`          // 经过的 outbound 链
}
```

### 11.4 SingBoxAdapter 当前实现

```go
// SingBoxAdapter 通过 Clash API 与 sing-box 交互
type SingBoxAdapter struct {
    clashAPIBase string       // "http://127.0.0.1:9090"
    httpClient   *http.Client
    configPath   string
    mu           sync.RWMutex
}

func NewSingBoxAdapter(clashAPIBase string, configPath string) *SingBoxAdapter {
    return &SingBoxAdapter{
        clashAPIBase: clashAPIBase,
        configPath:   configPath,
        httpClient:   &http.Client{Timeout: 5 * time.Second},
    }
}

func (a *SingBoxAdapter) GetProxies() ([]ProxyInfo, error) {
    resp, err := a.httpClient.Get(a.clashAPIBase + "/proxies")
    if err != nil {
        return nil, fmt.Errorf("clash API unreachable: %w", err)
    }
    defer resp.Body.Close()
    // 解析 Clash API 响应并映射到 ProxyInfo
    var raw map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&raw)
    // ... 映射逻辑
    return proxies, nil
}

func (a *SingBoxAdapter) TestLatency(proxyTag string, url string, timeout time.Duration) (int, error) {
    reqURL := fmt.Sprintf("%s/proxies/%s/delay?url=%s&timeout=%d",
        a.clashAPIBase, proxyTag, url, timeout.Milliseconds())
    resp, err := a.httpClient.Get(reqURL)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()
    var result struct{ Delay int `json:"delay"` }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Delay, nil
}

func (a *SingBoxAdapter) OnNetworkChanged() {
    // 通知 sing-box 重建网络接口
    // libbox 提供了 NetworkExtensionReset() 方法
    // 或通过 Clash API 触发重载
    a.Reload()
}

// ... 其他方法实现类似，全部通过 Clash API HTTP 调用
```

### 11.5 架构约束规则

```
✅ 允许:
   Tool Layer → EngineAdapter interface → (具体实现)
   Local Engine → EngineAdapter interface → (具体实现)

❌ 禁止:
   Tool Layer → 直接 import libbox
   Tool Layer → 直接 HTTP 调用 localhost:9090
   Agent Core → 任何 sing-box 特有的数据结构

所有 sing-box 特有的类型（如 Clash API 的 JSON 格式）
必须在 SingBoxAdapter 内部完成转换，
上层只看到 EngineAdapter 定义的通用类型。
```

### 11.6 iOS 边界情况处理（在 Adapter 层封装）

```go
// SingBoxAdapter 的 iOS 特化处理（编译时通过 build tags 区分）

// +build ios

func (a *SingBoxAdapter) setupiOSHandlers() {
    // 1. 网络切换处理
    // NEPacketTunnelProvider 的 Swift 层检测到网络变化后
    // 通过 gomobile 桥调用此方法
    // → sing-box 重建连接池

    // 2. 低电量模式
    // Swift 层监听 NSProcessInfoPowerStateDidChange
    // 通知 Go 层降低 health check 频率

    // 3. NE 进程重启恢复
    // 从 App Group UserDefaults 读取上次的 Overlay 配置
    // 重新应用到 sing-box
}

// 内存预算监控
func (a *SingBoxAdapter) GetMemoryUsage() uint64 {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    return m.Alloc  // 如果接近 45MB 阈值，触发规则精简
}
```

---

## 十二、参考资源

- [sing-box 官方文档](https://sing-box.sagernet.org/)
- [sing-box Clash API](https://sing-box.sagernet.org/configuration/experimental/clash-api/)
- [Anthropic Tool Use 文档](https://docs.anthropic.com/en/docs/build-with-claude/tool-use)
- [Claude Code 源码](https://github.com/anthropics/claude-code) — 架构模式参考
- [tvytlx/ai-agent-deep-dive](https://github.com/tvytlx/ai-agent-deep-dive) — Claude Code 源码逆向研究报告，Agent OS 架构分析
- [tvytlx/claude-code-deep-dive](https://github.com/tvytlx/claude-code-deep-dive) — Claude Code prompt/tool/agent 调度链深挖
- [affaan-m/everything-claude-code](https://github.com/affaan-m/everything-claude-code) — Claude Code 配置系统最佳实践
- [RFC 6902 - JSON Patch](https://datatracker.ietf.org/doc/html/rfc6902)

---

*最后更新: 2026-04-06 (第九轮 — Hiddify 源码分析整合) | Phase 0 — 架构讨论中*
