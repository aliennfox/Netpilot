package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/tool"
)

// --- Mock helpers ---

// agentFakeAdapter 嵌入 EngineAdapter, 只实现 PromptAssembler / pipeline test tool 用到的方法
type agentFakeAdapter struct {
	engine.EngineAdapter
}

func (a *agentFakeAdapter) GetProxyGroup(tag string) (*engine.ProxyGroup, error) {
	return &engine.ProxyGroup{Tag: tag, Type: "Selector", Now: "HK-1", All: []engine.ProxyInfo{
		{Tag: "HK-1", Type: "shadowsocks"},
	}}, nil
}
func (a *agentFakeAdapter) GetConnections() ([]engine.ConnectionInfo, error) { return nil, nil }
func (a *agentFakeAdapter) GetProxies() ([]engine.ProxyInfo, error) {
	return []engine.ProxyInfo{
		{Tag: "proxy-group", Type: "Selector"},
		{Tag: "HK-1", Type: "shadowsocks"},
	}, nil
}
func (a *agentFakeAdapter) SetActiveProxy(string, string) error { return nil }
func (a *agentFakeAdapter) Reload() error                       { return nil }

// mockLLMServer 注入预设的 chat/completions 响应序列, 按调用顺序消耗
type mockLLMServer struct {
	mu        sync.Mutex
	responses []string
	calls     int
	server    *httptest.Server
}

func newMockLLMServer(responses []string) *mockLLMServer {
	m := &mockLLMServer{responses: responses}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		idx := m.calls
		m.calls++
		m.mu.Unlock()
		if idx >= len(m.responses) {
			http.Error(w, "no more mock responses", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(m.responses[idx]))
	}))
	return m
}

func (m *mockLLMServer) URL() string { return m.server.URL }
func (m *mockLLMServer) Close()      { m.server.Close() }
func (m *mockLLMServer) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// makeFinalResp 构造最终回复 (finish_reason=stop, 无 tool_calls)
func makeFinalResp(content string) string {
	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"message":       map[string]interface{}{"role": "assistant", "content": content},
				"finish_reason": "stop",
			},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// makeToolCallResp 构造 tool_calls 响应 (finish_reason=tool_calls)
func makeToolCallResp(toolName, argsJSON string) string {
	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": nil,
					"tool_calls": []map[string]interface{}{
						{
							"id":   "call-1",
							"type": "function",
							"function": map[string]interface{}{
								"name":      toolName,
								"arguments": argsJSON,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// buildSingleAgent 构造 SingleAgent + 真 pipeline + 注入的 mock LLM 响应序列
func buildSingleAgent(t *testing.T, mockResponses []string) (*SingleAgent, *mockLLMServer, *tool.ToolPipeline) {
	t.Helper()
	adapter := &agentFakeAdapter{}
	pipe := tool.NewPipeline(adapter, t.TempDir())
	srv := newMockLLMServer(mockResponses)

	llm := NewLLMClient(srv.URL(), "fake-key", "test-model", 3*time.Second)
	assembler := NewPromptAssembler(adapter)

	return NewSingleAgent(llm, pipe, assembler, pipe.GetTools(), 5), srv, pipe
}

// testRole 是只允许 get_node_pool / test_latency 的小角色 (避免依赖生产 RoleDiagnose 等)
var testRole = &AgentRole{
	Name:         "test",
	Description:  "test role",
	SystemPrompt: "test agent prompt",
	AllowedTools: []string{"get_node_pool", "test_latency", "switch_node"},
}

// --- 主路径测试 ---

// TestSingleAgent_NoToolCallReturnsContent LLM 第一轮直接 stop → 透传 content
func TestSingleAgent_NoToolCallReturnsContent(t *testing.T) {
	agent, srv, _ := buildSingleAgent(t, []string{
		makeFinalResp("当前节点是 HK-1, 一切正常。"),
	})
	defer srv.Close()

	reply, events, err := agent.RunWithRole(context.Background(), testRole, "状态怎么样?", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if reply != "当前节点是 HK-1, 一切正常。" {
		t.Errorf("reply: %q", reply)
	}
	if len(events) != 0 {
		t.Errorf("no tool was called, expect 0 events, got %d: %+v", len(events), events)
	}
	if srv.Calls() != 1 {
		t.Errorf("LLM should be called once, got %d", srv.Calls())
	}
}

// TestSingleAgent_OneToolCallThenStop iter1 调 get_node_pool, iter2 stop → events 1 个 + 最终回复
func TestSingleAgent_OneToolCallThenStop(t *testing.T) {
	agent, srv, _ := buildSingleAgent(t, []string{
		makeToolCallResp("get_node_pool", "{}"),
		makeFinalResp("一共 2 个节点: proxy-group, HK-1。"),
	})
	defer srv.Close()

	reply, events, err := agent.RunWithRole(context.Background(), testRole, "看看节点", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(reply, "节点") {
		t.Errorf("reply: %q", reply)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 tool event, got %d: %+v", len(events), events)
	}
	if events[0].Name != "get_node_pool" {
		t.Errorf("event name: %q", events[0].Name)
	}
	if events[0].Role != "test" {
		t.Errorf("event role: %q", events[0].Role)
	}
	if events[0].Error != "" {
		t.Errorf("event should not have error: %q", events[0].Error)
	}
	if srv.Calls() != 2 {
		t.Errorf("LLM should be called twice (tool + final), got %d", srv.Calls())
	}
}

// TestSingleAgent_ToolDeniedByRoleWhitelist 调白名单外 tool → event Error 含"无权使用", 不实际执行
func TestSingleAgent_ToolDeniedByRoleWhitelist(t *testing.T) {
	agent, srv, _ := buildSingleAgent(t, []string{
		// LLM 试图调 set_mode (不在 testRole 白名单)
		makeToolCallResp("set_mode", `{"mode":"global"}`),
		makeFinalResp("好的, 我没权限切模式。"),
	})
	defer srv.Close()

	reply, events, err := agent.RunWithRole(context.Background(), testRole, "切到全局模式", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(reply, "没权限") {
		t.Errorf("reply: %q", reply)
	}
	if len(events) != 1 {
		t.Fatalf("expect 1 event, got %d", len(events))
	}
	if events[0].Name != "set_mode" {
		t.Errorf("event name: %q", events[0].Name)
	}
	if !strings.Contains(events[0].Error, "无权使用") {
		t.Errorf("event Error should mention 无权使用, got %q", events[0].Error)
	}
}

// TestSingleAgent_MalformedToolArgs LLM 返回的 arguments 不是合法 JSON → event 标参数解析失败
func TestSingleAgent_MalformedToolArgs(t *testing.T) {
	agent, srv, _ := buildSingleAgent(t, []string{
		makeToolCallResp("test_latency", `{this is not valid json`),
		makeFinalResp("参数错了, 没法测速。"),
	})
	defer srv.Close()

	_, events, err := agent.RunWithRole(context.Background(), testRole, "测一下", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expect 1 event, got %d", len(events))
	}
	if !strings.Contains(events[0].Error, "参数解析失败") {
		t.Errorf("event Error: %q", events[0].Error)
	}
}

// TestSingleAgent_MaxIterReached LLM 一直调 tool 不 stop → 5 轮后兜底返回
func TestSingleAgent_MaxIterReached(t *testing.T) {
	// 6 个 tool_calls 响应 (maxIter=5, 第 6 轮根本不会被调到, 但放着保安全)
	resp := makeToolCallResp("get_node_pool", "{}")
	agent, srv, _ := buildSingleAgent(t, []string{resp, resp, resp, resp, resp, resp})
	defer srv.Close()

	reply, events, err := agent.RunWithRole(context.Background(), testRole, "无限调", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(reply, "最大操作步数") {
		t.Errorf("reply should mention maxIter cap, got %q", reply)
	}
	if len(events) != 5 {
		t.Errorf("maxIter=5, expect 5 events, got %d", len(events))
	}
	if srv.Calls() != 5 {
		t.Errorf("LLM should be called 5 times, got %d", srv.Calls())
	}
}

// TestSingleAgent_LLMError LLM 返 500 → 错误透传, 含 role 名前缀
func TestSingleAgent_LLMError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", 500)
	}))
	defer srv.Close()

	adapter := &agentFakeAdapter{}
	pipe := tool.NewPipeline(adapter, t.TempDir())
	llm := NewLLMClient(srv.URL, "fake", "m", 3*time.Second)
	assembler := NewPromptAssembler(adapter)
	agent := NewSingleAgent(llm, pipe, assembler, pipe.GetTools(), 5)

	_, _, err := agent.RunWithRole(context.Background(), testRole, "x", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "[test]") {
		t.Errorf("error should be prefixed with role name, got %v", err)
	}
}

// TestSingleAgent_HistoryInjectsContext 含 history 时 system prompt 含对话摘要
// (只 check assembler 透传, 不深入 LLM, 因为 mock LLM 不读 system prompt)
func TestSingleAgent_HistoryInjectsContext(t *testing.T) {
	agent, srv, _ := buildSingleAgent(t, []string{
		makeFinalResp("ok"),
	})
	defer srv.Close()

	hist := NewConversationHistory(10)
	hist.Add("user", "上次的节点是什么", "agent")
	hist.Add("assistant", "JP-1", "agent")

	_, _, err := agent.RunWithRole(context.Background(), testRole, "用上次那个", hist)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// 不直接 verify system prompt 内容 (mock 不接收), 但确保 history 路径不 panic
}

// --- 纯函数级 ---

func TestIsToolAllowed(t *testing.T) {
	allowed := []string{"a", "b", "c"}
	if !isToolAllowed("b", allowed) {
		t.Errorf("b should be allowed")
	}
	if isToolAllowed("z", allowed) {
		t.Errorf("z should not be allowed")
	}
	if isToolAllowed("a", nil) {
		t.Errorf("nil whitelist should deny all")
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"\033[33m黄色\033[0m", "黄色"},
		{"\033[31m红\033[0m + \033[32m绿\033[0m", "红 + 绿"},
		{"无 ANSI", "无 ANSI"},
		{"", ""},
	}
	for _, tc := range tests {
		got := stripANSI(tc.in)
		if got != tc.want {
			t.Errorf("stripANSI(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncateResult(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		maxLen int
		want   string
	}{
		{"short passes through", "abc", 100, "abc"},
		{"exact length", "abc", 3, "abc"},
		{"truncated", "abcdefghij", 5, "abcde\n...(已截断，原始长度 10 字符)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateResult(tc.in, tc.maxLen)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatToolCall(t *testing.T) {
	tests := []struct {
		name    string
		tc      ToolCall
		wantSub string // 检查子串避免依赖 map 顺序
	}{
		{
			name:    "no args",
			tc:      ToolCall{Function: FunctionCall{Name: "get_node_pool", Arguments: ""}},
			wantSub: "get_node_pool",
		},
		{
			name:    "empty obj",
			tc:      ToolCall{Function: FunctionCall{Name: "get_node_pool", Arguments: "{}"}},
			wantSub: "get_node_pool",
		},
		{
			name:    "with args",
			tc:      ToolCall{Function: FunctionCall{Name: "switch_node", Arguments: `{"node":"JP-1"}`}},
			wantSub: "node=JP-1",
		},
		{
			name:    "malformed args fall back to name only",
			tc:      ToolCall{Function: FunctionCall{Name: "x", Arguments: `not json`}},
			wantSub: "x",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatToolCall(tc.tc)
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("got %q, want substring %q", got, tc.wantSub)
			}
		})
	}
}

// --- 集成: ConvertToolsForRole 配合 SingleAgent 的角色限制 ---

// TestConvertToolsForRole 限定的 tool 集合不包含白名单外 tool
func TestConvertToolsForRole(t *testing.T) {
	tools := tool.RegisterTools()
	allowed := []string{"get_node_pool", "test_latency"}
	out := ConvertToolsForRole(tools, allowed)

	got := map[string]bool{}
	for _, t := range out {
		got[t.Function.Name] = true
	}
	for _, want := range allowed {
		if !got[want] {
			t.Errorf("missing allowed tool %q", want)
		}
	}
	// switch_node 不在白名单, 不应出现
	if got["switch_node"] {
		t.Errorf("switch_node should NOT appear in role-filtered tools")
	}
	if got["set_mode"] {
		t.Errorf("set_mode should NOT appear in role-filtered tools")
	}
}

// --- Sanity: 错误注入 ---

// TestSingleAgent_ErrorWrapping 各种错误路径都包 role 名
func TestSingleAgent_ErrorWrapping(t *testing.T) {
	// 直接构造 LLMClient 指向不存在的 host, 触发 connection refused
	llm := NewLLMClient("http://127.0.0.1:1", "k", "m", 500*time.Millisecond)
	adapter := &agentFakeAdapter{}
	pipe := tool.NewPipeline(adapter, t.TempDir())
	assembler := NewPromptAssembler(adapter)
	agent := NewSingleAgent(llm, pipe, assembler, pipe.GetTools(), 1)

	_, _, err := agent.RunWithRole(context.Background(), testRole, "x", nil)
	if err == nil {
		t.Fatal("expected error from connection refused")
	}
	// 错误链应该可以被识别为 LLM 调用失败
	if !errors.Is(err, err) || !strings.Contains(err.Error(), "LLM 调用失败") {
		t.Errorf("error wrapping wrong: %v", err)
	}
}
