package tool

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
	"github.com/foxnetpilot/netpilot/internal/subscription"
)

// macroFakeAdapter 嵌入 fakeAdapter, 加 TestLatency 让 macro_tools 测试能跑。
//
// latencyMap: tag → ms; 不存在的 tag 返回 0+error 模拟死节点。
// connections: 可选, GetConnections 返回它 (默认空)。
type macroFakeAdapter struct {
	fakeAdapter
	latencyMap  map[string]int
	connections []engine.ConnectionInfo
}

func (m *macroFakeAdapter) TestLatency(tag string, _ string, _ time.Duration) (int, error) {
	if ms, ok := m.latencyMap[tag]; ok && ms > 0 {
		return ms, nil
	}
	return 0, errors.New("timeout")
}

// Reload no-op — overlay.Apply 在写盘后会调 adapter.Reload, 测试不实际重启
func (m *macroFakeAdapter) Reload() error { return nil }

func (m *macroFakeAdapter) SetConfigPath(_ string) {}

// GetConnections 返回预置连接列表 (diagnose_connectivity 测试用)
func (m *macroFakeAdapter) GetConnections() ([]engine.ConnectionInfo, error) {
	return m.connections, nil
}

// newMacroAdapter 构造能跑 setup_app_chain 的 fake — 包含 4 个节点 (3 活 1 死) + selector
func newMacroAdapter() *macroFakeAdapter {
	return &macroFakeAdapter{
		fakeAdapter: fakeAdapter{
			proxies: []engine.ProxyInfo{
				{Tag: "proxy-group", Type: "Selector"},
				{Tag: "JP-1", Type: "vmess"},
				{Tag: "HK-1", Type: "shadowsocks"},
				{Tag: "DEAD-NODE", Type: "trojan"},
				{Tag: "US-1", Type: "vless"},
			},
			groups: map[string]*engine.ProxyGroup{
				"proxy-group": {Tag: "proxy-group", Type: "Selector", Now: "JP-1"},
			},
		},
		latencyMap: map[string]int{
			"JP-1": 120,
			"HK-1": 85,
			"US-1": 240,
			// DEAD-NODE 故意不在 map 里 -> 测速 fail
		},
	}
}

// newTestOverlay 构造一个临时 ConfigOverlay 指向 testdata/minimal.json
func newTestOverlay(t *testing.T) *overlay.ConfigOverlay {
	t.Helper()
	// 构造一个最小可运行的 base config
	cfg := `{
		"log": {"level": "info"},
		"outbounds": [
			{"type": "selector", "tag": "proxy-group", "outbounds": ["direct-out","JP-1","HK-1","US-1","DEAD-NODE"]},
			{"type": "direct", "tag": "direct-out"},
			{"type": "vmess", "tag": "JP-1", "server": "jp.example.com", "server_port": 443, "uuid": "00000000-0000-0000-0000-000000000001"},
			{"type": "shadowsocks", "tag": "HK-1", "server": "hk.example.com", "server_port": 8388, "method": "aes-128-gcm", "password": "x"},
			{"type": "vless", "tag": "US-1", "server": "us.example.com", "server_port": 443, "uuid": "00000000-0000-0000-0000-000000000002"},
			{"type": "trojan", "tag": "DEAD-NODE", "server": "dead.example.com", "server_port": 443, "password": "x"}
		]
	}`
	dir := t.TempDir()
	basePath := filepath.Join(dir, "minimal.json")
	if err := writeFile(basePath, cfg); err != nil {
		t.Fatalf("write base config: %v", err)
	}
	return overlay.NewConfigOverlay(basePath, dir)
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// TestSetupAppChain_HappyPath 全活节点路径 — 走完 5 步, Success=true
func TestSetupAppChain_HappyPath(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true) // 假设 VPN 已起
	ov := newTestOverlay(t)
	adapter := newMacroAdapter()
	tool := toolSetupAppChain(ctrl, ov)

	res, err := tool.Execute(context.Background(), adapter, map[string]interface{}{
		"app_pkg":   "com.android.chrome",
		"nodes":     []interface{}{"JP-1", "HK-1"},
		"chain_tag": "test-chain",
	})

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected Success=true, got msg=%q", res.Message)
	}
	if !strings.Contains(res.Message, "[1/5]") || !strings.Contains(res.Message, "[5/5]") {
		t.Errorf("trace incomplete: %q", res.Message)
	}
	if !strings.Contains(res.Message, "OK") {
		t.Errorf("expected OK markers in trace: %q", res.Message)
	}
	data, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected Data to be map")
	}
	if data["app_pkg"] != "com.android.chrome" {
		t.Errorf("Data.app_pkg = %v", data["app_pkg"])
	}
	if data["chain_tag"] != "_agent:test-chain" {
		t.Errorf("Data.chain_tag = %v, expected _agent:test-chain", data["chain_tag"])
	}
}

// TestSetupAppChain_DeadNodeFailFast 链路含死节点 — 第 3 步 fail-fast, 不写盘
func TestSetupAppChain_DeadNodeFailFast(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	ov := newTestOverlay(t)
	adapter := newMacroAdapter()
	tool := toolSetupAppChain(ctrl, ov)

	res, _ := tool.Execute(context.Background(), adapter, map[string]interface{}{
		"app_pkg": "com.android.chrome",
		"nodes":   []interface{}{"JP-1", "DEAD-NODE"},
	})

	if res.Success {
		t.Fatal("expected Success=false on dead node")
	}
	if !strings.Contains(res.Message, "DEAD-NODE") {
		t.Errorf("error message should mention dead node: %q", res.Message)
	}
	if !strings.Contains(res.Message, "[3/5]") {
		t.Errorf("should fail at step 3/5: %q", res.Message)
	}
}

// TestSetupAppChain_RejectShortNodes nodes < 2 直接 reject
func TestSetupAppChain_RejectShortNodes(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	ov := newTestOverlay(t)
	adapter := newMacroAdapter()
	tool := toolSetupAppChain(ctrl, ov)

	_, err := tool.Execute(context.Background(), adapter, map[string]interface{}{
		"app_pkg": "com.android.chrome",
		"nodes":   []interface{}{"JP-1"},
	})

	if err == nil {
		t.Fatal("expected err on nodes<2")
	}
	if !strings.Contains(err.Error(), "至少需要 2") {
		t.Errorf("err should mention min 2 nodes: %v", err)
	}
}

// TestSetupAppChain_MissingAppPkg 没填 app_pkg 直接 reject
func TestSetupAppChain_MissingAppPkg(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	ov := newTestOverlay(t)
	adapter := newMacroAdapter()
	tool := toolSetupAppChain(ctrl, ov)

	_, err := tool.Execute(context.Background(), adapter, map[string]interface{}{
		"nodes": []interface{}{"JP-1", "HK-1"},
	})

	if err == nil {
		t.Fatal("expected err on missing app_pkg")
	}
}

// TestSetupAppChain_AutoStartVpn VPN 没起时 macro 内部 WaitVpnReady 自动拉起
func TestSetupAppChain_AutoStartVpn(t *testing.T) {
	ctrl := &fakeVpnController{readyAfter: 300 * time.Millisecond}
	// running 默认 false
	ov := newTestOverlay(t)
	adapter := newMacroAdapter()
	tool := toolSetupAppChain(ctrl, ov)

	res, err := tool.Execute(context.Background(), adapter, map[string]interface{}{
		"app_pkg": "com.android.chrome",
		"nodes":   []interface{}{"JP-1", "HK-1"},
	})

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected Success=true, got msg=%q", res.Message)
	}
	if ctrl.startCalls.Load() != 1 {
		t.Errorf("expected exactly 1 RequestStart in auto-bootstrap path, got %d", ctrl.startCalls.Load())
	}
	if !ctrl.IsRunning() {
		t.Error("expected VPN running after macro completed")
	}
}

// TestSwitchToFastestNode_HappyPath VPN 已开 + 节点全活 → 选最低延迟切 selector
func TestSwitchToFastestNode_HappyPath(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	adapter := newMacroAdapter()
	tool := toolSwitchToFastestNode(ctrl)

	res, err := tool.Execute(context.Background(), adapter, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected Success, got msg=%q", res.Message)
	}
	// HK-1 (85ms) 是 newMacroAdapter latencyMap 里最低活节点
	data := res.Data.(map[string]interface{})
	if data["node"] != "HK-1" {
		t.Errorf("expected best=HK-1, got %v", data["node"])
	}
	if data["latency_ms"] != 85 {
		t.Errorf("expected latency_ms=85, got %v", data["latency_ms"])
	}
	if adapter.groups["proxy-group"].Now != "HK-1" {
		t.Errorf("expected adapter switched to HK-1, got %q", adapter.groups["proxy-group"].Now)
	}
}

// TestSwitchToFastestNode_RegionMatch region_match 限定候选, 仅在白名单内选
func TestSwitchToFastestNode_RegionMatch(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	adapter := newMacroAdapter()
	tool := toolSwitchToFastestNode(ctrl)

	// "JP" 仅命中 JP-1 (120ms), HK-1 / US-1 排除
	res, _ := tool.Execute(context.Background(), adapter, map[string]interface{}{
		"region_match": []interface{}{"JP"},
	})
	if !res.Success {
		t.Fatalf("expected Success: %q", res.Message)
	}
	data := res.Data.(map[string]interface{})
	if data["node"] != "JP-1" {
		t.Errorf("region_match=[JP] expected JP-1, got %v", data["node"])
	}
}

// TestSwitchToFastestNode_RegionMatchNoCandidate region 没匹配 → fail-fast
func TestSwitchToFastestNode_RegionMatchNoCandidate(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	adapter := newMacroAdapter()
	tool := toolSwitchToFastestNode(ctrl)

	res, _ := tool.Execute(context.Background(), adapter, map[string]interface{}{
		"region_match": []interface{}{"XX-Nonexistent"},
	})
	if res.Success {
		t.Fatal("expected fail when no candidate matches region")
	}
	if !strings.Contains(res.Message, "无匹配节点") {
		t.Errorf("err msg should mention 无匹配节点: %q", res.Message)
	}
}

// TestSwitchToFastestNode_AutoStartVpn VPN 没开 → 内部 WaitVpnReady 自动拉起
func TestSwitchToFastestNode_AutoStartVpn(t *testing.T) {
	ctrl := &fakeVpnController{readyAfter: 200 * time.Millisecond}
	adapter := newMacroAdapter()
	tool := toolSwitchToFastestNode(ctrl)

	res, err := tool.Execute(context.Background(), adapter, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected Success: %q", res.Message)
	}
	if ctrl.startCalls.Load() != 1 {
		t.Errorf("expected 1 RequestStart, got %d", ctrl.startCalls.Load())
	}
}

// macroSSURI 构造 ss:// URI, 节点名作为 fragment, SanitizeTag 后等于 fragment 原值
func macroSSURI(name, host string, port int) string {
	userInfo := base64.RawStdEncoding.EncodeToString([]byte("aes-256-gcm:pass-" + name))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, host, port, name)
}

// macroSSList 把多条 SS URI 拼成 base64 订阅 body
func macroSSList(uris ...string) string {
	plain := strings.Join(uris, "\n")
	return base64.StdEncoding.EncodeToString([]byte(plain))
}

// buildImportFixture 起 httptest server + 实例化 SubscriptionManager (overlay+adapter 真实)
// 返回的 mgr 已注入到 macroFakeAdapter 的 latencyMap 用 names 配, 让 toolImportAndActivateSubscription 测速正确
func buildImportFixture(t *testing.T, latency map[string]int, uris []string) (*subscription.SubscriptionManager, *macroFakeAdapter, string, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, macroSSList(uris...))
	}))

	dir := t.TempDir()
	basePath := filepath.Join(dir, "minimal.json")
	cfg := `{
		"outbounds": [
			{"type": "selector", "tag": "proxy-group", "outbounds": ["direct-out"]},
			{"type": "direct", "tag": "direct-out"}
		],
		"route": {"final": "proxy-group"}
	}`
	if err := writeFile(basePath, cfg); err != nil {
		t.Fatalf("write base: %v", err)
	}
	store := subscription.NewSubscriptionStore(dir)
	ov := overlay.NewConfigOverlay(basePath, dir)

	adapter := &macroFakeAdapter{latencyMap: latency}
	mgr := subscription.NewSubscriptionManager(store, ov, adapter)
	return mgr, adapter, srv.URL, func() { srv.Close() }
}

// TestImportAndActivateSubscription_HappyPath 导入 → 测速 → 切到最快
func TestImportAndActivateSubscription_HappyPath(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	mgr, adapter, url, closeFn := buildImportFixture(t,
		map[string]int{"JP-Sub": 220, "HK-Sub": 80, "US-Sub": 350},
		[]string{
			macroSSURI("JP-Sub", "1.1.1.1", 443),
			macroSSURI("HK-Sub", "2.2.2.2", 443),
			macroSSURI("US-Sub", "3.3.3.3", 443),
		},
	)
	defer closeFn()
	// 让 adapter.SetActiveProxy 不 nil-panic
	adapter.groups = map[string]*engine.ProxyGroup{
		"proxy-group": {Tag: "proxy-group", Type: "Selector", Now: ""},
	}

	tool := toolImportAndActivateSubscription(ctrl, mgr)
	res, err := tool.Execute(context.Background(), adapter, map[string]interface{}{
		"url": url,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected Success, got msg=%q", res.Message)
	}
	data := res.Data.(map[string]interface{})
	if data["node"] != "HK-Sub" {
		t.Errorf("expected fastest=HK-Sub, got %v", data["node"])
	}
	if data["latency_ms"] != 80 {
		t.Errorf("expected latency_ms=80, got %v", data["latency_ms"])
	}
	if adapter.groups["proxy-group"].Now != "HK-Sub" {
		t.Errorf("expected selector switched to HK-Sub, got %q", adapter.groups["proxy-group"].Now)
	}
}

// TestImportAndActivateSubscription_PickFastestFalse 直接选第一个 tag, 不测速
func TestImportAndActivateSubscription_PickFastestFalse(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	mgr, adapter, url, closeFn := buildImportFixture(t,
		map[string]int{"JP-Sub": 220, "HK-Sub": 80}, // 即使 HK 更快, false 时也走第一个
		[]string{
			macroSSURI("JP-Sub", "1.1.1.1", 443),
			macroSSURI("HK-Sub", "2.2.2.2", 443),
		},
	)
	defer closeFn()
	adapter.groups = map[string]*engine.ProxyGroup{
		"proxy-group": {Tag: "proxy-group", Type: "Selector"},
	}

	tool := toolImportAndActivateSubscription(ctrl, mgr)
	res, _ := tool.Execute(context.Background(), adapter, map[string]interface{}{
		"url":          url,
		"pick_fastest": false,
	})
	if !res.Success {
		t.Fatalf("expected Success, got msg=%q", res.Message)
	}
	data := res.Data.(map[string]interface{})
	if data["node"] != "JP-Sub" {
		t.Errorf("pick_fastest=false should pick first tag JP-Sub, got %v", data["node"])
	}
	if _, has := data["latency_ms"]; has {
		t.Errorf("pick_fastest=false should NOT measure latency, but Data has latency_ms=%v", data["latency_ms"])
	}
}

// TestImportAndActivateSubscription_MissingURL 缺 url 必须 fail-fast
func TestImportAndActivateSubscription_MissingURL(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	tool := toolImportAndActivateSubscription(ctrl, nil) // mgr 不会被调到, 参数校验先 fail
	_, err := tool.Execute(context.Background(), &macroFakeAdapter{}, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("err msg should mention url: %v", err)
	}
}

// TestImportAndActivateSubscription_AllNodesDead pick_fastest=true + 所有节点 timeout → 报错不切
func TestImportAndActivateSubscription_AllNodesDead(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	mgr, adapter, url, closeFn := buildImportFixture(t,
		map[string]int{}, // 空 map → 所有 tag 都 timeout
		[]string{
			macroSSURI("JP-Sub", "1.1.1.1", 443),
			macroSSURI("HK-Sub", "2.2.2.2", 443),
		},
	)
	defer closeFn()
	adapter.groups = map[string]*engine.ProxyGroup{
		"proxy-group": {Tag: "proxy-group", Type: "Selector"},
	}

	tool := toolImportAndActivateSubscription(ctrl, mgr)
	res, _ := tool.Execute(context.Background(), adapter, map[string]interface{}{
		"url": url,
	})
	if res.Success {
		t.Fatal("expected fail when all nodes timeout")
	}
	if !strings.Contains(res.Message, "超时") {
		t.Errorf("err msg should mention 超时: %q", res.Message)
	}
}

// TestFindSubscriptionTags_NameMatch findSubscriptionTags 优先 name 精确匹配
func TestFindSubscriptionTags_NameMatch(t *testing.T) {
	mgr, _, url, closeFn := buildImportFixture(t,
		map[string]int{"JP-Sub": 100},
		[]string{macroSSURI("JP-Sub", "1.1.1.1", 443)},
	)
	defer closeFn()

	if _, _, err := mgr.AddSubscription("my-sub", url); err != nil {
		t.Fatalf("setup add: %v", err)
	}
	tags := findSubscriptionTags(mgr, "my-sub", "")
	if len(tags) != 1 || tags[0] != "JP-Sub" {
		t.Errorf("name match should return [JP-Sub], got %v", tags)
	}
	// URL fallback
	tags2 := findSubscriptionTags(mgr, "", url)
	if len(tags2) != 1 || tags2[0] != "JP-Sub" {
		t.Errorf("url match should return [JP-Sub], got %v", tags2)
	}
	// neither match → fallback last
	tags3 := findSubscriptionTags(mgr, "nope", "https://nope.example")
	if len(tags3) != 1 {
		t.Errorf("fallback should return last entry tags, got %v", tags3)
	}
}

// TestDiagnoseConnectivity_VpnStopped 不跑 VPN → suspect=VPN_STOPPED 且不调测速
func TestDiagnoseConnectivity_VpnStopped(t *testing.T) {
	ctrl := &fakeVpnController{}
	// 不 Store(true), 默认 false
	ov := newTestOverlay(t)
	adapter := newMacroAdapter()
	tool := toolDiagnoseConnectivity(ctrl, ov)

	res, err := tool.Execute(context.Background(), adapter, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected Success (read-only never fails), got %q", res.Message)
	}
	data := res.Data.(map[string]interface{})
	if data["vpn_running"] != false {
		t.Errorf("vpn_running expected false, got %v", data["vpn_running"])
	}
	suspect, _ := data["suspect"].(string)
	if !strings.HasPrefix(suspect, "VPN_STOPPED") {
		t.Errorf("expected suspect=VPN_STOPPED..., got %q", suspect)
	}
	if !strings.Contains(res.Message, "节点: skip") {
		t.Errorf("VPN 未启动应跳过测速: %q", res.Message)
	}
}

// TestDiagnoseConnectivity_Healthy VPN 运行 + 多数节点活 + 当前出口非 direct → HEALTHY
func TestDiagnoseConnectivity_Healthy(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	ov := newTestOverlay(t)
	adapter := newMacroAdapter()
	// 给 1 条假活动连接, 让 IDLE 路径不命中
	adapter.connections = []engine.ConnectionInfo{
		{ID: "1", Destination: "x.example.com:443", Protocol: "tcp", Chain: "proxy-group"},
	}
	tool := toolDiagnoseConnectivity(ctrl, ov)

	res, _ := tool.Execute(context.Background(), adapter, map[string]interface{}{})
	if !res.Success {
		t.Fatalf("expected Success: %q", res.Message)
	}
	data := res.Data.(map[string]interface{})
	suspect := data["suspect"].(string)
	if !strings.HasPrefix(suspect, "HEALTHY") {
		t.Errorf("expected HEALTHY, got %q (data=%v)", suspect, data)
	}
	if data["best_node"] != "HK-1" {
		t.Errorf("best_node expected HK-1 (85ms), got %v", data["best_node"])
	}
	// 4 节点中 1 个 dead (DEAD-NODE) + 3 活 (JP-1/HK-1/US-1) + 1 selector group 已被 filterRealNodes 滤掉
	if data["nodes_live"] != 3 || data["nodes_dead"] != 1 {
		t.Errorf("nodes_live/dead expected 3/1, got %v/%v", data["nodes_live"], data["nodes_dead"])
	}
}

// TestDiagnoseConnectivity_AllNodesDead VPN 运行但所有节点 timeout → suspect=ALL_NODES_DEAD
func TestDiagnoseConnectivity_AllNodesDead(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	ov := newTestOverlay(t)
	adapter := newMacroAdapter()
	adapter.latencyMap = map[string]int{} // 全部 timeout
	tool := toolDiagnoseConnectivity(ctrl, ov)

	res, _ := tool.Execute(context.Background(), adapter, map[string]interface{}{})
	data := res.Data.(map[string]interface{})
	suspect := data["suspect"].(string)
	if !strings.HasPrefix(suspect, "ALL_NODES_DEAD") {
		t.Errorf("expected ALL_NODES_DEAD, got %q", suspect)
	}
	if data["nodes_live"] != 0 {
		t.Errorf("nodes_live should be 0, got %v", data["nodes_live"])
	}
}

// TestDiagnoseConnectivity_Idle VPN 运行 + 节点活 + 0 连接 → IDLE
func TestDiagnoseConnectivity_Idle(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	ov := newTestOverlay(t)
	adapter := newMacroAdapter()
	// connections 为 nil/空
	tool := toolDiagnoseConnectivity(ctrl, ov)

	res, _ := tool.Execute(context.Background(), adapter, map[string]interface{}{})
	data := res.Data.(map[string]interface{})
	suspect := data["suspect"].(string)
	if !strings.HasPrefix(suspect, "IDLE") {
		t.Errorf("expected IDLE, got %q", suspect)
	}
	if data["active_connections"] != 0 {
		t.Errorf("active_connections expected 0, got %v", data["active_connections"])
	}
}

// TestDiagnoseSuspect 单元测试 suspect 决策表 — 不依赖 fake adapter
func TestDiagnoseSuspect(t *testing.T) {
	cases := []struct {
		name        string
		vpnRunning  bool
		live        int
		dead        int
		currentNode string
		conns       int
		wantPrefix  string
	}{
		{"vpn stopped", false, 0, 0, "", 0, "VPN_STOPPED"},
		{"all dead", true, 0, 5, "HK-1", 0, "ALL_NODES_DEAD"},
		{"selector empty", true, 3, 0, "", 0, "NO_CURRENT_NODE"},
		{"selector direct", true, 3, 0, "direct-out", 0, "NO_CURRENT_NODE"},
		{"mostly dead", true, 1, 5, "HK-1", 0, "MOSTLY_DEAD"},
		{"idle healthy", true, 3, 1, "HK-1", 0, "IDLE"},
		{"healthy active", true, 3, 1, "HK-1", 7, "HEALTHY"},
	}
	for _, tc := range cases {
		got := diagnoseSuspect(tc.vpnRunning, tc.live, tc.dead, tc.currentNode, tc.conns)
		if !strings.HasPrefix(got, tc.wantPrefix) {
			t.Errorf("[%s] want prefix %q, got %q", tc.name, tc.wantPrefix, got)
		}
	}
}

// TestSetupAppChain_DefaultChainTag 留空 chain_tag 时自动从 app_pkg 派生
func TestSetupAppChain_DefaultChainTag(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	ov := newTestOverlay(t)
	adapter := newMacroAdapter()
	tool := toolSetupAppChain(ctrl, ov)

	res, _ := tool.Execute(context.Background(), adapter, map[string]interface{}{
		"app_pkg": "com.android.chrome",
		"nodes":   []interface{}{"JP-1", "HK-1"},
	})

	if !res.Success {
		t.Fatalf("expected Success: %q", res.Message)
	}
	data := res.Data.(map[string]interface{})
	wantTag := "_agent:com-android-chrome-chain"
	if data["chain_tag"] != wantTag {
		t.Errorf("default chain_tag: want %q got %v", wantTag, data["chain_tag"])
	}
}
