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

// AgentLoop 是 LLM Agent 主循环，实现 tool-use 闭环
type AgentLoop struct {
	llm       *LLMClient
	pipeline  *tool.ToolPipeline
	assembler *PromptAssembler
	tools     map[string]*tool.ToolDef
	maxIter   int
}

func NewAgentLoop(llm *LLMClient, pipeline *tool.ToolPipeline, assembler *PromptAssembler, tools map[string]*tool.ToolDef, maxIter int) *AgentLoop {
	return &AgentLoop{
		llm:       llm,
		pipeline:  pipeline,
		assembler: assembler,
		tools:     tools,
		maxIter:   maxIter,
	}
}

// Run 执行 Agent 主循环：用户消息 → LLM → tool calls → 执行 → 反馈 → LLM → ...
//
// 硅基流动对 tool_calls 消息格式的校验不稳定（相同请求时而通过时而 400），
// 因此不在消息历史中保留 tool_calls/tool 格式，而是将 tool 调用和结果
// 转化为纯文本 assistant 消息，再以 user 角色喂回结果。
func (a *AgentLoop) Run(ctx context.Context, userMessage string) (string, error) {
	// 1. 组装 system prompt
	systemPrompt := a.assembler.Assemble(ctx)

	// 2. 初始化消息列表
	messages := []Message{
		{Role: "system", Content: StringPtr(systemPrompt)},
		{Role: "user", Content: StringPtr(userMessage)},
	}

	// 3. 获取 tool schemas
	tools := ConvertToolsToSchema(a.tools)

	fmt.Print("\033[36m🤖 正在分析...\033[0m\n")

	// 4. Tool-use 循环
	for i := 0; i < a.maxIter; i++ {
		req := CompletionRequest{
			Model:    a.llm.Model,
			Messages: messages,
			Tools:    tools,
		}
		if debugAgent {
			dump, _ := json.MarshalIndent(req, "", "  ")
			fmt.Fprintf(os.Stderr, "\n[DEBUG] LLM 请求 (iter %d):\n%s\n", i, string(dump))
		}
		resp, err := a.llm.Complete(ctx, req)
		if err != nil {
			return "", fmt.Errorf("LLM 调用失败: %w", err)
		}

		choice := resp.Choices[0]

		if debugAgent {
			dump, _ := json.MarshalIndent(choice, "", "  ")
			fmt.Fprintf(os.Stderr, "\n[DEBUG] LLM 响应 (iter %d): finish_reason=%s\n%s\n", i, choice.FinishReason, string(dump))
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
			fmt.Printf("\033[36m🤖 [调用工具 %s]\033[0m\n", formatToolCall(tc))

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

			cleanMsg := stripANSI(result.Message)
			if result.Success {
				summaryParts = append(summaryParts,
					fmt.Sprintf("[%s] 成功: %s", tc.Function.Name, cleanMsg))
			} else {
				summaryParts = append(summaryParts,
					fmt.Sprintf("[%s] 失败: %s", tc.Function.Name, cleanMsg))
			}
		}

		// 将 tool 调用结果作为纯文本加入消息历史（避免 tool_calls 格式）
		// assistant: "我调用了 xxx"
		// user: "工具执行结果: ..."
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
			Content: StringPtr(fmt.Sprintf("工具执行结果:\n%s\n\n请根据结果回复用户。", strings.Join(summaryParts, "\n"))),
		})
	}

	return "已达到最大操作步数，停止自动操作。", nil
}

// ansiRegex 匹配 ANSI 转义序列（终端颜色等）
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI 去除字符串中的 ANSI 转义码
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
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
