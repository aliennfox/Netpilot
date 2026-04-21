package overlay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// RouteRule 是 overlay 中的一条路由规则
type RouteRule struct {
	Tag          string   `json:"tag"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
	Domain       []string `json:"domain,omitempty"`
	IPCidr       []string `json:"ip_cidr,omitempty"`
	ProcessName  []string `json:"process_name,omitempty"`
	Outbound     string   `json:"outbound"`
	Description  string   `json:"description"`
	Source       string   `json:"source"` // "agent" | "template:xxx" | "user"
}

// DNSServer 是 overlay 中的 DNS 服务器配置（sing-box 1.12+ 新格式）
type DNSServer struct {
	Type       string `json:"type"` // "tls", "udp", "local"
	Tag        string `json:"tag"`
	Server     string `json:"server,omitempty"` // IP 或域名
	ServerPort int    `json:"server_port,omitempty"`
	Detour     string `json:"detour,omitempty"`
}

// DNSRule 是 overlay 中的 DNS 路由规则（sing-box 1.12+ 新格式）
type DNSRule struct {
	Action       string   `json:"action"` // "route" | "reject"
	Server       string   `json:"server,omitempty"`
	Outbound     string   `json:"outbound,omitempty"`
	Domain       []string `json:"domain,omitempty"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
}

// DNSConfig 是 overlay 中的 DNS 配置
type DNSConfig struct {
	Servers  []DNSServer `json:"servers"`
	Rules    []DNSRule   `json:"rules,omitempty"`
	Final    string      `json:"final"` // 默认 DNS 服务器 tag
	Strategy string      `json:"strategy"`
}

// OverlayData 是持久化到文件的 overlay 数据
type OverlayData struct {
	RouteRules []RouteRule              `json:"route_rules"`
	Outbounds  []map[string]interface{} `json:"outbounds,omitempty"`
	DNS        *DNSConfig               `json:"dns,omitempty"`
}

// ConfigOverlay 管理增量配置叠加层
type ConfigOverlay struct {
	baseConfigPath   string // configs/minimal.json（只读）
	overlayPath      string // data/overlay.json
	mergedConfigPath string // data/merged.json（sing-box 实际用的）
	data             OverlayData
	mu               sync.RWMutex
}

func NewConfigOverlay(baseConfigPath, dataDir string) *ConfigOverlay {
	return &ConfigOverlay{
		baseConfigPath:   baseConfigPath,
		overlayPath:      filepath.Join(dataDir, "overlay.json"),
		mergedConfigPath: filepath.Join(dataDir, "merged.json"),
	}
}

// MergedConfigPath 返回合并后的配置文件路径
func (o *ConfigOverlay) MergedConfigPath() string {
	return o.mergedConfigPath
}

// Load 从文件加载 overlay 数据
func (o *ConfigOverlay) Load() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	data, err := os.ReadFile(o.overlayPath)
	if err != nil {
		if os.IsNotExist(err) {
			o.data = OverlayData{}
			return nil
		}
		return fmt.Errorf("读取 overlay 文件失败: %w", err)
	}
	return json.Unmarshal(data, &o.data)
}

// Save 保存 overlay 数据到文件
func (o *ConfigOverlay) Save() error {
	dir := filepath.Dir(o.overlayPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	data, err := json.MarshalIndent(o.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(o.overlayPath, data, 0644)
}

// AddRule 添加一条路由规则
func (o *ConfigOverlay) AddRule(rule RouteRule) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 如果已存在同 tag 的规则，覆盖
	for i, r := range o.data.RouteRules {
		if r.Tag == rule.Tag {
			o.data.RouteRules[i] = rule
			return o.saveLocked()
		}
	}
	o.data.RouteRules = append(o.data.RouteRules, rule)
	return o.saveLocked()
}

// RemoveRule 按 tag 删除规��
func (o *ConfigOverlay) RemoveRule(tag string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	found := false
	var kept []RouteRule
	for _, r := range o.data.RouteRules {
		if r.Tag == tag {
			found = true
			continue
		}
		kept = append(kept, r)
	}
	if !found {
		return fmt.Errorf("规则 %q 不存在", tag)
	}
	o.data.RouteRules = kept
	return o.saveLocked()
}

// ListRules 列出所有 overlay 规则
func (o *ConfigOverlay) ListRules() []RouteRule {
	o.mu.RLock()
	defer o.mu.RUnlock()
	result := make([]RouteRule, len(o.data.RouteRules))
	copy(result, o.data.RouteRules)
	return result
}

// ListOutbounds 列出所有 overlay 外加节点 (订阅/Agent 注入的)。
// Android/iOS 上当 VPN 未启动时 Clash API 不可达, UI 依赖此函数展示节点列表。
func (o *ConfigOverlay) ListOutbounds() []map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()
	result := make([]map[string]interface{}, len(o.data.Outbounds))
	for i, ob := range o.data.Outbounds {
		copyOB := make(map[string]interface{}, len(ob))
		for k, v := range ob {
			copyOB[k] = v
		}
		result[i] = copyOB
	}
	return result
}

// AddOutbound 添加额外出站节点（完整 outbound 配置）
func (o *ConfigOverlay) AddOutbound(ob map[string]interface{}) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	tag, _ := ob["tag"].(string)
	for i, existing := range o.data.Outbounds {
		if t, _ := existing["tag"].(string); t == tag {
			o.data.Outbounds[i] = ob
			return o.saveLocked()
		}
	}
	o.data.Outbounds = append(o.data.Outbounds, ob)
	return o.saveLocked()
}

// AddOutboundsBatch 批量添加出站节点（统一保存一次）
func (o *ConfigOverlay) AddOutboundsBatch(obs []map[string]interface{}) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	tagIdx := map[string]int{}
	for i, existing := range o.data.Outbounds {
		if t, _ := existing["tag"].(string); t != "" {
			tagIdx[t] = i
		}
	}

	for _, ob := range obs {
		tag, _ := ob["tag"].(string)
		if idx, exists := tagIdx[tag]; exists {
			o.data.Outbounds[idx] = ob
		} else {
			tagIdx[tag] = len(o.data.Outbounds)
			o.data.Outbounds = append(o.data.Outbounds, ob)
		}
	}
	return o.saveLocked()
}

// RemoveOutboundsByTags 按 tag 列表批量删除出站节点
func (o *ConfigOverlay) RemoveOutboundsByTags(tags []string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	remove := map[string]bool{}
	for _, t := range tags {
		remove[t] = true
	}
	var kept []map[string]interface{}
	for _, ob := range o.data.Outbounds {
		if tag, _ := ob["tag"].(string); !remove[tag] {
			kept = append(kept, ob)
		}
	}
	o.data.Outbounds = kept
	return o.saveLocked()
}

// FindOutboundByTag 在 base 配置和 overlay 中查找指定 tag 的 outbound（返回深拷贝）
func (o *ConfigOverlay) FindOutboundByTag(tag string) (map[string]interface{}, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// 先查 overlay
	for _, ob := range o.data.Outbounds {
		if t, _ := ob["tag"].(string); t == tag {
			return deepCopyMap(ob), nil
		}
	}

	// 再查 base config
	baseData, err := os.ReadFile(o.baseConfigPath)
	if err != nil {
		return nil, fmt.Errorf("读取 base config 失败: %w", err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(baseData, &config); err != nil {
		return nil, fmt.Errorf("解析 base config 失败: %w", err)
	}
	if raw, ok := config["outbounds"].([]interface{}); ok {
		for _, ob := range raw {
			if m, ok := ob.(map[string]interface{}); ok {
				if t, _ := m["tag"].(string); t == tag {
					return deepCopyMap(m), nil
				}
			}
		}
	}
	return nil, fmt.Errorf("未找到 outbound: %s", tag)
}

func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	b, _ := json.Marshal(m)
	var out map[string]interface{}
	_ = json.Unmarshal(b, &out)
	return out
}

// OutboundTags 返回所有 overlay 出站节点的 tag
func (o *ConfigOverlay) OutboundTags() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	var tags []string
	for _, ob := range o.data.Outbounds {
		if tag, _ := ob["tag"].(string); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// Apply 合并 base + overlay → merged，然后重载 sing-box
func (o *ConfigOverlay) Apply(adapter engine.EngineAdapter) error {
	o.mu.RLock()
	merged, err := MergeConfigs(o.baseConfigPath, &o.data)
	o.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("合并配置失败: %w", err)
	}

	// 写入 merged 配置
	dir := filepath.Dir(o.mergedConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(o.mergedConfigPath, merged, 0644); err != nil {
		return fmt.Errorf("写入 merged 配置失败: %w", err)
	}

	// 确保 adapter 使用 merged 配置路径
	absPath, _ := filepath.Abs(o.mergedConfigPath)
	type configPathSetter interface {
		SetConfigPath(string)
	}
	if setter, ok := adapter.(configPathSetter); ok {
		setter.SetConfigPath(absPath)
	}

	// 重载 sing-box
	if err := adapter.Reload(); err != nil {
		return fmt.Errorf("重载 sing-box 失败: %w", err)
	}
	return nil
}

// SetDNS 设置 DNS 配置
func (o *ConfigOverlay) SetDNS(dns *DNSConfig) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.data.DNS = dns
	return o.saveLocked()
}

// GetDNS 获取当前 DNS 配置
func (o *ConfigOverlay) GetDNS() *DNSConfig {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.data.DNS == nil {
		return nil
	}
	cp := *o.data.DNS
	cp.Servers = make([]DNSServer, len(o.data.DNS.Servers))
	copy(cp.Servers, o.data.DNS.Servers)
	cp.Rules = make([]DNSRule, len(o.data.DNS.Rules))
	copy(cp.Rules, o.data.DNS.Rules)
	return &cp
}

// GetData 返回当前 overlay 数据的副本
func (o *ConfigOverlay) GetData() OverlayData {
	o.mu.RLock()
	defer o.mu.RUnlock()
	result := OverlayData{
		RouteRules: make([]RouteRule, len(o.data.RouteRules)),
		Outbounds:  make([]map[string]interface{}, len(o.data.Outbounds)),
		DNS:        o.data.DNS,
	}
	copy(result.RouteRules, o.data.RouteRules)
	copy(result.Outbounds, o.data.Outbounds)
	return result
}

// SetData 整体替换 overlay 数据并立即持久化。供 backup 导入使用。
func (o *ConfigOverlay) SetData(d OverlayData) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.data = d
	return o.saveLocked()
}

func (o *ConfigOverlay) saveLocked() error {
	dir := filepath.Dir(o.overlayPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	data, err := json.MarshalIndent(o.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(o.overlayPath, data, 0644)
}
