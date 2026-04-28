// Command dispatch-probe 直接打 LLM API 验证 8 个高频意图的 tool dispatch 命中率。
//
// 设计动机: 把 "LLM 是否真选 macro tool 而不是拆步骤" 的验证从真机解耦。 真机方案要在
// Chat tab 输入文字, Gboard 自动纠错把 "切到日本最快" 改成 "切到日本最", 之前撞过 #M28
// 这条坑; 走 off-device probe 完全绕开输入法 + Android UI 状态机, 5 秒一轮可重复跑。
//
// 用法:
//
//	export LLM_API_KEY=sk-xxx
//	export LLM_BASE_URL=https://api.siliconflow.cn/v1   # 可选
//	export LLM_MODEL=Qwen/Qwen3.6-35B-A3B               # 可选
//	go run ./cmd/dispatch-probe
//
// 输出 markdown 表: prompt → got_tool vs expected_tool, hit/miss + 总命中率。
// 退出码: 全 hit=0, 任一 miss=1, 配置错误=2。
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/foxnetpilot/netpilot/internal/agent"
	"github.com/foxnetpilot/netpilot/internal/config"
	"github.com/foxnetpilot/netpilot/internal/overlay"
	"github.com/foxnetpilot/netpilot/internal/tool"
)

// stubVpnController 不会真启停 VPN, 只为 RegisterMacroTools 满足类型契约。
// LLM 看 tool Description 决定 dispatch, 不会真 Execute, 所以接口里所有方法被调到都视为编程错误。
type stubVpnController struct{}

func (stubVpnController) RequestStart() error { return nil }
func (stubVpnController) RequestStop() error  { return nil }
func (stubVpnController) IsRunning() bool     { return false }

// probeCase 一条 dispatch 测试用例: prompt 模拟用户输入, expected 是预期 LLM 调到的 tool 名,
// role 决定走 Diagnose (只读) 还是 Configure (可写) 系统提示词。
type probeCase struct {
	prompt   string
	expected string
	role     string // "diagnose" / "configure"
}

// 8 个高频意图覆盖 4 个 macro tool + 4 个底层 tool 各 1 case 作 baseline
var probeCases = []probeCase{
	// macro: 链式代理
	{"帮我给 Chrome 配链式代理 流量先 JP-1 再 HK-1", "setup_app_chain", "configure"},
	// macro: 切最快
	{"切到日本最快的节点", "switch_to_fastest_node", "configure"},
	// macro: 切最快 (无 region)
	{"换个延迟低的节点", "switch_to_fastest_node", "configure"},
	// macro: 导订阅 + 激活
	{"把这个订阅导进来用 https://example.com/sub.yaml", "import_and_activate_subscription", "configure"},
	// macro: 综合诊断
	{"我连不上谷歌 帮我看看哪里出问题", "diagnose_connectivity", "diagnose"},
	// 底层 baseline #1: VPN 状态查询 (确保 macro tool 没把简单 read 也吞了)
	{"VPN 现在开着吗", "vpn_status", "diagnose"},
	// 底层 baseline #2: 切到指定节点 (有具体节点名时不应触发 switch_to_fastest_node)
	{"切到 HK-1", "switch_node", "configure"},
	// 底层 baseline #3: 修单条规则 (不应误触 setup_app_chain)
	{"加条规则 youtube.com 走 proxy-group", "patch_route_rule", "configure"},
}

func main() {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "ERROR: LLM_API_KEY 未设置")
		fmt.Fprintln(os.Stderr, "  export LLM_API_KEY=sk-xxx (SiliconFlow / DashScope / OpenRouter 任一)")
		os.Exit(2)
	}
	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = config.DefaultLLMBaseURL
	}
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = config.DefaultLLMModel
	}

	fmt.Printf("# Dispatch Probe — %s\n\n", model)
	fmt.Printf("base=%s · cases=%d · ToolChoice=auto · 温度=%.2f\n\n",
		baseURL, len(probeCases), config.DefaultLLMTemperature)

	tools := buildAllToolDefs()
	llm := agent.NewLLMClient(baseURL, apiKey, model, time.Duration(config.DefaultLLMTimeout)*time.Second)

	hits := 0
	rows := make([]string, 0, len(probeCases))
	for i, tc := range probeCases {
		role := agent.AllRoles[tc.role]
		if role == nil {
			fmt.Fprintf(os.Stderr, "ERROR: case %d role=%q 不存在\n", i, tc.role)
			os.Exit(2)
		}
		got, gotArgs, err := dispatchOnce(llm, model, role, tools, tc.prompt)
		mark := "✗"
		if err != nil {
			rows = append(rows, fmt.Sprintf("| %d | `%s` | %s | _err_ | ✗ %s |",
				i+1, escapeMD(tc.prompt), tc.expected, err.Error()))
			fmt.Printf("[%d/%d] %s — ERR %v\n", i+1, len(probeCases), tc.prompt, err)
			continue
		}
		if got == tc.expected {
			mark = "✓"
			hits++
		}
		rows = append(rows, fmt.Sprintf("| %d | `%s` | %s | `%s` | %s %s |",
			i+1, escapeMD(tc.prompt), tc.expected, got, mark, truncate(gotArgs, 50)))
		fmt.Printf("[%d/%d] %s\n         expect=%s got=%s args=%s [%s]\n",
			i+1, len(probeCases), tc.prompt, tc.expected, got, truncate(gotArgs, 80), mark)
	}

	rate := float64(hits) / float64(len(probeCases)) * 100
	fmt.Println()
	fmt.Println("## Summary")
	fmt.Println()
	fmt.Println("| # | Prompt | Expected | Got | 结果 |")
	fmt.Println("|---|---|---|---|---|")
	for _, r := range rows {
		fmt.Println(r)
	}
	fmt.Printf("\n**命中率: %d/%d = %.1f%%**\n", hits, len(probeCases), rate)

	if hits != len(probeCases) {
		os.Exit(1)
	}
}

// dispatchOnce 单次 dispatch: build messages → call LLM → 取第一个 tool_call 的 name+args
// 不调用 tool, 只看 LLM 的"选择"。 LLM 选了文本回复(无 tool_call)时返回 "<text>" 让上层标 miss。
func dispatchOnce(llm *agent.LLMClient, model string, role *agent.AgentRole, tools map[string]*tool.ToolDef, prompt string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	roleTools := agent.ConvertToolsForRole(tools, role.AllowedTools)
	temp := config.DefaultLLMTemperature
	req := agent.CompletionRequest{
		Model: model,
		Messages: []agent.Message{
			{Role: "system", Content: agent.StringPtr(role.SystemPrompt)},
			{Role: "user", Content: agent.StringPtr(prompt)},
		},
		Tools:              roleTools,
		ToolChoice:         "auto",
		Temperature:        &temp,
		EnableThinking:     agent.BoolPtr(false),
		ChatTemplateKwargs: map[string]interface{}{"enable_thinking": false},
	}
	resp, err := llm.Complete(ctx, req)
	if err != nil {
		return "", "", err
	}
	if len(resp.Choices) == 0 {
		return "", "", fmt.Errorf("LLM 返回空 choices (provider 可能拒绝 tools 参数)")
	}
	calls := resp.Choices[0].Message.ToolCalls
	if len(calls) == 0 {
		txt := ""
		if resp.Choices[0].Message.Content != nil {
			txt = strings.TrimSpace(*resp.Choices[0].Message.Content)
		}
		return "<text>", truncate(txt, 80), nil
	}
	return calls[0].Function.Name, calls[0].Function.Arguments, nil
}

// buildAllToolDefs 注册所有真实 tool 但用 stub ctrl/ov/subMgr — LLM 只看 Description, 不会 Execute,
// 所以 stub 永远不会被解引用。 复用 RegisterX 函数避免 Description 文本漂移 (违反 DRY 后果是
// 在 macro_tools.go 改 Description 但 probe 还用旧文本评估命中率, 形成静默撒谎)。
func buildAllToolDefs() map[string]*tool.ToolDef {
	tmpDir, err := os.MkdirTemp("", "dispatch-probe-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: 创建临时目录失败: %v\n", err)
		os.Exit(2)
	}
	basePath := tmpDir + "/base.json"
	// 最小 base config — overlay.NewConfigOverlay 会在 Apply 时读, probe 不调 Apply
	if err := os.WriteFile(basePath, []byte(`{"outbounds":[]}`), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: 写 base.json 失败: %v\n", err)
		os.Exit(2)
	}
	ov := overlay.NewConfigOverlay(basePath, tmpDir)
	ctrl := stubVpnController{}

	all := map[string]*tool.ToolDef{}
	merge := func(m map[string]*tool.ToolDef) {
		for k, v := range m {
			all[k] = v
		}
	}
	merge(tool.RegisterTools())
	merge(tool.RegisterOverlayTools(ov))
	merge(tool.RegisterDNSTools(ov))
	merge(tool.RegisterVpnTools(ctrl))
	merge(tool.RegisterPerAppTools(ov))
	// subMgr=nil → import_and_activate_subscription 不注册, 但其它 macro 注册
	// probe 需要它, 单独构造 mgr 太重 (要起 httptest server / 落盘 store), 用 nil 路径下面
	// 单独处理: 直接 reflect-import schema 即可
	merge(tool.RegisterMacroTools(ctrl, ov, nil))
	// 手动补 import_and_activate_subscription 的占位 ToolDef (Description 跟 macro_tools.go 同步,
	// 不调 Execute 所以 fn 体留空)。 这条 case 走 LLM 时只看 Description + schema, schema
	// 已在 tool_schema.go 注册。
	all["import_and_activate_subscription"] = &tool.ToolDef{
		Name: "import_and_activate_subscription",
		Description: "ATOMIC: import a subscription URL and switch proxy-group to its best node in ONE call. " +
			"Internally: import_subscription → ensure VPN running → measure imported nodes → pick lowest-latency (or first if pick_fastest=false) → switch_node. " +
			"Use this for ANY '导这个订阅/导入这个订阅然后激活/import this sub and use it' request. " +
			"DO NOT call import_subscription + switch_node separately, this tool does both atomically and reports a single Success/failure.",
		IsWriteOp: true,
	}
	return all
}

func escapeMD(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
