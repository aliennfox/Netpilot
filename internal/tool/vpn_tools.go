package tool

import (
	"context"
	"fmt"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// VpnController 是 start_vpn / stop_vpn / vpn_status 三个工具需要的平台侧控制接口。
// mobile/netpilot.go 提供实现 (通过 gomobile reverse-binding callback 转给 Kotlin)。
//
// 设计原因: internal/tool 不能直接依赖 mobile 包 (循环 import), 也不应硬编码只有
// Android 一种控制后端。 这个 interface 把控制策略延迟到调用方注入, Tool 只关心语义。
type VpnController interface {
	// RequestStart 异步发起启动请求。 不阻塞等待 VPN 真正 up —— 平台侧可能弹用户授权对话框,
	// 返回给 Agent 的消息要明确告诉 LLM "下一步该做什么" (轮询 vpn_status 还是等用户)。
	RequestStart() error
	// RequestStop 异步发起停止请求。
	RequestStop() error
	// IsRunning 返回当前 VPN 运行状态 (平台侧 tunRunning 的只读快照)。
	IsRunning() bool
}

// RegisterVpnTools 构造三个 VPN 生命周期工具。
// ctrl 不能为 nil; 调用方若还没接入平台层, 应传一个返回 error 的占位实现而非 nil。
func RegisterVpnTools(ctrl VpnController) map[string]*ToolDef {
	return map[string]*ToolDef{
		"start_vpn":  toolStartVpn(ctrl),
		"stop_vpn":   toolStopVpn(ctrl),
		"vpn_status": toolVpnStatus(ctrl),
	}
}

// toolStartVpn 启动 VPN 数据面 (Android: VpnService + libbox TUN)。
//
// 返回消息对 LLM 友好, 引导它下一步调 vpn_status 或等 2-3s ——
// libbox 建链需要时间, 如果 LLM 下一轮立刻 get_node_pool 会撞 Clash API unreachable。
func toolStartVpn(ctrl VpnController) *ToolDef {
	return &ToolDef{
		Name:        "start_vpn",
		Description: "Start the local VPN tunnel (requires platform approval on first run)",
		IsWriteOp:   true,
		Execute: func(ctx context.Context, _ engine.EngineAdapter, _ map[string]interface{}) (*ToolResult, error) {
			if ctrl.IsRunning() {
				return &ToolResult{Success: true, Message: "VPN 已在运行。"}, nil
			}
			if err := ctrl.RequestStart(); err != nil {
				return &ToolResult{Success: false, Message: fmt.Sprintf("启动失败: %v", err)}, nil
			}
			return &ToolResult{
				Success: true,
				Message: "启动请求已发送。 VPN 约 3 秒后可用, 若需要后续操作请先用 vpn_status 确认状态。",
			}, nil
		},
	}
}

// toolStopVpn 停止 VPN 数据面。
func toolStopVpn(ctrl VpnController) *ToolDef {
	return &ToolDef{
		Name:        "stop_vpn",
		Description: "Stop the local VPN tunnel",
		IsWriteOp:   true,
		Execute: func(ctx context.Context, _ engine.EngineAdapter, _ map[string]interface{}) (*ToolResult, error) {
			if !ctrl.IsRunning() {
				return &ToolResult{Success: true, Message: "VPN 未在运行。"}, nil
			}
			if err := ctrl.RequestStop(); err != nil {
				return &ToolResult{Success: false, Message: fmt.Sprintf("停止失败: %v", err)}, nil
			}
			return &ToolResult{Success: true, Message: "已请求停止 VPN。"}, nil
		},
	}
}

// toolVpnStatus 只读, 查询 VPN 当前是否运行。
func toolVpnStatus(ctrl VpnController) *ToolDef {
	return &ToolDef{
		Name:        "vpn_status",
		Description: "Check whether the local VPN tunnel is currently running",
		IsWriteOp:   false,
		Execute: func(ctx context.Context, _ engine.EngineAdapter, _ map[string]interface{}) (*ToolResult, error) {
			running := ctrl.IsRunning()
			msg := "VPN 未在运行。"
			if running {
				msg = "VPN 正在运行。"
			}
			return &ToolResult{
				Success: true,
				Message: msg,
				Data:    map[string]interface{}{"running": running},
			}, nil
		},
	}
}
