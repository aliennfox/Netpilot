package overlay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// writeBaseConfig 给后面的 merger 测试提供一份最小 base config
func writeBaseConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	base := map[string]interface{}{
		"outbounds": []interface{}{
			map[string]interface{}{
				"type":      "selector",
				"tag":       "proxy-group",
				"outbounds": []interface{}{"direct-out"},
			},
			map[string]interface{}{"type": "direct", "tag": "direct-out"},
		},
		"route": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{"action": "sniff"},
			},
			"final": "proxy-group",
		},
	}
	raw, _ := json.Marshal(base)
	if err := os.WriteFile(basePath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return basePath
}

// TestMergeConfigs_RuleSetsAndMatchers 锁 Phase 10-A2:
// 1) overlay.RuleSets 落到 merged.route.rule_set 顶层数组
// 2) rule 里的新 matcher (domain_keyword/domain_regex/port/port_range/network/protocol/rule_set/geoip/geosite) 透传不丢
// 3) format 留空时 merger 不主动注入,让 sing-box 按扩展名兜底
func TestMergeConfigs_RuleSetsAndMatchers(t *testing.T) {
	basePath := writeBaseConfig(t)

	overlay := &OverlayData{
		RuleSets: []RuleSetConfig{
			BuiltinRuleSets["geoip-cn"],
			{
				Tag: "custom-remote", Type: "remote", URL: "https://x.example.com/list.srs",
				DownloadDetour: "proxy-group", UpdateInterval: "24h",
			},
			{
				Tag: "local-srs", Type: "local", Path: "/data/foo.srs",
			},
		},
		RouteRules: []RouteRule{
			{
				Tag:           "split-cn",
				RuleSet:       []string{"geoip-cn", "geosite-cn"},
				DomainKeyword: []string{"google"},
				Port:          []int{80, 443},
				PortRange:     []string{"8000:9000"},
				Network:       []string{"tcp"},
				Protocol:      []string{"tls", "http"},
				DomainRegex:   []string{`^api\.example\.com$`},
				Geoip:         []string{"private"},
				Geosite:       []string{"category-ads-all"},
				Outbound:      "direct-out",
				Source:        "user",
			},
		},
	}

	merged, err := MergeConfigs(basePath, overlay)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(merged, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	route, _ := got["route"].(map[string]interface{})
	if route == nil {
		t.Fatalf("missing route")
	}

	// 1) rule_set 顶层
	rsArr, ok := route["rule_set"].([]interface{})
	if !ok || len(rsArr) != 3 {
		t.Fatalf("route.rule_set want 3 items, got %+v", route["rule_set"])
	}
	byTag := map[string]map[string]interface{}{}
	for _, item := range rsArr {
		m, _ := item.(map[string]interface{})
		if tag, _ := m["tag"].(string); tag != "" {
			byTag[tag] = m
		}
	}
	cn := byTag["geoip-cn"]
	if cn == nil || cn["type"] != "remote" || cn["format"] != "binary" {
		t.Errorf("geoip-cn entry wrong: %+v", cn)
	}
	if cn["update_interval"] != "7d" {
		t.Errorf("geoip-cn update_interval lost: %+v", cn)
	}

	custom := byTag["custom-remote"]
	if custom["download_detour"] != "proxy-group" || custom["update_interval"] != "24h" {
		t.Errorf("custom-remote fields: %+v", custom)
	}
	if _, has := custom["format"]; has {
		t.Errorf("custom-remote 没显式 Format 时,merger 不应注入默认值(让 sing-box 按扩展名兜底): %+v", custom)
	}

	local := byTag["local-srs"]
	if local["type"] != "local" || local["path"] != "/data/foo.srs" {
		t.Errorf("local-srs fields: %+v", local)
	}
	if _, has := local["url"]; has {
		t.Errorf("local 类型不应带 url: %+v", local)
	}

	// 2) rule 里的 matcher 透传
	rules, _ := route["rules"].([]interface{})
	if len(rules) == 0 {
		t.Fatalf("rules empty")
	}
	first, _ := rules[0].(map[string]interface{})

	// String-listable matcher
	checkList := func(key string, want []string) {
		t.Helper()
		raw, ok := first[key].([]interface{})
		if !ok {
			t.Errorf("rule.%s missing / not array: %+v", key, first[key])
			return
		}
		got := make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok {
				got = append(got, s)
			}
		}
		sort.Strings(got)
		expected := append([]string(nil), want...)
		sort.Strings(expected)
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("rule.%s: want %v got %v", key, expected, got)
		}
	}
	checkList("rule_set", []string{"geoip-cn", "geosite-cn"})
	checkList("domain_keyword", []string{"google"})
	checkList("domain_regex", []string{`^api\.example\.com$`})
	checkList("port_range", []string{"8000:9000"})
	checkList("network", []string{"tcp"})
	checkList("protocol", []string{"tls", "http"})
	checkList("geoip", []string{"private"})
	checkList("geosite", []string{"category-ads-all"})

	// Int-listable matcher port — 注意 JSON 反序列化数字默认 float64
	portRaw, ok := first["port"].([]interface{})
	if !ok || len(portRaw) != 2 {
		t.Fatalf("rule.port missing: %+v", first["port"])
	}
	gotPorts := []int{}
	for _, v := range portRaw {
		if f, ok := v.(float64); ok {
			gotPorts = append(gotPorts, int(f))
		}
	}
	sort.Ints(gotPorts)
	if !reflect.DeepEqual(gotPorts, []int{80, 443}) {
		t.Errorf("rule.port: want [80 443] got %v", gotPorts)
	}

	// outbound 不能丢
	if first["outbound"] != "direct-out" {
		t.Errorf("rule.outbound: %+v", first["outbound"])
	}
}

// TestConfigOverlay_RuleSetCRUD 覆盖 CRUD + EnableBuiltinRuleSet 路径
func TestConfigOverlay_RuleSetCRUD(t *testing.T) {
	dir := t.TempDir()
	basePath := writeBaseConfig(t)
	o := NewConfigOverlay(basePath, dir)
	if err := o.Load(); err != nil {
		t.Fatal(err)
	}

	// Happy path: builtin alias
	if err := o.EnableBuiltinRuleSet("geoip-cn"); err != nil {
		t.Fatalf("enable builtin: %v", err)
	}
	sets := o.ListRuleSets()
	if len(sets) != 1 || sets[0].Tag != "geoip-cn" || sets[0].Source != "builtin:geoip-cn" {
		t.Errorf("after enable: %+v", sets)
	}

	// Unknown alias
	if err := o.EnableBuiltinRuleSet("not-exist"); err == nil {
		t.Errorf("unknown alias should fail")
	}

	// Add custom remote
	custom := RuleSetConfig{Tag: "my", Type: "remote", URL: "https://x/y.srs", Source: "user"}
	if err := o.AddRuleSet(custom); err != nil {
		t.Fatalf("add custom: %v", err)
	}
	if len(o.ListRuleSets()) != 2 {
		t.Errorf("want 2 rule-sets, got %d", len(o.ListRuleSets()))
	}

	// Overwrite same tag
	custom2 := custom
	custom2.UpdateInterval = "1d"
	if err := o.AddRuleSet(custom2); err != nil {
		t.Fatal(err)
	}
	sets = o.ListRuleSets()
	if len(sets) != 2 {
		t.Errorf("overwrite should not grow count: got %d", len(sets))
	}
	var mySet RuleSetConfig
	for _, rs := range sets {
		if rs.Tag == "my" {
			mySet = rs
		}
	}
	if mySet.UpdateInterval != "1d" {
		t.Errorf("overwrite didn't land: %+v", mySet)
	}

	// Validation: remote 缺 URL
	if err := o.AddRuleSet(RuleSetConfig{Tag: "bad", Type: "remote"}); err == nil {
		t.Errorf("remote 缺 url 应失败")
	}
	// Validation: local 缺 Path
	if err := o.AddRuleSet(RuleSetConfig{Tag: "bad", Type: "local"}); err == nil {
		t.Errorf("local 缺 path 应失败")
	}
	// Validation: 未知 type
	if err := o.AddRuleSet(RuleSetConfig{Tag: "bad", Type: "mystery"}); err == nil {
		t.Errorf("未知 type 应失败")
	}

	// Remove
	if err := o.RemoveRuleSet("my"); err != nil {
		t.Fatal(err)
	}
	if err := o.RemoveRuleSet("my"); err == nil {
		t.Errorf("二次删除应失败")
	}
	if len(o.ListRuleSets()) != 1 {
		t.Errorf("after remove, want 1 left, got %d", len(o.ListRuleSets()))
	}

	// Persist 到磁盘再重新加载能还原
	o2 := NewConfigOverlay(basePath, dir)
	if err := o2.Load(); err != nil {
		t.Fatal(err)
	}
	if len(o2.ListRuleSets()) != 1 || o2.ListRuleSets()[0].Tag != "geoip-cn" {
		t.Errorf("after reload: %+v", o2.ListRuleSets())
	}
}

// TestMergeConfigs_WireguardGoesToEndpoints 锁 #M25 (Phase 9 A1) 修复:
// type=="wireguard" 的 overlay outbound 必须分流到 merged.json.endpoints,
// 其它 type (ss/vmess/…) 仍进 merged.json.outbounds。 selector 在 outbounds 区域,
// 引用 wireguard tag 时 sing-box 知道去 endpoints 找 (共享 tag 命名空间)。
func TestMergeConfigs_WireguardGoesToEndpoints(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	base := map[string]interface{}{
		"outbounds": []interface{}{
			map[string]interface{}{
				"type":      "selector",
				"tag":       "proxy-group",
				"outbounds": []interface{}{"direct-out"},
			},
			map[string]interface{}{
				"type": "direct",
				"tag":  "direct-out",
			},
		},
	}
	raw, _ := json.Marshal(base)
	if err := os.WriteFile(basePath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	overlay := &OverlayData{
		Outbounds: []map[string]interface{}{
			{
				"type":        "wireguard",
				"tag":         "WG-JP",
				"private_key": "PRIV",
				"peers": []interface{}{
					map[string]interface{}{
						"address":    "1.2.3.4",
						"port":       51820,
						"public_key": "PUB",
					},
				},
			},
			{
				"type":        "shadowsocks",
				"tag":         "SS-US",
				"server":      "5.6.7.8",
				"server_port": 443,
			},
		},
	}
	merged, err := MergeConfigs(basePath, overlay)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(merged, &got); err != nil {
		t.Fatal(err)
	}

	// outbounds: 应有 selector, direct-out, SS-US (共 3), NO WG-JP
	outbounds, _ := got["outbounds"].([]interface{})
	tagsInOutbounds := map[string]bool{}
	for _, ob := range outbounds {
		if m, ok := ob.(map[string]interface{}); ok {
			tagsInOutbounds[m["tag"].(string)] = true
		}
	}
	if !tagsInOutbounds["SS-US"] {
		t.Errorf("SS-US should be in outbounds, got tags: %v", tagsInOutbounds)
	}
	if tagsInOutbounds["WG-JP"] {
		t.Errorf("WG-JP should NOT be in outbounds, got tags: %v", tagsInOutbounds)
	}

	// endpoints: 应有 WG-JP
	endpoints, ok := got["endpoints"].([]interface{})
	if !ok || len(endpoints) == 0 {
		t.Fatalf("endpoints missing: %+v", got["endpoints"])
	}
	foundWG := false
	for _, ep := range endpoints {
		if m, ok := ep.(map[string]interface{}); ok {
			if m["tag"] == "WG-JP" && m["type"] == "wireguard" {
				foundWG = true
				break
			}
		}
	}
	if !foundWG {
		t.Errorf("WG-JP wireguard endpoint not found in endpoints: %+v", endpoints)
	}

	// selector.outbounds 应含新增 tag (WG-JP 和 SS-US)
	for _, ob := range outbounds {
		m, _ := ob.(map[string]interface{})
		if m["type"] != "selector" {
			continue
		}
		sel, _ := m["outbounds"].([]interface{})
		selTags := map[string]bool{}
		for _, t := range sel {
			if s, ok := t.(string); ok {
				selTags[s] = true
			}
		}
		if !selTags["WG-JP"] {
			t.Errorf("selector should list WG-JP (endpoint tag shares namespace), got %v", selTags)
		}
		if !selTags["SS-US"] {
			t.Errorf("selector should list SS-US, got %v", selTags)
		}
	}
}
