package overlay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// RouteRule 是 overlay 中的一条路由规则
//
// sing-box v1.13.8 rule schema 参考: option/rule.go:DefaultRule
// matcher 都是 Listable[string/int/bool], 空切片在 JSON 里 omit。
// rule_set / geoip / geosite 必须在 overlay.RuleSets 顶层先声明后引用,
// 引用失败会被 sing-box 拒绝加载。
type RouteRule struct {
	Tag           string   `json:"tag"`
	DomainSuffix  []string `json:"domain_suffix,omitempty"`
	Domain        []string `json:"domain,omitempty"`
	DomainKeyword []string `json:"domain_keyword,omitempty"`
	DomainRegex   []string `json:"domain_regex,omitempty"`
	IPCidr        []string `json:"ip_cidr,omitempty"`
	ProcessName   []string `json:"process_name,omitempty"`
	Port          []int    `json:"port,omitempty"`
	PortRange     []string `json:"port_range,omitempty"`    // eg "8000:9000"
	Network       []string `json:"network,omitempty"`       // "tcp" | "udp"
	Protocol      []string `json:"protocol,omitempty"`      // "http" | "tls" | "quic" | "dns" | ...
	RuleSet       []string `json:"rule_set,omitempty"`      // 引用 OverlayData.RuleSets[].Tag
	Geoip         []string `json:"geoip,omitempty"`         // sing-box deprecated 字段 (建议改用 rule_set)
	Geosite       []string `json:"geosite,omitempty"`       // sing-box deprecated 字段 (建议改用 rule_set)
	IPIsPrivate   bool     `json:"ip_is_private,omitempty"` // sing-box 原生, 匹配 RFC1918 / loopback / 链路本地 IP; 替代 geoip-private 外部 .srs
	Outbound      string   `json:"outbound"`
	Description   string   `json:"description"`
	Source        string   `json:"source"` // "agent" | "template:xxx" | "user"
}

// RuleSetConfig 是 overlay 里声明的 sing-box rule-set.
// 会被 merger 写到 merged.json 的 route.rule_set 顶层数组, 供 rule.rule_set 引用.
//
// Type:
//   - "remote" (必填 URL; UpdateInterval 默认 7d 由 merger 兜底)
//   - "local"  (必填 Path)
//
// Format:
//   - "binary" (.srs, sing-box 官方默认, 体积小)
//   - "source" (.json, 可读但大几倍)
//     为空时 merger 按 URL/Path 扩展名自动填。
//
// 参考: sing-box 1.13.8 option/rule_set.go:RuleSet.
type RuleSetConfig struct {
	Tag            string `json:"tag"`
	Type           string `json:"type"`   // "remote" | "local"
	Format         string `json:"format"` // "binary" | "source"
	URL            string `json:"url,omitempty"`
	Path           string `json:"path,omitempty"`
	DownloadDetour string `json:"download_detour,omitempty"` // remote 下载走哪个 outbound
	UpdateInterval string `json:"update_interval,omitempty"` // 如 "7d" "24h", remote only
	Source         string `json:"source"`                    // "builtin:alias" | "user" | "template:xxx"
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
	RouteRules  []RouteRule              `json:"route_rules"`
	RuleSets    []RuleSetConfig          `json:"rule_sets,omitempty"`
	Outbounds   []map[string]interface{} `json:"outbounds,omitempty"`
	DNS         *DNSConfig               `json:"dns,omitempty"`
	TunOverride *TunInboundOverride      `json:"tun_override,omitempty"` // Agent 写 Per-App VPN, Android ConfigMerger.injectPerAppRules 会覆盖 (UI 优先)
}

// TunInboundOverride 控制 tun inbound 的 include_package / exclude_package 字段 (Android Per-App VPN)。
// Mode:
//   - "off"   → 不写, TUN inbound 不过滤包名 (所有 App 走代理, 默认行为)
//   - "allow" → include_package = Packages, 只有列表内 App 走代理
//   - "deny"  → exclude_package = Packages, 列表内 App 直连, 其他走代理
//
// 设计: Agent "让 Chrome 走 VPN" 型需求通过此结构写入; Android UI 的 PerAppVpnPrefs 是独立数据源
// (SharedPreferences), ConfigMerger.injectPerAppRules 在 Kotlin 层合并时会覆盖 overlay.json 的此字段
// (UI win)。 桌面 sing-box (mixed inbound) 场景 include_package 字段被忽略, 无副作用。
type TunInboundOverride struct {
	Mode     string   `json:"mode"`               // "off" / "allow" / "deny"
	Packages []string `json:"packages,omitempty"` // Android 包名, 如 "com.android.chrome"
}

// ConfigOverlay 管理增量配置叠加层
type ConfigOverlay struct {
	baseConfigPath   string // configs/minimal.json（只读）
	overlayPath      string // data/overlay.json
	mergedConfigPath string // data/merged.json（sing-box 实际用的）
	data             OverlayData
	mu               sync.RWMutex
	// applyHook Apply 写完 merged.json + adapter.Reload 后调一次 (无错时).
	// 用于 Android 模式 — adapter.Reload 在嵌入式 libbox 下是 noop, 必须靠平台层 (Kotlin
	// PilottyVpnService) 触发 libbox.startOrReloadService. 调用方在 mobile binding 里
	// 把 reloaderAdapter 包成本 hook 注入, 一次注入覆盖所有写 overlay 的 tool / 订阅 manager.
	// nil 时不调; CLI/Server 模式 hook 为 nil, 走 adapter.Reload 的 pkill+exec 兜底.
	applyHook func()
}

func NewConfigOverlay(baseConfigPath, dataDir string) *ConfigOverlay {
	return &ConfigOverlay{
		baseConfigPath:   baseConfigPath,
		overlayPath:      filepath.Join(dataDir, "overlay.json"),
		mergedConfigPath: filepath.Join(dataDir, "merged.json"),
	}
}

// SetApplyHook 注入 Apply 成功后的回调 (Android 用于触发 PilottyVpnService.requestReload).
// nil 等价不注入; 重复调用以最后一次为准. 线程安全 (Apply 与 hook 无并发竞争, 由 mu 保护).
func (o *ConfigOverlay) SetApplyHook(h func()) {
	o.mu.Lock()
	o.applyHook = h
	o.mu.Unlock()
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

// SetPerAppVpn 设置 TUN inbound 的应用级过滤 (Android Per-App VPN).
// mode="off" 时清除字段, 恢复默认 (所有 App 走代理).
func (o *ConfigOverlay) SetPerAppVpn(mode string, packages []string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "off", "allow", "deny":
	default:
		return fmt.Errorf("无效 mode: %q (允许 off/allow/deny)", mode)
	}
	if (mode == "allow" || mode == "deny") && len(packages) == 0 {
		return fmt.Errorf("mode=%s 需要至少一个 package", mode)
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if mode == "off" {
		o.data.TunOverride = nil
	} else {
		cleaned := make([]string, 0, len(packages))
		seen := make(map[string]struct{}, len(packages))
		for _, p := range packages {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			cleaned = append(cleaned, p)
		}
		o.data.TunOverride = &TunInboundOverride{Mode: mode, Packages: cleaned}
	}
	return o.saveLocked()
}

// GetPerAppVpn 返回当前 Per-App VPN 设置的只读副本, 未设置返回 nil.
func (o *ConfigOverlay) GetPerAppVpn() *TunInboundOverride {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.data.TunOverride == nil {
		return nil
	}
	cp := *o.data.TunOverride
	cp.Packages = append([]string(nil), o.data.TunOverride.Packages...)
	return &cp
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

// AddRuleSet 添加/覆盖一条 rule-set 声明
func (o *ConfigOverlay) AddRuleSet(rs RuleSetConfig) error {
	if rs.Tag == "" {
		return fmt.Errorf("rule_set 缺 tag")
	}
	switch rs.Type {
	case "remote":
		if rs.URL == "" {
			return fmt.Errorf("remote rule_set 缺 url (tag=%s)", rs.Tag)
		}
	case "local":
		if rs.Path == "" {
			return fmt.Errorf("local rule_set 缺 path (tag=%s)", rs.Tag)
		}
	default:
		return fmt.Errorf("不支持的 rule_set type: %q (tag=%s)", rs.Type, rs.Tag)
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	for i, existing := range o.data.RuleSets {
		if existing.Tag == rs.Tag {
			o.data.RuleSets[i] = rs
			return o.saveLocked()
		}
	}
	o.data.RuleSets = append(o.data.RuleSets, rs)
	return o.saveLocked()
}

// RemoveRuleSet 按 tag 删除一个 rule-set 声明。被引用的 rule 不会自动清理,
// 所以返回失败时上层应考虑先把引用该 tag 的 rule 清掉。
func (o *ConfigOverlay) RemoveRuleSet(tag string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	found := false
	var kept []RuleSetConfig
	for _, rs := range o.data.RuleSets {
		if rs.Tag == tag {
			found = true
			continue
		}
		kept = append(kept, rs)
	}
	if !found {
		return fmt.Errorf("rule_set %q 不存在", tag)
	}
	o.data.RuleSets = kept
	return o.saveLocked()
}

// ListRuleSets 返回所有 rule-set 声明的副本
func (o *ConfigOverlay) ListRuleSets() []RuleSetConfig {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]RuleSetConfig, len(o.data.RuleSets))
	copy(out, o.data.RuleSets)
	return out
}

// EnableBuiltinRuleSet 把一个内置别名(geoip-cn / geosite-cn / ...)加入 overlay。
// 如果同 tag 已存在, 用内置配置覆盖(允许用户通过"关闭+再启用"的方式重置被改过的条目)。
func (o *ConfigOverlay) EnableBuiltinRuleSet(alias string) error {
	rs, ok := BuiltinRuleSets[alias]
	if !ok {
		return fmt.Errorf("未知 builtin rule-set: %q (可选: %v)", alias, BuiltinRuleSetAliases())
	}
	return o.AddRuleSet(rs)
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

	// 重载 sing-box (CLI: pkill+exec; Android 嵌入式: noop, 真 reload 走下面 applyHook)
	if err := adapter.Reload(); err != nil {
		return fmt.Errorf("重载 sing-box 失败: %w", err)
	}
	// 平台 hook (Android: PilottyVpnService.requestReload → libbox.startOrReloadService).
	// 一处统一调, 让 patch_route_rule / create_chain / 订阅 CRUD / Per-App / DNS 等所有
	// 写 overlay 路径都能在 Android 上热生效, 不需要每个 tool 单独接 reloader.
	o.mu.RLock()
	hook := o.applyHook
	o.mu.RUnlock()
	if hook != nil {
		hook()
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
		RuleSets:   make([]RuleSetConfig, len(o.data.RuleSets)),
		Outbounds:  make([]map[string]interface{}, len(o.data.Outbounds)),
		DNS:        o.data.DNS,
	}
	copy(result.RouteRules, o.data.RouteRules)
	copy(result.RuleSets, o.data.RuleSets)
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
