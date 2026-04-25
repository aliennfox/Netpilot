package tool

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// fakeAdapter 嵌入 EngineAdapter 接口 (zero value), 只实现 SnapshotStore 真正会调到的
// 三个方法: GetProxies / GetProxyGroup / SetActiveProxy。 其它方法被调用时会 nil-panic,
// 这正是测试想要的: 任何非预期调用都会立即暴露。
type fakeAdapter struct {
	engine.EngineAdapter
	proxies     []engine.ProxyInfo
	groups      map[string]*engine.ProxyGroup
	setCalls    []setCall
	setProxyErr error
}

type setCall struct{ group, proxy string }

func (f *fakeAdapter) GetProxies() ([]engine.ProxyInfo, error) { return f.proxies, nil }

func (f *fakeAdapter) GetProxyGroup(tag string) (*engine.ProxyGroup, error) {
	g, ok := f.groups[tag]
	if !ok {
		return nil, errors.New("group not found")
	}
	return g, nil
}

func (f *fakeAdapter) SetActiveProxy(groupTag, proxyTag string) error {
	if f.setProxyErr != nil {
		return f.setProxyErr
	}
	f.setCalls = append(f.setCalls, setCall{groupTag, proxyTag})
	// 把新选中的 proxy 同步到 group state, 让多步 Save→Switch→Rollback 能 round-trip
	if g, ok := f.groups[groupTag]; ok {
		g.Now = proxyTag
	}
	return nil
}

func newFakeAdapter() *fakeAdapter {
	return &fakeAdapter{
		proxies: []engine.ProxyInfo{
			{Tag: "proxy-group", Type: "Selector"},
			{Tag: "HK-1", Type: "shadowsocks"},
			{Tag: "JP-1", Type: "vmess"},
		},
		groups: map[string]*engine.ProxyGroup{
			"proxy-group": {Tag: "proxy-group", Type: "Selector", Now: "HK-1"},
		},
	}
}

// TestSnapshotStore_SaveLatest Save() 落 Tool 字段, Latest 返回最新的
func TestSnapshotStore_SaveLatest(t *testing.T) {
	dir := t.TempDir()
	store := NewSnapshotStore(dir)
	adapter := newFakeAdapter()

	id, err := store.Save(adapter, "switch_node")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if id == "" {
		t.Fatal("save returned empty id")
	}

	latest := store.Latest()
	if latest == nil {
		t.Fatal("Latest returned nil after save")
	}
	if latest.ID != id {
		t.Errorf("Latest ID: want %q got %q", id, latest.ID)
	}
	if latest.Tool != "switch_node" {
		t.Errorf("Tool field: want switch_node got %q", latest.Tool)
	}
	if latest.ActiveProxies["proxy-group"] != "HK-1" {
		t.Errorf("active_proxies: want HK-1, got %v", latest.ActiveProxies)
	}
}

// TestSnapshotStore_RollbackRestoresPriorState 完整 round-trip:
// Save (HK-1) → Switch (JP-1) → Rollback → adapter 收到 SetActiveProxy(HK-1)
func TestSnapshotStore_RollbackRestoresPriorState(t *testing.T) {
	store := NewSnapshotStore(t.TempDir())
	adapter := newFakeAdapter()

	id, err := store.Save(adapter, "switch_node")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	// 模拟用户 / Agent 切了节点
	if err := adapter.SetActiveProxy("proxy-group", "JP-1"); err != nil {
		t.Fatalf("switch: %v", err)
	}
	adapter.setCalls = nil // 清掉 switch 的记录, 只看后续 rollback

	if err := store.Rollback(id, adapter); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if len(adapter.setCalls) != 1 {
		t.Fatalf("expected 1 SetActiveProxy call from rollback, got %d", len(adapter.setCalls))
	}
	got := adapter.setCalls[0]
	if got.group != "proxy-group" || got.proxy != "HK-1" {
		t.Errorf("rollback restored wrong state: %+v (want group=proxy-group proxy=HK-1)", got)
	}
}

// TestSnapshotStore_RollbackUnknownID 给不存在的 ID → 显式 error
func TestSnapshotStore_RollbackUnknownID(t *testing.T) {
	store := NewSnapshotStore(t.TempDir())
	adapter := newFakeAdapter()
	err := store.Rollback("snap-bogus-001", adapter)
	if err == nil {
		t.Fatal("expected error for unknown snapshot id, got nil")
	}
	if len(adapter.setCalls) != 0 {
		t.Errorf("no SetActiveProxy should be called on missing snapshot, got %d", len(adapter.setCalls))
	}
}

// TestSnapshotStore_RollbackPropagatesAdapterError 锁 #M2 的契约前提:
// SetActiveProxy 失败时 Rollback 必须返回非 nil error, 让 pipeline.go 的
// auto-rollback 路径能感知失败并升级用户消息。
func TestSnapshotStore_RollbackPropagatesAdapterError(t *testing.T) {
	store := NewSnapshotStore(t.TempDir())
	adapter := newFakeAdapter()

	id, err := store.Save(adapter, "switch_node")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	adapter.setProxyErr = errors.New("clash api 500")
	err = store.Rollback(id, adapter)
	if err == nil {
		t.Fatal("expected error to propagate from adapter, got nil")
	}
}

// TestSnapshotStore_TrimToMaxCount 超过 maxCount=20 时最旧的被弹出 + 磁盘文件被删
func TestSnapshotStore_TrimToMaxCount(t *testing.T) {
	dir := t.TempDir()
	store := NewSnapshotStore(dir)
	adapter := newFakeAdapter()

	var firstID string
	// 写 25 个, 应该保留最近 20 个
	for i := 0; i < 25; i++ {
		id, err := store.Save(adapter, "switch_node")
		if err != nil {
			t.Fatalf("save #%d: %v", i, err)
		}
		if i == 0 {
			firstID = id
		}
	}

	if got := len(store.List()); got != 20 {
		t.Errorf("after 25 saves, expect 20 retained, got %d", got)
	}

	// 第一个的磁盘文件应该被清掉
	firstFile := filepath.Join(dir, firstID+".json")
	if _, err := os.Stat(firstFile); !os.IsNotExist(err) {
		t.Errorf("oldest snapshot file should be removed, but Stat=%v", err)
	}
}

// TestSnapshotStore_PersistAcrossInstances 同 dataDir 重新构造 store, loadFromDisk
// 应能恢复之前所有 snapshot, 顺序按 Timestamp 升序。
func TestSnapshotStore_PersistAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	adapter := newFakeAdapter()

	store1 := NewSnapshotStore(dir)
	id1, _ := store1.Save(adapter, "switch_node")
	// 强制时间间隔, 避免 ID 因为同毫秒 timestamp 而冲突
	time.Sleep(2 * time.Millisecond)
	id2, _ := store1.Save(adapter, "set_per_app_vpn")

	store2 := NewSnapshotStore(dir)
	list := store2.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 loaded snapshots, got %d", len(list))
	}
	if list[0].ID != id1 || list[1].ID != id2 {
		t.Errorf("snapshot order wrong: got [%s, %s], want [%s, %s]",
			list[0].ID, list[1].ID, id1, id2)
	}
	// Tool 字段必须穿透 JSON 持久化 (D2 Safety Card 真名显示依赖此)
	if list[1].Tool != "set_per_app_vpn" {
		t.Errorf("Tool field lost across persistence: %q", list[1].Tool)
	}
}

// TestSnapshotStore_ListReturnsCopy List() 返回的切片改了不能影响内部状态,
// 否则 UI 拿去渲染会污染 SnapshotStore 内部 slice。
func TestSnapshotStore_ListReturnsCopy(t *testing.T) {
	store := NewSnapshotStore(t.TempDir())
	adapter := newFakeAdapter()

	if _, err := store.Save(adapter, "switch_node"); err != nil {
		t.Fatalf("save: %v", err)
	}

	list := store.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(list))
	}
	list[0].Tool = "MUTATED"

	list2 := store.List()
	if list2[0].Tool == "MUTATED" {
		t.Errorf("List() should return a defensive copy, internal state was mutated")
	}
}

// TestSnapshotStore_SaveCounterMonotonic counter 单调递增, ID 不重复 (即使同一秒)
func TestSnapshotStore_SaveCounterMonotonic(t *testing.T) {
	store := NewSnapshotStore(t.TempDir())
	adapter := newFakeAdapter()

	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		id, err := store.Save(adapter, "switch_node")
		if err != nil {
			t.Fatalf("save #%d: %v", i, err)
		}
		if seen[id] {
			t.Errorf("duplicate id %q on save #%d", id, i)
		}
		seen[id] = true
	}
}
