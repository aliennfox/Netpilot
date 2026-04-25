package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

type Snapshot struct {
	ID            string            `json:"id"`
	Timestamp     time.Time         `json:"timestamp"`
	ActiveProxies map[string]string `json:"active_proxies"` // group → selected proxy
	// Tool 触发本次自动 snapshot 的 tool 名 (e.g. set_per_app_vpn / switch_node).
	// Manual snapshot / 0.x 历史 snapshot 可能为空, UI 需兜底显示 "snapshot".
	// 用于 D2 Safety card 真实 tool 显示, 不再靠 active_proxies 差分推断 (那只能识别 switch_node).
	Tool string `json:"tool,omitempty"`
}

type SnapshotStore struct {
	mu        sync.Mutex
	snapshots []Snapshot
	maxCount  int
	dataDir   string
	counter   int
}

func NewSnapshotStore(dataDir string) *SnapshotStore {
	s := &SnapshotStore{
		maxCount: 20,
		dataDir:  dataDir,
	}
	s.loadFromDisk()
	return s
}

func (s *SnapshotStore) Save(adapter engine.EngineAdapter, tool string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++
	now := time.Now()
	id := fmt.Sprintf("snap-%s-%03d", now.Format("20060102"), s.counter)

	// Capture current proxy group selections
	activeProxies := make(map[string]string)
	proxies, err := adapter.GetProxies()
	if err != nil {
		return "", fmt.Errorf("snapshot: failed to get proxies: %w", err)
	}
	for _, p := range proxies {
		if isGroupType(p.Type) {
			group, err := adapter.GetProxyGroup(p.Tag)
			if err != nil {
				continue
			}
			activeProxies[group.Tag] = group.Now
		}
	}

	snap := Snapshot{
		ID:            id,
		Timestamp:     now,
		ActiveProxies: activeProxies,
		Tool:          tool,
	}

	s.snapshots = append(s.snapshots, snap)

	// Trim to max
	if len(s.snapshots) > s.maxCount {
		removed := s.snapshots[0]
		s.snapshots = s.snapshots[1:]
		os.Remove(filepath.Join(s.dataDir, removed.ID+".json"))
	}

	if err := s.saveToDisk(snap); err != nil {
		return id, fmt.Errorf("snapshot: saved in memory but failed to persist: %w", err)
	}

	return id, nil
}

func (s *SnapshotStore) Rollback(id string, adapter engine.EngineAdapter) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var snap *Snapshot
	for i := range s.snapshots {
		if s.snapshots[i].ID == id {
			snap = &s.snapshots[i]
			break
		}
	}
	if snap == nil {
		return fmt.Errorf("snapshot %q not found", id)
	}

	for groupTag, proxyTag := range snap.ActiveProxies {
		if err := adapter.SetActiveProxy(groupTag, proxyTag); err != nil {
			return fmt.Errorf("rollback: failed to restore %s → %s: %w", groupTag, proxyTag, err)
		}
	}
	return nil
}

func (s *SnapshotStore) Latest() *Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.snapshots) == 0 {
		return nil
	}
	snap := s.snapshots[len(s.snapshots)-1]
	return &snap
}

func (s *SnapshotStore) List() []Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Snapshot, len(s.snapshots))
	copy(out, s.snapshots)
	return out
}

func (s *SnapshotStore) saveToDisk(snap Snapshot) error {
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dataDir, snap.ID+".json"), data, 0644)
}

func (s *SnapshotStore) loadFromDisk() {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dataDir, e.Name()))
		if err != nil {
			continue
		}
		var snap Snapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}
		s.snapshots = append(s.snapshots, snap)
	}
	sort.Slice(s.snapshots, func(i, j int) bool {
		return s.snapshots[i].Timestamp.Before(s.snapshots[j].Timestamp)
	})
	s.counter = len(s.snapshots)
}

func isGroupType(t string) bool {
	switch t {
	case "Selector", "selector", "URLTest", "urltest", "Fallback", "fallback", "LoadBalance", "load-balance":
		return true
	}
	return false
}
