package overlay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
