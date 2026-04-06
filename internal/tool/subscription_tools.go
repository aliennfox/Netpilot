package tool

import (
	"context"
	"fmt"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/subscription"
)

// RegisterSubscriptionTools 注册订阅相关工具
func RegisterSubscriptionTools(mgr *subscription.SubscriptionManager) map[string]*ToolDef {
	return map[string]*ToolDef{
		"import_subscription": {
			Name:        "import_subscription",
			Description: "从订阅链接导入代理节点",
			IsWriteOp:   true,
			Execute: func(ctx context.Context, adapter engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
				url, _ := params["url"].(string)
				if url == "" {
					return &ToolResult{Success: false, Message: "缺少 url 参数"}, nil
				}
				name, _ := params["name"].(string)

				count, summary, err := mgr.AddSubscription(name, url)
				if err != nil {
					return &ToolResult{Success: false, Message: fmt.Sprintf("%v", err)}, nil
				}
				_ = count
				return &ToolResult{Success: true, Message: summary}, nil
			},
		},
		"list_subscriptions": {
			Name:        "list_subscriptions",
			Description: "列出所有已保存的订阅",
			IsWriteOp:   false,
			Execute: func(ctx context.Context, adapter engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
				subs := mgr.Store().List()
				if len(subs) == 0 {
					return &ToolResult{Success: true, Message: "没有已保存的订阅。使用 import <URL> 导入订阅。"}, nil
				}
				out := ""
				for _, s := range subs {
					ago := formatTimeAgo(s.LastUpdate)
					autoStr := fmt.Sprintf("每%d分钟", s.Interval)
					if !s.AutoUpdate {
						autoStr = "关闭"
					}
					out += fmt.Sprintf("  \033[36m%-8s\033[0m %-10s %d个节点  更新于 %s  自动更新: %s\n",
						s.ID, s.Name, s.NodeCount, ago, autoStr)
				}
				return &ToolResult{Success: true, Message: out}, nil
			},
		},
		"update_subscription": {
			Name:        "update_subscription",
			Description: "更新订阅（重新下载并导入节点）",
			IsWriteOp:   true,
			Execute: func(ctx context.Context, adapter engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
				id, _ := params["id"].(string)
				if id != "" {
					_, summary, err := mgr.UpdateSubscription(id)
					if err != nil {
						return &ToolResult{Success: false, Message: fmt.Sprintf("更新失败: %v", err)}, nil
					}
					return &ToolResult{Success: true, Message: summary}, nil
				}
				// 更新全部
				result := mgr.UpdateAll()
				return &ToolResult{Success: true, Message: result}, nil
			},
		},
		"remove_subscription": {
			Name:        "remove_subscription",
			Description: "删除订阅及其所有节点",
			IsWriteOp:   true,
			Execute: func(ctx context.Context, adapter engine.EngineAdapter, params map[string]interface{}) (*ToolResult, error) {
				id, _ := params["id"].(string)
				if id == "" {
					// 列出可删除的订阅
					subs := mgr.Store().List()
					if len(subs) == 0 {
						return &ToolResult{Success: true, Message: "没有可删除的订阅。"}, nil
					}
					out := "请指定要删除的订阅 ID:\n"
					for _, s := range subs {
						out += fmt.Sprintf("  %s  %s  (%d个节点)\n", s.ID, s.Name, s.NodeCount)
					}
					return &ToolResult{Success: true, Message: out}, nil
				}
				msg, err := mgr.RemoveSubscription(id)
				if err != nil {
					return &ToolResult{Success: false, Message: fmt.Sprintf("%v", err)}, nil
				}
				return &ToolResult{Success: true, Message: msg}, nil
			},
		},
	}
}

// formatTimeAgo 格式化时间差
func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "从未"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d小时前", int(d.Hours()))
	default:
		return fmt.Sprintf("%d天前", int(d.Hours()/24))
	}
}
