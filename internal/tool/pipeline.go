package tool

import (
	"context"
	"fmt"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// ToolPipeline manages all tool execution through an 8-step pipeline.
// Write operations go through: PreHook → Snapshot → Execute → PostHook → Telemetry.
// Read operations go through: Execute → Telemetry.
type ToolPipeline struct {
	tools     map[string]*ToolDef
	hooks     *HookEngine
	snapshots *SnapshotStore
	telemetry *TelemetryLogger
	adapter   engine.EngineAdapter
	perms     *PermissionManager
}

func NewPipeline(adapter engine.EngineAdapter, dataDir string) *ToolPipeline {
	return &ToolPipeline{
		tools:     RegisterTools(),
		hooks:     NewHookEngine(adapter),
		snapshots: NewSnapshotStore(dataDir + "/snapshots"),
		telemetry: NewTelemetryLogger(dataDir),
		adapter:   adapter,
		perms:     NewPermissionManager(TrustAuto, nil), // 默认 auto，调用方可覆盖
	}
}

// SetPermissionManager 由调用方注入信任策略与 approver
func (p *ToolPipeline) SetPermissionManager(pm *PermissionManager) {
	if pm != nil {
		p.perms = pm
	}
}

func (p *ToolPipeline) Permissions() *PermissionManager { return p.perms }

// RegisterExtraTools 注册额外的工具（如 overlay tools）
func (p *ToolPipeline) RegisterExtraTools(extra map[string]*ToolDef) {
	for name, def := range extra {
		p.tools[name] = def
	}
}

func (p *ToolPipeline) Snapshots() *SnapshotStore     { return p.snapshots }
func (p *ToolPipeline) Telemetry() *TelemetryLogger   { return p.telemetry }
func (p *ToolPipeline) GetTools() map[string]*ToolDef { return p.tools }

// Execute runs a tool through the full pipeline.
func (p *ToolPipeline) Execute(ctx context.Context, toolName string, params map[string]interface{}) *ToolResult {
	start := time.Now()

	// Step 1: Find tool
	tool, ok := p.tools[toolName]
	if !ok {
		return &ToolResult{Success: false, Message: fmt.Sprintf("未知工具: %s", toolName)}
	}

	var snapshotID string

	if tool.IsWriteOp {
		// Step 2a: Permission check (user approval for write ops)
		if p.perms != nil {
			permDecision := p.perms.Check(toolName, params)
			if permDecision == PermDeny {
				p.telemetry.Log(TelemetryEntry{
					Timestamp:  time.Now(),
					Tool:       toolName,
					Params:     params,
					Success:    false,
					DurationMs: time.Since(start).Milliseconds(),
					Error:      "permission denied by user",
				})
				return &ToolResult{Success: false, Message: "操作被用户拒绝。"}
			}
		}

		// Step 2b: Pre-Hooks
		decision := p.hooks.RunPreHooks(toolName, params)
		if decision.Action == "deny" {
			return &ToolResult{Success: false, Message: fmt.Sprintf("操作被拒绝: %s", decision.Reason)}
		}

		// Step 3: Auto snapshot before write (D2: tool 名记进 snapshot, Safety card 直接显示)
		var err error
		snapshotID, err = p.snapshots.Save(p.adapter, toolName)
		if err != nil {
			fmt.Printf("\033[31m[Snapshot] 保存失败: %v\033[0m\n", err)
		} else {
			fmt.Printf("\033[33m[Snapshot] 已保存: %s\033[0m\n", snapshotID)
		}
	}

	// Step 4: Execute tool
	result, execErr := tool.Execute(ctx, p.adapter, params)

	// Step 5: Failure handling
	if execErr != nil {
		// Auto rollback on write failure
		rolledBack := false
		if tool.IsWriteOp && snapshotID != "" {
			if rbErr := p.snapshots.Rollback(snapshotID, p.adapter); rbErr != nil {
				fmt.Printf("\033[31m[Rollback] 回滚失败 %s: %v (配置可能处于不一致状态)\033[0m\n", snapshotID, rbErr)
			} else {
				fmt.Printf("\033[31m[Rollback] 自动回滚到 %s\033[0m\n", snapshotID)
				rolledBack = true
			}
		}
		friendlyMsg := p.hooks.RunFailureHooks(toolName, params, execErr)

		// Step 8: Telemetry
		p.telemetry.Log(TelemetryEntry{
			Timestamp:  time.Now(),
			Tool:       toolName,
			Params:     params,
			Success:    false,
			DurationMs: time.Since(start).Milliseconds(),
			SnapshotID: snapshotID,
			RolledBack: rolledBack,
			Error:      execErr.Error(),
		})

		return &ToolResult{Success: false, Message: friendlyMsg}
	}

	// Step 6: Post-Hooks (write ops only — health check)
	if tool.IsWriteOp {
		health := p.hooks.RunPostHooks(toolName, params, result, p.adapter)

		// Step 7: Post-Hook failure → auto rollback
		if !health.OK && snapshotID != "" {
			rolledBack := false
			rbReason := health.Reason
			if rbErr := p.snapshots.Rollback(snapshotID, p.adapter); rbErr != nil {
				fmt.Printf("\033[31m[Rollback] 回滚失败 %s: %v (配置可能处于不一致状态)\033[0m\n", snapshotID, rbErr)
				rbReason = fmt.Sprintf("%s; 回滚也失败: %v", health.Reason, rbErr)
			} else {
				fmt.Printf("\033[31m[Rollback] 自动回滚到 %s\033[0m\n", snapshotID)
				rolledBack = true
			}

			p.telemetry.Log(TelemetryEntry{
				Timestamp:  time.Now(),
				Tool:       toolName,
				Params:     params,
				Success:    false,
				DurationMs: time.Since(start).Milliseconds(),
				SnapshotID: snapshotID,
				RolledBack: rolledBack,
				Error:      rbReason,
			})

			msg := fmt.Sprintf("切换失败，已自动回滚。原因: %s", health.Reason)
			if !rolledBack {
				msg = fmt.Sprintf("切换失败 + 回滚失败，配置可能处于不一致状态。原因: %s", rbReason)
			}
			return &ToolResult{
				Success: false,
				Message: msg,
			}
		}
	}

	// Step 8: Telemetry
	p.telemetry.Log(TelemetryEntry{
		Timestamp:  time.Now(),
		Tool:       toolName,
		Params:     params,
		Success:    true,
		DurationMs: time.Since(start).Milliseconds(),
		SnapshotID: snapshotID,
	})

	return result
}

// ManualSnapshot saves a snapshot on user request.
func (p *ToolPipeline) ManualSnapshot() *ToolResult {
	id, err := p.snapshots.Save(p.adapter, "manual")
	if err != nil {
		return &ToolResult{Success: false, Message: fmt.Sprintf("快照保存失败: %v", err)}
	}
	return &ToolResult{
		Success: true,
		Message: fmt.Sprintf("\033[32m已保存快照: %s\033[0m", id),
	}
}

// ManualRollback rolls back to a specific snapshot or the latest one.
func (p *ToolPipeline) ManualRollback(id string) *ToolResult {
	if id == "" {
		latest := p.snapshots.Latest()
		if latest == nil {
			return &ToolResult{Success: false, Message: "没有可用的快照。"}
		}
		id = latest.ID
	}

	if err := p.snapshots.Rollback(id, p.adapter); err != nil {
		return &ToolResult{Success: false, Message: fmt.Sprintf("回滚失败: %v", err)}
	}

	// Show current state after rollback
	snap := p.snapshots.Latest()
	detail := ""
	if snap != nil {
		for g, n := range snap.ActiveProxies {
			detail += fmt.Sprintf("\n  %s = %s", g, n)
		}
	}

	return &ToolResult{
		Success: true,
		Message: fmt.Sprintf("\033[32m已回滚到 %s\033[0m%s", id, detail),
	}
}
