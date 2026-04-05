package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// PromptAssembler 动态组装 system prompt
type PromptAssembler struct {
	adapter engine.EngineAdapter
}

func NewPromptAssembler(adapter engine.EngineAdapter) *PromptAssembler {
	return &PromptAssembler{adapter: adapter}
}

const staticPrefix = `你是 NetPilot 的网络管理 Agent。你通过调用工具来帮助用户管理网络代理。

核心规则：
1. 你只能通过提供的工具操作网络，不能直接输出配置代码让用户手动执行
2. 修改操作前先了解当前状态（调用读工具）
3. 切换节点前先测延迟，选最优的
4. 如果操作失败，分析原因并告诉用户，不要重复尝试超过 3 次
5. 用简洁的中文回复用户

你可用的工具：
- get_node_pool: 获取所有可用节点和分组
- test_latency: 测试指定节点延迟（参数: tag）
- test_latency_all: 测试所有节点延迟
- switch_node: 切换活跃节点（参数: group, node）
- set_mode: 切换代理模式（参数: mode，可选值: global/direct/rule）
- get_connections: 获取当前活跃连接
- get_logs: 获取 sing-box 日志（参数: level，可选）
- patch_route_rule: 添加路由规则，将匹配的流量导向指定出站（参数: tag, outbound, domain_suffix/domain）
- remove_route_rule: 删除 Agent 添加的路由规则（参数: tag）
- list_route_rules: 列出所有 Agent 添加的路由规则`

// Assemble 组装完整的 system prompt = 静态前缀 + 动态网络状态
func (pa *PromptAssembler) Assemble(ctx context.Context) string {
	var b strings.Builder
	b.WriteString(staticPrefix)

	// 尝试获取动态网络状态，失败则跳过（sing-box 可能未运行）
	dynamic := pa.buildDynamicSuffix()
	if dynamic != "" {
		b.WriteString("\n\n")
		b.WriteString(dynamic)
	}

	return b.String()
}

func (pa *PromptAssembler) buildDynamicSuffix() string {
	var parts []string

	// 获取当前代理组状态
	group, err := pa.adapter.GetProxyGroup("proxy-group")
	if err != nil {
		return "" // sing-box 未运行或不可达，跳过动态部分
	}

	parts = append(parts, "## 当前网络状态")

	// 活跃节点
	if group.Now != "" {
		parts = append(parts, fmt.Sprintf("- 活跃节点: %s", group.Now))
	}

	// 可用节点数
	parts = append(parts, fmt.Sprintf("- 可用节点数: %d", len(group.All)))

	// 活跃连接数
	conns, err := pa.adapter.GetConnections()
	if err == nil {
		parts = append(parts, fmt.Sprintf("- 活跃连接数: %d", len(conns)))
	}

	return strings.Join(parts, "\n")
}
