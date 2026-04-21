// Package backup 实现 Pilotty 用户数据的导出/导入。
//
// 导出范围：
//   - overlay：路由规则、agent outbounds、DNS 配置
//   - subscriptions：订阅源列表（含 ID/URL/Tags/UserInfo）
//
// 不包含：
//   - sing-box 内核基础配置（base config，由内置文件提供）
//   - 对话历史、快照、日志（瞬态）
//   - LLM API key（敏感信息，平台层另行管理）
//
// 文件格式：单个 JSON 对象，version 字段用于未来兼容。
package backup

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/foxnetpilot/netpilot/internal/overlay"
	"github.com/foxnetpilot/netpilot/internal/subscription"
)

// Version 当前备份文件版本号。导入时校验。
const Version = "netpilot-backup/1"

// Snapshot 是一份完整的用户数据备份。
type Snapshot struct {
	Version       string                      `json:"version"`
	ExportedAt    time.Time                   `json:"exported_at"`
	Overlay       overlay.OverlayData         `json:"overlay"`
	Subscriptions []subscription.Subscription `json:"subscriptions"`
}

// Manager 协调 overlay 与 subscription store 的导出/导入。
type Manager struct {
	overlay  *overlay.ConfigOverlay
	subStore *subscription.SubscriptionStore
}

func NewManager(ov *overlay.ConfigOverlay, store *subscription.SubscriptionStore) *Manager {
	return &Manager{overlay: ov, subStore: store}
}

// Export 生成当前用户数据快照（已序列化为 JSON 字节流）。
func (m *Manager) Export() ([]byte, error) {
	snap := Snapshot{
		Version:       Version,
		ExportedAt:    time.Now().UTC(),
		Overlay:       m.overlay.GetData(),
		Subscriptions: m.subStore.List(),
	}
	return json.MarshalIndent(snap, "", "  ")
}

// Import 从 JSON 字节流恢复用户数据。
//
// 当前策略：整体替换。调用方应在 UI 上提示「会覆盖现有规则与订阅」。
// 导入成功后，调用方应触发订阅重新拉取以获取最新节点。
func (m *Manager) Import(data []byte) (*Snapshot, error) {
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("解析备份文件失败: %w", err)
	}
	if snap.Version == "" {
		return nil, fmt.Errorf("备份文件缺少 version 字段")
	}
	if snap.Version != Version {
		return nil, fmt.Errorf("备份版本不兼容: 期望 %s，实际 %s", Version, snap.Version)
	}
	if err := m.overlay.SetData(snap.Overlay); err != nil {
		return nil, fmt.Errorf("写入 overlay 失败: %w", err)
	}
	if err := m.subStore.ReplaceAll(snap.Subscriptions); err != nil {
		return nil, fmt.Errorf("写入订阅失败: %w", err)
	}
	return &snap, nil
}
