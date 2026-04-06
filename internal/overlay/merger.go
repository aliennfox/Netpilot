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

// mergeOutbounds 将 overlay 出站追加到 outbounds 末尾（跳过重复 tag），
// 并将新节点 tag 添加到 selector group 的 outbounds 列表中
func mergeOutbounds(config map[string]interface{}, outbounds []map[string]interface{}) {
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

	// 追加不重复的 overlay 出站，并收集新增 tag
	var newTags []string
	for _, ob := range outbounds {
		tag, _ := ob["tag"].(string)
		if existingTags[tag] {
			continue
		}
		existingOutbounds = append(existingOutbounds, ob)
		existingTags[tag] = true
		newTags = append(newTags, tag)
	}

	config["outbounds"] = existingOutbounds

	// 将新增节点 tag 添加到第一个 selector group 的 outbounds 中
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
