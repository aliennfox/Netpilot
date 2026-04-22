package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMClient 是 OpenAI 兼容的 chat completions 客户端
type LLMClient struct {
	baseURL    string
	apiKey     string
	Model      string
	httpClient *http.Client
}

func NewLLMClient(baseURL, apiKey, model string, timeout time.Duration) *LLMClient {
	return &LLMClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		Model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// --- Request/Response types ---

type Message struct {
	Role       string     `json:"role"`
	Content    *string    `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// StringPtr 返回字符串指针，用于构造 Message.Content
func StringPtr(s string) *string {
	return &s
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type CompletionRequest struct {
	Model      string    `json:"model"`
	Messages   []Message `json:"messages"`
	Tools      []Tool    `json:"tools,omitempty"`
	ToolChoice string    `json:"tool_choice,omitempty"` // "auto", "none", or "required"
	Stream     bool      `json:"stream,omitempty"`
}

type CompletionResponse struct {
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Complete 发送 chat completion 请求
func (c *LLMClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 429 限流: 重试一次
	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		time.Sleep(2 * time.Second)
		retryReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("创建重试请求失败: %w", err)
		}
		retryReq.Header.Set("Content-Type", "application/json")
		retryReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		resp2, err := c.httpClient.Do(retryReq)
		if err != nil {
			return nil, fmt.Errorf("重试请求失败: %w", err)
		}
		defer resp2.Body.Close()
		respBody, err = io.ReadAll(resp2.Body)
		if err != nil {
			return nil, fmt.Errorf("读取重试响应失败: %w", err)
		}
		if resp2.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("LLM API 错误 (重试后): HTTP %d: %s", resp2.StatusCode, string(respBody))
		}
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API 错误: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result CompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w\n原始响应: %s", err, string(respBody))
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("LLM 返回空 choices")
	}

	return &result, nil
}

// --- Streaming (Phase 7.3) ---

// StreamDelta 是单次 SSE chunk 的解析结果。
// OpenAI 协议里 content / tool_calls 每次只给 delta, 需要上层累积。
type StreamDelta struct {
	ContentDelta   string
	ToolCallDeltas []ToolCallDelta // 本 chunk 里出现的 tool_call 分片 (带 index)
	FinishReason   string          // "stop" / "tool_calls" / "length" / ""
}

// ToolCallDelta: OpenAI stream tool_calls 分片, 同 index 的多个 delta 会拼成一个 ToolCall。
// 典型路径: 第一个 delta 给 {Index, ID, Function.Name}, 后续给 Function.Arguments 片段。
type ToolCallDelta struct {
	Index     int
	ID        string
	Type      string
	FuncName  string
	ArgsDelta string
}

// streamChunkRaw 是 SSE payload 的 OpenAI-兼容原始 schema。
type streamChunkRaw struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string                   `json:"role,omitempty"`
			Content   string                   `json:"content,omitempty"`
			ToolCalls []streamChunkRawToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

type streamChunkRawToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

// CompleteStream 以 SSE 方式请求 chat completion; 每收到一个 chunk 调 onDelta 一次。
// onDelta 返回 error 则立即终止 stream, 关闭连接。 [DONE] 标志正常结束, 返回 nil。
//
// 限制: 不做 429 重试 (stream 重试复杂, 失败直接返回 error 让上层决定)。
// 超时: 用 httpClient.Timeout 覆盖整段连接 (含 SSE 读取), 不单独设 per-chunk deadline。
func (c *LLMClient) CompleteStream(ctx context.Context, req CompletionRequest, onDelta func(StreamDelta) error) error {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LLM API 错误: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	// 单行最大 1MB (tool_call arguments 可能较长)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue // event: / : comment / blank line
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			return nil
		}
		var raw streamChunkRaw
		if err := json.Unmarshal([]byte(payload), &raw); err != nil {
			continue // 忽略坏 chunk, 别因为一个畸形包中断整个 stream
		}
		if len(raw.Choices) == 0 {
			continue
		}
		ch := raw.Choices[0]
		delta := StreamDelta{
			ContentDelta: ch.Delta.Content,
			FinishReason: ch.FinishReason,
		}
		for _, tc := range ch.Delta.ToolCalls {
			delta.ToolCallDeltas = append(delta.ToolCallDeltas, ToolCallDelta{
				Index:     tc.Index,
				ID:        tc.ID,
				Type:      tc.Type,
				FuncName:  tc.Function.Name,
				ArgsDelta: tc.Function.Arguments,
			})
		}
		if err := onDelta(delta); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("SSE 读取失败: %w", err)
	}
	return nil
}

// AccumulateToolCalls 把 ToolCallDelta 序列按 index 合并为完整 ToolCall 列表。
// 调用方在 stream 过程中把所有 delta 攒起来, stream 结束后一次性 flatten。
func AccumulateToolCalls(deltas []ToolCallDelta) []ToolCall {
	byIndex := map[int]*ToolCall{}
	order := []int{}
	for _, d := range deltas {
		tc, ok := byIndex[d.Index]
		if !ok {
			tc = &ToolCall{Type: "function"}
			byIndex[d.Index] = tc
			order = append(order, d.Index)
		}
		if d.ID != "" {
			tc.ID = d.ID
		}
		if d.Type != "" {
			tc.Type = d.Type
		}
		if d.FuncName != "" {
			tc.Function.Name = d.FuncName
		}
		tc.Function.Arguments += d.ArgsDelta
	}
	result := make([]ToolCall, 0, len(order))
	for _, idx := range order {
		result = append(result, *byIndex[idx])
	}
	return result
}
