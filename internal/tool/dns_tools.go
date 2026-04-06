package tool

import (
	"context"
	"fmt"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
)

// RegisterDNSTools 注册 DNS 相关工具
func RegisterDNSTools(ov *overlay.ConfigOverlay) map[string]*ToolDef {
	return map[string]*ToolDef{
		"get_dns_config": {
			Name:        "get_dns_config",
			Description: "获取当前 DNS 配置和模式",
			IsWriteOp:   false,
			Execute: func(ctx context.Context, adapter engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
				dns := ov.GetDNS()
				if dns == nil {
					dns = overlay.DefaultDNSConfig("secure")
				}
				mode := inferDNSMode(dns)
				out := fmt.Sprintf("当前 DNS 模式: \033[36m%s\033[0m (%s)\n", mode, dnsModeSummary(mode))
				for _, s := range dns.Servers {
					detour := "直连"
					if s.Detour != "" && s.Detour != "direct-out" {
						detour = "通过 " + s.Detour
					}
					addr := s.Server
					if s.Type == "tls" {
						addr = fmt.Sprintf("tls://%s", s.Server)
					} else if s.Type == "local" {
						addr = "local"
					}
					out += fmt.Sprintf("  \033[36m%-12s\033[0m %s (%s)\n", s.Tag+":", addr, detour)
				}
				return &ToolResult{Success: true, Message: out}, nil
			},
		},
	}
}

// inferDNSMode 从 DNS 配置推断当前模式
func inferDNSMode(dns *overlay.DNSConfig) string {
	if dns.Final == "direct-dns" {
		return "local"
	}
	// 有 split 规则（直连流量走 direct-dns）
	for _, r := range dns.Rules {
		if r.Server == "direct-dns" && r.Outbound == "direct-out" {
			return "split"
		}
	}
	return "secure"
}

func dnsModeSummary(mode string) string {
	switch mode {
	case "secure":
		return "防泄露"
	case "split":
		return "分离查询"
	case "local":
		return "本地DNS"
	default:
		return mode
	}
}
