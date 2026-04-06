package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/foxnetpilot/netpilot/internal/tool"
)

// Orchestrator 协调多角色 Agent 完成任务。
// 根据 ClassifyTask 的结果决定走哪些角色，按 Diagnose → Configure → Verify 顺序执行。
type Orchestrator struct {
	agent   *SingleAgent
	pipeline *tool.ToolPipeline
}

func NewOrchestrator(llm *LLMClient, pipeline *tool.ToolPipeline, assembler *PromptAssembler, tools map[string]*tool.ToolDef) *Orchestrator {
	// 每个角色最多 5 次迭代
	agent := NewSingleAgent(llm, pipeline, assembler, tools, 5)
	return &Orchestrator{
		agent:   agent,
		pipeline: pipeline,
	}
}

// Run 执行编排：分类 → 按需走角色流水线
func (o *Orchestrator) Run(ctx context.Context, userMessage string) (string, error) {
	plan := ClassifyTask(userMessage)

	fmt.Printf("\033[90m[分类: %s]\033[0m\n", plan.Reason)

	// 快速路径：只需要一个角色
	if plan.NeedDiagnose && !plan.NeedConfigure && !plan.NeedVerify {
		return o.runDiagnoseOnly(ctx, userMessage)
	}
	if plan.NeedConfigure && !plan.NeedDiagnose && !plan.NeedVerify {
		return o.runConfigureOnly(ctx, userMessage)
	}

	// 多角色流水线
	var diagnosis, configResult, verification string
	var err error

	// Diagnose 阶段
	if plan.NeedDiagnose {
		fmt.Print("\033[36m🔍 [诊断中...]\033[0m\n")
		diagnosis, err = o.agent.RunWithRole(ctx, RoleDiagnose, userMessage)
		if err != nil {
			return "", fmt.Errorf("诊断阶段失败: %w", err)
		}
		fmt.Printf("\033[36m🔍 诊断完成\033[0m\n")
	}

	// Configure 阶段
	if plan.NeedConfigure {
		fmt.Print("\033[36m🔧 [配置中...]\033[0m\n")

		configInput := userMessage
		if diagnosis != "" {
			configInput = fmt.Sprintf("用户请求: %s\n\n诊断报告:\n%s", userMessage, diagnosis)
		}

		configResult, err = o.agent.RunWithRole(ctx, RoleConfigure, configInput)
		if err != nil {
			return "", fmt.Errorf("配置阶段失败: %w", err)
		}
		fmt.Printf("\033[36m🔧 配置完成\033[0m\n")
	}

	// Verify 阶段（只要 plan 需要验证就跑，不管 configResult 是否为空）
	if plan.NeedVerify {
		fmt.Print("\033[36m✅ [验证中...]\033[0m\n")

		verifyInput := "请验证最近的配置操作是否成功。检查节点状态、路由规则和日志。"
		if configResult != "" {
			verifyInput = fmt.Sprintf("请验证以下配置操作是否成功:\n%s", configResult)
		}
		verification, err = o.agent.RunWithRole(ctx, RoleVerify, verifyInput)
		if err != nil {
			return "", fmt.Errorf("验证阶段失败: %w", err)
		}

		// 验证失败 → 自动回滚
		if isVerificationFailed(verification) {
			rollbackResult := o.pipeline.Execute(ctx, "rollback", nil)
			if rollbackResult.Success {
				verification += "\n\n⚠️ 验证失败，已自动回滚到之前状态。"
			}
		}
		fmt.Printf("\033[36m✅ 验证完成\033[0m\n")
	}

	return formatFinalResponse(plan, diagnosis, configResult, verification), nil
}

// runDiagnoseOnly 快速路径：只跑诊断
func (o *Orchestrator) runDiagnoseOnly(ctx context.Context, userMessage string) (string, error) {
	fmt.Print("\033[36m🔍 [诊断中...]\033[0m\n")
	result, err := o.agent.RunWithRole(ctx, RoleDiagnose, userMessage)
	if err != nil {
		return "", err
	}
	return result, nil
}

// runConfigureOnly 快速路径：只跑配置（无验证）
func (o *Orchestrator) runConfigureOnly(ctx context.Context, userMessage string) (string, error) {
	fmt.Print("\033[36m🔧 [配置中...]\033[0m\n")
	result, err := o.agent.RunWithRole(ctx, RoleConfigure, userMessage)
	if err != nil {
		return "", err
	}
	return result, nil
}

// isVerificationFailed 判断验证结果是否包含失败标记
func isVerificationFailed(verification string) bool {
	lower := strings.ToLower(verification)
	failKeywords := []string{
		"验证失败", "失败", "未生效", "不成功",
		"verification failed", "failed",
		"未通过", "不通过",
	}
	passKeywords := []string{
		"验证通过", "通过", "成功", "已生效",
		"verification passed", "passed",
	}

	hasPass := false
	hasFail := false
	for _, kw := range failKeywords {
		if strings.Contains(lower, kw) {
			hasFail = true
		}
	}
	for _, kw := range passKeywords {
		if strings.Contains(lower, kw) {
			hasPass = true
		}
	}

	// 如果同时包含通过和失败，以失败为准（保守策略）
	if hasFail && !hasPass {
		return true
	}
	return false
}

// formatFinalResponse 组装多阶段最终回复
func formatFinalResponse(plan TaskPlan, diagnosis, configResult, verification string) string {
	var parts []string

	if diagnosis != "" {
		parts = append(parts, diagnosis)
	}
	if configResult != "" {
		parts = append(parts, configResult)
	}
	if verification != "" {
		parts = append(parts, verification)
	}

	if len(parts) == 0 {
		return "操作完成，无额外输出。"
	}

	// 单阶段直接返回
	if len(parts) == 1 {
		return parts[0]
	}

	// 多阶段用分隔符拼接
	var result strings.Builder
	if diagnosis != "" {
		result.WriteString("📋 诊断报告:\n")
		result.WriteString(diagnosis)
		result.WriteString("\n\n")
	}
	if configResult != "" {
		result.WriteString("🔧 配置结果:\n")
		result.WriteString(configResult)
		result.WriteString("\n\n")
	}
	if verification != "" {
		result.WriteString("✅ 验证结果:\n")
		result.WriteString(verification)
	}

	return strings.TrimSpace(result.String())
}
