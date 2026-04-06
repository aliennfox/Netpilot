package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

// rateLimiter 简单的 token bucket 限流器
type rateLimiter struct {
	mu       sync.Mutex
	tokens   int
	max      int
	interval time.Duration
	lastFill time.Time
}

func newRateLimiter(maxPerInterval int, interval time.Duration) *rateLimiter {
	return &rateLimiter{
		tokens:   maxPerInterval,
		max:      maxPerInterval,
		interval: interval,
		lastFill: time.Now(),
	}
}

func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if now.Sub(rl.lastFill) >= rl.interval {
		rl.tokens = rl.max
		rl.lastFill = now
	}
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

// authMiddleware Bearer token 认证中间件
func authMiddleware(apiToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 开发模式：未设置 token 则跳过认证
		if apiToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		// GET 请求不需要认证
		if r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		// 写操作需要 Bearer token
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || auth[7:] != apiToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(APIResponse{Success: false, Error: "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// APIResponse 统一 API 响应格式
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ChatResponse Agent 聊天响应
type ChatResponse struct {
	Reply   string       `json:"reply"`
	Source  string       `json:"source"`
	Stages []string     `json:"stages,omitempty"`
	Actions []ChatAction `json:"actions,omitempty"`
}

// ChatAction Agent 建议的后续操作
type ChatAction struct {
	Label   string `json:"label"`
	Command string `json:"command"`
}

// Server 持有所有依赖
type Server struct {
	adapter      engine.EngineAdapter
	pipeline     *tool.ToolPipeline
	localEngine  *local.Engine
	overlay      *overlay.ConfigOverlay
	subMgr       *subscription.SubscriptionManager
	templates    *template.TemplateStore
	orchestrator *agent.Orchestrator
	history      *agent.ConversationHistory
	intentRouter *router.IntentRouter
	chatLimiter  *rateLimiter
}

func main() {
	cfg := config.Default()
	adapter := engine.NewSingBoxAdapter(cfg.ClashAPIAddr)
	pipeline := tool.NewPipeline(adapter, cfg.DataDir)
	intentRouter := router.NewIntentRouter()
	localEngine := local.NewEngine(adapter, pipeline, "proxy-group")

	// 初始化 Config Overlay
	baseConfig := "configs/minimal.json"
	ov := overlay.NewConfigOverlay(baseConfig, cfg.DataDir)
	if err := ov.Load(); err != nil {
		log.Printf("加载 overlay 失败: %v", err)
	}

	// 初始化订阅管理器
	subStore := subscription.NewSubscriptionStore(cfg.DataDir)
	if err := subStore.Load(); err != nil {
		log.Printf("加载订阅数据失败: %v", err)
	}
	subMgr := subscription.NewSubscriptionManager(subStore, ov, adapter)

	// 注册额外 tools
	pipeline.RegisterExtraTools(tool.RegisterOverlayTools(ov))
	pipeline.RegisterExtraTools(tool.RegisterSubscriptionTools(subMgr))
	pipeline.RegisterExtraTools(tool.RegisterDNSTools(ov))

	// 设置 adapter 配置路径
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

	// 初始化 Agent Orchestrator
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

	// 对话历史
	history := agent.NewConversationHistory(40)

	srv := &Server{
		adapter:      adapter,
		pipeline:     pipeline,
		localEngine:  localEngine,
		overlay:      ov,
		subMgr:       subMgr,
		templates:    ts,
		orchestrator: orchestrator,
		history:      history,
		intentRouter: intentRouter,
		chatLimiter:  newRateLimiter(10, time.Minute),
	}

	mux := http.NewServeMux()

	// 状态 & 节点
	mux.HandleFunc("GET /api/status", srv.handleGetStatus)
	mux.HandleFunc("GET /api/nodes", srv.handleGetNodes)
	mux.HandleFunc("POST /api/switch", srv.handleSwitchNode)
	mux.HandleFunc("POST /api/mode", srv.handleSetMode)

	// 延迟测试
	mux.HandleFunc("POST /api/latency/test", srv.handleTestLatency)
	mux.HandleFunc("POST /api/latency/all", srv.handleTestLatencyAll)

	// 聊天
	mux.HandleFunc("POST /api/chat", srv.handleChat)

	// 规则
	mux.HandleFunc("GET /api/rules", srv.handleGetRules)
	mux.HandleFunc("POST /api/rules", srv.handleAddRule)
	mux.HandleFunc("DELETE /api/rules/{tag}", srv.handleDeleteRule)

	// 模板
	mux.HandleFunc("GET /api/templates", srv.handleGetTemplates)
	mux.HandleFunc("POST /api/templates/apply", srv.handleApplyTemplate)

	// 订阅
	mux.HandleFunc("GET /api/subscriptions", srv.handleGetSubscriptions)
	mux.HandleFunc("POST /api/subscriptions", srv.handleAddSubscription)
	mux.HandleFunc("DELETE /api/subscriptions/{id}", srv.handleDeleteSubscription)
	mux.HandleFunc("POST /api/subscriptions/update", srv.handleUpdateSubscriptions)

	// 连接 & 日志
	mux.HandleFunc("GET /api/connections", srv.handleGetConnections)
	mux.HandleFunc("GET /api/logs", srv.handleGetLogs)

	// 认证中间件
	apiToken := os.Getenv("NETPILOT_API_TOKEN")
	handler := authMiddleware(apiToken, mux)

	addr := "127.0.0.1:8080"
	log.Printf("NetPilot HTTP Server starting on %s", addr)
	log.Printf("Clash API: %s", cfg.ClashAPIAddr)
	if apiToken != "" {
		log.Printf("Auth: Bearer token 已启用")
	} else {
		log.Printf("Auth: 开发模式（无认证）")
	}
	if orchestrator != nil {
		log.Printf("AI Agent: %s (%s)", config.DefaultLLMModel, config.DefaultLLMBaseURL)
	} else {
		log.Printf("AI Agent: 未启用 (设置 SILICONFLOW_API_KEY)")
	}

	// 优雅关闭
	httpServer := &http.Server{Addr: addr, Handler: handler}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("正在关闭服务器...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("服务器已关闭")
}

// --- 响应辅助 ---

func writeJSON(w http.ResponseWriter, status int, resp APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func okJSON(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: data})
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, APIResponse{Success: false, Error: msg})
}

func decodeBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// --- Status ---

type StatusData struct {
	CurrentNode string `json:"current_node"`
	Mode        string `json:"mode"`
	NodeCount   int    `json:"node_count"`
	Connections int    `json:"connections"`
	Upload      int64  `json:"upload"`
	Download    int64  `json:"download"`
	AgentReady  bool   `json:"agent_ready"`
}

func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	data := StatusData{
		AgentReady: s.orchestrator != nil,
	}

	// 当前节点
	if group, err := s.adapter.GetProxyGroup("proxy-group"); err == nil {
		data.CurrentNode = group.Now
		data.NodeCount = len(group.All)
		// 尝试获取模式
		data.Mode = group.Type
	}

	// 连接数
	if conns, err := s.adapter.GetConnections(); err == nil {
		data.Connections = len(conns)
	}

	// 流量
	if traffic, err := s.adapter.GetTrafficStats(); err == nil && traffic != nil {
		data.Upload = traffic.Upload
		data.Download = traffic.Download
	}

	okJSON(w, data)
}

// --- Nodes ---

type NodeData struct {
	Tag      string `json:"tag"`
	Type     string `json:"type"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Alive    bool   `json:"alive"`
	Latency  int    `json:"latency"`
	GroupTag string `json:"group_tag"`
	Active   bool   `json:"active"`
}

func (s *Server) handleGetNodes(w http.ResponseWriter, r *http.Request) {
	group, err := s.adapter.GetProxyGroup("proxy-group")
	if err != nil {
		errJSON(w, http.StatusInternalServerError, fmt.Sprintf("获取节点失败: %v", err))
		return
	}

	nodes := make([]NodeData, 0, len(group.All))
	for _, p := range group.All {
		nodes = append(nodes, NodeData{
			Tag:      p.Tag,
			Type:     p.Type,
			Server:   p.Server,
			Port:     p.Port,
			Alive:    p.Alive,
			Latency:  p.Latency,
			GroupTag:  p.GroupTag,
			Active:   p.Tag == group.Now,
		})
	}
	okJSON(w, nodes)
}

// --- Switch Node ---

func (s *Server) handleSwitchNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Group string `json:"group"`
		Node  string `json:"node"`
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Group == "" {
		req.Group = "proxy-group"
	}

	result := s.pipeline.Execute(context.Background(), "switch_node", map[string]interface{}{
		"group": req.Group,
		"node":  req.Node,
	})
	if !result.Success {
		errJSON(w, http.StatusInternalServerError, result.Message)
		return
	}
	okJSON(w, map[string]string{"message": result.Message})
}

// --- Set Mode ---

func (s *Server) handleSetMode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result := s.pipeline.Execute(context.Background(), "set_mode", map[string]interface{}{
		"mode": req.Mode,
	})
	if !result.Success {
		errJSON(w, http.StatusInternalServerError, result.Message)
		return
	}
	okJSON(w, map[string]string{"message": result.Message})
}

// --- Latency ---

func (s *Server) handleTestLatency(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Node string `json:"node"`
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result := s.pipeline.Execute(context.Background(), "test_latency", map[string]interface{}{
		"tag": req.Node,
	})
	if !result.Success {
		errJSON(w, http.StatusInternalServerError, result.Message)
		return
	}
	okJSON(w, result.Data)
}

func (s *Server) handleTestLatencyAll(w http.ResponseWriter, r *http.Request) {
	result := s.pipeline.Execute(context.Background(), "test_latency_all", nil)
	if !result.Success {
		errJSON(w, http.StatusInternalServerError, result.Message)
		return
	}
	okJSON(w, result.Data)
}

// --- Chat ---

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Message == "" {
		errJSON(w, http.StatusBadRequest, "message is required")
		return
	}

	// 速率限制
	if !s.chatLimiter.allow() {
		errJSON(w, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
		return
	}

	// 1. 先试 Intent Router（本地处理）
	routing := s.intentRouter.Route(req.Message)
	if routing.Matched {
		routing.Params["_input"] = req.Message
		result, err := s.localEngine.Execute(routing.ActionID, routing.Params)
		if err != nil {
			errJSON(w, http.StatusInternalServerError, err.Error())
			return
		}

		s.history.Add("user", req.Message, "local")
		s.history.Add("assistant", stripANSI(result), "local")

		resp := ChatResponse{
			Reply:  stripANSI(result),
			Source: "local",
			Actions: suggestActions(routing.ActionID),
		}
		okJSON(w, resp)
		return
	}

	// 2. 走 Agent
	if s.orchestrator == nil {
		errJSON(w, http.StatusServiceUnavailable, "AI Agent 未启用")
		return
	}

	reply, err := s.orchestrator.Run(context.Background(), req.Message, s.history)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, fmt.Sprintf("Agent 错误: %v", err))
		return
	}

	s.history.Add("user", req.Message, "agent")
	s.history.Add("assistant", reply, "agent")

	resp := ChatResponse{
		Reply:  reply,
		Source: "agent",
		Stages: detectStages(reply),
		Actions: suggestActionsFromReply(reply),
	}
	okJSON(w, resp)
}

// --- Rules ---

func (s *Server) handleGetRules(w http.ResponseWriter, r *http.Request) {
	rules := s.overlay.ListRules()
	okJSON(w, rules)
}

func (s *Server) handleAddRule(w http.ResponseWriter, r *http.Request) {
	var rule overlay.RouteRule
	if err := decodeBody(r, &rule); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if rule.Tag == "" || rule.Outbound == "" {
		errJSON(w, http.StatusBadRequest, "tag and outbound are required")
		return
	}

	if err := s.overlay.AddRule(rule); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.overlay.Apply(s.adapter); err != nil {
		errJSON(w, http.StatusInternalServerError, fmt.Sprintf("应用规则失败: %v", err))
		return
	}
	okJSON(w, map[string]string{"message": "规则已添加"})
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	if tag == "" {
		errJSON(w, http.StatusBadRequest, "tag is required")
		return
	}

	if err := s.overlay.RemoveRule(tag); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.overlay.Apply(s.adapter); err != nil {
		errJSON(w, http.StatusInternalServerError, fmt.Sprintf("应用规则失败: %v", err))
		return
	}
	okJSON(w, map[string]string{"message": "规则已删除"})
}

// --- Templates ---

func (s *Server) handleGetTemplates(w http.ResponseWriter, r *http.Request) {
	templates := s.templates.List()
	okJSON(w, templates)
}

func (s *Server) handleApplyTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := map[string]string{"_input": req.ID}
	result, err := s.localEngine.Execute("apply_template", params)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	okJSON(w, map[string]string{"message": stripANSI(result)})
}

// --- Subscriptions ---

func (s *Server) handleGetSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs := s.subMgr.Store().List()
	okJSON(w, subs)
}

func (s *Server) handleAddSubscription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL == "" {
		errJSON(w, http.StatusBadRequest, "url is required")
		return
	}
	if req.Name == "" {
		req.Name = "未命名订阅"
	}

	count, summary, err := s.subMgr.AddSubscription(req.Name, req.URL)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	okJSON(w, map[string]interface{}{
		"node_count": count,
		"summary":    summary,
	})
}

func (s *Server) handleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "id is required")
		return
	}

	msg, err := s.subMgr.RemoveSubscription(id)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	okJSON(w, map[string]string{"message": stripANSI(msg)})
}

func (s *Server) handleUpdateSubscriptions(w http.ResponseWriter, r *http.Request) {
	result := s.subMgr.UpdateAll()
	okJSON(w, map[string]string{"message": stripANSI(result)})
}

// --- Connections ---

func (s *Server) handleGetConnections(w http.ResponseWriter, r *http.Request) {
	conns, err := s.adapter.GetConnections()
	if err != nil {
		errJSON(w, http.StatusInternalServerError, fmt.Sprintf("获取连接失败: %v", err))
		return
	}
	okJSON(w, conns)
}

// --- Logs ---

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	level := r.URL.Query().Get("level")
	if level == "" {
		level = "info"
	}
	logs, err := s.adapter.GetLogs(level, 50)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, fmt.Sprintf("获取日志失败: %v", err))
		return
	}
	okJSON(w, logs)
}

// --- 辅助函数 ---

func absPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

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

// suggestActions 根据动作 ID 建议后续操作
func suggestActions(actionID string) []ChatAction {
	switch actionID {
	case "switch_best_node", "latency_test_all":
		return []ChatAction{
			{Label: "查看节点", Command: "节点列表"},
			{Label: "测速", Command: "测速"},
		}
	case "show_nodes":
		return []ChatAction{
			{Label: "换最快节点", Command: "换节点"},
			{Label: "测速", Command: "测速"},
		}
	case "show_status":
		return []ChatAction{
			{Label: "查看节点", Command: "节点列表"},
			{Label: "查看连接", Command: "当前连接"},
		}
	default:
		return nil
	}
}

// suggestActionsFromReply 从 Agent 回复中推断建议操作
func suggestActionsFromReply(reply string) []ChatAction {
	var actions []ChatAction
	if strings.Contains(reply, "节点") || strings.Contains(reply, "延迟") {
		actions = append(actions, ChatAction{Label: "测速", Command: "测速"})
		actions = append(actions, ChatAction{Label: "查看节点", Command: "节点列表"})
	}
	if strings.Contains(reply, "规则") || strings.Contains(reply, "分流") {
		actions = append(actions, ChatAction{Label: "查看规则", Command: "规则列表"})
	}
	return actions
}

// detectStages 从回复中检测经过了哪些阶段
func detectStages(reply string) []string {
	var stages []string
	if strings.Contains(reply, "诊断") {
		stages = append(stages, "diagnose")
	}
	if strings.Contains(reply, "配置") || strings.Contains(reply, "切换") {
		stages = append(stages, "configure")
	}
	if strings.Contains(reply, "验证") {
		stages = append(stages, "verify")
	}
	return stages
}
