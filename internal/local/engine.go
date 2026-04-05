package local

import (
	"fmt"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/tool"
)

// Engine dispatches action IDs to their handler functions.
type Engine struct {
	adapter  engine.EngineAdapter
	pipeline *tool.ToolPipeline
	actions  map[string]ActionFunc
	groupTag string // the primary selector group tag
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
	}
	return e
}

func (e *Engine) Execute(actionID string, params map[string]string) (string, error) {
	fn, ok := e.actions[actionID]
	if !ok {
		return "", fmt.Errorf("unknown action: %s", actionID)
	}
	return fn(params)
}
