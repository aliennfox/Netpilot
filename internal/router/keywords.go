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
	"list_subscriptions": {
		"订阅列表", "我的订阅", "有哪些订阅", "所有订阅",
		"subs", "list subscriptions",
	},
	"update_subscription": {
		"更新订阅", "刷新订阅",
		"update subscription", "refresh subscription",
	},
	"remove_subscription": {
		"删除订阅", "移除订阅",
		"remove subscription", "delete subscription",
	},
	"show_dns": {
		"DNS状态", "DNS配置", "当前DNS",
		"dns status", "dns config",
	},
	"set_dns_mode": {
		"DNS防泄露", "DNS模式", "DNS安全", "切换DNS",
		"dns leak", "dns protect", "dns mode",
	},
	"live_connections": {
		"实时连接", "连接监控", "实时流量",
		"live", "live connections",
	},
	"show_connections": {
		"当前连接", "连接列表", "活跃连接",
		"connections", "active connections",
	},
	"clear_history": {
		"清空对话", "新对话", "重新开始", "/clear",
	},
	"show_history": {
		"对话记录", "聊天记录", "/history",
	},

	// Phase 10-D: VPN 生命周期的本地快捷路由, 不经 LLM / Clash API。
	// 重要: LLM API 本身需要 VPN 才能访问 (硅基流动在墙外), 所以 "开启 vpn" 必须本地闭环,
	// 否则会撞 "Chat 叫它开 VPN 做不到,但又因为没开 VPN 根本联不上 LLM" 的鸡生蛋循环。
	// 关键词覆盖带/不带空格两种写法 (substring 匹配) 和 中英双语。
	"start_vpn": {
		"开启vpn", "开启 vpn", "打开vpn", "打开 vpn", "启动vpn", "启动 vpn",
		"开 vpn", "开vpn", "连vpn", "连 vpn", "连接vpn", "连接 vpn",
		"turn on vpn", "start vpn", "enable vpn", "开启VPN", "打开VPN", "启动VPN",
	},
	"stop_vpn": {
		"关闭vpn", "关闭 vpn", "停止vpn", "停止 vpn", "断开vpn", "断开 vpn",
		"关vpn", "关 vpn", "停vpn", "停 vpn", "关掉vpn", "关掉 vpn",
		"turn off vpn", "stop vpn", "disable vpn", "disconnect vpn",
	},
	"vpn_status": {
		// 带 vpn/翻墙/代理 + 询问词缀的各种口语表述。 substring 匹配, 短语要够独特不撞 start_vpn
		"vpn状态", "vpn 状态", "vpn在吗", "vpn 在吗", "vpn运行", "vpn 运行",
		"vpn开着", "vpn 开着", "vpn开没开", "vpn 开没开", "vpn开了吗", "vpn 开了吗",
		"vpn现在开", "vpn 现在开", "vpn是不是", "vpn 是不是", "vpn好了吗", "vpn 好了吗",
		"翻墙开着", "翻墙 开着", "翻墙开没", "翻墙 开没", "翻墙在吗", "翻墙 在吗",
		"翻墙好了吗", "翻墙好没", "翻墙开了", "翻墙现在", "翻墙是不是",
		"代理开着", "代理 开着", "代理开没", "代理 开没", "代理启用", "代理 启用",
		"代理在跑", "代理在吗", "代理现在", "代理是不是",
		"vpn running", "vpn status", "is vpn on", "is vpn running",
	},
}
