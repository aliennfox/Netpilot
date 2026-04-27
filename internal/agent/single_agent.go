package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/foxnetpilot/netpilot/internal/config"
	"github.com/foxnetpilot/netpilot/internal/tool"
)

var debugAgent = os.Getenv("NETPILOT_DEBUG") != ""

// toolFailureCircuitBreakThreshold #M27: 同一 tool 在一次 RunWithRole 内累计失败达此次数
// 即立即熔断, 不再喂回 LLM 让它继续试。 避免 "Agent 在折腾但毫无进展" 的用户感知 ——
// maxIter 是无限循环兜底 (5 轮全部不同 tool 也会触发), 这个是 "同一坏 tool 反复试" 兜底。
// 阈值 2 是经验值: 留 1 次容错给 LLM 自己改参数 / 换法子, 第 2 次还失败说明该 tool 当前
// 真有问题, 跑下去只是浪费 LLM token + 拖延用户。
const toolFailureCircuitBreakThreshold = 2

// shouldCircuitBreak 检查是否有任意 tool 失败次数达到熔断阈值。 返回 (toolName, lastError, true)
// 当任一 tool 触发, 否则 ("", "", false)。 同时触发多个时返回最早超阈的那个 (map 迭代顺序无关
// — 行为上等价: 反正都已经超了)。
func shouldCircuitBreak(failCount map[string]int, lastErr map[string]string) (string, string, bool) {
	for name, n := range failCount {
		if n >= toolFailureCircuitBreakThreshold {
			return name, lastErr[name], true
		}
	}
	return "", "", false
}

// formatCircuitBreakReply 熔断时给用户的友好回复。 含 tool 名 + 阈值 + 最后一次错误摘要。
func formatCircuitBreakReply(toolName, lastErr string) string {
	if lastErr == "" {
		return fmt.Sprintf("工具 %s 连续失败 %d 次, 已停止自动重试 (避免无效循环)。", toolName, toolFailureCircuitBreakThreshold)
	}
	return fmt.Sprintf("工具 %s 连续失败 %d 次, 已停止自动重试 (避免无效循环)。最后错误: %s",
		toolName, toolFailureCircuitBreakThreshold, lastErr)
}

// formatToolFailure 把 tool 失败包成给 LLM 看的结构化错误 + 自纠提示
// 参考 Claude Code toolExecution.ts 的 corrective context 模式: 失败不是终点,
// 是给 LLM 的"下一步该做什么"hint。 失败回灌 (error + params received + next-step
// suggestion) 比裸 error message 显著降低反复幻觉的频率。
func formatToolFailure(toolName string, params map[string]interface{}, errMsg, hint string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "[%s] 失败: %s", toolName, errMsg)
	if len(params) > 0 {
		if b, err := json.Marshal(params); err == nil && len(b) > 0 {
			fmt.Fprintf(&sb, "\n  入参: %s", truncateResult(string(b), 200))
		}
	}
	if hint != "" {
		fmt.Fprintf(&sb, "\n  下一步建议: %s", hint)
	}
	return sb.String()
}

// suggestRecovery 根据错误文本启发式给出下一步建议。 关键词匹配, 中英文都覆盖。
// 这层 hint 是给 LLM 看的, 不是给用户的; 用 imperative 语气让 LLM 直接照做。
func suggestRecovery(errMsg string) string {
	e := strings.ToLower(errMsg)
	switch {
	case (strings.Contains(errMsg, "节点") && (strings.Contains(errMsg, "不存在") || strings.Contains(errMsg, "未找到"))) ||
		(strings.Contains(e, "node") && (strings.Contains(e, "not found") || strings.Contains(e, "unknown"))):
		return "先调用 get_node_pool 看准确的节点 tag 列表, 再用精确名重试"
	case (strings.Contains(errMsg, "vpn") || strings.Contains(e, "vpn")) &&
		(strings.Contains(errMsg, "未启动") || strings.Contains(e, "not running") || strings.Contains(e, "not started")):
		return "先调 start_vpn 启动 VPN 隧道, 等就绪后再调本工具"
	case strings.Contains(errMsg, "权限") || strings.Contains(e, "permission denied") || strings.Contains(e, "forbidden"):
		return "此操作需要 Configure 角色; 让 orchestrator 委派或换只读 tool"
	case strings.Contains(errMsg, "参数") || strings.Contains(e, "missing field") || strings.Contains(e, "invalid param") || strings.Contains(e, "required"):
		return "params schema 有缺失或类型错的字段; 对照 tool 定义补全后重试"
	case strings.Contains(errMsg, "订阅") || strings.Contains(e, "subscription") && strings.Contains(e, "not found"):
		return "先调 list_subscriptions 看可用订阅 name, 再用精确名重试"
	case strings.Contains(errMsg, "超时") || strings.Contains(e, "timeout") || strings.Contains(e, "deadline"):
		return "上游响应超时; 不要立即 retry 同一调用, 改换查询型 tool 收集更多状态"
	default:
		return "如错误持续, 先调相关查询 tool (list_*/ get_*) 确认前置状态后再重试; 不要重复同样参数"
	}
}

// formatUnknownTool 全局未知 tool 名: LLM 调了不在 registry 里的 tool。
// 参考 Claude Code buildSchemaNotSentHint 模式 — 不熔断, 列出当前角色可用 tool 让 LLM 自纠。
func formatUnknownTool(toolName, roleName string, allowed []string) string {
	avail := strings.Join(allowed, ", ")
	if len(avail) > 400 {
		avail = avail[:400] + "..."
	}
	return fmt.Sprintf(
		"[%s] 未知工具 — 此 tool 不在已注册集合内, 不会被执行。 当前角色 %s 可用工具: [%s]. "+
			"请用准确的 tool 名重试 (检查拼写或换近义工具)。",
		toolName, roleName, avail,
	)
}

// formatRoleDeny 角色拒绝重导向: 不是 dead end, 是告诉 LLM "这事不归你管, 让上层接管"。
// 让 LLM 选别的同语义合规 tool 或主动放弃当前 tool 调用 (orchestrator 会在下个角色再试)。
func formatRoleDeny(toolName, roleName string) string {
	return fmt.Sprintf(
		"[%s] 角色限制 — 当前 %s 角色无权使用此工具 (orchestrator 将在 Configure 阶段委派写操作). "+
			"如果当前轮次只是诊断, 请用只读工具收集证据后给出结论, 不要硬拗写工具.",
		toolName, roleName,
	)
}

// SingleAgent 是单角色 LLM Agent 执行器，实现 tool-use 闭环。
// Orchestrator 用它来运行每个角色阶段。
type SingleAgent struct {
	llm       *LLMClient
	pipeline  *tool.ToolPipeline
	assembler *PromptAssembler
	tools     map[string]*tool.ToolDef
	maxIter   int
}

func NewSingleAgent(llm *LLMClient, pipeline *tool.ToolPipeline, assembler *PromptAssembler, tools map[string]*tool.ToolDef, maxIter int) *SingleAgent {
	return &SingleAgent{
		llm:       llm,
		pipeline:  pipeline,
		assembler: assembler,
		tools:     tools,
		maxIter:   maxIter,
	}
}

// RunWithRole 用指定角色执行 Agent 循环。
// role 决定 system prompt 和可用 tool 列表。
//
// 返回值 events 记录本角色内每次 tool 调用的可观测细节, 供 Chat Tool-Call Timeline UI (D3) 渲染。
//
// 硅基流动对 tool_calls 消息格式的校验不稳定（相同请求时而通过时而 400），
// 因此不在消息历史中保留 tool_calls/tool 格式，而是将 tool 调用和结果
// 转化为纯文本 assistant 消息，再以 user 角色喂回结果。
func (a *SingleAgent) RunWithRole(ctx context.Context, role *AgentRole, userMessage string, history *ConversationHistory) (string, []ToolEvent, error) {
	// 1. 组装角色专用 system prompt（含对话历史摘要）
	systemPrompt := a.assembler.AssembleForRole(ctx, role, history)

	// 2. 初始化消息列表
	messages := []Message{
		{Role: "system", Content: StringPtr(systemPrompt)},
		{Role: "user", Content: StringPtr(userMessage)},
	}

	// 3. 获取角色限定的 tool schemas
	tools := ConvertToolsForRole(a.tools, role.AllowedTools)

	var events []ToolEvent
	// #M27 熔断: 跨 iter 累计每个 tool 的失败次数 + 最近一次错误消息
	toolFailCount := map[string]int{}
	lastToolError := map[string]string{}

	// 4. Tool-use 循环
	for i := 0; i < a.maxIter; i++ {
		// 2026-04-25 实验:diagnose 首轮 tool_choice=required 强制调 tool (防 fabricate)。
		// 结论:SiliconFlow/DeepSeek-V3 收到 required 后返回 choices=[] 空响应 (14 token 被消费), 不兼容。
		// 回退 auto, fabricate 风险靠 temperature 降低缓解 + local router keyword 兜底。
		req := CompletionRequest{
			Model:              a.llm.Model,
			Messages:           messages,
			Tools:              tools,
			ToolChoice:         "auto",
			Temperature:        Float64Ptr(config.DefaultLLMTemperature),
			EnableThinking:     BoolPtr(false),
			ChatTemplateKwargs: map[string]interface{}{"enable_thinking": false},
		}
		if debugAgent {
			dump, _ := json.MarshalIndent(req, "", "  ")
			fmt.Fprintf(os.Stderr, "\n[DEBUG] [%s] LLM 请求 (iter %d):\n%s\n", role.Name, i, string(dump))
		}
		resp, err := a.llm.Complete(ctx, req)
		if err != nil {
			return "", events, fmt.Errorf("[%s] LLM 调用失败: %w", role.Name, err)
		}

		choice := resp.Choices[0]

		if debugAgent {
			dump, _ := json.MarshalIndent(choice, "", "  ")
			fmt.Fprintf(os.Stderr, "\n[DEBUG] [%s] LLM 响应 (iter %d): finish_reason=%s\n%s\n", role.Name, i, choice.FinishReason, string(dump))
		}

		// LLM 完成（不再调用工具），返回最终回复
		if choice.FinishReason == "stop" || len(choice.Message.ToolCalls) == 0 {
			if choice.Message.Content != nil {
				return *choice.Message.Content, events, nil
			}
			return "", events, nil
		}

		// LLM 要调用工具 → 执行所有 tool calls，将结果拼成纯文本
		var summaryParts []string

		for _, tc := range choice.Message.ToolCalls {
			// 全局未知 tool 名: LLM 幻觉了不存在的 tool, 不熔断, 返回可用列表让自纠
			if _, exists := a.tools[tc.Function.Name]; !exists {
				msg := formatUnknownTool(tc.Function.Name, role.Name, role.AllowedTools)
				summaryParts = append(summaryParts, msg)
				events = append(events, ToolEvent{
					Name:        tc.Function.Name,
					ArgsSummary: truncateResult(tc.Function.Arguments, 120),
					Error:       "unknown tool",
					Role:        role.Name,
				})
				toolFailCount[tc.Function.Name]++
				lastToolError[tc.Function.Name] = "unknown tool"
				continue
			}

			// 角色拒绝: 重导向 hint 而非 dead end
			if !isToolAllowed(tc.Function.Name, role.AllowedTools) {
				denyMsg := formatRoleDeny(tc.Function.Name, role.Name)
				summaryParts = append(summaryParts, denyMsg)
				events = append(events, ToolEvent{
					Name:        tc.Function.Name,
					ArgsSummary: truncateResult(tc.Function.Arguments, 120),
					Error:       "role denied",
					Role:        role.Name,
				})
				toolFailCount[tc.Function.Name]++
				lastToolError[tc.Function.Name] = "role denied"
				continue
			}

			fmt.Printf("\033[36m  [%s] 调用 %s\033[0m\n", role.Name, formatToolCall(tc))

			// 解析参数
			var params map[string]interface{}
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					msg := formatToolFailure(tc.Function.Name, nil,
						"参数 JSON 解析失败: "+err.Error(),
						"确保 arguments 是合法 JSON 对象, 字段类型对照 tool schema")
					summaryParts = append(summaryParts, msg)
					events = append(events, ToolEvent{
						Name:        tc.Function.Name,
						ArgsSummary: truncateResult(tc.Function.Arguments, 120),
						Error:       "params parse: " + err.Error(),
						Role:        role.Name,
					})
					toolFailCount[tc.Function.Name]++
					lastToolError[tc.Function.Name] = "params parse: " + err.Error()
					continue
				}
			}

			// 通过 Pipeline 执行（走完整的 hook/snapshot/telemetry 流程）
			startedAt := time.Now()
			result := a.pipeline.Execute(ctx, tc.Function.Name, params)
			durationMs := time.Since(startedAt).Milliseconds()

			cleanMsg := truncateResult(stripANSI(result.Message), 2000)
			evt := ToolEvent{
				Name:          tc.Function.Name,
				ArgsSummary:   truncateResult(tc.Function.Arguments, 120),
				DurationMs:    durationMs,
				OutputPreview: truncateResult(stripANSI(result.Message), 200),
				Role:          role.Name,
			}
			if result.Success {
				summaryParts = append(summaryParts,
					fmt.Sprintf("[%s] 成功: %s", tc.Function.Name, cleanMsg))
				delete(toolFailCount, tc.Function.Name)
				delete(lastToolError, tc.Function.Name)
			} else {
				rawErr := stripANSI(result.Message)
				evt.Error = truncateResult(rawErr, 200)
				msg := formatToolFailure(tc.Function.Name, params,
					truncateResult(rawErr, 600), suggestRecovery(rawErr))
				summaryParts = append(summaryParts, msg)
				toolFailCount[tc.Function.Name]++
				lastToolError[tc.Function.Name] = evt.Error
			}
			events = append(events, evt)
		}

		// #M27 熔断检查: 当前轮处理完所有 tool 后, 如有任一 tool 累计失败 >= 阈值, 直接返回
		if name, lastErr, breaking := shouldCircuitBreak(toolFailCount, lastToolError); breaking {
			return formatCircuitBreakReply(name, lastErr), events, nil
		}

		// 将 tool 调用结果作为纯文本加入消息历史（避免 tool_calls 格式）
		var callDescs []string
		for _, tc := range choice.Message.ToolCalls {
			callDescs = append(callDescs, fmt.Sprintf("%s(%s)", tc.Function.Name, tc.Function.Arguments))
		}
		messages = append(messages, Message{
			Role:    "assistant",
			Content: StringPtr(fmt.Sprintf("我需要调用工具: %s", strings.Join(callDescs, ", "))),
		})
		messages = append(messages, Message{
			Role:    "user",
			Content: StringPtr(fmt.Sprintf("工具执行结果:\n%s\n\n如果还有后续步骤需要执行，继续调用工具；如果所有操作已完成，回复用户最终结果。", strings.Join(summaryParts, "\n"))),
		})
	}

	return "已达到最大操作步数，停止自动操作。", events, nil
}

// StreamSink 接收 SingleAgent 流式执行过程中的事件 (Phase 7.3)。
// 实现层可以是 orchestrator 的包装 sink, 最终把事件 forward 给 mobile 层 Kotlin callback。
type StreamSink interface {
	// OnText 每次收到 LLM content delta 时触发 (可能很短, 1-5 char)。
	// 仅在 "最终回复" 迭代触发; tool-use 迭代里 LLM 不输出 content。
	OnText(delta string)
	// OnToolStart 在 pipeline.Execute 之前触发, 让 UI 立刻渲染 "▸ 调用中" 占位。
	// evt 此时只有 Name / ArgsSummary / Role, Duration/Output 为空。
	OnToolStart(evt ToolEvent)
	// OnToolEnd 在 pipeline.Execute 结束后触发, evt 含完整字段。
	OnToolEnd(evt ToolEvent)
}

// nopStreamSink 在 orchestrator 非流式路径或未提供 sink 时兜底, 避免 nil 检查散布各处。
type nopStreamSink struct{}

func (nopStreamSink) OnText(string)         {}
func (nopStreamSink) OnToolStart(ToolEvent) {}
func (nopStreamSink) OnToolEnd(ToolEvent)   {}

// RunWithRoleStream 是 RunWithRole 的流式版本; 逻辑基本一致但 LLM 调用走 CompleteStream。
// 返回值语义与 RunWithRole 完全相同 (finalReply, events, err), 方便 orchestrator 切换。
//
// sink 不可为 nil; 非流式上下文传 nopStreamSink{}。
func (a *SingleAgent) RunWithRoleStream(ctx context.Context, role *AgentRole, userMessage string, history *ConversationHistory, sink StreamSink) (string, []ToolEvent, error) {
	if sink == nil {
		sink = nopStreamSink{}
	}
	systemPrompt := a.assembler.AssembleForRole(ctx, role, history)
	messages := []Message{
		{Role: "system", Content: StringPtr(systemPrompt)},
		{Role: "user", Content: StringPtr(userMessage)},
	}
	tools := ConvertToolsForRole(a.tools, role.AllowedTools)

	var events []ToolEvent
	// #M27 熔断: 跨 iter 累计每个 tool 的失败次数 + 最近一次错误消息
	toolFailCount := map[string]int{}
	lastToolError := map[string]string{}

	for i := 0; i < a.maxIter; i++ {
		req := CompletionRequest{
			Model:              a.llm.Model,
			Messages:           messages,
			Tools:              tools,
			ToolChoice:         "auto",
			Temperature:        Float64Ptr(config.DefaultLLMTemperature),
			EnableThinking:     BoolPtr(false),
			ChatTemplateKwargs: map[string]interface{}{"enable_thinking": false},
		}

		var contentBuf strings.Builder
		var toolDeltas []ToolCallDelta
		var finishReason string

		err := a.llm.CompleteStream(ctx, req, func(d StreamDelta) error {
			if d.ContentDelta != "" {
				contentBuf.WriteString(d.ContentDelta)
				sink.OnText(d.ContentDelta)
			}
			if len(d.ToolCallDeltas) > 0 {
				toolDeltas = append(toolDeltas, d.ToolCallDeltas...)
			}
			if d.FinishReason != "" {
				finishReason = d.FinishReason
			}
			return nil
		})
		if err != nil {
			return "", events, fmt.Errorf("[%s] LLM 调用失败: %w", role.Name, err)
		}

		toolCalls := AccumulateToolCalls(toolDeltas)

		// 最终回复: 没 tool 调用, 或明确 finish_reason=stop
		if finishReason == "stop" || len(toolCalls) == 0 {
			return contentBuf.String(), events, nil
		}

		// 有 tool calls → 执行并回填
		var summaryParts []string
		for _, tc := range toolCalls {
			// 全局未知 tool 名: 不熔断, 列出可用 tool 让 LLM 自纠
			if _, exists := a.tools[tc.Function.Name]; !exists {
				msg := formatUnknownTool(tc.Function.Name, role.Name, role.AllowedTools)
				evt := ToolEvent{
					Name:        tc.Function.Name,
					ArgsSummary: truncateResult(tc.Function.Arguments, 120),
					Error:       "unknown tool",
					Role:        role.Name,
				}
				sink.OnToolEnd(evt)
				events = append(events, evt)
				summaryParts = append(summaryParts, msg)
				toolFailCount[tc.Function.Name]++
				lastToolError[tc.Function.Name] = "unknown tool"
				continue
			}

			// 角色拒绝: 重导向 hint
			if !isToolAllowed(tc.Function.Name, role.AllowedTools) {
				denyMsg := formatRoleDeny(tc.Function.Name, role.Name)
				evt := ToolEvent{
					Name:        tc.Function.Name,
					ArgsSummary: truncateResult(tc.Function.Arguments, 120),
					Error:       "role denied",
					Role:        role.Name,
				}
				sink.OnToolEnd(evt)
				events = append(events, evt)
				summaryParts = append(summaryParts, denyMsg)
				toolFailCount[tc.Function.Name]++
				lastToolError[tc.Function.Name] = "role denied"
				continue
			}

			startEvt := ToolEvent{
				Name:        tc.Function.Name,
				ArgsSummary: truncateResult(tc.Function.Arguments, 120),
				Role:        role.Name,
			}
			sink.OnToolStart(startEvt)

			var params map[string]interface{}
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					msg := formatToolFailure(tc.Function.Name, nil,
						"参数 JSON 解析失败: "+err.Error(),
						"确保 arguments 是合法 JSON 对象, 字段类型对照 tool schema")
					evt := ToolEvent{
						Name:        tc.Function.Name,
						ArgsSummary: truncateResult(tc.Function.Arguments, 120),
						Error:       "params parse: " + err.Error(),
						Role:        role.Name,
					}
					sink.OnToolEnd(evt)
					events = append(events, evt)
					summaryParts = append(summaryParts, msg)
					toolFailCount[tc.Function.Name]++
					lastToolError[tc.Function.Name] = "params parse: " + err.Error()
					continue
				}
			}

			startedAt := time.Now()
			result := a.pipeline.Execute(ctx, tc.Function.Name, params)
			durationMs := time.Since(startedAt).Milliseconds()

			cleanMsg := truncateResult(stripANSI(result.Message), 2000)
			evt := ToolEvent{
				Name:          tc.Function.Name,
				ArgsSummary:   truncateResult(tc.Function.Arguments, 120),
				DurationMs:    durationMs,
				OutputPreview: truncateResult(stripANSI(result.Message), 200),
				Role:          role.Name,
			}
			if !result.Success {
				evt.Error = truncateResult(stripANSI(result.Message), 200)
			}
			sink.OnToolEnd(evt)
			events = append(events, evt)
			if result.Success {
				summaryParts = append(summaryParts,
					fmt.Sprintf("[%s] 成功: %s", tc.Function.Name, cleanMsg))
				delete(toolFailCount, tc.Function.Name)
				delete(lastToolError, tc.Function.Name)
			} else {
				rawErr := stripANSI(result.Message)
				msg := formatToolFailure(tc.Function.Name, params,
					truncateResult(rawErr, 600), suggestRecovery(rawErr))
				summaryParts = append(summaryParts, msg)
				toolFailCount[tc.Function.Name]++
				lastToolError[tc.Function.Name] = evt.Error
			}
		}

		// #M27 熔断检查: 当前轮处理完所有 tool 后, 如有任一 tool 累计失败 >= 阈值, 直接返回
		if name, lastErr, breaking := shouldCircuitBreak(toolFailCount, lastToolError); breaking {
			return formatCircuitBreakReply(name, lastErr), events, nil
		}

		// 把 tool 结果作为纯文本塞回 messages, 进入下一轮 (与 RunWithRole 保持一致的规避 tool_calls 校验策略)
		var callDescs []string
		for _, tc := range toolCalls {
			callDescs = append(callDescs, fmt.Sprintf("%s(%s)", tc.Function.Name, tc.Function.Arguments))
		}
		messages = append(messages, Message{
			Role:    "assistant",
			Content: StringPtr(fmt.Sprintf("我需要调用工具: %s", strings.Join(callDescs, ", "))),
		})
		messages = append(messages, Message{
			Role:    "user",
			Content: StringPtr(fmt.Sprintf("工具执行结果:\n%s\n\n如果还有后续步骤需要执行，继续调用工具；如果所有操作已完成，回复用户最终结果。", strings.Join(summaryParts, "\n"))),
		})
	}

	return "已达到最大操作步数，停止自动操作。", events, nil
}

// isToolAllowed 检查 tool 是否在角色白名单中
func isToolAllowed(toolName string, allowed []string) bool {
	for _, name := range allowed {
		if name == toolName {
			return true
		}
	}
	return false
}

// ansiRegex 匹配 ANSI 转义序列（终端颜色等）
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI 去除字符串中的 ANSI 转义码
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// truncateResult 截断过长的 tool 结果，避免 LLM 超时
func truncateResult(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("\n...(已截断，原始长度 %d 字符)", len(s))
}

// formatToolCall 格式化 tool call 用于终端显示
func formatToolCall(tc ToolCall) string {
	if tc.Function.Arguments == "" || tc.Function.Arguments == "{}" {
		return tc.Function.Name
	}
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
		return tc.Function.Name
	}
	s := tc.Function.Name + ":"
	first := true
	for k, v := range params {
		if !first {
			s += ","
		}
		s += fmt.Sprintf(" %s=%v", k, v)
		first = false
	}
	return s
}
