package agent

// AgentRole 定义 Agent 角色约束：专用 prompt + Tool 白名单
type AgentRole struct {
	Name         string   // "diagnose", "configure", "verify"
	Description  string   // 人类可读描述
	SystemPrompt string   // 角色专用 system prompt（会和动态网络状态拼接）
	AllowedTools []string // Tool 白名单
}

// Prompt 设计原则 (2026-04-23 Boris 方法论): 负面约束 > 正面规定, 每 role ≤10 行。
// 不列 tool schema —— function_call Tools 字段 LLM 原生看到, 重复浪费 context。
// 不给 "好的回复长这样" 示例 —— 让 LLM 自己判断, 只砍反模式。
var (
	// RoleDiagnose — 只读，分析网络状态和问题
	RoleDiagnose = &AgentRole{
		Name:        "diagnose",
		Description: "网络诊断专家（只读）",
		SystemPrompt: `你是资深网络运维工程师。 只读权限, 通过 function_call 拉数据。

禁止:
- 开场白 (让我.../首先.../以下是...) 、道歉、鼓励话术
- markdown 标题 (# ##)、表情符号
- 画蛇添足: 用户问具体事实时不主动加建议, 用户问列表时不只给摘要
- 重复同一意思、补充用户没问的旁路信息
- 改写 tool 返回的列表/表格排版 —— 已经对齐好了, 原样透传`,
		AllowedTools: []string{
			"get_node_pool",
			"test_latency",
			"test_latency_all",
			"get_connections",
			"get_logs",
			"list_route_rules",
			"list_subscriptions",
			"get_dns_config",
			"get_per_app_vpn",
			"vpn_status",
		},
	}

	// RoleConfigure — 可写，执行配置变更
	RoleConfigure = &AgentRole{
		Name:        "configure",
		Description: "配置工程师（可读写）",
		SystemPrompt: `你是资深网络运维工程师。 有写权限, 通过 tool_calls 字段改配置 ——
在文本里写 "我将调用 xxx" 不会执行任何操作, 必须生成 tool_calls。

禁止:
- "好的/稍等/已为您..." 话术、道歉
- markdown 标题、表情符号
- 步骤清单 (除非用户要求)
- 未被要求的建议或旁路信息
- 改写 tool 返回的列表/表格排版 —— 已经对齐好了, 原样透传
- 把 set_mode 当万金油 —— 它只管 global/direct/rule 三选, "测速 / 隐蔽 / 防泄露 / 不想被看到" 都不是它能解决的
- 对用户的 "帮我 配/搭/建/加 X" 反问 "请确认 / 请告诉我 / 需要明确" —— 这类措辞已是授权, 直接生成 tool_calls`,
		AllowedTools: []string{
			"start_vpn",
			"stop_vpn",
			"vpn_status",
			"switch_node",
			"set_mode",
			"patch_route_rule",
			"remove_route_rule",
			"create_chain",
			"get_node_pool",
			"test_latency",
			"test_latency_all",
			"import_subscription",
			"list_subscriptions",
			"update_subscription",
			"remove_subscription",
			"get_dns_config",
			"set_per_app_vpn",
			"get_per_app_vpn",
		},
	}

	// RoleVerify — 只读，验证变更结果
	RoleVerify = &AgentRole{
		Name:        "verify",
		Description: "验证专家（只读）",
		SystemPrompt: `你是资深网络运维工程师。 只读, 验证上一步变更是否生效。

禁止:
- 开头不是 "通过" 或 "失败"
- 超过 2 行 (失败时可加 1 行建议)
- markdown 标题、表情符号、鼓励话术`,
		AllowedTools: []string{
			"get_node_pool",
			"test_latency",
			"test_latency_all",
			"get_connections",
			"get_logs",
			"list_route_rules",
			"get_dns_config",
			"get_per_app_vpn",
			"vpn_status",
		},
	}

	// AllRoles 角色注册表
	AllRoles = map[string]*AgentRole{
		"diagnose":  RoleDiagnose,
		"configure": RoleConfigure,
		"verify":    RoleVerify,
	}
)
