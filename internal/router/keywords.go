package router

// KeywordMap maps action IDs to their trigger keywords (Chinese + English).
// Matching is done via substring containment on the normalized input.
var KeywordMap = map[string][]string{
	"switch_best_node": {
		"换节点", "换个节点", "切换节点", "节点不行了", "太慢了", "连不上",
		"帮我换", "自动选", "选个快的", "帮我找个快的", "找个快的",
		"switch node", "change node", "auto select",
	},
	"latency_test_all": {
		"测速", "测试速度", "哪个快", "测一下", "延迟测试",
		"test speed", "latency test", "which is fastest", "ping all",
	},
	"set_mode_global": {
		"全局模式", "全局代理", "全部走代理",
		"global mode", "proxy all",
	},
	"set_mode_direct": {
		"直连模式", "直连", "关闭代理", "不用代理",
		"direct mode", "no proxy",
	},
	"set_mode_rule": {
		"规则模式", "自动分流", "智能模式",
		"rule mode", "auto route",
	},
	"show_status": {
		"状态", "当前状态", "什么情况", "现在怎么样",
		"status", "current status",
	},
	"show_nodes": {
		"节点列表", "看看节点", "有哪些节点", "所有节点",
		"list nodes", "show nodes",
	},
	"show_snapshots": {
		"快照", "快照列表",
		"snapshots", "list snapshots",
	},
	"do_rollback": {
		"回滚", "撤销", "恢复",
		"rollback", "undo",
	},
	"show_telemetry": {
		"操作日志", "历史记录",
		"telemetry", "history",
	},
	"apply_template": {
		"配netflix", "netflix分流", "奈飞分流", "网飞分流",
		"配google", "谷歌分流",
		"社交分流", "twitter分流", "推特分流",
		"ai分流", "chatgpt分流",
		"流媒体分流", "配分流",
		"apply template",
	},
	"list_templates": {
		"有哪些模板", "模板列表", "分流模板",
		"templates", "list templates",
	},
	"list_rules": {
		"规则列表", "当前规则", "看看规则",
		"rules", "list rules",
	},
	"remove_rule": {
		"删除规则", "移除规则",
		"remove rule", "delete rule",
	},
	"import_subscription": {
		"导入订阅", "添加订阅", "订阅链接",
		"import subscription", "add subscription",
	},
	"clear_history": {
		"清空对话", "新对话", "重新开始", "/clear",
	},
	"show_history": {
		"对话记录", "聊天记录", "/history",
	},
}
