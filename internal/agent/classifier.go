package agent

import "strings"

// TaskPlan 描述一个用户请求需要哪些角色参与
type TaskPlan struct {
	NeedDiagnose  bool
	NeedConfigure bool
	NeedVerify    bool
	Reason        string
}

// ClassifyTask 用关键词判断用户请求需要哪些角色（不调 LLM）
func ClassifyTask(input string) TaskPlan {
	lower := strings.ToLower(input)

	// 诊断/查询类：只走 Diagnose
	diagnoseKeywords := []string{
		"正常吗", "有没有问题", "检查", "诊断", "分析",
		"怎么回事", "什么情况", "什么状态",
		"有哪些节点", "当前状态", "延迟多少", "看看",
		"连接数", "日志", "状态",
		// 查询类关键词
		"可用", "代理节点", "列出", "显示",
		"看看有什么", "有什么", "有哪些",
		// Phase 8: VPN 只读查询
		"vpn 状态", "vpn状态", "vpn 运行", "vpn 在吗", "vpn running", "vpn status",
	}
	if matchesAny(lower, diagnoseKeywords) && !matchesAny(lower, configKeywords()) {
		return TaskPlan{
			NeedDiagnose:  true,
			NeedConfigure: false,
			NeedVerify:    false,
			Reason:        "诊断/查询类请求",
		}
	}

	// 故障修复类：走全部三个角色
	troubleshootKeywords := []string{
		"连不上", "断了", "不行了", "修复", "帮我解决",
		"帮我修", "挂了", "不能用", "有问题",
		"好像有点慢", "太慢了", "网络不好",
	}
	if matchesAny(lower, troubleshootKeywords) {
		return TaskPlan{
			NeedDiagnose:  true,
			NeedConfigure: true,
			NeedVerify:    true,
			Reason:        "故障修复类请求",
		}
	}

	// 配置类：走 Configure + Verify
	if matchesAny(lower, configKeywords()) {
		return TaskPlan{
			NeedDiagnose:  false,
			NeedConfigure: true,
			NeedVerify:    true,
			Reason:        "配置变更类请求",
		}
	}

	// "找个/选个/切到/用" + 地区名 → 配置变更
	if matchesRegionAction(lower) {
		return TaskPlan{
			NeedDiagnose:  false,
			NeedConfigure: true,
			NeedVerify:    true,
			Reason:        "地区节点选择请求",
		}
	}

	// 无法分类：默认只走 Diagnose（只读、最短路径）。
	// 之前默认走全 3 阶段, 对聊天类/含糊查询产出 3 倍长的无关输出;
	// 用户反馈 "回答太长" 后改为只读单阶段, 若用户确实想改配置会用明确动作词命中 Configure 路径。
	return TaskPlan{
		NeedDiagnose:  true,
		NeedConfigure: false,
		NeedVerify:    false,
		Reason:        "未分类，默认只诊断（最短路径）",
	}
}

func configKeywords() []string {
	return []string{
		"让", "走", "配置", "设置", "切换到", "添加规则",
		"改成", "换成", "删除规则", "移除规则",
		"全局模式", "直连模式", "规则模式",
		"switch", "set", "patch", "remove",
		// Phase 8: VPN 生命周期动作 (命中后走 Configure+Verify)
		"打开", "启动", "开启", "连接", "连上",
		"关闭", "停止", "关掉", "断开",
		"turn on", "turn off", "start vpn", "stop vpn",
		"connect", "disconnect", "enable", "disable",
	}
}

func matchesAny(input string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(input, kw) {
			return true
		}
	}
	return false
}

// matchesRegionAction 检测"动作词+地区名"组合，如"帮我找个最快的日本节点"
func matchesRegionAction(input string) bool {
	actions := []string{"找个", "选个", "切到", "用", "换到", "帮我找", "帮我选"}
	regions := []string{
		"日本", "香港", "美国", "韩国", "新加坡", "英国", "德国",
		"台湾", "法国", "巴西", "土耳其", "越南", "泰国", "俄罗斯", "马来西亚",
	}
	hasAction := false
	for _, a := range actions {
		if strings.Contains(input, a) {
			hasAction = true
			break
		}
	}
	if !hasAction {
		return false
	}
	for _, r := range regions {
		if strings.Contains(input, r) {
			return true
		}
	}
	return false
}
