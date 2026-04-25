package tool

import (
	"strings"
	"testing"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// hookFakeAdapter 仅记录 TestLatency 调用的 timeout, 让我们断言 chain vs 单跳的差异
type hookFakeAdapter struct {
	engine.EngineAdapter
	gotTag     string
	gotTimeout time.Duration
	returnMs   int
	returnErr  error
}

func (a *hookFakeAdapter) TestLatency(tag string, _ string, timeout time.Duration) (int, error) {
	a.gotTag = tag
	a.gotTimeout = timeout
	return a.returnMs, a.returnErr
}

// TestLatencyCheck_NormalNodeUses3sTimeout 普通单跳节点保持 3s timeout
func TestLatencyCheck_NormalNodeUses3sTimeout(t *testing.T) {
	a := &hookFakeAdapter{returnMs: 120}
	h := &LatencyCheckPostHook{}
	st := h.Run("switch_node", map[string]interface{}{"node": "HK-1"}, nil, a)
	if !st.OK {
		t.Errorf("healthy node should pass, got reason=%q", st.Reason)
	}
	if a.gotTimeout != 3*time.Second {
		t.Errorf("normal node timeout should be 3s, got %v", a.gotTimeout)
	}
	if a.gotTag != "HK-1" {
		t.Errorf("tag passed wrong: %q", a.gotTag)
	}
}

// TestLatencyCheck_ChainNodeUses8sTimeout `_agent:` 前缀视为链式代理, timeout 8s
func TestLatencyCheck_ChainNodeUses8sTimeout(t *testing.T) {
	a := &hookFakeAdapter{returnMs: 4500}
	h := &LatencyCheckPostHook{}
	st := h.Run("switch_node", map[string]interface{}{"node": "_agent:chain-jp-hk"}, nil, a)
	if !st.OK {
		t.Errorf("4500ms within 8s should pass for chain, got reason=%q", st.Reason)
	}
	if a.gotTimeout != 8*time.Second {
		t.Errorf("chain timeout should be 8s, got %v", a.gotTimeout)
	}
}

// TestLatencyCheck_ChainTimeoutHasFriendlyReason chain 超时给的错误 reason 解释多跳成本
func TestLatencyCheck_ChainTimeoutHasFriendlyReason(t *testing.T) {
	a := &hookFakeAdapter{returnMs: 0} // 模拟超时返回 0
	h := &LatencyCheckPostHook{}
	st := h.Run("switch_node", map[string]interface{}{"node": "_agent:chain-jp-hk"}, nil, a)
	if st.OK {
		t.Fatal("0ms should fail")
	}
	if !strings.Contains(st.Reason, "链式代理") {
		t.Errorf("chain-specific reason missing: %q", st.Reason)
	}
	if !strings.Contains(st.Reason, "8s") {
		t.Errorf("reason should mention 8s threshold: %q", st.Reason)
	}
}

// TestLatencyCheck_NormalTimeoutKeepsOriginalReason 单跳超时不要混淆为链式
func TestLatencyCheck_NormalTimeoutKeepsOriginalReason(t *testing.T) {
	a := &hookFakeAdapter{returnMs: 0}
	h := &LatencyCheckPostHook{}
	st := h.Run("switch_node", map[string]interface{}{"node": "HK-1"}, nil, a)
	if st.OK {
		t.Fatal("0ms should fail")
	}
	if strings.Contains(st.Reason, "链式代理") {
		t.Errorf("normal node fail should not mention 链式代理: %q", st.Reason)
	}
	if !strings.Contains(st.Reason, "HK-1") {
		t.Errorf("reason should name node: %q", st.Reason)
	}
}

// TestLatencyCheck_SkipsNonSwitchTools switch_node 和 set_mode 之外不做检查
func TestLatencyCheck_SkipsNonSwitchTools(t *testing.T) {
	a := &hookFakeAdapter{returnMs: 0, returnErr: nil}
	h := &LatencyCheckPostHook{}
	st := h.Run("get_node_pool", nil, nil, a)
	if !st.OK {
		t.Errorf("non-switch tool should always pass, got %q", st.Reason)
	}
	if a.gotTag != "" {
		t.Errorf("TestLatency should not have been called for read tool, but got tag=%q", a.gotTag)
	}
}

// TestLatencyCheck_SetModeDirectSkips set_mode mode=direct 不需要测延迟 (直连)
func TestLatencyCheck_SetModeDirectSkips(t *testing.T) {
	a := &hookFakeAdapter{returnMs: 0}
	h := &LatencyCheckPostHook{}
	st := h.Run("set_mode", map[string]interface{}{"mode": "direct"}, nil, a)
	if !st.OK {
		t.Errorf("direct mode should pass without latency check, got %q", st.Reason)
	}
	if a.gotTag != "" {
		t.Errorf("direct mode should NOT call TestLatency")
	}
}

// TestLatencyCheck_NoNodeSkips 没 node 字段时直接放过 (避免误报)
func TestLatencyCheck_NoNodeSkips(t *testing.T) {
	a := &hookFakeAdapter{returnMs: 0}
	h := &LatencyCheckPostHook{}
	st := h.Run("switch_node", map[string]interface{}{}, nil, a)
	if !st.OK {
		t.Errorf("missing node should pass (PreHook 应该先拦, 这里防御性 skip), got %q", st.Reason)
	}
	if a.gotTag != "" {
		t.Errorf("missing node should NOT call TestLatency")
	}
}

// TestLatencyCheck_SetModeFromResultData set_mode 从 result.Data 推断当前节点
func TestLatencyCheck_SetModeFromResultData(t *testing.T) {
	a := &hookFakeAdapter{returnMs: 200}
	h := &LatencyCheckPostHook{}
	res := &ToolResult{Data: map[string]string{"node": "JP-1"}}
	st := h.Run("set_mode", map[string]interface{}{"mode": "global"}, res, a)
	if !st.OK {
		t.Errorf("global mode with healthy backing node should pass, got %q", st.Reason)
	}
	if a.gotTag != "JP-1" {
		t.Errorf("should test the node from result.Data, got %q", a.gotTag)
	}
	if a.gotTimeout != 3*time.Second {
		t.Errorf("non-chain node should use 3s timeout, got %v", a.gotTimeout)
	}
}
