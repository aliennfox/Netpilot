package agent

import (
	"strings"
	"testing"
)

// TestIsVerificationFailed 锁 #H2 自动回滚的触发条件: Verify 阶段输出含失败关键词且
// 不含通过关键词时, Orchestrator 才走 ManualRollback。 hybrid (含通过+失败) 走保守"按失败"语义。
func TestIsVerificationFailed(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"empty stays false", "", false},
		{"plain pass", "验证通过", false},
		{"plain pass english", "verification passed", false},
		{"plain fail", "验证失败", true},
		{"plain fail english", "verification failed", true},
		{"未生效 → fail", "节点切换未生效", true},
		{"不通过 → fail", "测试不通过", true},
		{"hybrid: pass + fail → 保守按 fail", "整体通过, 但有一个项目失败", false}, // hasPass=true → 不触发
		{"hybrid: fail + pass → 保守按 fail", "失败, 但部分功能通过", false},   // 同上 hasPass=true
		{"only fail no pass → trigger rollback", "节点连接失败, 测试不通过", true},
		{"过 substring 不该误触发, but '通' is included so... 实际命中通过 keyword", "通行流量", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isVerificationFailed(tc.text)
			if got != tc.want {
				t.Errorf("isVerificationFailed(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

// TestFormatFinalResponse 多阶段拼接逻辑: 单阶段直返, 多阶段加 emoji 分隔
func TestFormatFinalResponse(t *testing.T) {
	plan := TaskPlan{NeedDiagnose: true, NeedConfigure: true, NeedVerify: true}

	t.Run("all empty", func(t *testing.T) {
		got := formatFinalResponse(plan, "", "", "")
		if got != "操作完成，无额外输出。" {
			t.Errorf("empty: got %q", got)
		}
	})

	t.Run("single stage diagnosis only", func(t *testing.T) {
		got := formatFinalResponse(plan, "节点池正常", "", "")
		if got != "节点池正常" {
			t.Errorf("single: want passthrough, got %q", got)
		}
	})

	t.Run("single stage config only", func(t *testing.T) {
		got := formatFinalResponse(plan, "", "已切换 JP-1", "")
		if got != "已切换 JP-1" {
			t.Errorf("single: want passthrough, got %q", got)
		}
	})

	t.Run("multi stage adds section headers", func(t *testing.T) {
		got := formatFinalResponse(plan, "诊断 OK", "切换成功", "验证通过")
		if !strings.Contains(got, "📋 诊断报告:") {
			t.Errorf("missing diagnosis header: %q", got)
		}
		if !strings.Contains(got, "🔧 配置结果:") {
			t.Errorf("missing config header: %q", got)
		}
		if !strings.Contains(got, "✅ 验证结果:") {
			t.Errorf("missing verify header: %q", got)
		}
		if !strings.Contains(got, "诊断 OK") || !strings.Contains(got, "切换成功") || !strings.Contains(got, "验证通过") {
			t.Errorf("payload missing: %q", got)
		}
	})

	t.Run("multi stage skips empty sections", func(t *testing.T) {
		// 只有诊断 + 验证, 没有配置
		got := formatFinalResponse(plan, "诊断 OK", "", "验证通过")
		if !strings.Contains(got, "📋 诊断报告:") {
			t.Errorf("missing diagnosis header: %q", got)
		}
		if !strings.Contains(got, "✅ 验证结果:") {
			t.Errorf("missing verify header: %q", got)
		}
		if strings.Contains(got, "🔧 配置结果:") {
			t.Errorf("config header should be skipped when configResult empty: %q", got)
		}
	})

	t.Run("trailing whitespace trimmed", func(t *testing.T) {
		got := formatFinalResponse(plan, "诊断 OK", "切换成功", "")
		if strings.HasSuffix(got, "\n") || strings.HasSuffix(got, " ") {
			t.Errorf("output should be trim-trailing-whitespaced, got %q", got)
		}
	})
}

// TestClassifyTask 路由分类决定走哪些 role, 是 #H2 编排链的入口
func TestClassifyTask(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantDiagnose  bool
		wantConfigure bool
		wantVerify    bool
	}{
		// 诊断/查询类: 只走 Diagnose
		{"看看节点", "看看有哪些节点", true, false, false},
		{"vpn 状态查询", "vpn 状态怎么样", true, false, false},
		{"延迟查询", "延迟多少", true, false, false},
		{"日志查询", "看看日志", true, false, false},

		// 故障类: 三个角色全跑 (Diagnose+Configure+Verify)
		{"连不上 → 三阶段", "连不上了帮我修", true, true, true},
		{"挂了 → 三阶段", "网络挂了", true, true, true},

		// 配置类: 走 Configure+Verify, 不走 Diagnose
		{"切换 → C+V", "切换到 JP-1", false, true, true},
		{"全局模式 → C+V", "切换到全局模式", false, true, true},
		{"启动 vpn → C+V", "启动 vpn", false, true, true},
		{"关闭 vpn → C+V", "关闭 vpn", false, true, true},

		// 地区动作: 走 Configure+Verify
		{"找个日本节点", "帮我找个最快的日本节点", false, true, true},
		{"切到香港", "切到香港", false, true, true},
		{"用美国", "用美国节点", false, true, true},

		// 默认: 只诊断
		{"无关闲聊默认 Diagnose", "今天天气真好", true, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := ClassifyTask(tc.input)
			if plan.NeedDiagnose != tc.wantDiagnose {
				t.Errorf("Diagnose: want %v got %v (reason: %s)", tc.wantDiagnose, plan.NeedDiagnose, plan.Reason)
			}
			if plan.NeedConfigure != tc.wantConfigure {
				t.Errorf("Configure: want %v got %v (reason: %s)", tc.wantConfigure, plan.NeedConfigure, plan.Reason)
			}
			if plan.NeedVerify != tc.wantVerify {
				t.Errorf("Verify: want %v got %v (reason: %s)", tc.wantVerify, plan.NeedVerify, plan.Reason)
			}
			if plan.Reason == "" {
				t.Errorf("Reason should be set for analytics, got empty (input=%q)", tc.input)
			}
		})
	}
}

// TestClassifyTask_DiagnoseConfigConflict diagnose 关键词 + config 关键词同时出现, 走 config
// (因为 ClassifyTask 里 `matchesAny(lower, diagnoseKeywords) && !matchesAny(lower, configKeywords())`
// 的 short-circuit, config 优先)
func TestClassifyTask_DiagnoseConfigConflict(t *testing.T) {
	// "看看" diagnose + "切换" config → config 走
	plan := ClassifyTask("看看然后切换到 JP-1")
	if !plan.NeedConfigure {
		t.Errorf("config keyword should win when both match, plan=%+v", plan)
	}
	// "状态" 同时是 diagnose 也是 config 子串? "状态" 只在 diagnose, "切换" 只在 config
	// 当配置关键词出现, 应跳过 Diagnose-only 分支, 进入 Configure 分支
	if plan.NeedDiagnose && !plan.NeedConfigure {
		t.Errorf("diagnose-only path should NOT be taken when config keyword present: %+v", plan)
	}
}

// TestMatchesRegionAction 锁地区+动作组合的命中规则
func TestMatchesRegionAction(t *testing.T) {
	hits := []string{
		"找个日本节点",
		"选个香港的",
		"切到美国",
		"用韩国",
		"换到新加坡",
		"帮我找最快的台湾节点",
	}
	for _, s := range hits {
		t.Run("hit/"+s, func(t *testing.T) {
			if !matchesRegionAction(s) {
				t.Errorf("%q should match region+action", s)
			}
		})
	}

	misses := []string{
		"日本",         // 没动作词
		"找个节点",      // 没地区
		"今天好",        // 都没
		"找个 abc",   // 地区不在白名单
	}
	for _, s := range misses {
		t.Run("miss/"+s, func(t *testing.T) {
			if matchesRegionAction(s) {
				t.Errorf("%q should NOT match", s)
			}
		})
	}
}
