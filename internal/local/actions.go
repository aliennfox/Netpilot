package local

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/monitor"
	"github.com/foxnetpilot/netpilot/internal/overlay"
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

func (e *Engine) importSubscription(params map[string]string) (string, error) {
	if e.subMgr == nil {
		return "", fmt.Errorf("订阅管理器未初始化")
	}

	input := params["_input"]
	url := params["url"]
	if url == "" {
		url = extractURL(input)
	}
	if url == "" {
		return "请提供订阅链接。用法: import <URL> [名称]\n", nil
	}

	// 尝试提取名称：import <url> <name> 或 导入订阅 <url> <name>
	name := extractNameAfterURL(input, url)

	result := e.pipeline.Execute(context.Background(), "import_subscription", map[string]interface{}{
		"url":  url,
		"name": name,
	})
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (e *Engine) listSubscriptions(_ map[string]string) (string, error) {
	if e.subMgr == nil {
		return "", fmt.Errorf("订阅管理器未初始化")
	}
	result := e.pipeline.Execute(context.Background(), "list_subscriptions", nil)
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (e *Engine) updateSubscription(params map[string]string) (string, error) {
	if e.subMgr == nil {
		return "", fmt.Errorf("订阅管理器未初始化")
	}
	p := map[string]interface{}{}
	// 尝试从输入中提取 ID
	if id := extractSubID(params["_input"]); id != "" {
		p["id"] = id
	}
	result := e.pipeline.Execute(context.Background(), "update_subscription", p)
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (e *Engine) removeSubscription(params map[string]string) (string, error) {
	if e.subMgr == nil {
		return "", fmt.Errorf("订阅管理器未初始化")
	}
	p := map[string]interface{}{}
	if id := extractSubID(params["_input"]); id != "" {
		p["id"] = id
	}
	result := e.pipeline.Execute(context.Background(), "remove_subscription", p)
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

// extractNameAfterURL 提取 URL 后面的名称参数
func extractNameAfterURL(input, url string) string {
	idx := strings.Index(input, url)
	if idx < 0 {
		return ""
	}
	after := strings.TrimSpace(input[idx+len(url):])
	if after != "" {
		return after
	}
	return ""
}

// extractSubID 从输入中提取 sub-XX 格式的 ID（sub- 后面跟数字）
func extractSubID(input string) string {
	for _, word := range strings.Fields(input) {
		if len(word) >= 5 && word[:4] == "sub-" {
			// 确保 sub- 后面是数字
			rest := word[4:]
			allDigits := true
			for _, c := range rest {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits && len(rest) > 0 {
				return word
			}
		}
	}
	return ""
}

// --- DNS Actions ---

func (e *Engine) showDNS(_ map[string]string) (string, error) {
	result := e.pipeline.Execute(context.Background(), "get_dns_config", nil)
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (e *Engine) setDNSMode(params map[string]string) (string, error) {
	if e.overlay == nil {
		return "", fmt.Errorf("Overlay 未初始化")
	}

	input := strings.ToLower(params["_input"])

	// 尝试从输入中提取模式
	mode := ""
	for _, m := range []string{"secure", "split", "local"} {
		if strings.Contains(input, m) {
			mode = m
			break
		}
	}

	if mode == "" {
		// 显示当前状态和可选模式，提示用户输入
		dns := e.overlay.GetDNS()
		if dns == nil {
			dns = overlay.DefaultDNSConfig("secure")
		}
		currentMode := inferDNSModeLocal(dns)

		out := fmt.Sprintf("当前: \033[36m%s\033[0m\n", currentMode)
		out += "可选:\n"
		out += "  \033[36msecure\033[0m — 全部走代理 DNS（最安全）\n"
		out += "  \033[36msplit\033[0m  — 代理/直连分开查询（平衡）\n"
		out += "  \033[36mlocal\033[0m  — 全部走本地 DNS（最快，有泄露风险）\n"
		out += "输入模式名切换: "
		fmt.Print(out)

		// 读取用户输入
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return "已取消。", nil
		}
		mode = strings.TrimSpace(strings.ToLower(scanner.Text()))
	}

	switch mode {
	case "secure", "split", "local":
	default:
		return fmt.Sprintf("无效的 DNS 模式: %s（可选: secure, split, local）", mode), nil
	}

	dns := overlay.DefaultDNSConfig(mode)
	if err := e.overlay.SetDNS(dns); err != nil {
		return "", fmt.Errorf("保存 DNS 配置失败: %v", err)
	}
	if err := e.overlay.Apply(e.adapter); err != nil {
		return "", fmt.Errorf("应用配置失败: %v", err)
	}

	return fmt.Sprintf("\033[32m已切换到 %s DNS 模式，配置已重载。\033[0m", mode), nil
}

func inferDNSModeLocal(dns *overlay.DNSConfig) string {
	if dns.Final == "direct-dns" {
		return "local"
	}
	for _, r := range dns.Rules {
		if r.Server == "direct-dns" && r.Outbound == "direct-out" {
			return "split"
		}
	}
	return "secure"
}

// --- Connection Actions ---

func (e *Engine) showConnections(_ map[string]string) (string, error) {
	conns, err := e.adapter.GetConnections()
	if err != nil {
		return "", fmt.Errorf("获取连接失败: %v", err)
	}
	if len(conns) == 0 {
		return "当前没有活跃连接。\n", nil
	}

	sort.Slice(conns, func(i, j int) bool {
		return conns[i].Download > conns[j].Download
	})

	out := fmt.Sprintf("活跃连接: \033[36m%d\033[0m\n", len(conns))
	out += fmt.Sprintf("  %-30s %-6s %-14s %8s %8s\n", "目标", "协议", "节点", "↑", "↓")
	for _, c := range conns {
		dest := c.Destination
		if len(dest) > 30 {
			dest = dest[:27] + "..."
		}
		node := extractChainNode(c.Chain)
		if len(node) > 14 {
			node = node[:11] + "..."
		}
		out += fmt.Sprintf("  %-30s %-6s %-14s %8s %8s\n",
			dest, strings.ToUpper(c.Protocol), node,
			monitor.FormatBytes(c.Upload), monitor.FormatBytes(c.Download))
	}
	return out, nil
}

func (e *Engine) liveConnections(_ map[string]string) (string, error) {
	mon := monitor.NewConnectionMonitor(e.adapter, 2*time.Second)
	mon.RunLive()
	return "", nil
}

func extractChainNode(chain string) string {
	if chain == "" {
		return "-"
	}
	parts := strings.Split(chain, " → ")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return chain
}

// extractURL 从文本中提取 URL
func extractURL(s string) string {
	for _, word := range strings.Fields(s) {
		if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
			return word
		}
	}
	return ""
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
