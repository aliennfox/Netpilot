package tool

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// --- 测试 helper ---

// stubPreHook 可注入的 PreHook
type stubPreHook struct {
	decision HookDecision
	calls    int
}

func (s *stubPreHook) Name() string { return "stub-pre" }
func (s *stubPreHook) Run(string, map[string]interface{}) HookDecision {
	s.calls++
	return s.decision
}

// stubPostHook 可注入的 PostHook
type stubPostHook struct {
	status HealthStatus
	calls  int
}

func (s *stubPostHook) Name() string { return "stub-post" }
func (s *stubPostHook) Run(string, map[string]interface{}, *ToolResult, engine.EngineAdapter) HealthStatus {
	s.calls++
	return s.status
}

// 自定义 ToolDef factory: 可控 Execute 行为
func mkWriteTool(name string, exec func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error)) *ToolDef {
	return &ToolDef{Name: name, IsWriteOp: true, Execute: exec}
}

func mkReadTool(name string, exec func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error)) *ToolDef {
	return &ToolDef{Name: name, IsWriteOp: false, Execute: exec}
}

// 构造 pipeline 用 stub hooks (跳过 Logging/LatencyCheck 默认 hook 的副作用), 真 SnapshotStore + Telemetry, mock adapter
func buildTestPipeline(t *testing.T, adapter engine.EngineAdapter, perms *PermissionManager,
	preHooks []PreHook, postHooks []PostHook,
) (*ToolPipeline, *stubPreHook, *stubPostHook) {
	t.Helper()
	pre := &stubPreHook{decision: HookDecision{Action: "allow"}}
	post := &stubPostHook{status: HealthStatus{OK: true}}
	if preHooks == nil {
		preHooks = []PreHook{pre}
	}
	if postHooks == nil {
		postHooks = []PostHook{post}
	}
	if perms == nil {
		perms = NewPermissionManager(TrustAuto, nil)
	}
	pipe := &ToolPipeline{
		tools: map[string]*ToolDef{},
		hooks: &HookEngine{
			preHooks:     preHooks,
			postHooks:    postHooks,
			failureHooks: []FailureHook{&DefaultFailureHook{}},
		},
		snapshots: NewSnapshotStore(t.TempDir()),
		telemetry: NewTelemetryLogger(t.TempDir()),
		adapter:   adapter,
		perms:     perms,
	}
	return pipe, pre, post
}

// TestPipeline_ReadToolBypassesHooksAndSnapshot 读 tool 不走 hook / snapshot
func TestPipeline_ReadToolBypassesHooksAndSnapshot(t *testing.T) {
	adapter := newFakeAdapter()
	pipe, pre, post := buildTestPipeline(t, adapter, nil, nil, nil)

	pipe.tools["get_status"] = mkReadTool("get_status",
		func(context.Context, engine.EngineAdapter, map[string]interface{}) (*ToolResult, error) {
			return &ToolResult{Success: true, Message: "ok"}, nil
		})

	res := pipe.Execute(context.Background(), "get_status", nil)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
	if pre.calls != 0 {
		t.Errorf("read tool should not invoke PreHook, got %d calls", pre.calls)
	}
	if post.calls != 0 {
		t.Errorf("read tool should not invoke PostHook, got %d calls", post.calls)
	}
	if got := pipe.snapshots.Latest(); got != nil {
		t.Errorf("read tool should not create snapshot, got %+v", got)
	}
	if entries := pipe.telemetry.Recent(10); len(entries) != 1 || !entries[0].Success {
		t.Errorf("read tool should still emit success telemetry, got %+v", entries)
	}
}

// TestPipeline_WriteToolFullCycle 写 tool 走完整 8 步, 全 OK
func TestPipeline_WriteToolFullCycle(t *testing.T) {
	adapter := newFakeAdapter()
	pipe, pre, post := buildTestPipeline(t, adapter, nil, nil, nil)

	pipe.tools["switch_node"] = mkWriteTool("switch_node",
		func(_ context.Context, a engine.EngineAdapter, p map[string]interface{}) (*ToolResult, error) {
			node, _ := p["node"].(string)
			if err := a.SetActiveProxy("proxy-group", node); err != nil {
				return nil, err
			}
			return &ToolResult{Success: true, Message: "switched"}, nil
		})

	res := pipe.Execute(context.Background(), "switch_node", map[string]interface{}{"node": "JP-1"})
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
	if pre.calls != 1 || post.calls != 1 {
		t.Errorf("expect both hooks called once, pre=%d post=%d", pre.calls, post.calls)
	}
	if pipe.snapshots.Latest() == nil {
		t.Errorf("write tool should create snapshot")
	}
	entries := pipe.telemetry.Recent(10)
	if len(entries) != 1 || !entries[0].Success || entries[0].SnapshotID == "" {
		t.Errorf("telemetry entry wrong: %+v", entries)
	}
	if entries[0].Tool != "switch_node" || entries[0].RolledBack {
		t.Errorf("telemetry fields: %+v", entries[0])
	}
	// 真实切到了 JP-1
	if adapter.groups["proxy-group"].Now != "JP-1" {
		t.Errorf("expected adapter switched to JP-1, got %q", adapter.groups["proxy-group"].Now)
	}
}

// TestPipeline_PermissionDeny strict mode 拒写 → 不创建 snapshot, telemetry error
func TestPipeline_PermissionDeny(t *testing.T) {
	adapter := newFakeAdapter()
	perms := NewPermissionManager(TrustStrict, nil)
	pipe, pre, post := buildTestPipeline(t, adapter, perms, nil, nil)

	executed := false
	pipe.tools["switch_node"] = mkWriteTool("switch_node",
		func(context.Context, engine.EngineAdapter, map[string]interface{}) (*ToolResult, error) {
			executed = true
			return &ToolResult{Success: true}, nil
		})

	res := pipe.Execute(context.Background(), "switch_node", nil)
	if res.Success {
		t.Fatalf("strict mode should deny, got %+v", res)
	}
	if !strings.Contains(res.Message, "拒绝") {
		t.Errorf("deny message should mention 拒绝, got %q", res.Message)
	}
	if executed {
		t.Errorf("Execute should not be called when permission denies")
	}
	if pre.calls != 0 || post.calls != 0 {
		t.Errorf("hooks should not run on permission deny, pre=%d post=%d", pre.calls, post.calls)
	}
	if pipe.snapshots.Latest() != nil {
		t.Errorf("no snapshot on permission deny")
	}
	entries := pipe.telemetry.Recent(10)
	if len(entries) != 1 || entries[0].Success || !strings.Contains(entries[0].Error, "permission denied") {
		t.Errorf("telemetry should record permission denial: %+v", entries)
	}
}

// TestPipeline_PreHookDeny PreHook deny 阻断执行, 不进 snapshot/Execute/PostHook
func TestPipeline_PreHookDeny(t *testing.T) {
	adapter := newFakeAdapter()
	denyPre := &stubPreHook{decision: HookDecision{Action: "deny", Reason: "policy violation"}}
	post := &stubPostHook{status: HealthStatus{OK: true}}
	pipe, _, _ := buildTestPipeline(t, adapter, nil, []PreHook{denyPre}, []PostHook{post})

	executed := false
	pipe.tools["switch_node"] = mkWriteTool("switch_node",
		func(context.Context, engine.EngineAdapter, map[string]interface{}) (*ToolResult, error) {
			executed = true
			return &ToolResult{Success: true}, nil
		})

	res := pipe.Execute(context.Background(), "switch_node", nil)
	if res.Success {
		t.Fatalf("PreHook deny should fail the call, got %+v", res)
	}
	if !strings.Contains(res.Message, "policy violation") {
		t.Errorf("deny message should include reason, got %q", res.Message)
	}
	if executed {
		t.Errorf("Execute should not run after PreHook deny")
	}
	if post.calls != 0 {
		t.Errorf("PostHook should not run after PreHook deny, got %d", post.calls)
	}
}

// TestPipeline_ExecuteFailureAutoRollback Execute 抛 err → 自动回滚 + telemetry rolled_back=true
func TestPipeline_ExecuteFailureAutoRollback(t *testing.T) {
	adapter := newFakeAdapter()
	pipe, _, _ := buildTestPipeline(t, adapter, nil, nil, nil)

	pipe.tools["switch_node"] = mkWriteTool("switch_node",
		func(_ context.Context, a engine.EngineAdapter, p map[string]interface{}) (*ToolResult, error) {
			// 模拟先切了节点再失败 (snapshot 在 execute 前已存了 HK-1, rollback 应回到 HK-1)
			a.SetActiveProxy("proxy-group", "JP-1")
			return nil, errors.New("simulated execute failure")
		})

	res := pipe.Execute(context.Background(), "switch_node", map[string]interface{}{"node": "JP-1"})
	if res.Success {
		t.Fatalf("expected failure, got %+v", res)
	}
	if !strings.Contains(res.Message, "simulated execute failure") {
		t.Errorf("error msg should propagate: %q", res.Message)
	}
	// rollback 应该把 group 恢复到 HK-1
	if adapter.groups["proxy-group"].Now != "HK-1" {
		t.Errorf("auto rollback should restore HK-1, got %q", adapter.groups["proxy-group"].Now)
	}
	entries := pipe.telemetry.Recent(10)
	if len(entries) != 1 || entries[0].Success || !entries[0].RolledBack {
		t.Errorf("telemetry: expect failure with rolled_back=true, got %+v", entries)
	}
}

// TestPipeline_PostHookUnhealthyAutoRollback PostHook 不健康 → 自动回滚, 用户看到"自动回滚"消息
func TestPipeline_PostHookUnhealthyAutoRollback(t *testing.T) {
	adapter := newFakeAdapter()
	unhealthy := &stubPostHook{status: HealthStatus{OK: false, Reason: "latency check timeout"}}
	pre := &stubPreHook{decision: HookDecision{Action: "allow"}}
	pipe, _, _ := buildTestPipeline(t, adapter, nil, []PreHook{pre}, []PostHook{unhealthy})

	pipe.tools["switch_node"] = mkWriteTool("switch_node",
		func(_ context.Context, a engine.EngineAdapter, p map[string]interface{}) (*ToolResult, error) {
			a.SetActiveProxy("proxy-group", "JP-1")
			return &ToolResult{Success: true}, nil
		})

	res := pipe.Execute(context.Background(), "switch_node", map[string]interface{}{"node": "JP-1"})
	if res.Success {
		t.Fatalf("expected failure due to unhealthy post-hook, got %+v", res)
	}
	if !strings.Contains(res.Message, "已自动回滚") || !strings.Contains(res.Message, "latency check timeout") {
		t.Errorf("user msg should mention rollback + reason, got %q", res.Message)
	}
	if adapter.groups["proxy-group"].Now != "HK-1" {
		t.Errorf("post-hook unhealthy should rollback to HK-1, got %q", adapter.groups["proxy-group"].Now)
	}
	entries := pipe.telemetry.Recent(10)
	if len(entries) != 1 || !entries[0].RolledBack {
		t.Errorf("telemetry rolled_back should be true, got %+v", entries)
	}
}

// TestPipeline_PostHookUnhealthyButRollbackFails 锁 #M2 修补的端到端场景:
// PostHook 不健康触发 rollback, 但 rollback 也失败 → 用户消息升级为"配置不一致"
// telemetry RolledBack=false (反映真实状态而非"假装回滚成功")
func TestPipeline_PostHookUnhealthyButRollbackFails(t *testing.T) {
	adapter := newFakeAdapter()
	unhealthy := &stubPostHook{status: HealthStatus{OK: false, Reason: "node down"}}
	pre := &stubPreHook{decision: HookDecision{Action: "allow"}}
	pipe, _, _ := buildTestPipeline(t, adapter, nil, []PreHook{pre}, []PostHook{unhealthy})

	pipe.tools["switch_node"] = mkWriteTool("switch_node",
		func(_ context.Context, a engine.EngineAdapter, p map[string]interface{}) (*ToolResult, error) {
			a.SetActiveProxy("proxy-group", "JP-1")
			// 让接下来的 rollback (也是 SetActiveProxy) 失败 — 模拟 Clash API 500 / 网络断
			f := a.(*fakeAdapter)
			f.setProxyErr = errors.New("clash api 500")
			return &ToolResult{Success: true}, nil
		})

	res := pipe.Execute(context.Background(), "switch_node", map[string]interface{}{"node": "JP-1"})
	if res.Success {
		t.Fatalf("expected failure, got %+v", res)
	}
	if !strings.Contains(res.Message, "配置可能处于不一致状态") {
		t.Errorf("when rollback fails, user msg should mention 不一致, got %q", res.Message)
	}
	if !strings.Contains(res.Message, "clash api 500") {
		t.Errorf("user msg should contain rollback error: %q", res.Message)
	}
	entries := pipe.telemetry.Recent(10)
	if len(entries) != 1 || entries[0].RolledBack {
		t.Errorf("telemetry RolledBack should be FALSE when rollback fails, got %+v", entries)
	}
}

// TestPipeline_UnknownTool 未知 tool → friendly message + 不写 telemetry / snapshot
func TestPipeline_UnknownTool(t *testing.T) {
	pipe, _, _ := buildTestPipeline(t, newFakeAdapter(), nil, nil, nil)

	res := pipe.Execute(context.Background(), "no_such_tool", nil)
	if res.Success {
		t.Fatalf("unknown tool should fail, got %+v", res)
	}
	if !strings.Contains(res.Message, "未知工具") {
		t.Errorf("msg should mention 未知工具, got %q", res.Message)
	}
	if entries := pipe.telemetry.Recent(10); len(entries) != 0 {
		t.Errorf("unknown tool should not emit telemetry, got %+v", entries)
	}
}

// TestPipeline_ManualSnapshotAndRollback ManualSnapshot 写入 Tool="manual",
// ManualRollback("") 用 Latest, ManualRollback(id) 显式回滚
func TestPipeline_ManualSnapshotAndRollback(t *testing.T) {
	adapter := newFakeAdapter()
	pipe, _, _ := buildTestPipeline(t, adapter, nil, nil, nil)

	// 切到 JP-1 之前先存 snap1
	r1 := pipe.ManualSnapshot()
	if !r1.Success {
		t.Fatalf("ManualSnapshot: %+v", r1)
	}
	adapter.SetActiveProxy("proxy-group", "JP-1")

	// 切到 JP-1 之后存 snap2
	r2 := pipe.ManualSnapshot()
	if !r2.Success {
		t.Fatalf("ManualSnapshot 2: %+v", r2)
	}
	if adapter.groups["proxy-group"].Now != "JP-1" {
		t.Fatalf("setup: expect JP-1 active, got %q", adapter.groups["proxy-group"].Now)
	}

	// ManualRollback("") 回 Latest, 应该是 snap2 (active=JP-1) — 实际上是 no-op (group 已 JP-1)
	r3 := pipe.ManualRollback("")
	if !r3.Success {
		t.Fatalf("rollback latest: %+v", r3)
	}
	if adapter.groups["proxy-group"].Now != "JP-1" {
		t.Errorf("rollback latest should keep JP-1, got %q", adapter.groups["proxy-group"].Now)
	}

	// 显式回滚到第一个 snapshot (active=HK-1)
	all := pipe.snapshots.List()
	if len(all) != 2 {
		t.Fatalf("expect 2 snapshots, got %d", len(all))
	}
	r4 := pipe.ManualRollback(all[0].ID)
	if !r4.Success {
		t.Fatalf("explicit rollback: %+v", r4)
	}
	if adapter.groups["proxy-group"].Now != "HK-1" {
		t.Errorf("explicit rollback should restore HK-1, got %q", adapter.groups["proxy-group"].Now)
	}

	// Tool 字段穿透
	if all[0].Tool != "manual" || all[1].Tool != "manual" {
		t.Errorf("ManualSnapshot should set Tool=manual, got %q / %q", all[0].Tool, all[1].Tool)
	}
}

// TestPipeline_ManualRollback_NoSnapshots 无快照时 ManualRollback("") 显式失败
func TestPipeline_ManualRollback_NoSnapshots(t *testing.T) {
	pipe, _, _ := buildTestPipeline(t, newFakeAdapter(), nil, nil, nil)
	res := pipe.ManualRollback("")
	if res.Success {
		t.Fatalf("expected failure on no snapshots, got %+v", res)
	}
	if !strings.Contains(res.Message, "没有可用的快照") {
		t.Errorf("msg should mention 没有可用的快照, got %q", res.Message)
	}
}

// TestPipeline_VpnGuardSkippedForUnregisteredTool — VpnGuard 注册后, 不在
// requireVPNTool 列表里的 tool 不该触发 ensureFn (避免纯 overlay 写盘也强拉 VPN)
func TestPipeline_VpnGuardSkippedForUnregisteredTool(t *testing.T) {
	pipe, _, _ := buildTestPipeline(t, newFakeAdapter(), nil, nil, nil)
	guardCalls := 0
	pipe.SetVpnGuard(
		func() error { guardCalls++; return nil },
		[]string{"switch_node"},
	)
	pipe.tools["patch_route_rule"] = mkWriteTool("patch_route_rule",
		func(context.Context, engine.EngineAdapter, map[string]interface{}) (*ToolResult, error) {
			return &ToolResult{Success: true, Message: "rule added"}, nil
		})

	res := pipe.Execute(context.Background(), "patch_route_rule", nil)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
	if guardCalls != 0 {
		t.Errorf("VpnGuard should NOT fire for tool not in requireVPNTool, got %d calls", guardCalls)
	}
}

// TestPipeline_VpnGuardTriggeredForRegisteredTool — 注册了 switch_node 时,
// 调 switch_node 必须先调 ensureFn (Phase 1.5 auto-bootstrap 核心契约)
func TestPipeline_VpnGuardTriggeredForRegisteredTool(t *testing.T) {
	pipe, _, _ := buildTestPipeline(t, newFakeAdapter(), nil, nil, nil)
	guardCalls := 0
	executeCalled := false
	pipe.SetVpnGuard(
		func() error { guardCalls++; return nil },
		[]string{"switch_node", "create_chain"},
	)
	pipe.tools["switch_node"] = mkWriteTool("switch_node",
		func(_ context.Context, a engine.EngineAdapter, p map[string]interface{}) (*ToolResult, error) {
			executeCalled = true
			node, _ := p["node"].(string)
			_ = a.SetActiveProxy("proxy-group", node)
			return &ToolResult{Success: true, Message: "switched"}, nil
		})

	res := pipe.Execute(context.Background(), "switch_node", map[string]interface{}{"node": "JP-1"})
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
	if guardCalls != 1 {
		t.Errorf("VpnGuard should fire exactly once for registered tool, got %d", guardCalls)
	}
	if !executeCalled {
		t.Error("Execute should run after guard returns nil")
	}
}

// TestPipeline_VpnGuardFailureBlocksExecute — guard 报错 (用户拒绝 VPN 授权 / 超时)
// 必须阻断 Execute 且 telemetry 标 vpn-guard 错误, 用户消息明示哪个 tool 因 VPN 失败
func TestPipeline_VpnGuardFailureBlocksExecute(t *testing.T) {
	pipe, _, _ := buildTestPipeline(t, newFakeAdapter(), nil, nil, nil)
	pipe.SetVpnGuard(
		func() error { return errors.New("user denied VPN permission") },
		[]string{"create_chain"},
	)
	executed := false
	pipe.tools["create_chain"] = mkWriteTool("create_chain",
		func(context.Context, engine.EngineAdapter, map[string]interface{}) (*ToolResult, error) {
			executed = true
			return &ToolResult{Success: true}, nil
		})

	res := pipe.Execute(context.Background(), "create_chain", nil)
	if res.Success {
		t.Fatalf("guard failure should block tool, got %+v", res)
	}
	if executed {
		t.Error("Execute must NOT run when guard fails")
	}
	if !strings.Contains(res.Message, "create_chain") || !strings.Contains(res.Message, "VPN") {
		t.Errorf("user msg should name the blocked tool + VPN reason, got %q", res.Message)
	}
	entries := pipe.telemetry.Recent(10)
	if len(entries) != 1 || entries[0].Success || !strings.Contains(entries[0].Error, "vpn-guard") {
		t.Errorf("telemetry should record vpn-guard error, got %+v", entries)
	}
}
