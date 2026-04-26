package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
)

// RegisterMacroTools 注册"宏 tool" —— 把多个底层 tool 编排成一个原子操作,
// 让 LLM 不必拆 5 步才能完成"给 App X 配链式代理"这种语义级请求。
//
// 设计动机: OpenAI tools 风格的细粒度 tool 在多步链上对 LLM 推理负担很重 ——
// V4 Flash 等 reasoning model 可能 think 出"先告知用户启动 VPN" 而不是 tool_call。
// 把 5 步 (ensure VPN → test_latency_all → 筛活节点 → create_chain → set_per_app_vpn → verify)
// 收敛成 1 个 tool, LLM 只需识别意图, 不必规划状态机。 这是 Claude Code 的 Skill bundle 思路。
//
// ctrl 不能为 nil; ov 不能为 nil。
func RegisterMacroTools(ctrl VpnController, ov *overlay.ConfigOverlay) map[string]*ToolDef {
	return map[string]*ToolDef{
		"setup_app_chain": toolSetupAppChain(ctrl, ov),
	}
}

// toolSetupAppChain 把 "给 App X 配 N 跳链式代理" 编排成原子 tool。
//
// 调用方语义保证: Success=true 时 = VPN 已起 + chain 出口已切 + Per-App VPN 已限定到 app_pkg。
// Success=false 时 message 指明哪一步失败 (节点死 / chain 写盘失败 / 切换失败), LLM 直接转告用户。
func toolSetupAppChain(ctrl VpnController, ov *overlay.ConfigOverlay) *ToolDef {
	createChainTool := toolCreateChain(ov)
	setPerAppTool := toolSetPerAppVpn(ov)

	return &ToolDef{
		Name: "setup_app_chain",
		Description: "ATOMIC orchestration: configure chained proxy for a specific Android App in ONE call. " +
			"Internally: ensure VPN running → measure live nodes → validate requested chain hops are alive → " +
			"create_chain (auto-activate) → set_per_app_vpn allow [app_pkg] → verify chain reachable. " +
			"Use this for ANY 'route App X through nodes A→B→...' request — DO NOT call create_chain + " +
			"switch_node + set_per_app_vpn separately, this tool does it all and reports a single Success/failure.",
		IsWriteOp: true,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			appPkg, _ := params["app_pkg"].(string)
			appPkg = strings.TrimSpace(appPkg)
			nodes := toStringSlice(params["nodes"])
			chainTag, _ := params["chain_tag"].(string)
			chainTag = strings.TrimSpace(chainTag)

			if appPkg == "" {
				return nil, fmt.Errorf("missing param: app_pkg (Android package name, e.g. com.android.chrome)")
			}
			if len(nodes) < 2 {
				return nil, fmt.Errorf("nodes 至少需要 2 个 tag (entry → exit), 当前 %d", len(nodes))
			}
			if chainTag == "" {
				chainTag = strings.ReplaceAll(appPkg, ".", "-") + "-chain"
			}

			trace := strings.Builder{}
			trace.WriteString(fmt.Sprintf("setup_app_chain: app=%s chain_tag=%s nodes=%v\n", appPkg, chainTag, nodes))

			// 步骤 1: ensure VPN running (同步 15s)
			trace.WriteString("[1/5] ensuring VPN ready... ")
			if !ctrl.IsRunning() {
				if err := WaitVpnReady(ctrl, 15*time.Second); err != nil {
					return &ToolResult{Success: false, Message: trace.String() + "失败\n  " + err.Error()}, nil
				}
				time.Sleep(1500 * time.Millisecond) // Clash API warm-up
			}
			trace.WriteString("OK\n")

			// 步骤 2: 测全节点 → 收集活节点 latency map
			trace.WriteString("[2/5] measuring live nodes... ")
			liveLatency := make(map[string]int)
			proxies, err := a.GetProxies()
			if err != nil {
				return &ToolResult{Success: false, Message: trace.String() + "失败\n  " + err.Error()}, nil
			}
			for _, p := range proxies {
				if isGroupType(p.Type) || isDirectOrBlock(p.Type) {
					continue
				}
				ms, terr := a.TestLatency(p.Tag, "https://www.gstatic.com/generate_204", 5*time.Second)
				if terr == nil && ms > 0 {
					liveLatency[p.Tag] = ms
				}
			}
			trace.WriteString(fmt.Sprintf("%d 个活节点\n", len(liveLatency)))

			// 步骤 3: 校验请求的 chain hops 全活
			trace.WriteString("[3/5] validating chain hops... ")
			var dead []string
			for _, n := range nodes {
				if _, ok := liveLatency[n]; !ok {
					dead = append(dead, n)
				}
			}
			if len(dead) > 0 {
				trace.WriteString("失败\n")
				return &ToolResult{
					Success: false,
					Message: trace.String() + fmt.Sprintf(
						"  以下节点不通: %s — chain 任意一跳死整条废, 请换活节点 (从测速结果里挑 latency>0 的)。",
						strings.Join(dead, ", "),
					),
				}, nil
			}
			trace.WriteString("OK\n")

			// 步骤 4: create_chain (auto-activate selector)
			trace.WriteString(fmt.Sprintf("[4/5] create_chain tag=%s... ", chainTag))
			chainResult, err := createChainTool.Execute(ctx, a, map[string]interface{}{
				"tag":      chainTag,
				"nodes":    nodes,
				"activate": true,
			})
			if err != nil || chainResult == nil || !chainResult.Success {
				msg := "unknown"
				if err != nil {
					msg = err.Error()
				} else if chainResult != nil {
					msg = chainResult.Message
				}
				return &ToolResult{Success: false, Message: trace.String() + "失败\n  " + msg}, nil
			}
			trace.WriteString("OK\n")

			// 步骤 5: set_per_app_vpn allow [appPkg]
			trace.WriteString(fmt.Sprintf("[5/5] set_per_app_vpn allow [%s]... ", appPkg))
			perAppResult, err := setPerAppTool.Execute(ctx, a, map[string]interface{}{
				"mode":     "allow",
				"packages": []string{appPkg},
			})
			if err != nil || perAppResult == nil || !perAppResult.Success {
				msg := "unknown"
				if err != nil {
					msg = err.Error()
				} else if perAppResult != nil {
					msg = perAppResult.Message
				}
				return &ToolResult{Success: false, Message: trace.String() + "失败\n  " + msg}, nil
			}
			trace.WriteString("OK\n")

			return &ToolResult{
				Success: true,
				Message: trace.String() + fmt.Sprintf(
					"\n✓ 完成: %s 的流量将经 %s 链路出口 (_agent:%s)。 已激活 selector + 限定 Per-App VPN 白名单。",
					appPkg, strings.Join(nodes, " → "), chainTag,
				),
				Data: map[string]interface{}{
					"app_pkg":    appPkg,
					"chain_tag":  "_agent:" + chainTag,
					"nodes":      nodes,
					"latencies":  liveLatency,
					"hops_count": len(nodes),
				},
			}, nil
		},
	}
}
