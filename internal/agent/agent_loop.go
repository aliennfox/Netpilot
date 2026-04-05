package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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
func (a *AgentLoop) Run(ctx context.Context, userMessage string) (string, error) {
	// 1. 组装 system prompt
	systemPrompt := a.assembler.Assemble(ctx)

	// 2. 初始化消息列表
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
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
			return choice.Message.Content, nil
		}

		// LLM 要调用工具
		// 把 assistant 消息加入历史（保留 tool_calls 信息）
		assistantMsg := choice.Message
		// 确保每个 tool_call 的 type 字段为 "function"（部分 API 响应可能省略）
		for j := range assistantMsg.ToolCalls {
			if assistantMsg.ToolCalls[j].Type == "" {
				assistantMsg.ToolCalls[j].Type = "function"
			}
		}
		messages = append(messages, assistantMsg)

		// 执行每个 tool call
		for _, tc := range choice.Message.ToolCalls {
			fmt.Printf("\033[36m🤖 [调用工具 %s]\033[0m\n", formatToolCall(tc))

			// 解析参数
			var params map[string]interface{}
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					// 参数解析失败，返回错误给 LLM
					messages = append(messages, Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    fmt.Sprintf(`{"success":false,"message":"参数解析失败: %s"}`, err.Error()),
					})
					continue
				}
			}

			// 通过 Pipeline 执行（走完整的 hook/snapshot/telemetry 流程）
			result := a.pipeline.Execute(ctx, tc.Function.Name, params)

			// 构造结构化结果返回给 LLM
			toolResult := map[string]interface{}{
				"success": result.Success,
				"message": result.Message,
			}
			if result.Data != nil {
				toolResult["data"] = result.Data
			}
			resultJSON, _ := json.Marshal(toolResult)

			messages = append(messages, Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    string(resultJSON),
			})
		}
	}

	return "已达到最大操作步数，停止自动操作。", nil
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
