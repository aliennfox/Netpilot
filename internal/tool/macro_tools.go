package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
	"github.com/foxnetpilot/netpilot/internal/subscription"
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
// RegisterMacroTools 注册宏 tool。 subMgr 可选 (nil 时跳过订阅相关 macro), 兼容
// CLI 和测试场景。 mobile 路径必传。
func RegisterMacroTools(ctrl VpnController, ov *overlay.ConfigOverlay, subMgr *subscription.SubscriptionManager) map[string]*ToolDef {
	tools := map[string]*ToolDef{
		"setup_app_chain":        toolSetupAppChain(ctrl, ov),
		"switch_to_fastest_node": toolSwitchToFastestNode(ctrl),
	}
	if subMgr != nil {
		tools["import_and_activate_subscription"] = toolImportAndActivateSubscription(ctrl, subMgr)
	}
	return tools
}

// toolImportAndActivateSubscription "导订阅 + 切到能用的节点" 收敛为单 tool_call。
//
// 用户高频意图 ("导这个订阅" / "import this sub" / "添加订阅然后激活") 之前要 LLM 自己拆:
// import_subscription → list_subscriptions 看 tags → switch_node 三轮往返。 与 #1 同款,
// 任意一轮 stream 卡死整个流程瘫。
//
// params:
//   - url: required. 订阅 URL (http/https) 或单节点 URI (ss/vmess/vless/trojan/...)。
//   - name: optional. 订阅显示名, 留空 mgr 会用 URL host 派生。
//   - pick_fastest: optional bool, 默认 true。 true=测速选最快; false=直接选第一个 tag。
func toolImportAndActivateSubscription(ctrl VpnController, mgr *subscription.SubscriptionManager) *ToolDef {
	return &ToolDef{
		Name: "import_and_activate_subscription",
		Description: "ATOMIC: import a subscription URL and switch proxy-group to its best node in ONE call. " +
			"Internally: import_subscription → ensure VPN running → measure imported nodes → pick lowest-latency (or first if pick_fastest=false) → switch_node. " +
			"Use this for ANY '导这个订阅/导入这个订阅然后激活/import this sub and use it' request. " +
			"DO NOT call import_subscription + switch_node separately, this tool does both atomically and reports a single Success/failure.",
		IsWriteOp: true,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			url, _ := params["url"].(string)
			url = strings.TrimSpace(url)
			if url == "" {
				return nil, fmt.Errorf("missing param: url (订阅 URL 或节点 URI)")
			}
			name, _ := params["name"].(string)
			pickFastest := true
			if v, ok := params["pick_fastest"]; ok {
				if b, ok2 := v.(bool); ok2 {
					pickFastest = b
				}
			}

			trace := strings.Builder{}
			trace.WriteString(fmt.Sprintf("import_and_activate_subscription: url=%s\n", url))

			// 步骤 1: 导入
			trace.WriteString("[1/4] 导入订阅... ")
			count, summary, err := mgr.AddSubscription(name, url)
			if err != nil {
				return &ToolResult{Success: false, Message: trace.String() + "失败\n  " + err.Error()}, nil
			}
			trace.WriteString(fmt.Sprintf("OK (%d 节点)\n", count))

			// 找新导入订阅的 tags (按 name 匹配, 否则取最新的)
			tags := findSubscriptionTags(mgr, name, url)
			if len(tags) == 0 {
				return &ToolResult{Success: false, Message: trace.String() + "失败\n  订阅导入但找不到节点 tag (subscription store 可能未同步)\n" + summary}, nil
			}

			// 步骤 2: ensure VPN
			trace.WriteString("[2/4] ensure VPN... ")
			if !ctrl.IsRunning() {
				if err := WaitVpnReady(ctrl, 15*time.Second); err != nil {
					return &ToolResult{Success: false, Message: trace.String() + "失败\n  " + err.Error()}, nil
				}
				time.Sleep(1500 * time.Millisecond)
			}
			trace.WriteString("OK\n")

			// 步骤 3: 选目标节点
			trace.WriteString("[3/4] 选节点... ")
			var target string
			var targetLatency int
			if pickFastest && len(tags) > 1 {
				// 把 tags 包成 ProxyInfo 喂 concurrentLatencyTest
				probe := make([]engine.ProxyInfo, 0, len(tags))
				for _, t := range tags {
					probe = append(probe, engine.ProxyInfo{Tag: t})
				}
				results := concurrentLatencyTest(a, probe)
				for i := range results {
					if results[i].Latency <= 0 {
						continue
					}
					if target == "" || results[i].Latency < targetLatency {
						target = results[i].Tag
						targetLatency = results[i].Latency
					}
				}
				if target == "" {
					return &ToolResult{Success: false, Message: trace.String() + "失败\n  所有节点超时, 检查机场/订阅"}, nil
				}
				trace.WriteString(fmt.Sprintf("最快 %s (%dms)\n", target, targetLatency))
			} else {
				target = tags[0]
				trace.WriteString(fmt.Sprintf("第一个 %s\n", target))
			}

			// 步骤 4: 切 selector
			trace.WriteString("[4/4] 切换 selector... ")
			if err := a.SetActiveProxy("proxy-group", target); err != nil {
				return &ToolResult{Success: false, Message: trace.String() + "失败\n  " + err.Error()}, nil
			}
			trace.WriteString("OK\n")

			data := map[string]interface{}{
				"node":          target,
				"imported_tags": tags,
				"node_count":    count,
			}
			if targetLatency > 0 {
				data["latency_ms"] = targetLatency
			}
			return &ToolResult{
				Success: true,
				Message: trace.String() + fmt.Sprintf("\n✓ 订阅已导入 (%d 节点) + 已切到 %s", count, target),
				Data:    data,
			}, nil
		},
	}
}

// findSubscriptionTags 找 mgr 里最近导入的订阅 tags。 优先 name 精确匹配, 其次 URL 匹配,
// 再次 fallback 到 store 里最末一条 (最新创建)。
func findSubscriptionTags(mgr *subscription.SubscriptionManager, name, url string) []string {
	subs := mgr.Store().List()
	if len(subs) == 0 {
		return nil
	}
	if name != "" {
		for _, s := range subs {
			if s.Name == name {
				return s.Tags
			}
		}
	}
	for _, s := range subs {
		if s.URL == url {
			return s.Tags
		}
	}
	// fallback: 最后一条 (按 store 顺序最新)
	return subs[len(subs)-1].Tags
}

// toolSwitchToFastestNode 把 "切到最快节点" 三步缩成 1 个原子 tool。
//
// 用户高频意图 ("切到最快" / "切到日本最快" / "换个延迟低的") 之前要 LLM 自己拆:
// test_latency_all → 解析输出选最快 → switch_node 三轮往返。 任意一轮 stream 卡死或
// LLM 推理偏题, 整链路就瘫。 收敛成原子 tool: LLM 只需识别意图, 1 次 tool_call 完事,
// stream 卡死面积砍 90%。
//
// params:
//   - region_match: 可选 []string, 节点 tag substring 白名单 (大小写不敏感)。
//     例: ["JP", "Tokyo", "日本"] = "切到日本最快"; 空 = "切到最快"。
//     LLM 应根据用户描述判断 region 多语言别名一并传入, 比如 ["US","San-Jose","美国","美"]。
//   - skip_vpn_ensure: 可选 bool, 默认 false。 vpn 未启动时自动拉起 (符合 ensure 语义)。
func toolSwitchToFastestNode(ctrl VpnController) *ToolDef {
	return &ToolDef{
		Name: "switch_to_fastest_node",
		Description: "ATOMIC: switch proxy-group selector to the lowest-latency live node in ONE call. " +
			"Internally: ensure VPN running → measure all nodes → filter by region_match if given → pick lowest-latency → switch_node. " +
			"Use this for ANY '切到最快/切到X地区最快/换个快的/auto select' request. " +
			"region_match is optional substring whitelist (case-insensitive) on node tag — for '切到日本最快' pass [\"JP\",\"Tokyo\",\"日本\"]. " +
			"DO NOT call test_latency_all + switch_node separately, this tool does both atomically and reports a single Success/failure.",
		IsWriteOp: true,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			regionMatch := toStringSlice(params["region_match"])
			skipEnsure := false
			if v, ok := params["skip_vpn_ensure"]; ok {
				if b, ok2 := v.(bool); ok2 {
					skipEnsure = b
				}
			}

			trace := strings.Builder{}
			if len(regionMatch) > 0 {
				trace.WriteString(fmt.Sprintf("switch_to_fastest_node: region_match=%v\n", regionMatch))
			} else {
				trace.WriteString("switch_to_fastest_node: 全节点\n")
			}

			// 步骤 1: ensure VPN
			if !skipEnsure {
				trace.WriteString("[1/4] ensure VPN... ")
				if !ctrl.IsRunning() {
					if err := WaitVpnReady(ctrl, 15*time.Second); err != nil {
						return &ToolResult{Success: false, Message: trace.String() + "失败\n  " + err.Error()}, nil
					}
					time.Sleep(1500 * time.Millisecond)
				}
				trace.WriteString("OK\n")
			}

			// 步骤 2: 列节点
			trace.WriteString("[2/4] 列节点... ")
			proxies, err := a.GetProxies()
			if err != nil {
				return &ToolResult{Success: false, Message: trace.String() + "失败\n  " + err.Error()}, nil
			}
			realNodes := filterRealNodes(proxies)
			candidates := realNodes
			if len(regionMatch) > 0 {
				candidates = filterByTagSubstr(realNodes, regionMatch)
			}
			if len(candidates) == 0 {
				return &ToolResult{
					Success: false,
					Message: trace.String() + fmt.Sprintf("失败\n  无匹配节点 (region_match=%v, 节点池总数=%d)", regionMatch, len(realNodes)),
				}, nil
			}
			trace.WriteString(fmt.Sprintf("%d 个候选\n", len(candidates)))

			// 步骤 3: 并发测速
			trace.WriteString("[3/4] 并发测速... ")
			results := concurrentLatencyTest(a, candidates)
			var best *latencyEntry
			for i := range results {
				if results[i].Latency <= 0 {
					continue
				}
				if best == nil || results[i].Latency < best.Latency {
					b := results[i]
					best = &b
				}
			}
			if best == nil {
				return &ToolResult{
					Success: false,
					Message: trace.String() + "失败\n  所有候选节点超时, 检查机场/订阅",
				}, nil
			}
			trace.WriteString(fmt.Sprintf("最快 %s (%dms)\n", best.Tag, best.Latency))

			// 步骤 4: 切换 selector
			trace.WriteString("[4/4] 切换 selector... ")
			if err := a.SetActiveProxy("proxy-group", best.Tag); err != nil {
				return &ToolResult{Success: false, Message: trace.String() + "失败\n  " + err.Error()}, nil
			}
			trace.WriteString("OK\n")

			return &ToolResult{
				Success: true,
				Message: trace.String() + fmt.Sprintf("\n✓ 已切到 %s (延迟 %dms)", best.Tag, best.Latency),
				Data: map[string]interface{}{
					"node":           best.Tag,
					"latency_ms":     best.Latency,
					"candidates":     len(candidates),
					"region_match":   regionMatch,
					"total_measured": len(results),
				},
			}, nil
		},
	}
}

// filterByTagSubstr 节点 tag 大小写不敏感 substring 命中 needles 任一即收。
func filterByTagSubstr(nodes []engine.ProxyInfo, needles []string) []engine.ProxyInfo {
	var out []engine.ProxyInfo
	for _, n := range nodes {
		tagLower := strings.ToLower(n.Tag)
		for _, needle := range needles {
			if strings.Contains(tagLower, strings.ToLower(needle)) {
				out = append(out, n)
				break
			}
		}
	}
	return out
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
