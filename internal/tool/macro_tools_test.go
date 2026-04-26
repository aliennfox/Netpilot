package tool

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
)

// macroFakeAdapter 嵌入 fakeAdapter, 加 TestLatency 让 macro_tools 测试能跑。
//
// latencyMap: tag → ms; 不存在的 tag 返回 0+error 模拟死节点。
type macroFakeAdapter struct {
	fakeAdapter
	latencyMap map[string]int
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
			"JP-1":  120,
			"HK-1":  85,
			"US-1":  240,
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
