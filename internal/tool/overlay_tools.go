package tool

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
)

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?$`)

func validateDomains(domains []string) error {
	for _, d := range domains {
		if len(d) > 253 || !domainRegex.MatchString(d) {
			return fmt.Errorf("无效的域名格式: %s", d)
		}
	}
	return nil
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
	}
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

			// 校验域名格式
			if err := validateDomains(rule.Domain); err != nil {
				return nil, err
			}
			if err := validateDomains(rule.DomainSuffix); err != nil {
				return nil, err
			}
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
