package tool

import (
	"sync"
	"testing"
)

// recordingApprover 记录 Ask 调用次数和参数, 按预设序列返回 decision
type recordingApprover struct {
	mu       sync.Mutex
	calls    []string // toolName 序列
	decision PermissionDecision
}

func (r *recordingApprover) Ask(toolName string, _ map[string]interface{}) PermissionDecision {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, toolName)
	return r.decision
}

func (r *recordingApprover) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// TestPermission_TrustAuto auto 模式所有写操作直接放行, 不调 approver
func TestPermission_TrustAuto(t *testing.T) {
	approver := &recordingApprover{decision: PermDeny} // approver 即使返回 deny 也不该被调
	pm := NewPermissionManager(TrustAuto, approver)

	d := pm.Check("switch_node", nil)
	if d != PermAllowOnce {
		t.Errorf("auto mode: want PermAllowOnce, got %q", d)
	}
	if approver.callCount() != 0 {
		t.Errorf("auto mode: approver should not be called, got %d calls", approver.callCount())
	}
}

// TestPermission_TrustStrict strict 模式所有写操作直接拒, 不调 approver
func TestPermission_TrustStrict(t *testing.T) {
	approver := &recordingApprover{decision: PermAllowOnce}
	pm := NewPermissionManager(TrustStrict, approver)

	d := pm.Check("switch_node", nil)
	if d != PermDeny {
		t.Errorf("strict mode: want PermDeny, got %q", d)
	}
	if approver.callCount() != 0 {
		t.Errorf("strict mode: approver should not be called, got %d calls", approver.callCount())
	}
}

// TestPermission_AskWithoutApproverDenies 锁 #M5 文档化的安全失败语义:
// ask 模式但没注入 approver 时, 必须返回 PermDeny (默认拒绝, 不"放任跑")。
func TestPermission_AskWithoutApproverDenies(t *testing.T) {
	pm := NewPermissionManager(TrustAsk, nil)
	d := pm.Check("switch_node", nil)
	if d != PermDeny {
		t.Errorf("ask mode without approver: want PermDeny (safe fail), got %q", d)
	}
}

// TestPermission_AskDelegatesToApprover ask 模式有 approver 时透传决策
func TestPermission_AskDelegatesToApprover(t *testing.T) {
	tests := []struct {
		decision PermissionDecision
		want     PermissionDecision
	}{
		{PermAllowOnce, PermAllowOnce},
		{PermAllowSession, PermAllowSession},
		{PermDeny, PermDeny},
	}
	for _, tc := range tests {
		t.Run(string(tc.decision), func(t *testing.T) {
			approver := &recordingApprover{decision: tc.decision}
			pm := NewPermissionManager(TrustAsk, approver)
			got := pm.Check("switch_node", nil)
			if got != tc.want {
				t.Errorf("approver %q: want %q got %q", tc.decision, tc.want, got)
			}
		})
	}
}

// TestPermission_AllowSessionWhitelistsTool allow_session 决定后, 同名 tool 后续调用
// 直接放行, 不再问 approver。 不同 tool 名仍要问。
func TestPermission_AllowSessionWhitelistsTool(t *testing.T) {
	approver := &recordingApprover{decision: PermAllowSession}
	pm := NewPermissionManager(TrustAsk, approver)

	// 第一次 switch_node: approver 被叫, 返回 allow_session
	if d := pm.Check("switch_node", nil); d != PermAllowSession {
		t.Fatalf("first call: want PermAllowSession, got %q", d)
	}
	if approver.callCount() != 1 {
		t.Fatalf("first call: approver should be invoked once, got %d", approver.callCount())
	}

	// 第二次 switch_node: 直接放行 (PermAllowOnce 是放行的语义), 不再叫 approver
	if d := pm.Check("switch_node", nil); d != PermAllowOnce {
		t.Errorf("session-whitelisted tool: want PermAllowOnce, got %q", d)
	}
	if approver.callCount() != 1 {
		t.Errorf("session-whitelisted tool: approver should NOT be called again, got %d total calls", approver.callCount())
	}

	// 不同 tool 名: 仍要问
	if d := pm.Check("set_per_app_vpn", nil); d != PermAllowSession {
		t.Errorf("different tool: should re-ask approver, got %q", d)
	}
	if approver.callCount() != 2 {
		t.Errorf("different tool name: approver should be called again, got %d", approver.callCount())
	}
}

// TestPermission_AllowOnceDoesNotWhitelist allow_once 不入 session 白名单, 下次还要问
func TestPermission_AllowOnceDoesNotWhitelist(t *testing.T) {
	approver := &recordingApprover{decision: PermAllowOnce}
	pm := NewPermissionManager(TrustAsk, approver)

	pm.Check("switch_node", nil)
	pm.Check("switch_node", nil)
	if approver.callCount() != 2 {
		t.Errorf("allow_once should NOT whitelist; expect 2 approver calls, got %d", approver.callCount())
	}
}

// TestPermission_ResetSessionClearsWhitelist ResetSession 后白名单清空
func TestPermission_ResetSessionClearsWhitelist(t *testing.T) {
	approver := &recordingApprover{decision: PermAllowSession}
	pm := NewPermissionManager(TrustAsk, approver)

	pm.Check("switch_node", nil) // approver 1, 加入白名单
	pm.Check("switch_node", nil) // 直接放行, 不叫 approver
	if approver.callCount() != 1 {
		t.Fatalf("setup: expect 1 approver call, got %d", approver.callCount())
	}

	pm.ResetSession()

	pm.Check("switch_node", nil) // 白名单清了, approver 重新被叫
	if approver.callCount() != 2 {
		t.Errorf("after ResetSession: expect 2 approver calls total, got %d", approver.callCount())
	}
}

// TestPermission_SetModeRuntime 运行时切 mode 立即生效
func TestPermission_SetModeRuntime(t *testing.T) {
	approver := &recordingApprover{decision: PermAllowOnce}
	pm := NewPermissionManager(TrustStrict, approver)

	if d := pm.Check("switch_node", nil); d != PermDeny {
		t.Fatalf("strict initial: want PermDeny, got %q", d)
	}

	pm.SetMode(TrustAuto)
	if pm.Mode() != TrustAuto {
		t.Errorf("Mode() should reflect SetMode, got %q", pm.Mode())
	}
	if d := pm.Check("switch_node", nil); d != PermAllowOnce {
		t.Errorf("after SetMode(auto): want PermAllowOnce, got %q", d)
	}
}

// TestAutoApprover_AlwaysAllowOnce 不管 toolName / params 都返回 allow_once
func TestAutoApprover_AlwaysAllowOnce(t *testing.T) {
	a := AutoApprover{}
	if d := a.Ask("any_tool", map[string]interface{}{"x": 1}); d != PermAllowOnce {
		t.Errorf("AutoApprover.Ask: want PermAllowOnce, got %q", d)
	}
	if d := a.Ask("", nil); d != PermAllowOnce {
		t.Errorf("AutoApprover.Ask empty input: want PermAllowOnce, got %q", d)
	}
}
