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
		// geosite-cn 必须声明 (否则 merger 的 sanity filter 会从 rule.rule_set 里过滤掉它的引用,
		// 断言 checkList("rule_set", [geoip-cn, geosite-cn]) 会失败)
		RuleSets: []RuleSetConfig{
			BuiltinRuleSets["geoip-cn"],
			BuiltinRuleSets["geosite-cn"],
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

	// 1) rule_set 顶层 (geoip-cn + geosite-cn + custom-remote + local-srs)
	rsArr, ok := route["rule_set"].([]interface{})
	if !ok || len(rsArr) != 4 {
		t.Fatalf("route.rule_set want 4 items, got %+v", route["rule_set"])
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

// TestMergeTunInbound 验证 Per-App VPN 的 include_package / exclude_package 正确 patch 到 tun inbound。
// 覆盖: allow 白名单 / deny 黑名单 / off 空操作 / 没有 tun inbound 静默跳过 / 切换 mode 时清理旧字段。
func TestMergeTunInbound(t *testing.T) {
	buildConfig := func(withTun bool, extraTun map[string]interface{}) map[string]interface{} {
		cfg := map[string]interface{}{
			"inbounds": []interface{}{},
		}
		if withTun {
			tun := map[string]interface{}{
				"type": "tun",
				"tag":  "tun-in",
			}
			for k, v := range extraTun {
				tun[k] = v
			}
			cfg["inbounds"] = []interface{}{tun}
		}
		return cfg
	}

	t.Run("allow writes include_package clears exclude", func(t *testing.T) {
		cfg := buildConfig(true, map[string]interface{}{
			"exclude_package": []interface{}{"stale"},
		})
		mergeTunInbound(cfg, &TunInboundOverride{Mode: "allow", Packages: []string{"com.android.chrome"}})
		tun := cfg["inbounds"].([]interface{})[0].(map[string]interface{})
		if _, has := tun["exclude_package"]; has {
			t.Errorf("exclude_package should be cleared, got: %v", tun["exclude_package"])
		}
		inc, ok := tun["include_package"].([]interface{})
		if !ok || len(inc) != 1 || inc[0] != "com.android.chrome" {
			t.Errorf("include_package = %v, want [com.android.chrome]", tun["include_package"])
		}
	})

	t.Run("deny writes exclude_package clears include", func(t *testing.T) {
		cfg := buildConfig(true, map[string]interface{}{
			"include_package": []interface{}{"stale"},
		})
		mergeTunInbound(cfg, &TunInboundOverride{Mode: "deny", Packages: []string{"com.tencent.mm"}})
		tun := cfg["inbounds"].([]interface{})[0].(map[string]interface{})
		if _, has := tun["include_package"]; has {
			t.Errorf("include_package should be cleared")
		}
		exc, ok := tun["exclude_package"].([]interface{})
		if !ok || len(exc) != 1 || exc[0] != "com.tencent.mm" {
			t.Errorf("exclude_package = %v, want [com.tencent.mm]", tun["exclude_package"])
		}
	})

	t.Run("off is no-op", func(t *testing.T) {
		cfg := buildConfig(true, map[string]interface{}{
			"include_package": []interface{}{"untouched"},
		})
		mergeTunInbound(cfg, &TunInboundOverride{Mode: "off"})
		tun := cfg["inbounds"].([]interface{})[0].(map[string]interface{})
		if inc, _ := tun["include_package"].([]interface{}); len(inc) != 1 || inc[0] != "untouched" {
			t.Errorf("off should not touch tun, got include_package=%v", tun["include_package"])
		}
	})

	t.Run("no tun inbound silently skipped", func(t *testing.T) {
		cfg := buildConfig(false, nil)
		// mixed inbound 场景 (desktop), 不该 panic 也不该写任何东西
		mergeTunInbound(cfg, &TunInboundOverride{Mode: "allow", Packages: []string{"x"}})
		if len(cfg["inbounds"].([]interface{})) != 0 {
			t.Errorf("inbounds should stay empty")
		}
	})

	t.Run("allow then deny cleans up include", func(t *testing.T) {
		cfg := buildConfig(true, nil)
		mergeTunInbound(cfg, &TunInboundOverride{Mode: "allow", Packages: []string{"a"}})
		mergeTunInbound(cfg, &TunInboundOverride{Mode: "deny", Packages: []string{"b"}})
		tun := cfg["inbounds"].([]interface{})[0].(map[string]interface{})
		if _, has := tun["include_package"]; has {
			t.Errorf("switching allow→deny should clear include_package")
		}
		exc, _ := tun["exclude_package"].([]interface{})
		if len(exc) != 1 || exc[0] != "b" {
			t.Errorf("exclude_package = %v, want [b]", tun["exclude_package"])
		}
	})
}

// TestDefaultDNSConfig 锁三种 DNS 模式的关键差异 (final 服务器 + rules)。
// 这层是 set_dns_config Agent tool 的核心契约: secure→全代理 / split→直连走本地 / local→全本地。
func TestDefaultDNSConfig(t *testing.T) {
	tests := []struct {
		mode      string
		wantFinal string
		wantRules int
	}{
		{"secure", "proxy-dns", 0},
		{"split", "proxy-dns", 1},
		{"local", "direct-dns", 0},
		{"unknown-falls-back", "proxy-dns", 0}, // default branch == secure
	}
	for _, tc := range tests {
		t.Run(tc.mode, func(t *testing.T) {
			cfg := DefaultDNSConfig(tc.mode)
			if cfg.Final != tc.wantFinal {
				t.Errorf("final: want %q got %q", tc.wantFinal, cfg.Final)
			}
			if len(cfg.Rules) != tc.wantRules {
				t.Errorf("rules count: want %d got %d (rules=%+v)", tc.wantRules, len(cfg.Rules), cfg.Rules)
			}
			if len(cfg.Servers) != 3 {
				t.Errorf("expect 3 standard servers (proxy-dns/direct-dns/local-dns), got %d", len(cfg.Servers))
			}
			if cfg.Strategy != "prefer_ipv4" {
				t.Errorf("strategy: want prefer_ipv4 got %q", cfg.Strategy)
			}
		})
	}
}

// TestMergeDNS_ThreeLayerFallback 锁 mergeDNS 的优先级:
//
//	overlay.DNS != nil  ───→ 用 overlay
//	overlay.DNS == nil 且 base.dns 已有 ───→ 不动 base
//	两边都没 ───→ 注入默认 secure
func TestMergeDNS_ThreeLayerFallback(t *testing.T) {
	t.Run("overlay wins over base", func(t *testing.T) {
		cfg := map[string]interface{}{
			"dns": map[string]interface{}{
				"servers": []interface{}{map[string]interface{}{"tag": "base-dns", "type": "udp", "server": "1.1.1.1"}},
				"final":   "base-dns",
			},
		}
		overlayDNS := &DNSConfig{
			Servers: []DNSServer{{Type: "tls", Tag: "overlay-proxy-dns", Server: "8.8.8.8", ServerPort: 853, Detour: "proxy-group"}},
			Final:   "overlay-proxy-dns",
		}
		mergeDNS(cfg, overlayDNS)
		dns, _ := cfg["dns"].(map[string]interface{})
		if dns["final"] != "overlay-proxy-dns" {
			t.Errorf("overlay should win, got final=%v", dns["final"])
		}
	})

	t.Run("no overlay keeps base dns intact", func(t *testing.T) {
		baseFinal := "base-dns"
		cfg := map[string]interface{}{
			"dns": map[string]interface{}{
				"servers": []interface{}{map[string]interface{}{"tag": "base-dns", "type": "udp", "server": "1.1.1.1"}},
				"final":   baseFinal,
			},
		}
		mergeDNS(cfg, nil)
		dns, _ := cfg["dns"].(map[string]interface{})
		if dns["final"] != baseFinal {
			t.Errorf("base dns should be preserved, got final=%v", dns["final"])
		}
	})

	t.Run("both missing injects default secure", func(t *testing.T) {
		cfg := map[string]interface{}{}
		mergeDNS(cfg, nil)
		dns, ok := cfg["dns"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected dns block injected, got cfg=%+v", cfg)
		}
		if dns["final"] != "proxy-dns" {
			t.Errorf("default mode is secure (final=proxy-dns), got %v", dns["final"])
		}
		// ensureDefaultDomainResolver 副作用:route.default_domain_resolver
		route, _ := cfg["route"].(map[string]interface{})
		if route == nil || route["default_domain_resolver"] != "local-dns" {
			t.Errorf("default_domain_resolver should be local-dns, got route=%+v", route)
		}
	})
}

// TestMergeOutbounds_DedupAndSelectorInject 锁两条不变量:
//  1. 同 tag 的 outbound 不重复添加 (overlay 多次写入或 base 已有同 tag 时)
//  2. 新 tag 自动注入到 selector.outbounds 列表里 (#M6 关心的"动态 selector 成员"语义)
func TestMergeOutbounds_DedupAndSelectorInject(t *testing.T) {
	cfg := map[string]interface{}{
		"outbounds": []interface{}{
			map[string]interface{}{"type": "selector", "tag": "proxy-group", "outbounds": []interface{}{"direct-out"}},
			map[string]interface{}{"type": "direct", "tag": "direct-out"},
			map[string]interface{}{"type": "shadowsocks", "tag": "HK-1", "server": "old.example.com", "server_port": 443, "method": "aes-256-gcm", "password": "old"},
		},
	}

	mergeOutbounds(cfg, []map[string]interface{}{
		// 同 tag HK-1 应被去重 (保留 base 的 old.example.com 版本, 不被 overlay 的 new.example.com 覆盖)
		{"type": "shadowsocks", "tag": "HK-1", "server": "new.example.com", "server_port": 443, "password": "new"},
		// 新 tag 应入 outbounds + selector.outbounds
		{"type": "vmess", "tag": "JP-1", "server": "jp.example.com", "server_port": 443, "uuid": "u"},
	})

	outs, _ := cfg["outbounds"].([]interface{})

	// 不变量 1: HK-1 只出现 1 次
	hk1Count := 0
	var hk1Server string
	for _, ob := range outs {
		m, _ := ob.(map[string]interface{})
		if m["tag"] == "HK-1" {
			hk1Count++
			hk1Server, _ = m["server"].(string)
		}
	}
	if hk1Count != 1 {
		t.Errorf("HK-1 should appear once, got %d times", hk1Count)
	}
	if hk1Server != "old.example.com" {
		t.Errorf("dedupe should keep base version (server=old.example.com), got %q", hk1Server)
	}

	// 不变量 2: JP-1 加入了 outbounds, 同时进了 selector.outbounds
	jp1Found := false
	var selectorMembers []interface{}
	for _, ob := range outs {
		m, _ := ob.(map[string]interface{})
		if m["tag"] == "JP-1" {
			jp1Found = true
		}
		if m["type"] == "selector" {
			selectorMembers, _ = m["outbounds"].([]interface{})
		}
	}
	if !jp1Found {
		t.Errorf("JP-1 should be in outbounds: %+v", outs)
	}
	hasJP := false
	for _, m := range selectorMembers {
		if m == "JP-1" {
			hasJP = true
			break
		}
	}
	if !hasJP {
		t.Errorf("JP-1 should auto-inject into selector.outbounds, got %+v", selectorMembers)
	}

	// HK-1 因为已存在, 不应再次进 selector (避免重复)
	hk1InSelectorCount := 0
	for _, m := range selectorMembers {
		if m == "HK-1" {
			hk1InSelectorCount++
		}
	}
	if hk1InSelectorCount > 1 {
		t.Errorf("HK-1 should not be added to selector again, got %d", hk1InSelectorCount)
	}
}

// TestMergeConfigs_EmptyOverlayStillInjectsDNS 锁 line 23-29 的边界:
// overlay 完全为空 (RouteRules / Outbounds / DNS / TunOverride 都是 zero value),
// 应该走 early return 分支, 但仍然要注入默认 DNS (因为 base 没 dns)。
func TestMergeConfigs_EmptyOverlayStillInjectsDNS(t *testing.T) {
	basePath := writeBaseConfig(t) // base 没 dns
	merged, err := MergeConfigs(basePath, &OverlayData{})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(merged, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	dns, ok := got["dns"].(map[string]interface{})
	if !ok {
		t.Fatalf("empty overlay should still inject default dns, got %+v", got)
	}
	if dns["final"] != "proxy-dns" {
		t.Errorf("default mode is secure, want final=proxy-dns got %v", dns["final"])
	}
}

// TestMergeConfigs_NilOverlayInjectsDefaultDNS 锁: overlay 传 nil 也要兜底注入。
func TestMergeConfigs_NilOverlayInjectsDefaultDNS(t *testing.T) {
	basePath := writeBaseConfig(t)
	merged, err := MergeConfigs(basePath, nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(merged, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := got["dns"]; !ok {
		t.Fatalf("nil overlay should still inject default dns, got %+v", got)
	}
}
