// Package mobile 是 Pilotty 的 gomobile 绑定层。
//
// 设计约束：gomobile bind 只支持有限类型（string/int/int64/float64/bool/[]byte/error，
// 以及由这些类型组成的 struct 和带单返回值+error 的接口）。因此本包暴露的所有跨语言
// 方法统一采用 JSON 字符串 in / JSON 字符串 out 的模式，避免 map/interface{} 等不可绑定类型。
//
// Kotlin 侧使用方式：
//
//	val client = Mobile.newClient("/data/data/.../files", "127.0.0.1:9090", System.getenv("SILICONFLOW_API_KEY"))
//	val statusJSON = client.status()
//	val replyJSON = client.chat("换个最快的节点")
package mobile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/foxnetpilot/netpilot/internal/agent"
	"github.com/foxnetpilot/netpilot/internal/backup"
	"github.com/foxnetpilot/netpilot/internal/config"
	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/failover"
	"github.com/foxnetpilot/netpilot/internal/local"
	"github.com/foxnetpilot/netpilot/internal/overlay"
	"github.com/foxnetpilot/netpilot/internal/router"
	"github.com/foxnetpilot/netpilot/internal/subscription"
	"github.com/foxnetpilot/netpilot/internal/template"
	"github.com/foxnetpilot/netpilot/internal/tool"
	"github.com/foxnetpilot/netpilot/libcore"
)

// Client 是 Android/iOS 侧持有的核心句柄。线程安全。
type Client struct {
	mu sync.Mutex

	adapter      engine.EngineAdapter
	pipeline     *tool.ToolPipeline
	localEngine  *local.Engine
	overlay      *overlay.ConfigOverlay
	subMgr       *subscription.SubscriptionManager
	templates    *template.TemplateStore
	orchestrator *agent.Orchestrator
	history      *agent.ConversationHistory
	intentRouter *router.IntentRouter
	failover     *failover.Monitor

	// TUN 数据面: 由 libcore.BoxInstance 持有 sing-box 实例。
	// 3B-1 阶段 BoxInstance 的 Start/Close 是 stub, 3B-3 接入真 libbox CommandServer。
	box   *libcore.BoxInstance
	iface libcore.PlatformInterface
}

// NewClient 创建一个新的 Pilotty 客户端。
//
//   - dataDir:      可写数据目录（订阅、快照、日志等）。Android 侧通常为 context.filesDir.absolutePath
//   - clashAPIAddr: sing-box Clash API 地址，例如 "127.0.0.1:9090"
//   - apiKey:       SiliconFlow API key；为空则禁用 LLM Agent
//
// 该函数不会启动 sing-box 内核——内核由平台层（Android VpnService / iOS NEPacketTunnelProvider）负责生命周期。
func NewClient(dataDir, clashAPIAddr, apiKey string) *Client {
	if clashAPIAddr == "" {
		clashAPIAddr = config.Default().ClashAPIAddr
	}

	adapter := engine.NewSingBoxAdapter(clashAPIAddr)
	pipeline := tool.NewPipeline(adapter, dataDir)
	// 移动端默认 auto 模式：UI 层负责弹出确认对话框
	pipeline.SetPermissionManager(tool.NewPermissionManager(tool.TrustAuto, tool.AutoApprover{}))

	intentRouter := router.NewIntentRouter()
	localEngine := local.NewEngine(adapter, pipeline, "proxy-group")

	baseConfig := filepath.Join(dataDir, "configs", "minimal.json")
	ov := overlay.NewConfigOverlay(baseConfig, dataDir)
	_ = ov.Load() // 缺失文件不算错误

	subStore := subscription.NewSubscriptionStore(dataDir)
	_ = subStore.Load()
	subMgr := subscription.NewSubscriptionManager(subStore, ov, adapter)

	pipeline.RegisterExtraTools(tool.RegisterOverlayTools(ov))
	pipeline.RegisterExtraTools(tool.RegisterSubscriptionTools(subMgr))
	pipeline.RegisterExtraTools(tool.RegisterDNSTools(ov))

	mergedPath := ov.MergedConfigPath()
	if _, err := os.Stat(mergedPath); err == nil {
		adapter.SetConfigPath(mergedPath)
	} else {
		adapter.SetConfigPath(baseConfig)
	}

	ts := template.NewTemplateStore()
	localEngine.SetOverlay(ov)
	localEngine.SetTemplates(ts)
	localEngine.SetSubscriptionManager(subMgr)

	subMgr.StartAutoUpdate()

	var orchestrator *agent.Orchestrator
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

	fo := failover.New(adapter, failover.Config{})

	return &Client{
		adapter:      adapter,
		pipeline:     pipeline,
		localEngine:  localEngine,
		overlay:      ov,
		subMgr:       subMgr,
		templates:    ts,
		orchestrator: orchestrator,
		history:      agent.NewConversationHistory(40),
		intentRouter: intentRouter,
		failover:     fo,
	}
}

// StartFailover 启动后台节点健康监控与自动故障切换。
// configJSON 可选；为空使用默认配置。
func (c *Client) StartFailover(configJSON string) string {
	if configJSON != "" {
		var cfg failover.Config
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return errJSON(err)
		}
		c.failover.SetConfig(cfg)
	}
	c.failover.Start()
	return okJSON(c.failover.Status())
}

// StopFailover 停止后台监控。
func (c *Client) StopFailover() string {
	c.failover.Stop()
	return okJSON(c.failover.Status())
}

// FailoverStatus 返回当前监控状态快照。
func (c *Client) FailoverStatus() string {
	return okJSON(c.failover.Status())
}

// Snapshots 返回最近的快照列表 (D2 Safety card)。
// 按时间倒序,最新的在前。每项含 id / timestamp / active_proxies (group→tag)。
func (c *Client) Snapshots() string {
	return okJSON(c.pipeline.Snapshots().List())
}

// Rollback 手动回滚到指定快照 id;id 为空时回滚到最新快照 (D2 Safety card)。
func (c *Client) Rollback(id string) string {
	r := c.pipeline.ManualRollback(id)
	if r == nil {
		return errStr("rollback returned nil result")
	}
	if !r.Success {
		return errJSON(fmt.Errorf("%s", r.Message))
	}
	return okJSON(map[string]string{"message": r.Message})
}

// ExportBackup 导出当前用户数据为 JSON 字符串（overlay + subscriptions）。
func (c *Client) ExportBackup() string {
	mgr := backup.NewManager(c.overlay, c.subMgr.Store())
	data, err := mgr.Export()
	if err != nil {
		return errJSON(err)
	}
	return okJSON(json.RawMessage(data))
}

// ImportBackup 从 JSON 字符串恢复用户数据。
// ⚠️ 会整体覆盖现有 overlay 与订阅；调用方需先在 UI 提示用户确认。
func (c *Client) ImportBackup(data string) string {
	if data == "" {
		return errStr("backup data is required")
	}
	mgr := backup.NewManager(c.overlay, c.subMgr.Store())
	snap, err := mgr.Import([]byte(data))
	if err != nil {
		return errJSON(err)
	}
	return okJSON(map[string]interface{}{
		"version":            snap.Version,
		"exported_at":        snap.ExportedAt,
		"route_rules":        len(snap.Overlay.RouteRules),
		"outbounds":          len(snap.Overlay.Outbounds),
		"subscription_count": len(snap.Subscriptions),
	})
}

// SetPlatformInterface 由平台层 (Android VpnService / iOS NEPacketTunnelProvider) 在
// App 启动后、StartTun 之前调用一次, 注入平台能力 (如 Android VpnService.Builder、
// protect、WIFI 状态等)。Go 侧在 sing-box 启动 TUN 时会反向回调 iface.OpenTun 取 fd。
func (c *Client) SetPlatformInterface(iface libcore.PlatformInterface) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.iface = iface
}

// StartTun 启动内核数据面。
//
// sing-box libbox 的 tun inbound 强制通过 PlatformInterface.OpenTun() 反向回调拿 fd,
// 不支持预传 fd。所以 Kotlin/Swift 端必须先 SetPlatformInterface, 其 openTun()
// 方法内部再调 VpnService.Builder.establish() 返回 fd (见 docs/phase-3b-plan.md §6.3)。
//
// 入参:
//   - configJSON: sing-box 配置 JSON; 留空则使用当前 overlay 合并后的配置
//
// ⚠️ 3B-1 状态: libcore.BoxInstance.Start 是 stub, 真启动要等 3B-3。
// 本方法签名在 3B-3 无需再改, 平台端代码可按最终契约对接。
func (c *Client) StartTun(configJSON string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.box != nil && c.box.IsRunning() {
		return fmt.Errorf("tun already running")
	}
	if configJSON == "" {
		data, err := os.ReadFile(c.overlay.MergedConfigPath())
		if err != nil {
			return fmt.Errorf("read merged config: %w", err)
		}
		configJSON = string(data)
	}
	iface := c.iface
	if iface == nil {
		iface = libcore.NopPlatformInterface{}
	}
	box, err := libcore.NewBoxInstance(configJSON, iface)
	if err != nil {
		return err
	}
	if err := box.Start(); err != nil {
		_ = box.Close()
		return err
	}
	c.box = box
	return nil
}

// StopTun 停止内核数据面。
func (c *Client) StopTun() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.box != nil {
		_ = c.box.Close()
		c.box = nil
	}
}

// TunRunning 返回 TUN 数据面是否在运行。
func (c *Client) TunRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.box != nil && c.box.IsRunning()
}

// Shutdown 停止后台任务。Android Activity onDestroy / iOS app terminate 时调用。
func (c *Client) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.subMgr != nil {
		c.subMgr.StopAutoUpdate()
	}
	if c.failover != nil {
		c.failover.Stop()
	}
	if c.box != nil {
		_ = c.box.Close()
		c.box = nil
	}
}

// AgentReady 返回 LLM Agent 是否可用。
func (c *Client) AgentReady() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.orchestrator != nil
}

// Version 返回当前绑定层版本（用于 Kotlin 侧自检）。
func (c *Client) Version() string {
	return "netpilot-mobile/0.1.0"
}

// --- JSON 响应封装 ---

type response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func okJSON(data interface{}) string {
	b, _ := json.Marshal(response{Success: true, Data: data})
	return string(b)
}

func errJSON(err error) string {
	b, _ := json.Marshal(response{Success: false, Error: err.Error()})
	return string(b)
}

func errStr(msg string) string {
	b, _ := json.Marshal(response{Success: false, Error: msg})
	return string(b)
}

// --- Status / Nodes ---

// Status 返回当前代理状态 JSON：
//
//	{ "current_node": "...", "mode": "...", "node_count": N, "connections": N,
//	  "upload": int64, "download": int64, "agent_ready": bool }
func (c *Client) Status() string {
	c.mu.Lock()
	agentReady := c.orchestrator != nil
	c.mu.Unlock()
	data := map[string]interface{}{"agent_ready": agentReady}
	if g, err := c.adapter.GetProxyGroup("proxy-group"); err == nil {
		data["current_node"] = g.Now
		data["node_count"] = len(g.All)
		data["mode"] = g.Type
	}
	if conns, err := c.adapter.GetConnections(); err == nil {
		data["connections"] = len(conns)
	}
	if t, err := c.adapter.GetTrafficStats(); err == nil && t != nil {
		data["upload"] = t.Upload
		data["download"] = t.Download
	}
	return okJSON(data)
}

// Nodes 返回节点列表 JSON。
func (c *Client) Nodes() string {
	// 先从 overlay 拿 server/port 静态信息 (Clash API 不返回这俩)。
	obsByTag := map[string]map[string]interface{}{}
	for _, ob := range c.overlay.ListOutbounds() {
		if tag, ok := ob["tag"].(string); ok {
			obsByTag[tag] = ob
		}
	}
	extract := func(ob map[string]interface{}) (server string, port int) {
		if ob == nil {
			return
		}
		server, _ = ob["server"].(string)
		switch v := ob["server_port"].(type) {
		case float64:
			port = int(v)
		case int:
			port = v
		case int64:
			port = int(v)
		}
		return
	}

	// 优先走 Clash API: 拿到 alive / latency / active 实时数据
	if g, err := c.adapter.GetProxyGroup("proxy-group"); err == nil {
		out := make([]map[string]interface{}, 0, len(g.All))
		for _, p := range g.All {
			server, port := extract(obsByTag[p.Tag])
			if p.Server != "" {
				server = p.Server
			}
			if p.Port != 0 {
				port = p.Port
			}
			out = append(out, map[string]interface{}{
				"tag":       p.Tag,
				"type":      p.Type,
				"server":    server,
				"port":      port,
				"alive":     p.Alive,
				"latency":   p.Latency,
				"group_tag": p.GroupTag,
				"active":    p.Tag == g.Now,
			})
		}
		return okJSON(out)
	}
	// Fallback: VPN 未启动时 Clash API 不可达, 从 overlay 读节点静态信息
	// (alive/latency 未知, UI 照旧能展示、切换, 启动 VPN 后自动刷新真数据)
	obs := c.overlay.ListOutbounds()
	out := make([]map[string]interface{}, 0, len(obs))
	for _, ob := range obs {
		t, _ := ob["type"].(string)
		// 跳过 selector / urltest / 内置 direct / block / dns
		switch t {
		case "", "selector", "urltest", "direct", "block", "dns":
			continue
		}
		tag, _ := ob["tag"].(string)
		server, _ := ob["server"].(string)
		port := 0
		switch v := ob["server_port"].(type) {
		case float64:
			port = int(v)
		case int:
			port = v
		case int64:
			port = int(v)
		}
		out = append(out, map[string]interface{}{
			"tag":       tag,
			"type":      t,
			"server":    server,
			"port":      port,
			"alive":     false,
			"latency":   0,
			"group_tag": "proxy-group",
			"active":    false,
		})
	}
	return okJSON(out)
}

// SwitchNode 切换到指定节点。group 留空时使用 "proxy-group"。
func (c *Client) SwitchNode(group, node string) string {
	if group == "" {
		group = "proxy-group"
	}
	r := c.pipeline.Execute(context.Background(), "switch_node", map[string]interface{}{
		"group": group,
		"node":  node,
	})
	if !r.Success {
		return errStr(r.Message)
	}
	return okJSON(map[string]string{"message": r.Message})
}

// SetMode 切换代理模式（global/rule/direct）。
func (c *Client) SetMode(mode string) string {
	r := c.pipeline.Execute(context.Background(), "set_mode", map[string]interface{}{"mode": mode})
	if !r.Success {
		return errStr(r.Message)
	}
	return okJSON(map[string]string{"message": r.Message})
}

// TestLatency 测试单个节点延迟。
func (c *Client) TestLatency(node string) string {
	r := c.pipeline.Execute(context.Background(), "test_latency", map[string]interface{}{"tag": node})
	if !r.Success {
		return errStr(r.Message)
	}
	return okJSON(r.Data)
}

// TestLatencyAll 测试所有节点延迟。
func (c *Client) TestLatencyAll() string {
	r := c.pipeline.Execute(context.Background(), "test_latency_all", nil)
	if !r.Success {
		return errStr(r.Message)
	}
	return okJSON(r.Data)
}

// --- Chat ---

// Chat 处理一条用户消息：先尝试 Intent Router，失败回落到 LLM Agent。
// 返回 JSON：
//
//	{ "reply": "...", "source": "local"|"agent", "events": [{name, args_summary, duration_ms, output_preview, error, role}, ...] }
//
// local 来源没有 events (IntentRouter 不经 orchestrator), Kotlin 侧 UI 需优雅降级 (显示"本地路由 · 未调 Agent")。
func (c *Client) Chat(message string) string {
	if message == "" {
		return errStr("message is required")
	}

	routing := c.intentRouter.Route(message)
	if routing.Matched {
		routing.Params["_input"] = message
		result, err := c.localEngine.Execute(routing.ActionID, routing.Params)
		if err != nil {
			return errJSON(err)
		}
		clean := stripANSI(result)
		c.history.Add("user", message, "local")
		c.history.Add("assistant", clean, "local")
		return okJSON(map[string]interface{}{"reply": clean, "source": "local"})
	}

	c.mu.Lock()
	orch := c.orchestrator
	c.mu.Unlock()
	if orch == nil {
		return errStr("AI Agent 未启用")
	}

	result, err := orch.Run(context.Background(), message, c.history)
	if err != nil {
		return errJSON(fmt.Errorf("Agent 错误: %v", err))
	}
	c.history.Add("user", message, "agent")
	c.history.Add("assistant", result.Reply, "agent")
	return okJSON(map[string]interface{}{
		"reply":  result.Reply,
		"source": "agent",
		"events": result.Events,
	})
}

// SetAPIKey 热重载 LLM apiKey。Kotlin 侧用户在 Settings 改 key 后立即调这个, 无需重启 App。
// key 为空 → 关闭 orchestrator (Agent 回到"未启用"状态)。
// 线程安全：持 Client.mu 重建 orchestrator。
func (c *Client) SetAPIKey(key string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if key == "" {
		c.orchestrator = nil
		return okJSON(map[string]interface{}{"agent_ready": false})
	}
	llmClient := agent.NewLLMClient(
		config.DefaultLLMBaseURL,
		key,
		config.DefaultLLMModel,
		time.Duration(config.DefaultLLMTimeout)*time.Second,
	)
	assembler := agent.NewPromptAssembler(c.adapter)
	c.orchestrator = agent.NewOrchestrator(llmClient, c.pipeline, assembler, c.pipeline.GetTools())
	return okJSON(map[string]interface{}{"agent_ready": true})
}

// ClearHistory 清空对话历史。
func (c *Client) ClearHistory() {
	c.history.Clear()
}

// --- Subscriptions ---

// Subscriptions 列出所有订阅（JSON 数组）。
func (c *Client) Subscriptions() string {
	return okJSON(c.subMgr.Store().List())
}

// AddSubscription 添加订阅。name 留空时自动推断。
func (c *Client) AddSubscription(name, url string) string {
	if url == "" {
		return errStr("url is required")
	}
	count, summary, err := c.subMgr.AddSubscription(name, url)
	if err != nil {
		return errJSON(err)
	}
	return okJSON(map[string]interface{}{
		"node_count": count,
		"summary":    summary,
	})
}

// RemoveSubscription 删除订阅。
func (c *Client) RemoveSubscription(id string) string {
	msg, err := c.subMgr.RemoveSubscription(id)
	if err != nil {
		return errJSON(err)
	}
	return okJSON(map[string]string{"message": stripANSI(msg)})
}

// UpdateAllSubscriptions 更新全部订阅。
func (c *Client) UpdateAllSubscriptions() string {
	return okJSON(map[string]string{"message": stripANSI(c.subMgr.UpdateAll())})
}

// ImportNodeURI 从单个 proxy URI (vmess://, ss://, vless://, trojan://, tuic://, hysteria2://, ...)
// 解析并导入节点。 Kotlin 侧 MainActivity.onNewIntent / 剪贴板自动检测都走这个入口。
// 返回 { success, data: { node_count, message } }, Kotlin 侧 MessageDto 只看 message。
func (c *Client) ImportNodeURI(uri string) string {
	count, summary, err := c.subMgr.ImportNodeURI(uri)
	if err != nil {
		return errJSON(err)
	}
	return okJSON(map[string]interface{}{
		"node_count": count,
		"message":    summary,
	})
}

// --- Rules / Templates ---

// Rules 返回当前路由规则列表。
func (c *Client) Rules() string {
	return okJSON(c.overlay.ListRules())
}

// AddRule 添加一条路由规则 (M18)。
// ruleJSON 是 overlay.RouteRule 的 JSON 序列化,格式:
//
//	{"tag":"my-rule","domain_suffix":["example.com"],"outbound":"proxy","description":"..."}
//
// 字段 source 由服务端固定填 "user", 避免被客户端伪造成 "agent"。
func (c *Client) AddRule(ruleJSON string) string {
	var rule overlay.RouteRule
	if err := json.Unmarshal([]byte(ruleJSON), &rule); err != nil {
		return errJSON(fmt.Errorf("规则 JSON 解析失败: %v", err))
	}
	if rule.Tag == "" || rule.Outbound == "" {
		return errStr("tag and outbound are required")
	}
	rule.Source = "user" // 强制覆盖, 用户 UI 创建的规则永远是 user 来源
	if err := c.overlay.AddRule(rule); err != nil {
		return errJSON(err)
	}
	if err := c.overlay.Apply(c.adapter); err != nil {
		return errJSON(fmt.Errorf("应用规则失败: %v", err))
	}
	return okJSON(map[string]string{"message": "规则已添加"})
}

// RemoveRule 按 tag 删除一条路由规则 (M18)。
func (c *Client) RemoveRule(tag string) string {
	if tag == "" {
		return errStr("tag is required")
	}
	if err := c.overlay.RemoveRule(tag); err != nil {
		return errJSON(err)
	}
	if err := c.overlay.Apply(c.adapter); err != nil {
		return errJSON(fmt.Errorf("应用规则失败: %v", err))
	}
	return okJSON(map[string]string{"message": "规则已删除"})
}

// Templates 返回内置模板列表。
func (c *Client) Templates() string {
	return okJSON(c.templates.List())
}

// ApplyTemplate 应用指定模板。
func (c *Client) ApplyTemplate(id string) string {
	result, err := c.localEngine.Execute("apply_template", map[string]string{"_input": id})
	if err != nil {
		return errJSON(err)
	}
	return okJSON(map[string]string{"message": stripANSI(result)})
}

// stripANSI 去除终端颜色码。
func stripANSI(s string) string {
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
