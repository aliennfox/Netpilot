package tool

import (
	"context"
	"fmt"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
	"github.com/foxnetpilot/netpilot/internal/subscription"
)

// RegisterSubscriptionTools 注册订阅相关工具
func RegisterSubscriptionTools(ov *overlay.ConfigOverlay) map[string]*ToolDef {
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

				fmt.Println("正在下载订阅...")
				nodes, err := subscription.FetchAndParse(url)
				if err != nil {
					return &ToolResult{Success: false, Message: fmt.Sprintf("订阅解析失败: %v", err)}, nil
				}

				// 转换为 sing-box outbound 格式
				var outbounds []map[string]interface{}
				var skipped int
				tagCount := map[string]int{}
				for _, node := range nodes {
					ob, err := subscription.ConvertToSingboxOutbound(node)
					if err != nil {
						fmt.Printf("  ⚠️  跳过节点 %s: %v\n", node.Name, err)
						skipped++
						continue
					}
					// 处理 tag 重复
					tag, _ := ob["tag"].(string)
					tagCount[tag]++
					if tagCount[tag] > 1 {
						ob["tag"] = fmt.Sprintf("%s-%d", tag, tagCount[tag])
					}
					outbounds = append(outbounds, ob)
				}

				if len(outbounds) == 0 {
					return &ToolResult{Success: false, Message: "没有有效的节点可导入"}, nil
				}

				// 批量写入 overlay
				if err := ov.AddOutboundsBatch(outbounds); err != nil {
					return &ToolResult{Success: false, Message: fmt.Sprintf("写入 overlay 失败: %v", err)}, nil
				}

				// 合并配置并重载
				if err := ov.Apply(adapter); err != nil {
					return &ToolResult{Success: false, Message: fmt.Sprintf("应用配置失败: %v", err)}, nil
				}

				summary := subscription.SummarizeNodes(nodes)
				msg := fmt.Sprintf("成功导入 %d 个节点 (%s)", len(outbounds), summary)
				if skipped > 0 {
					msg += fmt.Sprintf("，跳过 %d 个无效节点", skipped)
				}
				msg += "\n已合并配置，sing-box 已重载。"

				return &ToolResult{Success: true, Message: msg}, nil
			},
		},
	}
}
