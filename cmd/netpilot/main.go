package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/foxnetpilot/netpilot/internal/agent"
	"github.com/foxnetpilot/netpilot/internal/config"
	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/local"
	"github.com/foxnetpilot/netpilot/internal/overlay"
	"github.com/foxnetpilot/netpilot/internal/router"
	"github.com/foxnetpilot/netpilot/internal/subscription"
	"github.com/foxnetpilot/netpilot/internal/template"
	"github.com/foxnetpilot/netpilot/internal/tool"
)

const (
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorCyan  = "\033[36m"
)

func main() {
	cfg := config.Default()
	adapter := engine.NewSingBoxAdapter(cfg.ClashAPIAddr)
	pipeline := tool.NewPipeline(adapter, cfg.DataDir)
	// Permission system: CLI 默认 ask 模式，写操作需用户确认
	pipeline.SetPermissionManager(tool.NewPermissionManager(tool.TrustAsk, tool.NewCLIApprover()))
	intentRouter := router.NewIntentRouter()
	localEngine := local.NewEngine(adapter, pipeline, "proxy-group")

	// 初始化 Config Overlay + 模板系统
	baseConfig := "configs/minimal.json"
	ov := overlay.NewConfigOverlay(baseConfig, cfg.DataDir)
	if err := ov.Load(); err != nil {
		fmt.Printf("%s加载 overlay 失败: %v%s\n", colorRed, err, colorReset)
	}

	// 初始化订阅管理器
	subStore := subscription.NewSubscriptionStore(cfg.DataDir)
	if err := subStore.Load(); err != nil {
		fmt.Printf("%s加载订阅数据失败: %v%s\n", colorRed, err, colorReset)
	}
	subMgr := subscription.NewSubscriptionManager(subStore, ov, adapter)

	// 注册 overlay tools、订阅 tools、DNS tools 到 pipeline
	pipeline.RegisterExtraTools(tool.RegisterOverlayTools(ov))
	pipeline.RegisterExtraTools(tool.RegisterSubscriptionTools(subMgr))
	pipeline.RegisterExtraTools(tool.RegisterDNSTools(ov))
	pipeline.RegisterExtraTools(tool.RegisterPerAppTools(ov))

	// 设置 adapter 的配置路径（如果有 merged 配置就用它）
	mergedPath := ov.MergedConfigPath()
	if _, err := os.Stat(mergedPath); err == nil {
		adapter.SetConfigPath(absPath(mergedPath))
	} else {
		adapter.SetConfigPath(absPath(baseConfig))
	}

	// 初始化模板
	ts := template.NewTemplateStore()
	localEngine.SetOverlay(ov)
	localEngine.SetTemplates(ts)
	localEngine.SetSubscriptionManager(subMgr)

	// 启动订阅自动更新
	subMgr.StartAutoUpdate()
	defer subMgr.StopAutoUpdate()

	// 初始化 Agent Orchestrator（如果 API Key 可用）
	var orchestrator *agent.Orchestrator
	apiKey := os.Getenv("SILICONFLOW_API_KEY")
	if apiKey != "" {
		llmClient := agent.NewLLMClient(
			config.DefaultLLMBaseURL,
			apiKey,
			config.DefaultLLMModel,
			time.Duration(config.DefaultLLMTimeout)*time.Second,
		)
		assembler := agent.NewPromptAssembler(adapter)
		orchestrator = agent.NewOrchestrator(llmClient, pipeline, assembler, pipeline.GetTools())
	}

	// 初始化对话历史（最多保留 40 条消息 = 20 轮对话）
	history := agent.NewConversationHistory(40)

	fmt.Printf("%sPilotty CLI v0.7 (Phase 1 — Multi-turn Context)%s\n", colorCyan, colorReset)
	fmt.Printf("Clash API: %s\n", cfg.ClashAPIAddr)
	if orchestrator != nil {
		fmt.Printf("AI Agent: %s (%s)\n", config.DefaultLLMModel, config.DefaultLLMBaseURL)
	} else {
		fmt.Printf("AI Agent: 未启用 (设置 SILICONFLOW_API_KEY 环境变量以启用)\n")
	}
	fmt.Printf("输入自然语言指令（如 换个节点、测速），或输入 help 查看帮助\n\n")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("netpilot> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if line == "quit" || line == "exit" || line == "q" {
			fmt.Println("Bye.")
			return
		}

		if line == "help" {
			printHelp(orchestrator != nil)
			continue
		}

		// 会话管理命令
		if line == "/clear" {
			history.Clear()
			fmt.Println("对话历史已清空。")
			continue
		}
		if line == "/history" {
			fmt.Print(history.FormatDisplay())
			continue
		}
		if strings.HasPrefix(line, "/trust") {
			parts := strings.Fields(line)
			pm := pipeline.Permissions()
			if len(parts) == 1 {
				fmt.Printf("当前信任模式: %s (ask=每次询问 / auto=全部放行 / strict=全部拒绝)\n", pm.Mode())
				continue
			}
			switch parts[1] {
			case "ask", "auto", "strict":
				pm.SetMode(tool.TrustMode(parts[1]))
				pm.ResetSession()
				fmt.Printf("信任模式已切换为: %s\n", parts[1])
			case "reset":
				pm.ResetSession()
				fmt.Println("session 白名单已清空。")
			default:
				fmt.Println("用法: /trust [ask|auto|strict|reset]")
			}
			continue
		}

		// 1. Try Intent Router first
		routing := intentRouter.Route(line)
		if routing.Matched {
			// 会话管理命令（不记入历史）
			if routing.ActionID == "clear_history" {
				history.Clear()
				fmt.Println("对话历史已清空。")
				continue
			}
			if routing.ActionID == "show_history" {
				fmt.Print(history.FormatDisplay())
				continue
			}

			// 把原始输入传给 action，模板搜索等需要用
			routing.Params["_input"] = line
			result, err := localEngine.Execute(routing.ActionID, routing.Params)
			if err != nil {
				fmt.Printf("%s错误: %v%s\n", colorRed, err, colorReset)
			} else {
				// 记录 Local Engine 对话到历史
				history.Add("user", line, "local")
				history.Add("assistant", stripANSIForHistory(result), "local")
				printResult(result)
			}
			continue
		}

		// 2. Fall back to raw commands (backward compatibility)
		parts := strings.Fields(line)
		cmd := parts[0]
		handled := true

		switch cmd {
		case "nodes":
			r := pipeline.Execute(context.Background(), "get_node_pool", nil)
			printToolResult(r)
		case "switch":
			if len(parts) < 3 {
				fmt.Println("Usage: switch <group> <node>")
				continue
			}
			r := pipeline.Execute(context.Background(), "switch_node", map[string]interface{}{
				"group": parts[1],
				"node":  parts[2],
			})
			printToolResult(r)
		case "delay":
			if len(parts) < 2 {
				fmt.Println("Usage: delay <node>")
				continue
			}
			r := pipeline.Execute(context.Background(), "test_latency", map[string]interface{}{
				"tag": parts[1],
			})
			printToolResult(r)
		case "connections", "conns":
			r := pipeline.Execute(context.Background(), "get_connections", nil)
			printToolResult(r)
		case "logs":
			params := map[string]interface{}{}
			if len(parts) >= 2 {
				params["level"] = parts[1]
			}
			r := pipeline.Execute(context.Background(), "get_logs", params)
			printToolResult(r)
		case "snapshots":
			result, _ := localEngine.Execute("show_snapshots", nil)
			printResult(result)
		case "rollback":
			params := map[string]string{}
			if len(parts) >= 2 {
				params["id"] = parts[1]
			}
			result, err := localEngine.Execute("do_rollback", params)
			if err != nil {
				fmt.Printf("%s错误: %v%s\n", colorRed, err, colorReset)
			} else {
				printResult(result)
			}
		case "telemetry":
			result, _ := localEngine.Execute("show_telemetry", nil)
			printResult(result)
		case "import":
			if len(parts) < 2 {
				fmt.Println("Usage: import <subscription-url> [name]")
				continue
			}
			params := map[string]string{
				"url":    parts[1],
				"_input": line,
			}
			result, err := localEngine.Execute("import_subscription", params)
			if err != nil {
				fmt.Printf("%s错误: %v%s\n", colorRed, err, colorReset)
			} else {
				history.Add("user", line, "local")
				history.Add("assistant", stripANSIForHistory(result), "local")
				printResult(result)
			}
		case "subs":
			result, err := localEngine.Execute("list_subscriptions", nil)
			if err != nil {
				fmt.Printf("%s错误: %v%s\n", colorRed, err, colorReset)
			} else {
				printResult(result)
			}
		case "sub-update":
			p := map[string]string{"_input": line}
			result, err := localEngine.Execute("update_subscription", p)
			if err != nil {
				fmt.Printf("%s错误: %v%s\n", colorRed, err, colorReset)
			} else {
				printResult(result)
			}
		case "sub-remove":
			if len(parts) < 2 {
				fmt.Println("Usage: sub-remove <sub-id>")
				continue
			}
			p := map[string]string{"_input": line}
			result, err := localEngine.Execute("remove_subscription", p)
			if err != nil {
				fmt.Printf("%s错误: %v%s\n", colorRed, err, colorReset)
			} else {
				printResult(result)
			}
		case "live":
			localEngine.Execute("live_connections", nil) //nolint:errcheck
		case "dns":
			result, err := localEngine.Execute("show_dns", nil)
			if err != nil {
				fmt.Printf("%s错误: %v%s\n", colorRed, err, colorReset)
			} else {
				printResult(result)
			}
		default:
			handled = false
		}

		if handled {
			continue
		}

		// 3. Try Agent (LLM)
		if orchestrator != nil {
			result, err := orchestrator.Run(context.Background(), line, history)
			if err != nil {
				fmt.Printf("%s🤖 Agent 错误: %v%s\n", colorRed, err, colorReset)
			} else {
				// 记录 Agent 对话到历史
				history.Add("user", line, "agent")
				history.Add("assistant", result.Reply, "agent")
				fmt.Printf("\033[36m🤖 %s\033[0m\n", result.Reply)
			}
			continue
		}

		// 4. No agent available
		fmt.Printf("%sAI Agent 未启用。请设置环境变量: export SILICONFLOW_API_KEY=\"你的key\"%s\n", colorRed, colorReset)
		fmt.Printf("提示: 你仍然可以使用本地指令，如\"测速\"、\"换节点\"、\"状态\"等。\n")
	}
}

func printResult(s string) {
	fmt.Print(s)
	if len(s) > 0 && s[len(s)-1] != '\n' {
		fmt.Println()
	}
}

func printToolResult(r *tool.ToolResult) {
	if !r.Success {
		fmt.Printf("%s错误: %s%s\n", colorRed, r.Message, colorReset)
	} else {
		printResult(r.Message)
	}
}

// stripANSIForHistory 去除 ANSI 转义码，避免历史记录中包含颜色代码
func stripANSIForHistory(s string) string {
	// 简单去除常见 ANSI 转义序列
	result := s
	for strings.Contains(result, "\033[") {
		start := strings.Index(result, "\033[")
		end := start + 2
		for end < len(result) && result[end] != 'm' {
			end++
		}
		if end < len(result) {
			result = result[:start] + result[end+1:]
		} else {
			break
		}
	}
	return result
}

func absPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

func printHelp(agentEnabled bool) {
	fmt.Printf(`%s自然语言指令:%s
  换节点 / 换个节点 / 太慢了     自动测速并切换到最快节点
  测速 / 哪个快                 测试所有节点延迟
  全局模式 / 全局代理            切换到全局代理
  直连模式 / 直连               切换到直连
  状态 / 当前状态               查看当前状态
  节点列表 / 有哪些节点          列出所有节点
  快照                          列出所有快照
  回滚 / 撤销                   回滚到最新快照
  操作日志                      查看操作历史

%s分流与模板:%s
  配netflix / netflix分流       应用 Netflix 分流模板
  有哪些模板 / 模板列表          列出所有分流模��
  规则列表 / 当前规则            查看当前路由规则
  删除规则 <tag>                删除指定路由规则

%s会话管理:%s
  /history                     查看对话记录
  /clear                       清空对话历史

%s订阅管理:%s
  导入订阅 <URL> [名称]         从订阅链接导入节点
  import <URL> [名称]          从订阅链接导入节点
  订阅列表 / subs              列出所有订阅
  更新订阅 / sub-update [id]   更新指定订阅或全部
  删除订阅 / sub-remove <id>   删除订阅及其节点

%sDNS 管理:%s
  DNS状态 / dns              查看当前 DNS 配置
  DNS模式 / DNS防泄露         切换 DNS 模式 (secure/split/local)

%s连接监控:%s
  当前连接 / connections      一次性查看活跃连接
  实时连接 / live             进入实时连接监控（按 q 退出）

%s原始命令（向后兼容）:%s
  nodes                        列出节点
  switch <group> <node>        手动切换（经过 Pipeline）
  delay <node>                 测单个延迟
  connections                  查看连接
  logs [level]                 查看日志
  snapshots                    列出快照
  rollback [snap-id]           回滚到指定快照
  telemetry                    查看操作日志
  quit                         退出
`, colorCyan, colorReset, colorCyan, colorReset, colorCyan, colorReset, colorCyan, colorReset, colorCyan, colorReset, colorCyan, colorReset, colorCyan, colorReset)

	if agentEnabled {
		fmt.Printf(`
%sAI Agent (已启用):%s
  输入任何未被上述匹配的自然语言，将自动交给 AI Agent 处理。
  例如: "帮我找个延迟最低的日本节点"、"分析一下当前网络状态"
`, colorCyan, colorReset)
	} else {
		fmt.Printf(`
%sAI Agent (未启用):%s
  设置 SILICONFLOW_API_KEY 环境变量以启用 AI Agent。
`, colorRed, colorReset)
	}
}
