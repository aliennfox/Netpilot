package overlay

import "sort"

// BuiltinRuleSets 预置的 rule-set 别名表, 全部走 SagerNet 官方仓 binary (.srs) 格式。
// 用户调用 ConfigOverlay.EnableBuiltinRuleSet(alias) 把某条加入 overlay.RuleSets。
//
// 选 SagerNet 官方而非 MetaCubeX 镜像的理由:
//  1. sing-geoip / sing-geosite 由 sing-box 作者自己维护, 格式迭代最权威,
//     避免第三方镜像滞后导致 load 失败。
//  2. 国内 GitHub raw 访问不稳时, 用户可通过 rule_set.DownloadDetour 走代理拉取
//     (默认留空 = 走 route.final outbound)。 这已经是 sing-box 官方推荐做法。
//
// UpdateInterval 统一 7d: 地理/域名清单数据变化慢, 频繁 poll 只加 GitHub 负担。
// 需要紧急更新的用户可手动删除并重新 EnableBuiltinRuleSet。
//
// 相关 sing-box 文档: https://sing-box.sagernet.org/configuration/rule-set/
var BuiltinRuleSets = map[string]RuleSetConfig{
	"geoip-cn": {
		Tag:            "geoip-cn",
		Type:           "remote",
		Format:         "binary",
		URL:            "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs",
		UpdateInterval: "7d",
		Source:         "builtin:geoip-cn",
	},
	// 注:原 "geoip-private" 条目已移除 — SagerNet 从 sing-geoip/rule-set 分支删了该 .srs
	// (sing-box 内置原生 `ip_is_private: true` 匹配替代, 见 RouteRule.IPIsPrivate)。
	// 任何引用 "geoip-private" 的 overlay / template 都应改用 IPIsPrivate 字段,
	// 迁移期间 merger 的 sanity filter 会自动跳过失效引用。
	"geosite-cn": {
		Tag:            "geosite-cn",
		Type:           "remote",
		Format:         "binary",
		URL:            "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs",
		UpdateInterval: "7d",
		Source:         "builtin:geosite-cn",
	},
	"geosite-geolocation-!cn": {
		Tag:            "geosite-geolocation-!cn",
		Type:           "remote",
		Format:         "binary",
		URL:            "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-geolocation-!cn.srs",
		UpdateInterval: "7d",
		Source:         "builtin:geosite-geolocation-!cn",
	},
	"geosite-category-ads-all": {
		Tag:            "geosite-category-ads-all",
		Type:           "remote",
		Format:         "binary",
		URL:            "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-category-ads-all.srs",
		UpdateInterval: "7d",
		Source:         "builtin:geosite-category-ads-all",
	},
}

// BuiltinRuleSetAliases 返回按字典序排好的内置别名, 供 UI / API 枚举.
func BuiltinRuleSetAliases() []string {
	names := make([]string, 0, len(BuiltinRuleSets))
	for k := range BuiltinRuleSets {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
