package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
)

// PlatformReloader 是平台 (Android VpnService / iOS NE) 提供的 reload 通道:
// Agent 写完 overlay 后调一次, 让运行中的 sing-box 实例热加载 merged.json。
// 在 CLI/Server 模式下注入空实现 (overlay.Apply 内部已通过 adapter.Reload pkill+exec 兜底);
// 在 Android 上由 mobile.PlatformReloader 透传到 Kotlin VpnService。
type PlatformReloader interface {
	RequestReload()
}

// noopReloader 占位用, CLI / 测试场景注入它避免 nil-check 散布。
type noopReloader struct{}

func (noopReloader) RequestReload() {}

// NoopReloader 给上层 wiring 用的导出值。
var NoopReloader PlatformReloader = noopReloader{}

// RegisterPerAppTools 注册 Android Per-App VPN 相关工具。
// 语义: 写 overlay.json 的 tun_override, merger 合成 merged.json 时 patch tun inbound 的
// include_package / exclude_package 字段。 在桌面 (mixed inbound) 环境静默无效, Android 环境生效。
//
// reloader 在 Agent 写完 ov.Apply 后被调用, 让 Kotlin VpnService 触发 sing-box reload —
// 关键修复 (#H?): 没这个调用 sing-box 进程会一直吃旧 merged.json, Agent 改了用户感知不到。
// 桌面/CLI 模式可传 NoopReloader (overlay.Apply 内部已 pkill+exec 兜底)。
func RegisterPerAppTools(ov *overlay.ConfigOverlay, reloader PlatformReloader) map[string]*ToolDef {
	if reloader == nil {
		reloader = NoopReloader
	}
	return map[string]*ToolDef{
		"set_per_app_vpn": toolSetPerAppVpn(ov, reloader),
		"get_per_app_vpn": toolGetPerAppVpn(ov),
	}
}

func toolSetPerAppVpn(ov *overlay.ConfigOverlay, reloader PlatformReloader) *ToolDef {
	return &ToolDef{
		Name:        "set_per_app_vpn",
		Description: "Set Android Per-App VPN filter on the TUN inbound. mode=allow: only listed packages go through proxy; mode=deny: listed packages bypass proxy (all others go through); mode=off: disable app filtering. Packages are Android package names like com.android.chrome.",
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
			// 通知平台让 sing-box 热加载 merged.json. Android 上是 PilottyVpnService.requestReload,
			// CLI/Server 模式是 noop (overlay.Apply 已 pkill+exec). 注入失败 / VPN 未运行时 no-op.
			reloader.RequestReload()
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
