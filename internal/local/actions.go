package local

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// groupTypes are proxy group types (not real nodes).
var groupTypes = map[string]bool{
	"Selector": true, "selector": true,
	"URLTest": true, "urltest": true,
	"Fallback": true, "fallback": true,
	"LoadBalance": true, "load-balance": true,
}

func filterRealNodes(proxies []engine.ProxyInfo) []engine.ProxyInfo {
	var real []engine.ProxyInfo
	for _, p := range proxies {
		if !groupTypes[p.Type] {
			real = append(real, p)
		}
	}
	return real
}

type latencyEntry struct {
	Tag     string
	Latency int
	Err     error
}

func testAllLatency(adapter engine.EngineAdapter, nodes []engine.ProxyInfo) []latencyEntry {
	results := make([]latencyEntry, len(nodes))
	var wg sync.WaitGroup

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	for i, n := range nodes {
		wg.Add(1)
		go func(idx int, tag string) {
			defer wg.Done()
			ms, err := adapter.TestLatency(tag, "https://www.gstatic.com/generate_204", 3*time.Second)
			results[idx] = latencyEntry{Tag: tag, Latency: ms, Err: err}
		}(i, n.Tag)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}

	return results
}

func (e *Engine) switchBestNode(_ map[string]string) (string, error) {
	// Get current node
	group, err := e.adapter.GetProxyGroup(e.groupTag)
	if err != nil {
		return "", fmt.Errorf("获取代理分组失败: %w", err)
	}
	oldNode := group.Now

	// Get all proxies and filter real nodes
	proxies, err := e.adapter.GetProxies()
	if err != nil {
		return "", fmt.Errorf("获取节点列表失败: %w", err)
	}
	nodes := filterRealNodes(proxies)
	if len(nodes) == 0 {
		return "", fmt.Errorf("没有可用节点")
	}

	fmt.Println("\033[36m正在选择最快节点...\033[0m")

	results := testAllLatency(e.adapter, nodes)

	sort.Slice(results, func(i, j int) bool {
		li, lj := results[i].Latency, results[j].Latency
		if li > 0 && lj > 0 {
			return li < lj
		}
		return li > 0
	})

	var best *latencyEntry
	for i := range results {
		if results[i].Latency > 0 {
			best = &results[i]
			break
		}
	}
	if best == nil {
		return "", fmt.Errorf("所有节点均不可用")
	}

	// Switch via pipeline (goes through hooks + snapshot + health check)
	result := e.pipeline.Execute(context.Background(), "switch_node", map[string]interface{}{
		"group": e.groupTag,
		"node":  best.Tag,
	})
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}

	return fmt.Sprintf("\033[32m已切换到 %s（延迟 %dms），从 %s 切换\033[0m", best.Tag, best.Latency, oldNode), nil
}

func (e *Engine) latencyTestAll(_ map[string]string) (string, error) {
	result := e.pipeline.Execute(context.Background(), "test_latency_all", nil)
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (e *Engine) setModeGlobal(_ map[string]string) (string, error) {
	result := e.pipeline.Execute(context.Background(), "set_mode", map[string]interface{}{
		"mode":  "global",
		"group": e.groupTag,
	})
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (e *Engine) setModeDirect(_ map[string]string) (string, error) {
	result := e.pipeline.Execute(context.Background(), "set_mode", map[string]interface{}{
		"mode":  "direct",
		"group": e.groupTag,
	})
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (e *Engine) setModeRule(_ map[string]string) (string, error) {
	result := e.pipeline.Execute(context.Background(), "set_mode", map[string]interface{}{
		"mode":  "rule",
		"group": e.groupTag,
	})
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (e *Engine) showStatus(_ map[string]string) (string, error) {
	group, err := e.adapter.GetProxyGroup(e.groupTag)
	if err != nil {
		return "", fmt.Errorf("获取状态失败: %w", err)
	}

	conns, err := e.adapter.GetConnections()
	if err != nil {
		return "", fmt.Errorf("获取连接数失败: %w", err)
	}

	mode := "rule"
	switch group.Now {
	case "direct-out":
		mode = "direct"
	case "block-out":
		mode = "block"
	default:
		mode = "proxy"
	}

	out := fmt.Sprintf("  当前节点: \033[36m%s\033[0m\n", group.Now)
	out += fmt.Sprintf("  活跃连接: \033[36m%d\033[0m\n", len(conns))
	out += fmt.Sprintf("  代理模式: \033[36m%s\033[0m\n", mode)
	return out, nil
}

func (e *Engine) showNodes(_ map[string]string) (string, error) {
	result := e.pipeline.Execute(context.Background(), "get_node_pool", nil)
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (e *Engine) showSnapshots(_ map[string]string) (string, error) {
	snaps := e.pipeline.Snapshots().List()
	if len(snaps) == 0 {
		return "没有快照。\n", nil
	}
	out := ""
	for _, s := range snaps {
		proxies := ""
		for g, n := range s.ActiveProxies {
			if proxies != "" {
				proxies += ", "
			}
			proxies += fmt.Sprintf("%s=%s", g, n)
		}
		out += fmt.Sprintf("  %-24s %s  %s\n", s.ID, s.Timestamp.Format("2006-01-02 15:04:05"), proxies)
	}
	return out, nil
}

func (e *Engine) doRollback(params map[string]string) (string, error) {
	id := params["id"]
	result := e.pipeline.ManualRollback(id)
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (e *Engine) showTelemetry(_ map[string]string) (string, error) {
	return e.pipeline.Telemetry().FormatRecent(10), nil
}

func (e *Engine) applyTemplate(params map[string]string) (string, error) {
	if e.templates == nil || e.overlay == nil {
		return "", fmt.Errorf("模板系统未初始化")
	}

	// 从用户输入中搜索匹配的模板
	input := params["_input"]
	if input == "" {
		// 尝试从所有 params 找一个值
		for _, v := range params {
			if v != "" {
				input = v
				break
			}
		}
	}

	results := e.templates.Search(input)
	if len(results) == 0 {
		// 列出所有可用模板
		all := e.templates.List()
		out := "未找到匹配的模板。可用模板:\n"
		for _, t := range all {
			out += fmt.Sprintf("  \033[36m%s\033[0m — %s\n", t.ID, t.Name)
		}
		return out, nil
	}

	tmpl := results[0]

	// 确定出站节点：替换 __BEST_PROXY__ 占位符
	proxyTarget := e.resolveBestProxy()

	ruleCount := 0
	for _, rule := range tmpl.Rules {
		r := rule // copy
		if r.Outbound == "__BEST_PROXY__" {
			r.Outbound = proxyTarget
		}
		if err := e.overlay.AddRule(r); err != nil {
			return "", fmt.Errorf("添加规则失败: %w", err)
		}
		ruleCount += len(r.DomainSuffix) + len(r.Domain) + len(r.IPCidr) + len(r.ProcessName)
	}

	// 应用（合并+重载）
	if err := e.overlay.Apply(e.adapter); err != nil {
		return "", fmt.Errorf("应用配置失败: %w", err)
	}

	return fmt.Sprintf("\033[32m已应用模板: %s\n已添加 %d 条匹配规则 → %s\n规则已生效。\033[0m",
		tmpl.Name, ruleCount, proxyTarget), nil
}

func (e *Engine) listTemplates(_ map[string]string) (string, error) {
	if e.templates == nil {
		return "", fmt.Errorf("模板系统未初始化")
	}
	all := e.templates.List()
	if len(all) == 0 {
		return "没有可用模板。\n", nil
	}
	out := "可用分流模板:\n"
	for _, t := range all {
		ruleCount := 0
		for _, r := range t.Rules {
			ruleCount += len(r.DomainSuffix) + len(r.Domain) + len(r.IPCidr) + len(r.ProcessName)
		}
		out += fmt.Sprintf("  \033[36m%-14s\033[0m — %s (%d 条匹配规则)\n", t.ID, t.Name, ruleCount)
	}
	return out, nil
}

func (e *Engine) listRules(_ map[string]string) (string, error) {
	if e.overlay == nil {
		return "", fmt.Errorf("Overlay 未初始化")
	}
	result := e.pipeline.Execute(context.Background(), "list_route_rules", nil)
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (e *Engine) removeRule(params map[string]string) (string, error) {
	if e.overlay == nil {
		return "", fmt.Errorf("Overlay 未初始化")
	}

	// 尝试从参数获取 tag，或从原始输入中提取
	tag := params["tag"]
	if tag == "" {
		// 从原始输入中提取: "删除规则 netflix" → "netflix"
		input := params["_input"]
		tag = extractTagFromInput(input)
	}
	if tag == "" {
		// 列出现有规则让用户选择
		rules := e.overlay.ListRules()
		if len(rules) == 0 {
			return "没有可删除的规则。\n", nil
		}
		out := "请指定要删除的规则 tag:\n"
		for _, r := range rules {
			out += fmt.Sprintf("  %s — %s\n", r.Tag, r.Description)
		}
		return out, nil
	}

	result := e.pipeline.Execute(context.Background(), "remove_route_rule", map[string]interface{}{
		"tag": tag,
	})
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

// resolveBestProxy 找到当前最快的代理节点，找不到就用 direct-out
func (e *Engine) resolveBestProxy() string {
	proxies, err := e.adapter.GetProxies()
	if err != nil {
		return "direct-out"
	}
	nodes := filterRealNodes(proxies)

	// 过滤掉 direct/block 类型
	var proxyNodes []engine.ProxyInfo
	for _, n := range nodes {
		switch n.Type {
		case "Direct", "direct", "Reject", "reject", "Block", "block":
			continue
		default:
			proxyNodes = append(proxyNodes, n)
		}
	}

	if len(proxyNodes) == 0 {
		// 没有代理节点，用 direct-out
		return "direct-out"
	}

	// 测速找最快的
	results := testAllLatency(e.adapter, proxyNodes)
	best := "direct-out"
	bestLatency := 0
	for _, r := range results {
		if r.Latency > 0 && (bestLatency == 0 || r.Latency < bestLatency) {
			best = r.Tag
			bestLatency = r.Latency
		}
	}
	return best
}

// extractTagFromInput 从用户输入中提取 tag
// "删除规则 netflix" → "netflix"
// "remove rule google" → "google"
func extractTagFromInput(input string) string {
	// 去掉已知前缀
	prefixes := []string{"删除规则", "移除规则", "remove rule", "delete rule"}
	for _, p := range prefixes {
		if strings.HasPrefix(input, p) {
			tag := strings.TrimSpace(input[len(p):])
			if tag != "" {
				return tag
			}
		}
	}
	// 尝试取最后一个空格分隔的词
	parts := strings.Fields(input)
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}
