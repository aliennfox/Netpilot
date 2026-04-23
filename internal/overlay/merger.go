package overlay

import (
	"encoding/json"
	"fmt"
	"os"
)

// MergeConfigs 将 base config + overlay data 合并为最终配置
// overlay 规则插入到 route.rules 开头（优先匹配），outbound 追加到末尾
func MergeConfigs(basePath string, overlay *OverlayData) ([]byte, error) {
	baseData, err := os.ReadFile(basePath)
	if err != nil {
		return nil, fmt.Errorf("读取 base config 失���: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(baseData, &config); err != nil {
		return nil, fmt.Errorf("解析 base config 失败: %w", err)
	}

	if overlay == nil || (len(overlay.RouteRules) == 0 && len(overlay.Outbounds) == 0 && overlay.DNS == nil) {
		// 即使没有 overlay 数据，也注入默认 DNS
		if _, hasDNS := config["dns"]; !hasDNS {
			injectDefaultDNS(config)
		}
		return json.MarshalIndent(config, "", "  ")
	}

	// 合并 rule-set 声明 (顶层, 必须先于 rules 处理, 否则 rule 里引用空 tag 会被 sing-box 拒)
	if len(overlay.RuleSets) > 0 {
		mergeRuleSets(config, overlay.RuleSets)
	}

	// 合并路由规则
	if len(overlay.RouteRules) > 0 {
		mergeRouteRules(config, overlay.RouteRules)
	}

	// 合并出站节点
	if len(overlay.Outbounds) > 0 {
		mergeOutbounds(config, overlay.Outbounds)
	}

	// 合并 DNS 配置
	mergeDNS(config, overlay.DNS)

	return json.MarshalIndent(config, "", "  ")
}

// mergeRouteRules 将 overlay 规则插入到 route.rules 开头
func mergeRouteRules(config map[string]interface{}, rules []RouteRule) {
	// 确保 route 字段存在
	routeRaw, ok := config["route"]
	if !ok {
		routeRaw = map[string]interface{}{}
		config["route"] = routeRaw
	}
	route, ok := routeRaw.(map[string]interface{})
	if !ok {
		route = map[string]interface{}{}
		config["route"] = route
	}

	// 获取现有 rules 数组
	var existingRules []interface{}
	if rulesRaw, ok := route["rules"]; ok {
		if arr, ok := rulesRaw.([]interface{}); ok {
			existingRules = arr
		}
	}

	// 将 overlay 规则转为 sing-box route rule 格式，插入开头
	var overlayRules []interface{}
	for _, r := range rules {
		rule := map[string]interface{}{
			"outbound": r.Outbound,
		}
		if len(r.DomainSuffix) > 0 {
			rule["domain_suffix"] = toInterfaceSlice(r.DomainSuffix)
		}
		if len(r.Domain) > 0 {
			rule["domain"] = toInterfaceSlice(r.Domain)
		}
		if len(r.DomainKeyword) > 0 {
			rule["domain_keyword"] = toInterfaceSlice(r.DomainKeyword)
		}
		if len(r.DomainRegex) > 0 {
			rule["domain_regex"] = toInterfaceSlice(r.DomainRegex)
		}
		if len(r.IPCidr) > 0 {
			rule["ip_cidr"] = toInterfaceSlice(r.IPCidr)
		}
		if len(r.ProcessName) > 0 {
			rule["process_name"] = toInterfaceSlice(r.ProcessName)
		}
		if len(r.Port) > 0 {
			rule["port"] = toInterfaceIntSlice(r.Port)
		}
		if len(r.PortRange) > 0 {
			rule["port_range"] = toInterfaceSlice(r.PortRange)
		}
		if len(r.Network) > 0 {
			rule["network"] = toInterfaceSlice(r.Network)
		}
		if len(r.Protocol) > 0 {
			rule["protocol"] = toInterfaceSlice(r.Protocol)
		}
		if len(r.RuleSet) > 0 {
			rule["rule_set"] = toInterfaceSlice(r.RuleSet)
		}
		if len(r.Geoip) > 0 {
			rule["geoip"] = toInterfaceSlice(r.Geoip)
		}
		if len(r.Geosite) > 0 {
			rule["geosite"] = toInterfaceSlice(r.Geosite)
		}
		overlayRules = append(overlayRules, rule)
	}

	// overlay 规则在前，base 规则在后
	merged := append(overlayRules, existingRules...)
	route["rules"] = merged
}

// isEndpointType 判断 sing-box 1.13+ outbound type 是否属于 "endpoints" 注册表。
// 这些协议 schema 上不是 dialer 型, 必须放在 merged.json 的独立 endpoints 段。
// 当前仅 wireguard, 后续 sing-box 迁更多可在这里扩。
func isEndpointType(typ string) bool {
	return typ == "wireguard"
}

// mergeOutbounds 将 overlay 出站追加到 outbounds / endpoints 末尾（跳过重复 tag），
// 并将新节点 tag 添加到第一个 selector group 的 outbounds 列表中。
//
// #M25: endpoint 型 (wireguard) 必须放在 merged.endpoints 而非 outbounds,
// selector 仍引用其 tag (sing-box 把 endpoint.tag 和 outbound.tag 放同一命名空间)。
func mergeOutbounds(config map[string]interface{}, outbounds []map[string]interface{}) {
	var existingOutbounds []interface{}
	if raw, ok := config["outbounds"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			existingOutbounds = arr
		}
	}
	var existingEndpoints []interface{}
	if raw, ok := config["endpoints"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			existingEndpoints = arr
		}
	}

	// 收集已有 tag (outbounds 和 endpoints 共享 tag 命名空间)
	existingTags := map[string]bool{}
	for _, ob := range existingOutbounds {
		if m, ok := ob.(map[string]interface{}); ok {
			if tag, ok := m["tag"].(string); ok {
				existingTags[tag] = true
			}
		}
	}
	for _, ep := range existingEndpoints {
		if m, ok := ep.(map[string]interface{}); ok {
			if tag, ok := m["tag"].(string); ok {
				existingTags[tag] = true
			}
		}
	}

	// 按 type 分派: endpoint 型 → endpoints 段, 其它 → outbounds 段
	var newTags []string
	for _, ob := range outbounds {
		tag, _ := ob["tag"].(string)
		if existingTags[tag] {
			continue
		}
		typ, _ := ob["type"].(string)
		if isEndpointType(typ) {
			existingEndpoints = append(existingEndpoints, ob)
		} else {
			existingOutbounds = append(existingOutbounds, ob)
		}
		existingTags[tag] = true
		newTags = append(newTags, tag)
	}

	config["outbounds"] = existingOutbounds
	if len(existingEndpoints) > 0 {
		config["endpoints"] = existingEndpoints
	}

	// selector 在 outbounds 里 (它本身是 outbound 类型), 引用所有可选 tag 包括 endpoint tag
	if len(newTags) > 0 {
		addTagsToSelector(existingOutbounds, newTags)
	}
}

// addTagsToSelector 将新 tag 添加到第一个 selector 类型 outbound 的 outbounds 列表中
func addTagsToSelector(outbounds []interface{}, tags []string) {
	for _, ob := range outbounds {
		m, ok := ob.(map[string]interface{})
		if !ok {
			continue
		}
		t, _ := m["type"].(string)
		if t != "selector" {
			continue
		}
		// 找到 selector，添加 tag
		var existing []interface{}
		if raw, ok := m["outbounds"]; ok {
			if arr, ok := raw.([]interface{}); ok {
				existing = arr
			}
		}
		existingSet := map[string]bool{}
		for _, e := range existing {
			if s, ok := e.(string); ok {
				existingSet[s] = true
			}
		}
		for _, tag := range tags {
			if !existingSet[tag] {
				existing = append(existing, tag)
			}
		}
		m["outbounds"] = existing
		return // 只处理第一个 selector
	}
}

func toInterfaceSlice(ss []string) []interface{} {
	result := make([]interface{}, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}

func toInterfaceIntSlice(ns []int) []interface{} {
	result := make([]interface{}, len(ns))
	for i, n := range ns {
		result[i] = n
	}
	return result
}

// mergeRuleSets 把 overlay.RuleSets 写入 route.rule_set 顶层。
// 与 base config 已有的 rule_set 合并时按 tag 去重,overlay 同 tag 覆盖 base
// (允许用户通过改 overlay 调整默认 ruleset 的 update_interval 等)。
func mergeRuleSets(config map[string]interface{}, sets []RuleSetConfig) {
	routeRaw, ok := config["route"]
	if !ok {
		routeRaw = map[string]interface{}{}
		config["route"] = routeRaw
	}
	route, ok := routeRaw.(map[string]interface{})
	if !ok {
		route = map[string]interface{}{}
		config["route"] = route
	}

	// 收集 base 已有的 rule_set (按 tag 索引)
	existingByTag := map[string]int{}
	var existing []interface{}
	if raw, ok := route["rule_set"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			existing = arr
			for i, item := range existing {
				if m, ok := item.(map[string]interface{}); ok {
					if tag, _ := m["tag"].(string); tag != "" {
						existingByTag[tag] = i
					}
				}
			}
		}
	}

	for _, rs := range sets {
		if rs.Tag == "" {
			continue
		}
		obj := ruleSetConfigToMap(rs)
		if idx, has := existingByTag[rs.Tag]; has {
			existing[idx] = obj
		} else {
			existingByTag[rs.Tag] = len(existing)
			existing = append(existing, obj)
		}
	}
	route["rule_set"] = existing
}

// ruleSetConfigToMap 把 RuleSetConfig 平铺成 sing-box 能吃的 map.
// Format 为空且 URL/Path 扩展名能推出时, sing-box 自己兜底推断,
// 所以我们只把用户显式填的写进去, 不主动注入默认值。
func ruleSetConfigToMap(rs RuleSetConfig) map[string]interface{} {
	m := map[string]interface{}{
		"type": rs.Type,
		"tag":  rs.Tag,
	}
	if rs.Format != "" {
		m["format"] = rs.Format
	}
	switch rs.Type {
	case "remote":
		if rs.URL != "" {
			m["url"] = rs.URL
		}
		if rs.DownloadDetour != "" {
			m["download_detour"] = rs.DownloadDetour
		}
		if rs.UpdateInterval != "" {
			m["update_interval"] = rs.UpdateInterval
		}
	case "local":
		if rs.Path != "" {
			m["path"] = rs.Path
		}
	}
	return m
}

// mergeDNS 将 overlay DNS 配置写入最终配置；如果 overlay 没有 DNS 且 base 也没有，注入默认配置
func mergeDNS(config map[string]interface{}, dns *DNSConfig) {
	if dns != nil {
		config["dns"] = dnsConfigToMap(dns)
		ensureDefaultDomainResolver(config)
		return
	}
	// overlay 没有 DNS 配置，检查 base 是否已有
	if _, hasDNS := config["dns"]; !hasDNS {
		injectDefaultDNS(config)
	}
}

// injectDefaultDNS 注入默认的安全 DNS 配置（secure 模式：全部走代理 DNS）
func injectDefaultDNS(config map[string]interface{}) {
	config["dns"] = dnsConfigToMap(DefaultDNSConfig("secure"))
	ensureDefaultDomainResolver(config)
}

// ensureDefaultDomainResolver 确保 route.default_domain_resolver 存在（sing-box 1.12+ 必需）
func ensureDefaultDomainResolver(config map[string]interface{}) {
	routeRaw, ok := config["route"]
	if !ok {
		routeRaw = map[string]interface{}{}
		config["route"] = routeRaw
	}
	route, ok := routeRaw.(map[string]interface{})
	if !ok {
		route = map[string]interface{}{}
		config["route"] = route
	}
	if _, has := route["default_domain_resolver"]; !has {
		route["default_domain_resolver"] = "local-dns"
	}
}

// DefaultDNSConfig 根据模式生成默认 DNS 配置（sing-box 1.12+ 格式）
func DefaultDNSConfig(mode string) *DNSConfig {
	servers := []DNSServer{
		{Type: "tls", Tag: "proxy-dns", Server: "8.8.8.8", ServerPort: 853, Detour: "proxy-group"},
		{Type: "udp", Tag: "direct-dns", Server: "223.5.5.5", ServerPort: 53},
		{Type: "local", Tag: "local-dns"},
	}

	var rules []DNSRule
	finalServer := "proxy-dns"

	switch mode {
	case "split":
		// 直连流量走本地 DNS，其余走代理 DNS
		rules = []DNSRule{
			{Action: "route", Server: "direct-dns", Outbound: "direct-out"},
		}
		finalServer = "proxy-dns"
	case "local":
		finalServer = "direct-dns"
	default: // "secure"
		finalServer = "proxy-dns"
	}

	return &DNSConfig{
		Servers:  servers,
		Rules:    rules,
		Final:    finalServer,
		Strategy: "prefer_ipv4",
	}
}

// dnsConfigToMap 将 DNSConfig 转为 sing-box 1.12+ dns 配置的 map
func dnsConfigToMap(dns *DNSConfig) map[string]interface{} {
	var servers []interface{}
	for _, s := range dns.Servers {
		srv := map[string]interface{}{
			"type": s.Type,
			"tag":  s.Tag,
		}
		if s.Server != "" {
			srv["server"] = s.Server
		}
		if s.ServerPort > 0 {
			srv["server_port"] = s.ServerPort
		}
		if s.Detour != "" {
			srv["detour"] = s.Detour
		}
		servers = append(servers, srv)
	}

	var rules []interface{}
	for _, r := range dns.Rules {
		rule := map[string]interface{}{
			"action": r.Action,
		}
		if r.Server != "" {
			rule["server"] = r.Server
		}
		if r.Outbound != "" {
			rule["outbound"] = r.Outbound
		}
		if len(r.Domain) > 0 {
			rule["domain"] = toInterfaceSlice(r.Domain)
		}
		if len(r.DomainSuffix) > 0 {
			rule["domain_suffix"] = toInterfaceSlice(r.DomainSuffix)
		}
		rules = append(rules, rule)
	}

	result := map[string]interface{}{
		"servers": servers,
	}
	if len(rules) > 0 {
		result["rules"] = rules
	}
	if dns.Final != "" {
		result["final"] = dns.Final
	}
	if dns.Strategy != "" {
		result["strategy"] = dns.Strategy
	}
	return result
}
