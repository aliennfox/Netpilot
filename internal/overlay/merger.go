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

	if overlay == nil || (len(overlay.RouteRules) == 0 && len(overlay.Outbounds) == 0) {
		return json.MarshalIndent(config, "", "  ")
	}

	// 合并路由规则
	if len(overlay.RouteRules) > 0 {
		mergeRouteRules(config, overlay.RouteRules)
	}

	// 合并出站节点
	if len(overlay.Outbounds) > 0 {
		mergeOutbounds(config, overlay.Outbounds)
	}

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
		if len(r.IPCidr) > 0 {
			rule["ip_cidr"] = toInterfaceSlice(r.IPCidr)
		}
		if len(r.ProcessName) > 0 {
			rule["process_name"] = toInterfaceSlice(r.ProcessName)
		}
		overlayRules = append(overlayRules, rule)
	}

	// overlay 规则在前，base 规则在后
	merged := append(overlayRules, existingRules...)
	route["rules"] = merged
}

// mergeOutbounds 将 overlay 出站追加到 outbounds 末尾（跳过重复 tag）
func mergeOutbounds(config map[string]interface{}, outbounds []Outbound) {
	var existingOutbounds []interface{}
	if raw, ok := config["outbounds"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			existingOutbounds = arr
		}
	}

	// 收集已有的 tag
	existingTags := map[string]bool{}
	for _, ob := range existingOutbounds {
		if m, ok := ob.(map[string]interface{}); ok {
			if tag, ok := m["tag"].(string); ok {
				existingTags[tag] = true
			}
		}
	}

	// 追加不重复的 overlay 出站
	for _, ob := range outbounds {
		if existingTags[ob.Tag] {
			continue
		}
		entry := map[string]interface{}{
			"type": ob.Type,
			"tag":  ob.Tag,
		}
		if len(ob.Outbounds) > 0 {
			entry["outbounds"] = toInterfaceSlice(ob.Outbounds)
		}
		existingOutbounds = append(existingOutbounds, entry)
	}

	config["outbounds"] = existingOutbounds
}

func toInterfaceSlice(ss []string) []interface{} {
	result := make([]interface{}, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}
