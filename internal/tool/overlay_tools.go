package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
)

var jsonMarshal = json.Marshal
var jsonUnmarshal = json.Unmarshal

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?$`)

func validateDomains(domains []string) error {
	for _, d := range domains {
		if len(d) > 253 || !domainRegex.MatchString(d) {
			return fmt.Errorf("无效的域名格式: %s", d)
		}
	}
	return nil
}

// normalizeDomainSuffixes 规范化 domain_suffix 列表:
// 用户 / LLM 常写成带前导点的 NekoBox/Clash 风格(".netflix.com"),sing-box 原生期望不带前导点
// 的后缀("netflix.com" 会同时匹配 netflix.com 和 *.netflix.com)。这里统一剥掉前导点并做合法性校验,
// 同时也容忍空白和重复。
func normalizeDomainSuffixes(suffixes []string) ([]string, error) {
	seen := make(map[string]struct{}, len(suffixes))
	out := make([]string, 0, len(suffixes))
	for _, s := range suffixes {
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, ".")
		if s == "" {
			continue
		}
		if len(s) > 253 || !domainRegex.MatchString(s) {
			return nil, fmt.Errorf("无效的域名后缀: %s", s)
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out, nil
}

func validateCIDRs(cidrs []string) error {
	for _, c := range cidrs {
		if _, _, err := net.ParseCIDR(c); err != nil {
			return fmt.Errorf("无效的 CIDR 格式: %s", c)
		}
	}
	return nil
}

// RegisterOverlayTools 注册需要 ConfigOverlay 的工具
func RegisterOverlayTools(ov *overlay.ConfigOverlay) map[string]*ToolDef {
	return map[string]*ToolDef{
		"patch_route_rule":  toolPatchRouteRule(ov),
		"remove_route_rule": toolRemoveRouteRule(ov),
		"list_route_rules":  toolListRouteRules(ov),
		"create_chain":      toolCreateChain(ov),
	}
}

// toolCreateChain 创建链式代理（detour chain）
// params:
//
//	tag: 链路出口节点的新名字（自动加 _agent: 前缀）
//	nodes: 节点 tag 列表，顺序 entry→exit，例如 ["JP-node","HK-node"] 表示 traffic → JP → HK → target
func toolCreateChain(ov *overlay.ConfigOverlay) *ToolDef {
	return &ToolDef{
		Name:        "create_chain",
		Description: "Create a chained proxy: traffic flows through nodes in order (entry → ... → exit). The exit node is exposed as a new outbound tag for routing.",
		IsWriteOp:   true,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			chainTag, _ := params["tag"].(string)
			nodes := toStringSlice(params["nodes"])
			if chainTag == "" {
				return nil, fmt.Errorf("missing param: tag")
			}
			if len(nodes) < 2 {
				return nil, fmt.Errorf("nodes 至少需要 2 个节点（entry → exit）")
			}
			if !strings.HasPrefix(chainTag, "_agent:") {
				chainTag = "_agent:" + chainTag
			}

			// 验证所有节点都存在
			for _, n := range nodes {
				if _, err := ov.FindOutboundByTag(n); err != nil {
					return nil, fmt.Errorf("节点 %q 不存在", n)
				}
			}

			// 链路语义：nodes = [entry, mid..., exit]
			// 流量方向：client → entry → mid → ... → exit → target
			// sing-box detour 语义：outbound A 的 detour=B 表示"拨号 A 时先经过 B"
			// 因此 exit.detour = mid_via_entry, mid.detour = entry, entry.detour = direct
			// 即从 exit 往回构造，每一跳 detour 指向上一跳（更靠近 client 的那个）
			//
			// 中间跳和出口都需要 clone 一份新的 outbound 设置 detour，
			// 因为原始 outbound 不能被修改（它们可能也用作普通节点）。
			// entry 节点不需要克隆（它的 detour 默认为 direct）。

			var newObs []map[string]interface{}
			prevTag := nodes[0] // 首跳直接用原 tag

			for i := 1; i < len(nodes); i++ {
				original, err := ov.FindOutboundByTag(nodes[i])
				if err != nil {
					return nil, err
				}
				clone := deepCopyJSON(original)
				// 出口跳用 chainTag，中间跳用衍生 tag
				var newTag string
				if i == len(nodes)-1 {
					newTag = chainTag
				} else {
					newTag = fmt.Sprintf("%s:hop%d-%s", chainTag, i, nodes[i])
				}
				clone["tag"] = newTag
				clone["detour"] = prevTag
				newObs = append(newObs, clone)
				prevTag = newTag
			}

			if err := ov.AddOutboundsBatch(newObs); err != nil {
				return nil, fmt.Errorf("保存链路 outbound 失败: %w", err)
			}
			if err := ov.Apply(a); err != nil {
				return nil, fmt.Errorf("应用配置失败: %w", err)
			}

			return &ToolResult{
				Success: true,
				Message: fmt.Sprintf("已创建链路 %s: %s（共 %d 跳，%d 个新 outbound）",
					chainTag, strings.Join(nodes, " → "), len(nodes), len(newObs)),
				Data: map[string]interface{}{
					"chain_tag": chainTag,
					"nodes":     nodes,
					"hops":      len(nodes),
				},
			}, nil
		},
	}
}

func deepCopyJSON(m map[string]interface{}) map[string]interface{} {
	b, _ := jsonMarshal(m)
	var out map[string]interface{}
	_ = jsonUnmarshal(b, &out)
	return out
}

func toolPatchRouteRule(ov *overlay.ConfigOverlay) *ToolDef {
	return &ToolDef{
		Name:        "patch_route_rule",
		Description: "Add a route rule to direct matching traffic to a specified outbound",
		IsWriteOp:   true,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			tag, _ := params["tag"].(string)
			outbound, _ := params["outbound"].(string)
			if tag == "" || outbound == "" {
				return nil, fmt.Errorf("missing params: tag and outbound required")
			}

			// 自动加 _agent: 前缀
			if !strings.HasPrefix(tag, "_agent:") {
				tag = "_agent:" + tag
			}

			rule := overlay.RouteRule{
				Tag:          tag,
				DomainSuffix: toStringSlice(params["domain_suffix"]),
				Domain:       toStringSlice(params["domain"]),
				IPCidr:       toStringSlice(params["ip_cidr"]),
				ProcessName:  toStringSlice(params["process_name"]),
				Outbound:     outbound,
				Description:  strParam(params, "description"),
				Source:       "agent",
			}

			if len(rule.DomainSuffix) == 0 && len(rule.Domain) == 0 &&
				len(rule.IPCidr) == 0 && len(rule.ProcessName) == 0 {
				return nil, fmt.Errorf("至少需要一种匹配条件: domain_suffix, domain, ip_cidr, 或 process_name")
			}

			// 校验域名格式:Domain 精确匹配需严格 FQDN; DomainSuffix 允许用户写带前导点的 NekoBox 风格
			if err := validateDomains(rule.Domain); err != nil {
				return nil, err
			}
			normalizedSuffix, err := normalizeDomainSuffixes(rule.DomainSuffix)
			if err != nil {
				return nil, err
			}
			rule.DomainSuffix = normalizedSuffix
			// 校验 CIDR 格式
			if err := validateCIDRs(rule.IPCidr); err != nil {
				return nil, err
			}

			if err := ov.AddRule(rule); err != nil {
				return nil, fmt.Errorf("添加规则失败: %w", err)
			}

			if err := ov.Apply(a); err != nil {
				return nil, fmt.Errorf("应用规则失败: %w", err)
			}

			// 构造描述信息
			var matches []string
			for _, d := range rule.Domain {
				matches = append(matches, d)
			}
			for _, ds := range rule.DomainSuffix {
				matches = append(matches, ds)
			}
			matchStr := strings.Join(matches, ", ")
			if len(matchStr) > 80 {
				matchStr = matchStr[:77] + "..."
			}

			return &ToolResult{
				Success: true,
				Message: fmt.Sprintf("已添加路由规则: %s → %s (%s)", tag, outbound, matchStr),
				Data:    map[string]string{"tag": tag, "outbound": outbound},
			}, nil
		},
	}
}

func toolRemoveRouteRule(ov *overlay.ConfigOverlay) *ToolDef {
	return &ToolDef{
		Name:        "remove_route_rule",
		Description: "Remove an agent-added route rule by tag",
		IsWriteOp:   true,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			tag, _ := params["tag"].(string)
			if tag == "" {
				return nil, fmt.Errorf("missing param: tag")
			}

			// 自动补全 _agent: 前缀
			if !strings.HasPrefix(tag, "_agent:") {
				tag = "_agent:" + tag
			}

			if err := ov.RemoveRule(tag); err != nil {
				return nil, err
			}

			if err := ov.Apply(a); err != nil {
				return nil, fmt.Errorf("重载配置失败: %w", err)
			}

			return &ToolResult{
				Success: true,
				Message: fmt.Sprintf("已删除规则: %s，配置已重载", tag),
			}, nil
		},
	}
}

func toolListRouteRules(ov *overlay.ConfigOverlay) *ToolDef {
	return &ToolDef{
		Name:        "list_route_rules",
		Description: "List all agent-added route rules",
		IsWriteOp:   false,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			rules := ov.ListRules()
			if len(rules) == 0 {
				return &ToolResult{Success: true, Message: "没有 Agent 添加的路由规则。\n"}, nil
			}

			out := fmt.Sprintf("共 %d 条路由规则:\n", len(rules))
			for _, r := range rules {
				var matches []string
				matches = append(matches, r.Domain...)
				matches = append(matches, r.DomainSuffix...)
				matches = append(matches, r.IPCidr...)
				matches = append(matches, r.ProcessName...)
				out += fmt.Sprintf("  \033[36m[%s]\033[0m %s\n", r.Tag, r.Description)
				out += fmt.Sprintf("    %s → %s\n", strings.Join(matches, ", "), r.Outbound)
			}
			return &ToolResult{Success: true, Message: out}, nil
		},
	}
}

// --- helpers ---

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []interface{}:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return val
	case string:
		if val == "" {
			return nil
		}
		// 支持逗号分隔的字符串（硅基流动不支持 array ��型 schema）
		if strings.Contains(val, ",") {
			var result []string
			for _, s := range strings.Split(val, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					result = append(result, s)
				}
			}
			return result
		}
		return []string{val}
	}
	return nil
}

func strParam(params map[string]interface{}, key string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
