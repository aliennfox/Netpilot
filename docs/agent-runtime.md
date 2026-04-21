## 四、Agent 运行时系统（借鉴 Claude Code 源码架构）

> 以下设计借鉴 tvytlx/ai-agent-deep-dive 对 Claude Code 源码的逆向研究。
> 核心洞察：成熟 Agent 的价值不是一段 prompt，而是把 prompt、tool、permission、
> agent、hook、context 统一起来的 **Agent Operating System**。

### 4.1 动态 System Prompt 编排（不是静态文本）

Claude Code 的 `getSystemPrompt()` 分为**静态前缀**（可缓存，降低 token 成本）和**动态后缀**（按会话条件注入）。Pilotty 的 Agent system prompt 也必须动态组装。

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
    // "你是 Pilotty 的网络管理 Agent。你通过 Tool 操作 sing-box 内核。
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

Claude Code 的工具调用走完整的 runtime pipeline，不是模型直接裸调函数。Pilotty 的每个 Tool 调用也必须经过这个管道：

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

Claude Code 内建了 Explore/Plan/Verification 等专用角色。Pilotty 也不应该让一个 Agent 既诊断又配置又验证。

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

