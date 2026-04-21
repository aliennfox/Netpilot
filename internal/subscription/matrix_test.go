package subscription

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// M13 订阅 × 协议回归矩阵
//
// 目标: 每当 parser/converter 改动,scripts/subscription-matrix-test.sh
// 跑一次,保证已知真实格式(脱敏)还能 parse → convert → 产出合法 sing-box
// outbound JSON。 单测遍历 testdata/fixtures/ 下所有文件, 按后缀分格式派发。

type fixtureExpectation struct {
	// 节点数下限(脱敏后精确的, 保证回归时数量不倒退)
	MinNodes int
	// 必须出现的协议类型集合(只要少一种就算倒退)
	RequiredTypes []string
	// 某些节点的 tag 必须出现, 作为精细锚点
	RequiredTags []string
}

var fixtureExpectations = map[string]fixtureExpectation{
	"clash-mixed.yaml": {
		MinNodes:      8,
		RequiredTypes: []string{"shadowsocks", "vmess", "vless", "trojan", "hysteria2", "tuic", "anytls", "wireguard"},
		RequiredTags:  []string{"VLESS-Reality-US", "TUIC-KR", "AnyTLS-AU"},
	},
	"singbox-mixed.json": {
		MinNodes:      3, // direct/block/selector 会被 skip
		RequiredTypes: []string{"vless", "tuic", "trojan"},
		RequiredTags:  []string{"HK-VLESS", "JP-TUIC", "SG-Trojan"},
	},
	"base64-uri-mixed.txt": {
		MinNodes:      7,
		RequiredTypes: []string{"shadowsocks", "trojan", "vless", "hysteria2", "hysteria", "tuic", "anytls"},
		RequiredTags:  []string{"SS-HK-URI", "VLESS-Reality-URI", "TUIC-URI"},
	},
}

func TestSubscriptionFixtures(t *testing.T) {
	dir := filepath.Join("testdata", "fixtures")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "README.md" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			nodes, err := ParseSubscription(string(data))
			if err != nil {
				t.Fatalf("ParseSubscription: %v", err)
			}

			exp, has := fixtureExpectations[entry.Name()]
			if !has {
				// 未登记的 fixture 不做断言, 但至少要解析出 ≥1 个节点
				if len(nodes) == 0 {
					t.Errorf("fixture %s 解析出 0 节点", entry.Name())
				}
				return
			}

			realCount := 0
			typeSet := map[string]bool{}
			tagSet := map[string]bool{}
			for _, n := range nodes {
				if n.IsInfoEntry {
					continue
				}
				realCount++
				typeSet[n.Type] = true
				tagSet[n.Name] = true
			}

			if realCount < exp.MinNodes {
				t.Errorf("节点数倒退: want ≥%d, got %d; nodes=%+v", exp.MinNodes, realCount, nodes)
			}
			for _, tp := range exp.RequiredTypes {
				if !typeSet[tp] {
					t.Errorf("缺少协议 %q, 实际有: %v", tp, typeSet)
				}
			}
			for _, tag := range exp.RequiredTags {
				if !tagSet[tag] {
					t.Errorf("缺少节点名 %q, 实际有: %v", tag, tagSet)
				}
			}

			// 每个真节点过一遍 ConvertToSingboxOutbound,确保都能产出合法 map
			// (不跑 sing-box binary validate —— 那应该在 shell script 里额外做)
			for _, n := range nodes {
				if n.IsInfoEntry {
					continue
				}
				ob, err := ConvertToSingboxOutbound(n)
				if err != nil {
					t.Errorf("convert %s(%s): %v", n.Name, n.Type, err)
					continue
				}
				if ob == nil {
					t.Errorf("convert %s 返回 nil outbound", n.Name)
					continue
				}
				// marshal 能走通 = 结构至少合 JSON, 不能保证 sing-box 接受
				if _, err := json.Marshal(ob); err != nil {
					t.Errorf("marshal %s: %v", n.Name, err)
				}
			}
		})
	}
}
