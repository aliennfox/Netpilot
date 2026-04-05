package local

import (
	"context"
	"fmt"
	"sort"
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
