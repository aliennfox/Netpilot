package local

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/tool"
)

// --- Mock helpers ---

// fakeAdapter 嵌入 EngineAdapter 接口, 只实现 Execute 路由 + 几个 action 真正用到的方法。
// 其它方法 nil-panic, 当作"非预期调用"信号。
type fakeAdapter struct {
	engine.EngineAdapter
	groups      map[string]*engine.ProxyGroup
	connections []engine.ConnectionInfo
}

func (f *fakeAdapter) GetProxyGroup(tag string) (*engine.ProxyGroup, error) {
	g, ok := f.groups[tag]
	if !ok {
		return nil, errors.New("group not found")
	}
	return g, nil
}
func (f *fakeAdapter) GetConnections() ([]engine.ConnectionInfo, error) {
	return f.connections, nil
}
func (f *fakeAdapter) GetProxies() ([]engine.ProxyInfo, error) {
	return []engine.ProxyInfo{
		{Tag: "proxy-group", Type: "Selector"},
		{Tag: "HK-1", Type: "shadowsocks"},
		{Tag: "JP-1", Type: "vmess"},
	}, nil
}
func (f *fakeAdapter) SetActiveProxy(string, string) error { return nil }
func (f *fakeAdapter) Reload() error                       { return nil }

func newFakeAdapter() *fakeAdapter {
	return &fakeAdapter{
		groups: map[string]*engine.ProxyGroup{
			"proxy-group": {Tag: "proxy-group", Type: "Selector", Now: "HK-1"},
		},
	}
}

// mockVpnCtrl 可控 VpnController; runningSeq 按调用顺序返回, 用尽后返回最后一个值
type mockVpnCtrl struct {
	mu         sync.Mutex
	runningSeq []bool
	idx        int
	startCalls int
	stopCalls  int
	startErr   error
	stopErr    error
}

func (m *mockVpnCtrl) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.runningSeq) == 0 {
		return false
	}
	if m.idx < len(m.runningSeq) {
		v := m.runningSeq[m.idx]
		m.idx++
		return v
	}
	return m.runningSeq[len(m.runningSeq)-1]
}
func (m *mockVpnCtrl) RequestStart() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalls++
	return m.startErr
}
func (m *mockVpnCtrl) RequestStop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalls++
	return m.stopErr
}
func (m *mockVpnCtrl) StartCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startCalls
}
func (m *mockVpnCtrl) StopCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopCalls
}

func newEngine(t *testing.T) (*Engine, *fakeAdapter) {
	t.Helper()
	adapter := newFakeAdapter()
	pipe := tool.NewPipeline(adapter, t.TempDir())
	e := NewEngine(adapter, pipe, "proxy-group")
	return e, adapter
}

// --- Engine.Execute 路由 ---

// TestEngine_Execute_UnknownAction 未知 actionID → error
func TestEngine_Execute_UnknownAction(t *testing.T) {
	e, _ := newEngine(t)
	_, err := e.Execute("no_such_action", nil)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("error msg should mention unknown action, got %v", err)
	}
}

// TestEngine_Execute_KnownActionDispatches 已知 actionID 透传, 即使 handler 报错也透传
func TestEngine_Execute_KnownActionDispatches(t *testing.T) {
	e, _ := newEngine(t)
	// vpn_status 是简单 nil-guard 路径, 没注入 vpnCtrl → 应该有清晰错误
	_, err := e.Execute("vpn_status", nil)
	if err == nil {
		t.Fatal("expected error from vpn_status without injected vpnCtrl")
	}
	if !strings.Contains(err.Error(), "VPN 控制未接入") {
		t.Errorf("error should be from action handler not unknown-action: %v", err)
	}
}

// --- VPN 三件套 ---

func TestEngine_VpnStatus_NotInjected(t *testing.T) {
	e, _ := newEngine(t)
	_, err := e.vpnStatus(nil)
	if err == nil {
		t.Fatal("expected error when vpnCtrl is nil")
	}
	if !strings.Contains(err.Error(), "VPN 控制未接入") {
		t.Errorf("error: %v", err)
	}
}

func TestEngine_VpnStatus_Running(t *testing.T) {
	e, _ := newEngine(t)
	e.SetVpnController(&mockVpnCtrl{runningSeq: []bool{true}})
	out, err := e.vpnStatus(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "正在运行") {
		t.Errorf("running output: %q", out)
	}
}

func TestEngine_VpnStatus_NotRunning(t *testing.T) {
	e, _ := newEngine(t)
	e.SetVpnController(&mockVpnCtrl{}) // empty seq → 总返 false
	out, err := e.vpnStatus(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "未在运行") {
		t.Errorf("not-running output: %q", out)
	}
}

func TestEngine_StartVpn_NotInjected(t *testing.T) {
	e, _ := newEngine(t)
	_, err := e.startVpn(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// 已运行 → 立即返回, 不调 RequestStart
func TestEngine_StartVpn_AlreadyRunning(t *testing.T) {
	e, _ := newEngine(t)
	ctrl := &mockVpnCtrl{runningSeq: []bool{true}}
	e.SetVpnController(ctrl)
	out, err := e.startVpn(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "已经在运行") {
		t.Errorf("output: %q", out)
	}
	if ctrl.StartCallCount() != 0 {
		t.Errorf("RequestStart should not be called when already running, got %d", ctrl.StartCallCount())
	}
}

// 首次 IsRunning=false 触发 RequestStart, 下一轮轮询 true → "VPN 已启动"
func TestEngine_StartVpn_BecomesReadyOnPoll(t *testing.T) {
	e, _ := newEngine(t)
	// 序列: [false, false (轮询前), true (第一次 sleep 后)]
	// WaitVpnReady 流程: IsRunning() (1) → false → RequestStart → sleep 500ms → IsRunning() (2) → ?
	// runningSeq 第 1 次 false (启动检查), 第 2 次 false (loop 第 1 次), 第 3 次 true
	ctrl := &mockVpnCtrl{runningSeq: []bool{false, false, true}}
	e.SetVpnController(ctrl)

	start := time.Now()
	out, err := e.startVpn(nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "VPN 已启动") {
		t.Errorf("output: %q", out)
	}
	if ctrl.StartCallCount() != 1 {
		t.Errorf("RequestStart should be called exactly once, got %d", ctrl.StartCallCount())
	}
	if elapsed > 2*time.Second {
		t.Errorf("should return quickly when ready on first poll, took %v", elapsed)
	}
}

func TestEngine_StopVpn_NotInjected(t *testing.T) {
	e, _ := newEngine(t)
	_, err := e.stopVpn(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// 未运行 → 立即返回 "未在运行", 不调 RequestStop
func TestEngine_StopVpn_NotRunning(t *testing.T) {
	e, _ := newEngine(t)
	ctrl := &mockVpnCtrl{}
	e.SetVpnController(ctrl)
	out, err := e.stopVpn(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "未在运行") {
		t.Errorf("output: %q", out)
	}
	if ctrl.StopCallCount() != 0 {
		t.Errorf("RequestStop should not be called when not running, got %d", ctrl.StopCallCount())
	}
}

// 运行中 happy path: 调 RequestStop 成功
func TestEngine_StopVpn_HappyPath(t *testing.T) {
	e, _ := newEngine(t)
	ctrl := &mockVpnCtrl{runningSeq: []bool{true}}
	e.SetVpnController(ctrl)
	out, err := e.stopVpn(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "已请求停止") {
		t.Errorf("output: %q", out)
	}
	if ctrl.StopCallCount() != 1 {
		t.Errorf("RequestStop should be called once, got %d", ctrl.StopCallCount())
	}
}

// RequestStop 错误透传
func TestEngine_StopVpn_RequestStopErr(t *testing.T) {
	e, _ := newEngine(t)
	ctrl := &mockVpnCtrl{
		runningSeq: []bool{true},
		stopErr:    errors.New("vpn stop ipc broken"),
	}
	e.SetVpnController(ctrl)
	_, err := e.stopVpn(nil)
	if err == nil {
		t.Fatal("expected error from RequestStop")
	}
	if !strings.Contains(err.Error(), "停止 VPN 失败") {
		t.Errorf("error should wrap, got %v", err)
	}
}

// --- 简单 read action ---

// showStatus 走 adapter.GetProxyGroup + GetConnections, 输出包含节点名和连接数
func TestEngine_ShowStatus(t *testing.T) {
	e, adapter := newEngine(t)
	adapter.connections = []engine.ConnectionInfo{
		{ID: "c1"}, {ID: "c2"},
	}

	out, err := e.showStatus(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "HK-1") {
		t.Errorf("status should mention current node HK-1: %q", out)
	}
	if !strings.Contains(out, "2") {
		t.Errorf("status should mention 2 connections: %q", out)
	}
	// HK-1 不是 direct/block, 应该报 proxy 模式
	if !strings.Contains(out, "proxy") {
		t.Errorf("mode should be proxy: %q", out)
	}
}

// showSnapshots 空 store → "没有快照"; 有 snapshot 后透传
func TestEngine_ShowSnapshots_Empty(t *testing.T) {
	e, _ := newEngine(t)
	out, err := e.showSnapshots(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "没有快照") {
		t.Errorf("empty snapshot output: %q", out)
	}
}

// 通过 pipeline.Snapshots().Save 注一个 snapshot, 看 showSnapshots 输出
func TestEngine_ShowSnapshots_NotEmpty(t *testing.T) {
	e, adapter := newEngine(t)
	id, err := e.pipeline.Snapshots().Save(adapter, "switch_node")
	if err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	out, err := e.showSnapshots(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, id) {
		t.Errorf("output should list snapshot id %q: %q", id, out)
	}
	if !strings.Contains(out, "proxy-group=HK-1") {
		t.Errorf("output should show active_proxies: %q", out)
	}
}

// showTelemetry 空 logger 返回 "没有操作记录"; 有 entry 后透传
func TestEngine_ShowTelemetry_EmptyLogger(t *testing.T) {
	e, _ := newEngine(t)
	out, err := e.showTelemetry(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "没有操作记录") {
		t.Errorf("empty telemetry: %q", out)
	}
}

// 写 entry → showTelemetry 含 tool 名
func TestEngine_ShowTelemetry_WithEntry(t *testing.T) {
	e, _ := newEngine(t)
	e.pipeline.Telemetry().Log(tool.TelemetryEntry{
		Timestamp: time.Now(),
		Tool:      "switch_node",
		Success:   true,
	})
	out, err := e.showTelemetry(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "switch_node") {
		t.Errorf("telemetry should list switch_node: %q", out)
	}
}

// --- 集成: 路由 + setter 注入 ---

// TestEngine_AllActionsRegistered Execute 应能识别全部已注册 action (route table 完整性)
func TestEngine_AllActionsRegistered(t *testing.T) {
	e, _ := newEngine(t)
	expectedActions := []string{
		"switch_best_node", "latency_test_all",
		"set_mode_global", "set_mode_direct", "set_mode_rule",
		"show_status", "show_nodes", "show_snapshots",
		"do_rollback", "show_telemetry",
		"apply_template", "list_templates",
		"list_rules", "remove_rule",
		"import_subscription", "list_subscriptions", "update_subscription", "remove_subscription",
		"show_dns", "set_dns_mode",
		"live_connections", "show_connections",
		"start_vpn", "stop_vpn", "vpn_status",
	}
	for _, action := range expectedActions {
		if _, ok := e.actions[action]; !ok {
			t.Errorf("action %q not registered", action)
		}
	}
	// Sanity: 实际数量应等于期望数量 (catch 新增 action 忘 update test)
	if len(e.actions) != len(expectedActions) {
		t.Errorf("action count drift: registered=%d expected=%d (update this test if Engine added new action)",
			len(e.actions), len(expectedActions))
	}
}
