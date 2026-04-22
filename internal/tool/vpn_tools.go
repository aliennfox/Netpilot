package tool

import (
	"context"
	"fmt"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// WaitVpnReady 轮询 ctrl.IsRunning 直到 true 或超时。 用于 Clash-API-dependent 的 tools
// 在 VPN 未启动时 "请求启动 + 等就绪 + 继续操作" 模式 —— 跟 Nodes tab testAll 对齐。
//
// 返回 nil 表示 VPN 已就绪; 返回 error 表示超时或控制未接入。
// 调用方拿到 nil 后建议再 sleep 1-2s 等 Clash API 真正接受请求 (libbox 起来后还需 warm-up)。
func WaitVpnReady(ctrl VpnController, totalTimeout time.Duration) error {
	if ctrl == nil {
		return fmt.Errorf("VPN 控制未接入")
	}
	if ctrl.IsRunning() {
		return nil
	}
	if err := ctrl.RequestStart(); err != nil {
		return fmt.Errorf("请求启动 VPN 失败: %w", err)
	}
	deadline := time.Now().Add(totalTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if ctrl.IsRunning() {
			return nil
		}
	}
	return fmt.Errorf("等待 VPN 就绪超时 (%.0fs)", totalTimeout.Seconds())
}

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

// RegisterVpnTools 构造 VPN 生命周期工具 + 依赖 VPN 的测速工具。
//
// test_latency / test_latency_all 在此被覆盖为 "自动等 VPN 就绪再测" 的版本,
// 对齐 Nodes tab testAll 的行为 —— 用户反馈 Agent 测速不该因 VPN 没开就拒绝,
// 要自动把 VPN 拉起来再真测 (这样数字也与 VPN 开启后一致, 避免 TCP ping 失真)。
//
// ctrl 不能为 nil; 调用方若还没接入平台层, 传返回 error 的占位实现而非 nil。
func RegisterVpnTools(ctrl VpnController) map[string]*ToolDef {
	return map[string]*ToolDef{
		"start_vpn":        toolStartVpn(ctrl),
		"stop_vpn":         toolStopVpn(ctrl),
		"vpn_status":       toolVpnStatus(ctrl),
		"test_latency":     toolTestLatencyAutoVpn(ctrl),
		"test_latency_all": toolTestLatencyAllAutoVpn(ctrl),
	}
}

// toolTestLatencyAllAutoVpn 包装 test_latency_all: VPN 未启动时自动 "偷偷" 启动 → 测 → 恢复原状。
// 原状 = 测前 VPN 状态: 若测前 VPN 是关的, 测完自动关; 若测前 VPN 已开, 测完仍开。
// 开不开 VPN 的主权归用户, 测速只是暂时借用。
func toolTestLatencyAllAutoVpn(ctrl VpnController) *ToolDef {
	return &ToolDef{
		Name:        "test_latency_all",
		Description: "Test latency of all proxy nodes via proxy tunnel. Silently toggles VPN if off and restores original state.",
		IsWriteOp:   false,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			cleanup, err := ensureVpnReadyForMeasure(ctrl)
			if err != nil {
				return &ToolResult{Success: false, Message: err.Error()}, nil
			}
			defer cleanup()
			return invokeBaseTestLatencyAll(ctx, a, params)
		},
	}
}

// toolTestLatencyAutoVpn 包装 test_latency: 同上, 偷偷测 + 恢复原状。
func toolTestLatencyAutoVpn(ctrl VpnController) *ToolDef {
	return &ToolDef{
		Name:        "test_latency",
		Description: "Test latency of a single node (params: tag) via proxy tunnel. Silently toggles VPN if off and restores.",
		IsWriteOp:   false,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			cleanup, err := ensureVpnReadyForMeasure(ctrl)
			if err != nil {
				return &ToolResult{Success: false, Message: err.Error()}, nil
			}
			defer cleanup()
			return invokeBaseTestLatency(ctx, a, params)
		},
	}
}

// ensureVpnReadyForMeasure 测速前共享 VPN 就绪逻辑 + 返回 cleanup 函数恢复原状。
//
// 返回值语义:
//   - cleanup == 空操作: VPN 测前就是开的, 不该关
//   - cleanup == RequestStop: 测前是关的, 测完关掉还给用户原状态
//
// err != nil 时 cleanup 仍然返回 noop, 调用方 defer 安全。
func ensureVpnReadyForMeasure(ctrl VpnController) (cleanup func(), err error) {
	noop := func() {}
	if ctrl.IsRunning() {
		return noop, nil // 用户自己开着, 不碰
	}
	if e := WaitVpnReady(ctrl, 15*time.Second); e != nil {
		return noop, fmt.Errorf("VPN 未启动且自动拉起失败: %v", e)
	}
	// libbox 启动后 Clash API 需 1-2s 才真正响应 /proxies/<tag>/delay; 加短延时避免首发请求撞空。
	time.Sleep(1500 * time.Millisecond)
	// 我们拉起来的, 定义 cleanup 关掉以恢复原状。
	return func() { _ = ctrl.RequestStop() }, nil
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
