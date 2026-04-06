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

	// 诊断类：只走 Diagnose
	diagnoseKeywords := []string{
		"正常吗", "有没有问题", "检查", "诊断", "分析",
		"怎么回事", "什么情况", "什么状态",
		"有哪些节点", "当前状态", "延迟多少", "看看",
		"连接数", "日志", "状态",
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

	// 无法分类：走全部三个（保险）
	return TaskPlan{
		NeedDiagnose:  true,
		NeedConfigure: true,
		NeedVerify:    true,
		Reason:        "未分类，走完整流程",
	}
}

func configKeywords() []string {
	return []string{
		"让", "走", "配置", "设置", "切换到", "添加规则",
		"改成", "换成", "删除规则", "移除规则",
		"全局模式", "直连模式", "规则模式",
		"switch", "set", "patch", "remove",
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
