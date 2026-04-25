package subscription

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
)

// reloadAdapter 实现 EngineAdapter 中 Manager 真正会调到的方法 (只有 Reload).
// 其它方法由零值嵌入接口承担, 调用时会 nil-panic, 这正是测试想要的.
type reloadAdapter struct {
	engine.EngineAdapter
	mu        sync.Mutex
	reloads   int
	reloadErr error
}

func (r *reloadAdapter) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reloads++
	return r.reloadErr
}

func (r *reloadAdapter) ReloadCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reloads
}

// writeBase 给 ConfigOverlay 准备一份最小 base config (selector + direct outbound)
func writeBase(t *testing.T, dir string) string {
	t.Helper()
	base := map[string]interface{}{
		"outbounds": []interface{}{
			map[string]interface{}{
				"type":      "selector",
				"tag":       "proxy-group",
				"outbounds": []interface{}{"direct-out"},
			},
			map[string]interface{}{"type": "direct", "tag": "direct-out"},
		},
		"route": map[string]interface{}{"final": "proxy-group"},
	}
	raw, _ := json.Marshal(base)
	path := filepath.Join(dir, "base.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// makeSSURIList 构造 base64-编码的 SS URI 列表订阅 body
func makeSSURIList(uris ...string) string {
	plain := strings.Join(uris, "\n")
	return base64.StdEncoding.EncodeToString([]byte(plain))
}

// validSSURI 生成 SS URI; tag 唯一, 解析后 Name 字段就是这个 tag
func validSSURI(name, host string, port int) string {
	userInfo := base64.RawStdEncoding.EncodeToString([]byte("aes-256-gcm:pass-" + name))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, host, port, name)
}

// buildManager 给 manager test 一个完整环境: store + overlay + reloadAdapter
func buildManager(t *testing.T) (*SubscriptionManager, *SubscriptionStore, *overlay.ConfigOverlay, *reloadAdapter) {
	t.Helper()
	dir := t.TempDir()
	basePath := writeBase(t, dir)
	store := NewSubscriptionStore(dir)
	ov := overlay.NewConfigOverlay(basePath, dir)
	adapter := &reloadAdapter{}
	mgr := NewSubscriptionManager(store, ov, adapter)
	return mgr, store, ov, adapter
}

// TestManager_AddSubscription_HappyPath HTTP fetch 成功 + store 加条目 + overlay 多 outbound + adapter Reload 被调
func TestManager_AddSubscription_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, makeSSURIList(validSSURI("HK-A", "1.1.1.1", 443), validSSURI("JP-A", "2.2.2.2", 8388)))
	}))
	defer srv.Close()

	mgr, store, ov, adapter := buildManager(t)
	count, summary, err := mgr.AddSubscription("test", srv.URL)
	if err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	if count != 2 {
		t.Errorf("count: want 2, got %d", count)
	}
	if !strings.Contains(summary, "test") {
		t.Errorf("summary should contain name: %q", summary)
	}
	if got := len(store.List()); got != 1 {
		t.Errorf("store should have 1 sub, got %d", got)
	}
	// overlay 写入了 2 个 outbound
	outs := ov.ListOutbounds()
	if len(outs) != 2 {
		t.Errorf("overlay should have 2 outbounds, got %d", len(outs))
	}
	// adapter.Reload 至少被调 1 次 (AddSubscription 应触发)
	if adapter.ReloadCount() < 1 {
		t.Errorf("adapter.Reload should be called at least once, got %d", adapter.ReloadCount())
	}
}

// TestManager_AddSubscription_DuplicateURLAutoUpdates 同 URL 二次 Add → 路由到 UpdateSubscription
func TestManager_AddSubscription_DuplicateURLAutoUpdates(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			fmt.Fprint(w, makeSSURIList(validSSURI("HK-1", "1.1.1.1", 443)))
		} else {
			// 第二次返回 3 个节点
			fmt.Fprint(w, makeSSURIList(
				validSSURI("HK-1", "1.1.1.1", 443),
				validSSURI("JP-1", "2.2.2.2", 8388),
				validSSURI("US-1", "3.3.3.3", 9443),
			))
		}
	}))
	defer srv.Close()

	mgr, store, _, _ := buildManager(t)

	if _, _, err := mgr.AddSubscription("duped", srv.URL); err != nil {
		t.Fatalf("first add: %v", err)
	}
	// 第二次 Add 同 URL — 不创建第二个 store 条目, 而是 update 第一个
	count, summary, err := mgr.AddSubscription("不同名", srv.URL)
	if err != nil {
		t.Fatalf("second add: %v", err)
	}
	if count != 3 {
		t.Errorf("after auto-update, want 3 nodes, got %d", count)
	}
	if !strings.Contains(summary, "已更新") {
		t.Errorf("summary should mention update path, got %q", summary)
	}
	if got := len(store.List()); got != 1 {
		t.Errorf("store should still have 1 sub (no dup), got %d", got)
	}
}

// TestManager_AddSubscription_FetchFails URL 5xx → 错误透传, 不留 store 垃圾
func TestManager_AddSubscription_FetchFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", 500)
	}))
	defer srv.Close()

	mgr, store, _, _ := buildManager(t)
	_, _, err := mgr.AddSubscription("bad", srv.URL)
	if err == nil {
		t.Fatal("expected error on 5xx")
	}
	if got := len(store.List()); got != 0 {
		t.Errorf("store should be empty on fetch failure, got %d", got)
	}
}

// TestManager_AddSubscription_ZeroValidNodes 解析后 0 节点 → 删 store 不留垃圾, overlay 也不变
func TestManager_AddSubscription_ZeroValidNodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ssr://fake-only\nmieru://fake-only") // 都是不支持的协议
	}))
	defer srv.Close()

	mgr, store, ov, _ := buildManager(t)
	_, _, err := mgr.AddSubscription("empty", srv.URL)
	if err == nil {
		t.Fatal("expected error when zero valid nodes")
	}
	if got := len(store.List()); got != 0 {
		t.Errorf("store should be empty, got %d", got)
	}
	if got := len(ov.ListOutbounds()); got != 0 {
		t.Errorf("overlay should be empty, got %d", got)
	}
}

// TestManager_RemoveSubscription Remove 应同时清 store + overlay 那个 sub 的所有 tag
func TestManager_RemoveSubscription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, makeSSURIList(validSSURI("HK-X", "1.1.1.1", 443), validSSURI("JP-X", "2.2.2.2", 8388)))
	}))
	defer srv.Close()

	mgr, store, ov, adapter := buildManager(t)
	if _, _, err := mgr.AddSubscription("rm-test", srv.URL); err != nil {
		t.Fatalf("add: %v", err)
	}
	subID := store.List()[0].ID

	_ = adapter.ReloadCount() // baseline
	beforeReloads := adapter.ReloadCount()

	msg, err := mgr.RemoveSubscription(subID)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(msg, "已删除订阅") {
		t.Errorf("msg should confirm: %q", msg)
	}
	if got := len(store.List()); got != 0 {
		t.Errorf("store should be empty after remove, got %d", got)
	}
	if got := len(ov.ListOutbounds()); got != 0 {
		t.Errorf("overlay should be empty after remove, got %d", got)
	}
	if adapter.ReloadCount() <= beforeReloads {
		t.Errorf("Remove should trigger another Reload")
	}
}

// TestManager_RemoveSubscription_UnknownID 错误透传
func TestManager_RemoveSubscription_UnknownID(t *testing.T) {
	mgr, _, _, _ := buildManager(t)
	_, err := mgr.RemoveSubscription("does-not-exist")
	if err == nil {
		t.Fatal("expected error on unknown id")
	}
}

// TestManager_UpdateSubscription Update 应替换旧 tag 为新 tag
func TestManager_UpdateSubscription(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			fmt.Fprint(w, makeSSURIList(validSSURI("OLD-1", "1.1.1.1", 443), validSSURI("OLD-2", "2.2.2.2", 8388)))
		} else {
			// 一个 OLD-1 保留, 加 NEW-3
			fmt.Fprint(w, makeSSURIList(validSSURI("OLD-1", "1.1.1.1", 443), validSSURI("NEW-3", "3.3.3.3", 9443)))
		}
	}))
	defer srv.Close()

	mgr, store, ov, _ := buildManager(t)
	if _, _, err := mgr.AddSubscription("upd", srv.URL); err != nil {
		t.Fatalf("add: %v", err)
	}
	subID := store.List()[0].ID

	count, msg, err := mgr.UpdateSubscription(subID)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if count != 2 {
		t.Errorf("want 2 nodes after update, got %d", count)
	}
	if !strings.Contains(msg, "2→2") && !strings.Contains(msg, "无变化") {
		t.Errorf("update msg should describe diff: %q", msg)
	}

	// overlay 现在应该有 OLD-1 + NEW-3, 没有 OLD-2
	tags := map[string]bool{}
	for _, ob := range ov.ListOutbounds() {
		tags[ob["tag"].(string)] = true
	}
	if !tags["OLD-1"] || !tags["NEW-3"] {
		t.Errorf("overlay should contain OLD-1 + NEW-3, got %v", tags)
	}
	if tags["OLD-2"] {
		t.Errorf("OLD-2 should be removed by update, but still present")
	}
}

// TestManager_UpdateSubscription_UnknownID 错误透传
func TestManager_UpdateSubscription_UnknownID(t *testing.T) {
	mgr, _, _, _ := buildManager(t)
	_, _, err := mgr.UpdateSubscription("does-not-exist")
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestManager_ImportNodeURI 单 URI 直接进 overlay, 不创建 store 条目 (M16 Deep Link 后端)
func TestManager_ImportNodeURI(t *testing.T) {
	mgr, store, ov, adapter := buildManager(t)

	uri := validSSURI("DeepLink-1", "5.5.5.5", 443)
	count, msg, err := mgr.ImportNodeURI(uri)
	if err != nil {
		t.Fatalf("ImportNodeURI: %v", err)
	}
	if count != 1 {
		t.Errorf("count: want 1, got %d", count)
	}
	if !strings.Contains(msg, "DeepLink-1") {
		t.Errorf("msg should contain tag: %q", msg)
	}
	if got := len(store.List()); got != 0 {
		t.Errorf("ImportNodeURI should NOT create store entry, got %d", got)
	}
	outs := ov.ListOutbounds()
	if len(outs) != 1 || outs[0]["tag"] != "DeepLink-1" {
		t.Errorf("overlay should have DeepLink-1, got %+v", outs)
	}
	if adapter.ReloadCount() != 1 {
		t.Errorf("Reload should be called once, got %d", adapter.ReloadCount())
	}
}

// TestManager_ImportNodeURI_InvalidURI 解析失败时错误透传, 不动 overlay
func TestManager_ImportNodeURI_InvalidURI(t *testing.T) {
	mgr, _, ov, adapter := buildManager(t)

	_, _, err := mgr.ImportNodeURI("ssr://garbage-only")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(ov.ListOutbounds()) != 0 {
		t.Errorf("overlay should not be touched on parse error")
	}
	if adapter.ReloadCount() != 0 {
		t.Errorf("Reload should not be called on parse error")
	}
}

// TestManager_ImportFromData_HappyPath 本地数据导入 (Clash YAML / SS URI list 等)
func TestManager_ImportFromData_HappyPath(t *testing.T) {
	mgr, store, ov, _ := buildManager(t)

	body := makeSSURIList(validSSURI("LOCAL-A", "1.1.1.1", 443), validSSURI("LOCAL-B", "2.2.2.2", 8388))
	count, _, err := mgr.ImportFromData("local-fixture", []byte(body))
	if err != nil {
		t.Fatalf("ImportFromData: %v", err)
	}
	if count != 2 {
		t.Errorf("count: want 2, got %d", count)
	}
	if got := len(store.List()); got != 1 {
		t.Errorf("ImportFromData should create store entry, got %d", got)
	}
	if !strings.HasPrefix(store.List()[0].URL, "local://") {
		t.Errorf("local source URL should start with local://, got %q", store.List()[0].URL)
	}
	if got := len(ov.ListOutbounds()); got != 2 {
		t.Errorf("overlay outbounds: want 2, got %d", got)
	}
}

// TestManager_ImportFromData_EmptyData 空数据应被拒
func TestManager_ImportFromData_EmptyData(t *testing.T) {
	mgr, _, _, _ := buildManager(t)
	_, _, err := mgr.ImportFromData("empty", []byte{})
	if err == nil {
		t.Fatal("expected error on empty data")
	}
}

// TestManager_AddSubscription_AdapterReloadFailsRollback 当 adapter.Reload 失败时,
// AddSubscription 应该返回错误 (overlay.Apply 失败 → 错误透传)
func TestManager_AddSubscription_AdapterReloadFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, makeSSURIList(validSSURI("X", "1.1.1.1", 443)))
	}))
	defer srv.Close()

	mgr, store, _, adapter := buildManager(t)
	adapter.reloadErr = errors.New("singbox 重启失败")

	_, _, err := mgr.AddSubscription("rl-fail", srv.URL)
	if err == nil {
		t.Fatal("expected error to propagate from adapter.Reload")
	}
	if !strings.Contains(err.Error(), "应用配置失败") {
		t.Errorf("error should wrap adapter error: %v", err)
	}
	// store 已注册, 但 overlay 应用失败; 当前实现选择保留 store 条目 (重启后 retry 用),
	// 这里只断言错误传出, 不绑定 store 状态选择 (避免锁定实现细节)
	_ = store
}
