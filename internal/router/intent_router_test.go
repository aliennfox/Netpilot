package router

import (
	"testing"
)

// TestRoute_HappyPath 覆盖每个 actionID 至少一个代表性触发短语,
// 这是用户最常走的本地路径,断了就直接撞 LLM 收费 + 体验下降。
func TestRoute_HappyPath(t *testing.T) {
	r := NewIntentRouter()

	tests := []struct {
		input        string
		wantActionID string
	}{
		// VPN 生命周期 (Phase 10-D, 不能联 LLM 时的关键路径)
		{"开启 vpn", "start_vpn"},
		{"打开VPN", "start_vpn"},
		{"start vpn", "start_vpn"},
		{"关闭 vpn", "stop_vpn"},
		{"断开vpn", "stop_vpn"},
		{"vpn 状态", "vpn_status"},
		{"vpn 开了吗", "vpn_status"},
		{"翻墙开着吗", "vpn_status"},
		{"代理在跑吗", "vpn_status"},
		{"is vpn running", "vpn_status"},

		// 节点
		{"换个节点", "switch_best_node"},
		{"测速", "latency_test_all"},
		{"看看节点", "show_nodes"},

		// 模式
		{"全局模式", "set_mode_global"},
		{"切到直连", "set_mode_direct"},
		{"规则模式", "set_mode_rule"},

		// 状态/历史
		{"当前状态", "show_status"},
		{"快照列表", "show_snapshots"},
		{"回滚", "do_rollback"},

		// 订阅
		{"导入订阅", "import_subscription"},
		{"订阅列表", "list_subscriptions"},
		{"刷新订阅", "update_subscription"},
		{"删除订阅", "remove_subscription"},

		// DNS — 中文带空格 + 无空格 + 英文带空格全覆盖
		{"dns 模式", "set_dns_mode"},
		{"dns模式", "set_dns_mode"},
		{"dns mode", "set_dns_mode"},
		{"dns 状态", "show_dns"},
		{"dns status", "show_dns"},

		// 连接
		{"实时连接", "live_connections"},
		{"活跃连接", "show_connections"},

		// 模板
		{"配Netflix", "apply_template"},
		{"模板列表", "list_templates"},

		// 规则
		{"规则列表", "list_rules"},
		{"删除规则", "remove_rule"},

		// 对话
		{"清空对话", "clear_history"},
		{"对话记录", "show_history"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := r.Route(tc.input)
			if !got.Matched {
				t.Fatalf("input %q: expected matched, got %+v", tc.input, got)
			}
			if got.ActionID != tc.wantActionID {
				t.Errorf("input %q: ActionID want %q got %q", tc.input, tc.wantActionID, got.ActionID)
			}
		})
	}
}

// TestRoute_NoMatch 没命中应返回 Matched=false (回落给 LLM)。
func TestRoute_NoMatch(t *testing.T) {
	r := NewIntentRouter()
	tests := []string{
		"今天天气真好",
		"hello world",
		"",
		"abc def ghi",
	}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			got := r.Route(s)
			if got.Matched {
				t.Errorf("input %q should not match, got %+v", s, got)
			}
		})
	}
}

// TestRoute_ReferenceWordsBypass 关键词命中但含指代词 (用户在引用上下文)
// → Matched=false (交 Agent)。 这是 Local Engine 不能处理上下文的兜底。
func TestRoute_ReferenceWordsBypass(t *testing.T) {
	r := NewIntentRouter()
	tests := []string{
		"切换到第一个节点",  // "节点" 命中 show_nodes 但有"第一个"
		"换那个节点",     // "换节点" 命中但有"那个"
		"用你推荐的节点",   // "节点" 命中但有"你推荐的"
		"刚才的状态怎么样了", // "状态" 命中但有"刚才的"
	}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			got := r.Route(s)
			if got.Matched {
				t.Errorf("input %q has reference word, should bypass to Agent (matched=false), got %+v", s, got)
			}
		})
	}
}

// TestRoute_PerAppDirectNotFalsePositive 锁 keywords.go:20-22 的 invariant:
// 用户说 "让 Chrome 直连" 是 Per-App VPN 配置语义, 不能被 set_mode_direct 误命中。
// 用 set_mode_direct 切全局直连 vs Per-App 直连是完全不同的两回事。
func TestRoute_PerAppDirectNotFalsePositive(t *testing.T) {
	r := NewIntentRouter()
	tests := []string{
		"让 Chrome 直连",
		"微信直连",
		"让淘宝走直连",
	}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			got := r.Route(s)
			if got.Matched && got.ActionID == "set_mode_direct" {
				t.Errorf("input %q should NOT match set_mode_direct (Per-App semantic, must go to Agent for set_per_app_vpn), got %+v", s, got)
			}
		})
	}
}

// TestRoute_VPNStatusBeatsStartVPN 锁 #M28 兜底路径: "vpn 开了吗" / "vpn 现在开" 等
// 状态查询语句, 必须命中 vpn_status, 不能误触 start_vpn (会真启动 VPN, 用户没要求)。
// 难点: "vpn 开" 是 start_vpn 关键词, "vpn 开了吗" 同时含 "vpn 开了吗" (vpn_status) 和 "vpn 开" (start_vpn 子串)?
// 实际 start_vpn 关键词是 "开 vpn" / "开vpn" / "打开vpn" 等, 都以 "开" 在前, vpn_status 用 "vpn 开了吗" 顺序在后,
// 不该误命中 start_vpn。
func TestRoute_VPNStatusBeatsStartVPN(t *testing.T) {
	r := NewIntentRouter()
	tests := []string{
		"vpn 开了吗",
		"vpn 现在开着没",
		"vpn 是不是开着",
		"vpn 状态",
	}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			got := r.Route(s)
			if !got.Matched {
				t.Errorf("input %q should match vpn_status, got %+v", s, got)
				return
			}
			if got.ActionID != "vpn_status" {
				t.Errorf("input %q ActionID want vpn_status, got %q", s, got.ActionID)
			}
		})
	}
}

// TestRoute_PunctuationStripped normalize() 应剥除标点 + 转小写, 这样
// "VPN 状态?" / "VPN，状态" 等带标点的输入也能命中。
func TestRoute_PunctuationStripped(t *testing.T) {
	r := NewIntentRouter()
	tests := []struct {
		in           string
		wantActionID string
	}{
		{"VPN 状态?", "vpn_status"},
		{"VPN 状态!", "vpn_status"},
		{"换节点。", "switch_best_node"},
		{"START VPN.", "start_vpn"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := r.Route(tc.in)
			if !got.Matched || got.ActionID != tc.wantActionID {
				t.Errorf("input %q: want matched=%q, got %+v", tc.in, tc.wantActionID, got)
			}
		})
	}
}

// TestRoute_ConfidenceAndParams 命中后 Confidence=1.0, Params 是空 map (不 nil)。
// Local Engine 后续会按 ActionID 自己提取参数,这里只确认契约。
func TestRoute_ConfidenceAndParams(t *testing.T) {
	r := NewIntentRouter()
	got := r.Route("换节点")
	if !got.Matched {
		t.Fatalf("expected matched")
	}
	if got.Confidence != 1.0 {
		t.Errorf("Confidence: want 1.0, got %v", got.Confidence)
	}
	if got.Params == nil {
		t.Errorf("Params should be non-nil empty map, got nil")
	}
	if len(got.Params) != 0 {
		t.Errorf("Params should be empty for keyword routing, got %+v", got.Params)
	}
}
