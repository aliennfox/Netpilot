package tool

import (
	"fmt"
	"strings"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// --- Hook interfaces ---

type PreHook interface {
	Name() string
	Run(toolName string, params map[string]interface{}) HookDecision
}

type PostHook interface {
	Name() string
	Run(toolName string, params map[string]interface{}, result *ToolResult, adapter engine.EngineAdapter) HealthStatus
}

type FailureHook interface {
	Name() string
	Run(toolName string, params map[string]interface{}, err error) string
}

type HookDecision struct {
	Action string // "allow", "deny", "ask_user"
	Reason string
}

type HealthStatus struct {
	OK     bool
	Reason string
}

// --- Hook Engine ---

type HookEngine struct {
	preHooks     []PreHook
	postHooks    []PostHook
	failureHooks []FailureHook
}

func NewHookEngine(adapter engine.EngineAdapter) *HookEngine {
	return &HookEngine{
		preHooks:     []PreHook{&LoggingPreHook{}},
		postHooks:    []PostHook{&LatencyCheckPostHook{}},
		failureHooks: []FailureHook{&DefaultFailureHook{}},
	}
}

func (h *HookEngine) RunPreHooks(toolName string, params map[string]interface{}) HookDecision {
	for _, hook := range h.preHooks {
		decision := hook.Run(toolName, params)
		if decision.Action != "allow" {
			return decision
		}
	}
	return HookDecision{Action: "allow"}
}

func (h *HookEngine) RunPostHooks(toolName string, params map[string]interface{}, result *ToolResult, adapter engine.EngineAdapter) HealthStatus {
	for _, hook := range h.postHooks {
		status := hook.Run(toolName, params, result, adapter)
		if !status.OK {
			return status
		}
	}
	return HealthStatus{OK: true}
}

func (h *HookEngine) RunFailureHooks(toolName string, params map[string]interface{}, err error) string {
	for _, hook := range h.failureHooks {
		msg := hook.Run(toolName, params, err)
		if msg != "" {
			return msg
		}
	}
	return err.Error()
}

// --- LoggingPreHook: logs every write operation ---

type LoggingPreHook struct{}

func (h *LoggingPreHook) Name() string { return "logging" }

func (h *LoggingPreHook) Run(toolName string, params map[string]interface{}) HookDecision {
	fmt.Printf("\033[33m[Pre-Hook] 即将执行: %s %v\033[0m\n", toolName, formatParams(params))
	return HookDecision{Action: "allow"}
}

// --- LatencyCheckPostHook: verifies new node is healthy after switch ---

type LatencyCheckPostHook struct{}

func (h *LatencyCheckPostHook) Name() string { return "latency_check" }

func (h *LatencyCheckPostHook) Run(toolName string, params map[string]interface{}, result *ToolResult, adapter engine.EngineAdapter) HealthStatus {
	// Only check for switch_node and set_mode
	if toolName != "switch_node" && toolName != "set_mode" {
		return HealthStatus{OK: true}
	}

	// For set_mode direct, skip latency check — direct doesn't need it
	if mode, ok := params["mode"]; ok && mode == "direct" {
		return HealthStatus{OK: true}
	}

	// Determine which node to test
	node := ""
	if n, ok := params["node"]; ok {
		node, _ = n.(string)
	}
	if node == "" {
		// For set_mode, the node was set by the tool — get it from the result data
		if result != nil && result.Data != nil {
			if d, ok := result.Data.(map[string]string); ok {
				node = d["node"]
			}
		}
	}
	if node == "" || node == "direct-out" {
		return HealthStatus{OK: true}
	}

	// `_agent:` 前缀的链式代理 (create_chain 产出) 走多跳, 单次 generate_204
	// 端到端时延天然 2-5s, 给 8s; 普通单跳节点维持 3s。
	timeout := 3 * time.Second
	isChain := strings.HasPrefix(node, "_agent:")
	if isChain {
		timeout = 8 * time.Second
	}

	ms, err := adapter.TestLatency(node, "https://www.gstatic.com/generate_204", timeout)
	if err != nil || ms <= 0 {
		var reason string
		if isChain {
			reason = fmt.Sprintf("链式代理 %s 延迟测试超时 (>%s); 多跳出口本身较慢, 也可能是其中一跳节点不通", node, timeout)
		} else {
			reason = fmt.Sprintf("节点 %s 延迟测试超时", node)
		}
		fmt.Printf("\033[31m[Post-Hook] 延迟检查: %s 超时 ✗\033[0m\n", node)
		return HealthStatus{OK: false, Reason: reason}
	}

	fmt.Printf("\033[32m[Post-Hook] 延迟检查: %s %dms ✓\033[0m\n", node, ms)
	return HealthStatus{OK: true}
}

// --- DefaultFailureHook: formats error messages ---

type DefaultFailureHook struct{}

func (h *DefaultFailureHook) Name() string { return "default_failure" }

func (h *DefaultFailureHook) Run(toolName string, params map[string]interface{}, err error) string {
	return fmt.Sprintf("操作 %s 失败: %v", toolName, err)
}

// --- helpers ---

func formatParams(params map[string]interface{}) string {
	if len(params) == 0 {
		return "{}"
	}
	s := "{"
	first := true
	for k, v := range params {
		if !first {
			s += ", "
		}
		s += fmt.Sprintf("%s:%v", k, v)
		first = false
	}
	s += "}"
	return s
}
