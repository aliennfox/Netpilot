package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
