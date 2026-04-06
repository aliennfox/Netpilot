package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/foxnetpilot/netpilot/internal/tool"
)

var debugAgent = os.Getenv("NETPILOT_DEBUG") != ""

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
// 硅基流动对 tool_calls 消息格式的校验不稳定（相同请求时而通过时而 400），
// 因此不在消息历史中保留 tool_calls/tool 格式，而是将 tool 调用和结果
// 转化为纯文本 assistant 消息，再以 user 角色喂回结果。
func (a *SingleAgent) RunWithRole(ctx context.Context, role *AgentRole, userMessage string, history *ConversationHistory) (string, error) {
	// 1. 组装角色专用 system prompt（含对话历史摘要）
	systemPrompt := a.assembler.AssembleForRole(ctx, role, history)

	// 2. 初始化消息列表
	messages := []Message{
		{Role: "system", Content: StringPtr(systemPrompt)},
		{Role: "user", Content: StringPtr(userMessage)},
	}

	// 3. 获取角色限定的 tool schemas
	tools := ConvertToolsForRole(a.tools, role.AllowedTools)

	// 4. Tool-use 循环
	for i := 0; i < a.maxIter; i++ {
		req := CompletionRequest{
			Model:      a.llm.Model,
			Messages:   messages,
			Tools:      tools,
			ToolChoice: "auto",
		}
		if debugAgent {
			dump, _ := json.MarshalIndent(req, "", "  ")
			fmt.Fprintf(os.Stderr, "\n[DEBUG] [%s] LLM 请求 (iter %d):\n%s\n", role.Name, i, string(dump))
		}
		resp, err := a.llm.Complete(ctx, req)
		if err != nil {
			return "", fmt.Errorf("[%s] LLM 调用失败: %w", role.Name, err)
		}

		choice := resp.Choices[0]

		if debugAgent {
			dump, _ := json.MarshalIndent(choice, "", "  ")
			fmt.Fprintf(os.Stderr, "\n[DEBUG] [%s] LLM 响应 (iter %d): finish_reason=%s\n%s\n", role.Name, i, choice.FinishReason, string(dump))
		}

		// LLM 完成（不再调用工具），返回最终回复
		if choice.FinishReason == "stop" || len(choice.Message.ToolCalls) == 0 {
			if choice.Message.Content != nil {
				return *choice.Message.Content, nil
			}
			return "", nil
		}

		// LLM 要调用工具 → 执行所有 tool calls，将结果拼成纯文本
		var summaryParts []string

		for _, tc := range choice.Message.ToolCalls {
			// 硬性检查：只读角色不允许调写操作
			if !isToolAllowed(tc.Function.Name, role.AllowedTools) {
				summaryParts = append(summaryParts,
					fmt.Sprintf("[%s] 被拒绝: 当前角色 %s 无权使用此工具", tc.Function.Name, role.Name))
				continue
			}

			fmt.Printf("\033[36m  [%s] 调用 %s\033[0m\n", role.Name, formatToolCall(tc))

			// 解析参数
			var params map[string]interface{}
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					summaryParts = append(summaryParts,
						fmt.Sprintf("[%s] 参数解析失败: %s", tc.Function.Name, err.Error()))
					continue
				}
			}

			// 通过 Pipeline 执行（走完整的 hook/snapshot/telemetry 流程）
			result := a.pipeline.Execute(ctx, tc.Function.Name, params)

			cleanMsg := truncateResult(stripANSI(result.Message), 2000)
			if result.Success {
				summaryParts = append(summaryParts,
					fmt.Sprintf("[%s] 成功: %s", tc.Function.Name, cleanMsg))
			} else {
				summaryParts = append(summaryParts,
					fmt.Sprintf("[%s] 失败: %s", tc.Function.Name, cleanMsg))
			}
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

	return "已达到最大操作步数，停止自动操作。", nil
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
