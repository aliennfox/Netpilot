package local

import (
	"fmt"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
	"github.com/foxnetpilot/netpilot/internal/subscription"
	"github.com/foxnetpilot/netpilot/internal/template"
	"github.com/foxnetpilot/netpilot/internal/tool"
)

// Engine dispatches action IDs to their handler functions.
type Engine struct {
	adapter   engine.EngineAdapter
	pipeline  *tool.ToolPipeline
	overlay   *overlay.ConfigOverlay
	templates *template.TemplateStore
	subMgr    *subscription.SubscriptionManager
	actions   map[string]ActionFunc
	groupTag  string // the primary selector group tag
}

// ActionFunc executes an action and returns a human-readable result.
type ActionFunc func(params map[string]string) (string, error)

func NewEngine(adapter engine.EngineAdapter, pipeline *tool.ToolPipeline, primaryGroup string) *Engine {
	e := &Engine{
		adapter:  adapter,
		pipeline: pipeline,
		groupTag: primaryGroup,
	}
	e.actions = map[string]ActionFunc{
		"switch_best_node": e.switchBestNode,
		"latency_test_all": e.latencyTestAll,
		"set_mode_global":  e.setModeGlobal,
		"set_mode_direct":  e.setModeDirect,
		"set_mode_rule":    e.setModeRule,
		"show_status":      e.showStatus,
		"show_nodes":       e.showNodes,
		"show_snapshots":   e.showSnapshots,
		"do_rollback":      e.doRollback,
		"show_telemetry":   e.showTelemetry,
		"apply_template":   e.applyTemplate,
		"list_templates":   e.listTemplates,
		"list_rules":       e.listRules,
		"remove_rule":          e.removeRule,
		"import_subscription":  e.importSubscription,
		"list_subscriptions":   e.listSubscriptions,
		"update_subscription":  e.updateSubscription,
		"remove_subscription":  e.removeSubscription,
	}
	return e
}

// SetOverlay 设置 overlay（在 main 中初始化后注入）
func (e *Engine) SetOverlay(ov *overlay.ConfigOverlay) {
	e.overlay = ov
}

// SetTemplates 设置模板仓库
func (e *Engine) SetTemplates(ts *template.TemplateStore) {
	e.templates = ts
}

// SetSubscriptionManager 设置订阅管理器
func (e *Engine) SetSubscriptionManager(mgr *subscription.SubscriptionManager) {
	e.subMgr = mgr
}

func (e *Engine) Execute(actionID string, params map[string]string) (string, error) {
	fn, ok := e.actions[actionID]
	if !ok {
		return "", fmt.Errorf("unknown action: %s", actionID)
	}
	return fn(params)
}
