package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
)

// RegisterPerAppTools 注册 Android Per-App VPN 相关工具。
// 语义: 写 overlay.json 的 tun_override, merger 合成 merged.json 时 patch tun inbound 的
// include_package / exclude_package 字段。 在桌面 (mixed inbound) 环境静默无效, Android 环境生效。
//
// 平台 reload 由 overlay.ConfigOverlay.SetApplyHook 在 mobile binding 层一次性注入,
// Apply() 成功后自动触发 Kotlin PilottyVpnService.requestReload, 不再每个 tool 单独接 reloader.
func RegisterPerAppTools(ov *overlay.ConfigOverlay) map[string]*ToolDef {
	return map[string]*ToolDef{
		"set_per_app_vpn": toolSetPerAppVpn(ov),
		"get_per_app_vpn": toolGetPerAppVpn(ov),
	}
}

func toolSetPerAppVpn(ov *overlay.ConfigOverlay) *ToolDef {
	return &ToolDef{
		Name:        "set_per_app_vpn",
		Description: "Set Android Per-App VPN filter on TUN inbound. The ONLY correct tool for 'App X 走代理/直连' / 'only X uses VPN' / 'exclude X from VPN' on Android — matches by Android package name (UID), not domain. mode=allow: only listed packages go through proxy; mode=deny: listed packages bypass; mode=off: disable. Use Android package names (com.android.chrome, com.google.android.youtube, com.tencent.mm).",
		IsWriteOp:   true,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			mode, _ := params["mode"].(string)
			mode = strings.ToLower(strings.TrimSpace(mode))
			if mode == "" {
				return nil, fmt.Errorf("missing param: mode (allow/deny/off)")
			}
			pkgs := toStringSlice(params["packages"])
			if err := ov.SetPerAppVpn(mode, pkgs); err != nil {
				return nil, err
			}
			if err := ov.Apply(a); err != nil {
				return nil, fmt.Errorf("写 overlay 成功但 Apply 失败: %w", err)
			}
			// Apply 内部 hook 自动触发 PilottyVpnService.requestReload (Android) / noop (CLI).
			var msg string
			switch mode {
			case "off":
				msg = "已关闭 Per-App VPN, 所有 App 流量走代理"
			case "allow":
				msg = fmt.Sprintf("已设置白名单模式: 只有 %d 个 App 走代理 (%s)", len(pkgs), strings.Join(pkgs, ", "))
			case "deny":
				msg = fmt.Sprintf("已设置黑名单模式: %d 个 App 直连, 其他走代理 (%s)", len(pkgs), strings.Join(pkgs, ", "))
			}
			return &ToolResult{
				Success: true,
				Message: msg,
				Data:    map[string]interface{}{"mode": mode, "packages": pkgs},
			}, nil
		},
	}
}

func toolGetPerAppVpn(ov *overlay.ConfigOverlay) *ToolDef {
	return &ToolDef{
		Name:        "get_per_app_vpn",
		Description: "Query current Android Per-App VPN filter settings (mode + package list)",
		IsWriteOp:   false,
		Execute: func(ctx context.Context, _ engine.EngineAdapter, _ map[string]interface{}) (*ToolResult, error) {
			ov := ov.GetPerAppVpn()
			if ov == nil {
				return &ToolResult{Success: true, Message: "Per-App VPN 未启用, 所有 App 流量走代理"}, nil
			}
			msg := fmt.Sprintf("mode=%s, packages=[%s]", ov.Mode, strings.Join(ov.Packages, ", "))
			return &ToolResult{
				Success: true,
				Message: msg,
				Data:    map[string]interface{}{"mode": ov.Mode, "packages": ov.Packages},
			}, nil
		},
	}
}
