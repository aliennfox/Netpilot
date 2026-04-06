package tool

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

type ToolDef struct {
	Name        string
	Description string
	IsWriteOp   bool
	Execute     func(ctx context.Context, adapter engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error)
}

type ToolResult struct {
	Success bool
	Message string
	Data    interface{}
}

func RegisterTools() map[string]*ToolDef {
	return map[string]*ToolDef{
		"get_node_pool":    toolGetNodePool(),
		"get_connections":  toolGetConnections(),
		"get_logs":         toolGetLogs(),
		"test_latency":     toolTestLatency(),
		"test_latency_all": toolTestLatencyAll(),
		"switch_node":      toolSwitchNode(),
		"set_mode":         toolSetMode(),
		"snapshot":         toolSnapshot(),
		"rollback":         toolRollback(),
	}
}

// --- Read tools ---

func toolGetNodePool() *ToolDef {
	return &ToolDef{
		Name:        "get_node_pool",
		Description: "List all proxies and proxy groups",
		IsWriteOp:   false,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			proxies, err := a.GetProxies()
			if err != nil {
				return nil, err
			}
			var groups, nodes []engine.ProxyInfo
			for _, p := range proxies {
				if isGroupType(p.Type) {
					groups = append(groups, p)
				} else {
					nodes = append(nodes, p)
				}
			}
			out := ""
			if len(groups) > 0 {
				out += "\033[36m分组:\033[0m\n"
				for _, g := range groups {
					out += fmt.Sprintf("  [%s] %s\n", g.Type, g.Tag)
				}
			}
			if len(nodes) > 0 {
				out += "\033[36m节点:\033[0m\n"
				out += "  tag | type\n"
				for _, n := range nodes {
					out += fmt.Sprintf("  %s | %s\n", n.Tag, n.Type)
				}
			}
			return &ToolResult{Success: true, Message: out}, nil
		},
	}
}

func toolGetConnections() *ToolDef {
	return &ToolDef{
		Name:        "get_connections",
		Description: "List active connections",
		IsWriteOp:   false,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			conns, err := a.GetConnections()
			if err != nil {
				return nil, err
			}
			if len(conns) == 0 {
				return &ToolResult{Success: true, Message: "没有活跃连接。\n"}, nil
			}
			out := fmt.Sprintf("%d 个活跃连接:\n", len(conns))
			for _, c := range conns {
				out += fmt.Sprintf("  %s  %s  ↑%d ↓%d  %s\n",
					c.Destination, c.Protocol, c.Upload, c.Download, c.Chain)
			}
			return &ToolResult{Success: true, Message: out}, nil
		},
	}
}

func toolGetLogs() *ToolDef {
	return &ToolDef{
		Name:        "get_logs",
		Description: "Get recent logs",
		IsWriteOp:   false,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			level, _ := params["level"].(string)
			logs, err := a.GetLogs(level, 20)
			if err != nil {
				return nil, err
			}
			if len(logs) == 0 {
				return &ToolResult{Success: true, Message: "没有日志。\n"}, nil
			}
			out := ""
			for _, l := range logs {
				out += fmt.Sprintf("[%s] %s\n", l.Type, l.Payload)
			}
			return &ToolResult{Success: true, Message: out}, nil
		},
	}
}

func toolTestLatency() *ToolDef {
	return &ToolDef{
		Name:        "test_latency",
		Description: "Test latency of a single proxy node",
		IsWriteOp:   false,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			tag, _ := params["tag"].(string)
			if tag == "" {
				return nil, fmt.Errorf("missing param: tag")
			}
			ms, err := a.TestLatency(tag, "https://www.gstatic.com/generate_204", 5*time.Second)
			if err != nil {
				return nil, err
			}
			return &ToolResult{
				Success: true,
				Message: fmt.Sprintf("%s: %dms\n", tag, ms),
			}, nil
		},
	}
}

func toolTestLatencyAll() *ToolDef {
	return &ToolDef{
		Name:        "test_latency_all",
		Description: "Test latency of all real proxy nodes concurrently",
		IsWriteOp:   false,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			proxies, err := a.GetProxies()
			if err != nil {
				return nil, fmt.Errorf("获取节点列表失败: %w", err)
			}
			nodes := filterRealNodes(proxies)
			if len(nodes) == 0 {
				return &ToolResult{Success: true, Message: "没有可测试的节点。\n"}, nil
			}

			fmt.Println("\033[36m正在测试所有节点...\033[0m")
			results := concurrentLatencyTest(a, nodes)

			sort.Slice(results, func(i, j int) bool {
				li, lj := results[i].Latency, results[j].Latency
				if li > 0 && lj > 0 {
					return li < lj
				}
				return li > 0
			})

			out := ""
			for _, r := range results {
				if r.Latency > 0 {
					out += fmt.Sprintf("  %-20s %4dms  \033[32m✓\033[0m\n", r.Tag, r.Latency)
				} else {
					out += fmt.Sprintf("  %-20s 超时    \033[31m✗\033[0m\n", r.Tag)
				}
			}
			return &ToolResult{Success: true, Message: out}, nil
		},
	}
}

// --- Write tools ---

func toolSwitchNode() *ToolDef {
	return &ToolDef{
		Name:        "switch_node",
		Description: "Switch active proxy in a group",
		IsWriteOp:   true,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			group, _ := params["group"].(string)
			node, _ := params["node"].(string)
			if group == "" || node == "" {
				return nil, fmt.Errorf("missing params: group and node required")
			}
			if err := a.SetActiveProxy(group, node); err != nil {
				return nil, err
			}
			return &ToolResult{
				Success: true,
				Message: fmt.Sprintf("已切换 %s → %s", group, node),
				Data:    map[string]string{"group": group, "node": node},
			}, nil
		},
	}
}

func toolSetMode() *ToolDef {
	return &ToolDef{
		Name:        "set_mode",
		Description: "Switch proxy mode (global/direct/rule)",
		IsWriteOp:   true,
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			mode, _ := params["mode"].(string)
			group, _ := params["group"].(string)
			if group == "" {
				group = "proxy-group"
			}

			switch mode {
			case "direct":
				if err := a.SetActiveProxy(group, "direct-out"); err != nil {
					return nil, fmt.Errorf("切换失败: %w", err)
				}
				return &ToolResult{
					Success: true,
					Message: "\033[32m已切换到直连模式\033[0m",
					Data:    map[string]string{"node": "direct-out"},
				}, nil

			case "global":
				proxies, err := a.GetProxies()
				if err != nil {
					return nil, fmt.Errorf("获取节点列表失败: %w", err)
				}
				nodes := filterRealNodes(proxies)
				target := "direct-out"
				for _, n := range nodes {
					if !isDirectOrBlock(n.Type) {
						target = n.Tag
						break
					}
				}
				if err := a.SetActiveProxy(group, target); err != nil {
					return nil, fmt.Errorf("切换失败: %w", err)
				}
				return &ToolResult{
					Success: true,
					Message: fmt.Sprintf("\033[32m已切换到全局代理模式（通过 %s）\033[0m", target),
					Data:    map[string]string{"node": target},
				}, nil

			case "rule":
				return &ToolResult{
					Success: true,
					Message: "\033[33m规则模式需要配置分流规则，暂未实现。请使用全局或直连模式。\033[0m",
				}, nil

			default:
				return nil, fmt.Errorf("unknown mode: %s (use global/direct/rule)", mode)
			}
		},
	}
}

func toolSnapshot() *ToolDef {
	return &ToolDef{
		Name:        "snapshot",
		Description: "Manually save a config snapshot",
		IsWriteOp:   false, // snapshot itself doesn't modify engine state
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			// This is handled specially by the pipeline — see pipeline.go ManualSnapshot
			return &ToolResult{Success: true, Message: "快照由 Pipeline 处理"}, nil
		},
	}
}

func toolRollback() *ToolDef {
	return &ToolDef{
		Name:        "rollback",
		Description: "Rollback to a snapshot",
		IsWriteOp:   false, // rollback bypasses the normal pipeline hooks
		Execute: func(ctx context.Context, a engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
			// This is handled specially by the pipeline — see pipeline.go ManualRollback
			return &ToolResult{Success: true, Message: "回滚由 Pipeline 处理"}, nil
		},
	}
}

// --- helpers ---

type latencyEntry struct {
	Tag     string
	Latency int
	Err     error
}

func concurrentLatencyTest(adapter engine.EngineAdapter, nodes []engine.ProxyInfo) []latencyEntry {
	results := make([]latencyEntry, len(nodes))
	var wg sync.WaitGroup

	// 并发上限 10
	sem := make(chan struct{}, 10)
	// 全局超时
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for i, n := range nodes {
		wg.Add(1)
		go func(idx int, tag string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = latencyEntry{Tag: tag, Latency: 0, Err: ctx.Err()}
				return
			}
			// 单个节点超时 5 秒
			nodeCtx, nodeCancel := context.WithTimeout(ctx, 5*time.Second)
			defer nodeCancel()

			done := make(chan struct{})
			go func() {
				ms, err := adapter.TestLatency(tag, "https://www.gstatic.com/generate_204", 3*time.Second)
				results[idx] = latencyEntry{Tag: tag, Latency: ms, Err: err}
				close(done)
			}()

			select {
			case <-done:
			case <-nodeCtx.Done():
				results[idx] = latencyEntry{Tag: tag, Latency: 0, Err: nodeCtx.Err()}
			}
		}(i, n.Tag)
	}

	wg.Wait()
	return results
}

func filterRealNodes(proxies []engine.ProxyInfo) []engine.ProxyInfo {
	var real []engine.ProxyInfo
	for _, p := range proxies {
		if !isGroupType(p.Type) {
			real = append(real, p)
		}
	}
	return real
}

func isDirectOrBlock(t string) bool {
	switch t {
	case "Direct", "direct", "Reject", "reject", "Block", "block":
		return true
	}
	return false
}
