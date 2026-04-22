package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/foxnetpilot/netpilot/internal/tool"
)

// ToolEvent 是一次 tool 调用的可观测快照, 供 D3 Chat Tool-Call Timeline UI 渲染。
// OutputPreview 和 Error 各限 200 字节, 完整 output 只存 telemetry 磁盘。
type ToolEvent struct {
	Name          string `json:"name"`
	ArgsSummary   string `json:"args_summary,omitempty"`
	DurationMs    int64  `json:"duration_ms"`
	OutputPreview string `json:"output_preview,omitempty"`
	Error         string `json:"error,omitempty"`
	Role          string `json:"role,omitempty"` // Diagnose / Configure / Verify
}

// Result 是 Orchestrator.Run 的完整输出, 含自然语言回复 + 工具调用事件流。
// Kotlin 侧解析后, Reply 放到 Chat 气泡, Events 渲染成可折叠 timeline。
type Result struct {
	Reply  string      `json:"reply"`
	Events []ToolEvent `json:"events,omitempty"`
}

// Orchestrator 协调多角色 Agent 完成任务。
// 根据 ClassifyTask 的结果决定走哪些角色，按 Diagnose → Configure → Verify 顺序执行。
type Orchestrator struct {
	agent    *SingleAgent
	pipeline *tool.ToolPipeline
}

func NewOrchestrator(llm *LLMClient, pipeline *tool.ToolPipeline, assembler *PromptAssembler, tools map[string]*tool.ToolDef) *Orchestrator {
	// 每个角色最多 5 次迭代
	agent := NewSingleAgent(llm, pipeline, assembler, tools, 5)
	return &Orchestrator{
		agent:    agent,
		pipeline: pipeline,
	}
}

// Run 执行编排：分类 → 按需走角色流水线。history 可为 nil（无上下文）。
// 返回 Result.Events 记录跨阶段 tool 调用流水, Kotlin 侧据此渲染 timeline。
func (o *Orchestrator) Run(ctx context.Context, userMessage string, history *ConversationHistory) (Result, error) {
	plan := ClassifyTask(userMessage)

	fmt.Printf("\033[90m[分类: %s]\033[0m\n", plan.Reason)

	// 快速路径：只需要一个角色
	if plan.NeedDiagnose && !plan.NeedConfigure && !plan.NeedVerify {
		return o.runDiagnoseOnly(ctx, userMessage, history)
	}
	if plan.NeedConfigure && !plan.NeedDiagnose && !plan.NeedVerify {
		return o.runConfigureOnly(ctx, userMessage, history)
	}

	// 多角色流水线
	var diagnosis, configResult, verification string
	var allEvents []ToolEvent
	var err error

	// Diagnose 阶段
	if plan.NeedDiagnose {
		fmt.Print("\033[36m🔍 [诊断中...]\033[0m\n")
		var events []ToolEvent
		diagnosis, events, err = o.agent.RunWithRole(ctx, RoleDiagnose, userMessage, history)
		allEvents = append(allEvents, events...)
		if err != nil {
			return Result{Events: allEvents}, fmt.Errorf("诊断阶段失败: %w", err)
		}
		fmt.Printf("\033[36m🔍 诊断完成\033[0m\n")
	}

	// Configure 阶段
	if plan.NeedConfigure {
		fmt.Print("\033[36m🔧 [配置中...]\033[0m\n")

		configInput := userMessage
		if diagnosis != "" {
			configInput = fmt.Sprintf("用户请求: %s\n\n诊断报告:\n%s", userMessage, diagnosis)
		}

		var events []ToolEvent
		configResult, events, err = o.agent.RunWithRole(ctx, RoleConfigure, configInput, history)
		allEvents = append(allEvents, events...)
		if err != nil {
			return Result{Events: allEvents}, fmt.Errorf("配置阶段失败: %w", err)
		}
		fmt.Printf("\033[36m🔧 配置完成\033[0m\n")
	}

	// Verify 阶段（只要 plan 需要验证就跑，不管 configResult 是否为空）
	if plan.NeedVerify {
		fmt.Print("\033[36m✅ [验证中...]\033[0m\n")

		verifyInput := "请验证最近的配置操作是否成功。检查节点状态、路由规则和日志。"
		if configResult != "" {
			verifyInput = fmt.Sprintf("请验证以下配置操作是否成功:\n%s", configResult)
		}
		var events []ToolEvent
		verification, events, err = o.agent.RunWithRole(ctx, RoleVerify, verifyInput, history)
		allEvents = append(allEvents, events...)
		if err != nil {
			return Result{Events: allEvents}, fmt.Errorf("验证阶段失败: %w", err)
		}

		// 验证失败 → 自动回滚 (#H2 修复: 原先调 pipeline.Execute("rollback", nil) 会进入占位 tool,
		// 实际回滚 snapshot 必须走 ManualRollback)
		if isVerificationFailed(verification) {
			rollbackResult := o.pipeline.ManualRollback("")
			if rollbackResult != nil && rollbackResult.Success {
				verification += "\n\n⚠️ 验证失败，已自动回滚到之前状态。"
			}
		}
		fmt.Printf("\033[36m✅ 验证完成\033[0m\n")
	}

	return Result{
		Reply:  formatFinalResponse(plan, diagnosis, configResult, verification),
		Events: allEvents,
	}, nil
}

// OrchestratorSink 接收 RunStream 过程中的阶段级事件 (Phase 7.3)。
// mobile.Client 将其包装后把事件 JSON 推给 Kotlin callback。
type OrchestratorSink interface {
	// OnPhaseStart 在每个 role 阶段开始前触发 (Diagnose / Configure / Verify)。
	OnPhaseStart(role string)
	// OnText 在当前阶段 LLM content delta 到达时触发。 多阶段流水中同一 stream 期只有
	// 一个阶段在推 text, UI 可以靠 role 区分属于哪段。
	OnText(role string, delta string)
	// OnToolStart / OnToolEnd 用于 Chat tool-call timeline 的实时显示。
	OnToolStart(evt ToolEvent)
	OnToolEnd(evt ToolEvent)
	// OnPhaseEnd 在每个 role 阶段输出收敛后触发, summary 是该阶段的完整回复 (已去 ANSI)。
	OnPhaseEnd(role string, summary string)
}

// nopOrchestratorSink 用于非流式调用路径或测试。
type NopOrchestratorSink struct{}

func (NopOrchestratorSink) OnPhaseStart(string)       {}
func (NopOrchestratorSink) OnText(string, string)     {}
func (NopOrchestratorSink) OnToolStart(ToolEvent)     {}
func (NopOrchestratorSink) OnToolEnd(ToolEvent)       {}
func (NopOrchestratorSink) OnPhaseEnd(string, string) {}

// phaseSink 把 OrchestratorSink 包装成 SingleAgent.StreamSink, 附带当前 role 名。
type phaseSink struct {
	role  string
	outer OrchestratorSink
}

func (p phaseSink) OnText(delta string)     { p.outer.OnText(p.role, delta) }
func (p phaseSink) OnToolStart(e ToolEvent) { p.outer.OnToolStart(e) }
func (p phaseSink) OnToolEnd(e ToolEvent)   { p.outer.OnToolEnd(e) }

// RunStream 是 Run 的流式版本, 每阶段通过 sink 实时推送 text/tool 事件。
// 返回同一个 Result, 调用方可用于持久化或 logging。
func (o *Orchestrator) RunStream(ctx context.Context, userMessage string, history *ConversationHistory, sink OrchestratorSink) (Result, error) {
	if sink == nil {
		sink = NopOrchestratorSink{}
	}
	plan := ClassifyTask(userMessage)

	// 单角色快速路径
	if plan.NeedDiagnose && !plan.NeedConfigure && !plan.NeedVerify {
		return o.runPhaseStream(ctx, RoleDiagnose, userMessage, history, sink)
	}
	if plan.NeedConfigure && !plan.NeedDiagnose && !plan.NeedVerify {
		return o.runPhaseStream(ctx, RoleConfigure, userMessage, history, sink)
	}

	// 多角色流水线
	var diagnosis, configResult, verification string
	var allEvents []ToolEvent

	if plan.NeedDiagnose {
		sink.OnPhaseStart(RoleDiagnose.Name)
		reply, events, err := o.agent.RunWithRoleStream(ctx, RoleDiagnose, userMessage, history, phaseSink{role: RoleDiagnose.Name, outer: sink})
		allEvents = append(allEvents, events...)
		if err != nil {
			return Result{Events: allEvents}, fmt.Errorf("诊断阶段失败: %w", err)
		}
		diagnosis = reply
		sink.OnPhaseEnd(RoleDiagnose.Name, reply)
	}

	if plan.NeedConfigure {
		sink.OnPhaseStart(RoleConfigure.Name)
		configInput := userMessage
		if diagnosis != "" {
			configInput = fmt.Sprintf("用户请求: %s\n\n诊断报告:\n%s", userMessage, diagnosis)
		}
		reply, events, err := o.agent.RunWithRoleStream(ctx, RoleConfigure, configInput, history, phaseSink{role: RoleConfigure.Name, outer: sink})
		allEvents = append(allEvents, events...)
		if err != nil {
			return Result{Events: allEvents}, fmt.Errorf("配置阶段失败: %w", err)
		}
		configResult = reply
		sink.OnPhaseEnd(RoleConfigure.Name, reply)
	}

	if plan.NeedVerify {
		sink.OnPhaseStart(RoleVerify.Name)
		verifyInput := "请验证最近的配置操作是否成功。检查节点状态、路由规则和日志。"
		if configResult != "" {
			verifyInput = fmt.Sprintf("请验证以下配置操作是否成功:\n%s", configResult)
		}
		reply, events, err := o.agent.RunWithRoleStream(ctx, RoleVerify, verifyInput, history, phaseSink{role: RoleVerify.Name, outer: sink})
		allEvents = append(allEvents, events...)
		if err != nil {
			return Result{Events: allEvents}, fmt.Errorf("验证阶段失败: %w", err)
		}
		verification = reply
		if isVerificationFailed(verification) {
			rollbackResult := o.pipeline.ManualRollback("")
			if rollbackResult != nil && rollbackResult.Success {
				verification += "\n\n⚠️ 验证失败，已自动回滚到之前状态。"
			}
		}
		sink.OnPhaseEnd(RoleVerify.Name, verification)
	}

	return Result{
		Reply:  formatFinalResponse(plan, diagnosis, configResult, verification),
		Events: allEvents,
	}, nil
}

// runPhaseStream 单角色快速路径的流式版本。
func (o *Orchestrator) runPhaseStream(ctx context.Context, role *AgentRole, userMessage string, history *ConversationHistory, sink OrchestratorSink) (Result, error) {
	sink.OnPhaseStart(role.Name)
	reply, events, err := o.agent.RunWithRoleStream(ctx, role, userMessage, history, phaseSink{role: role.Name, outer: sink})
	if err != nil {
		return Result{Events: events}, err
	}
	sink.OnPhaseEnd(role.Name, reply)
	return Result{Reply: reply, Events: events}, nil
}

// runDiagnoseOnly 快速路径：只跑诊断
func (o *Orchestrator) runDiagnoseOnly(ctx context.Context, userMessage string, history *ConversationHistory) (Result, error) {
	fmt.Print("\033[36m🔍 [诊断中...]\033[0m\n")
	reply, events, err := o.agent.RunWithRole(ctx, RoleDiagnose, userMessage, history)
	if err != nil {
		return Result{Events: events}, err
	}
	return Result{Reply: reply, Events: events}, nil
}

// runConfigureOnly 快速路径：只跑配置（无验证）
func (o *Orchestrator) runConfigureOnly(ctx context.Context, userMessage string, history *ConversationHistory) (Result, error) {
	fmt.Print("\033[36m🔧 [配置中...]\033[0m\n")
	reply, events, err := o.agent.RunWithRole(ctx, RoleConfigure, userMessage, history)
	if err != nil {
		return Result{Events: events}, err
	}
	return Result{Reply: reply, Events: events}, nil
}

// isVerificationFailed 判断验证结果是否包含失败标记
func isVerificationFailed(verification string) bool {
	lower := strings.ToLower(verification)
	failKeywords := []string{
		"验证失败", "失败", "未生效", "不成功",
		"verification failed", "failed",
		"未通过", "不通过",
	}
	passKeywords := []string{
		"验证通过", "通过", "成功", "已生效",
		"verification passed", "passed",
	}

	hasPass := false
	hasFail := false
	for _, kw := range failKeywords {
		if strings.Contains(lower, kw) {
			hasFail = true
		}
	}
	for _, kw := range passKeywords {
		if strings.Contains(lower, kw) {
			hasPass = true
		}
	}

	// 如果同时包含通过和失败，以失败为准（保守策略）
	if hasFail && !hasPass {
		return true
	}
	return false
}

// formatFinalResponse 组装多阶段最终回复
func formatFinalResponse(plan TaskPlan, diagnosis, configResult, verification string) string {
	var parts []string

	if diagnosis != "" {
		parts = append(parts, diagnosis)
	}
	if configResult != "" {
		parts = append(parts, configResult)
	}
	if verification != "" {
		parts = append(parts, verification)
	}

	if len(parts) == 0 {
		return "操作完成，无额外输出。"
	}

	// 单阶段直接返回
	if len(parts) == 1 {
		return parts[0]
	}

	// 多阶段用分隔符拼接
	var result strings.Builder
	if diagnosis != "" {
		result.WriteString("📋 诊断报告:\n")
		result.WriteString(diagnosis)
		result.WriteString("\n\n")
	}
	if configResult != "" {
		result.WriteString("🔧 配置结果:\n")
		result.WriteString(configResult)
		result.WriteString("\n\n")
	}
	if verification != "" {
		result.WriteString("✅ 验证结果:\n")
		result.WriteString(verification)
	}

	return strings.TrimSpace(result.String())
}
