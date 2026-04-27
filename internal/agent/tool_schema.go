package agent

import (
	"encoding/json"

	"github.com/foxnetpilot/netpilot/internal/tool"
)

// Tool 是 OpenAI function calling 的 tools 参数格式
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// 每个 tool 的 parameters JSON Schema
// 不暴露 snapshot 和 rollback — 由 Pipeline 自动管理
var toolParameterSchemas = map[string]json.RawMessage{
	"get_node_pool": json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
	"get_connections": json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
	"get_logs": json.RawMessage(`{
		"type": "object",
		"properties": {
			"level": {
				"type": "string",
				"description": "日志级别过滤，如 error、warning、info"
			}
		},
		"required": []
	}`),
	"test_latency": json.RawMessage(`{
		"type": "object",
		"properties": {
			"tag": {
				"type": "string",
				"description": "要测试的节点名称"
			}
		},
		"required": ["tag"]
	}`),
	"test_latency_all": json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
	"switch_node": json.RawMessage(`{
		"type": "object",
		"properties": {
			"group": {
				"type": "string",
				"description": "代理分组名称，如 proxy-group"
			},
			"node": {
				"type": "string",
				"description": "目标节点名称"
			}
		},
		"required": ["group", "node"]
	}`),
	"set_mode": json.RawMessage(`{
		"type": "object",
		"properties": {
			"mode": {
				"type": "string",
				"description": "代理模式: global（全局代理）、direct（直连）、rule（规则）",
				"enum": ["global", "direct", "rule"]
			}
		},
		"required": ["mode"]
	}`),
	"patch_route_rule": json.RawMessage(`{
		"type": "object",
		"properties": {
			"tag": {
				"type": "string",
				"description": "规则标识，如 netflix、google-custom。自动加 _agent: 前缀"
			},
			"domain_suffix": {
				"type": "string",
				"description": "域名后缀匹配，逗号分隔，如 .netflix.com,.nflxvideo.net"
			},
			"domain": {
				"type": "string",
				"description": "精确域名匹配，逗号分隔，如 google.com,github.com"
			},
			"outbound": {
				"type": "string",
				"description": "目标出站节点名，如 direct-out、block-out 或其他代理节点"
			},
			"description": {
				"type": "string",
				"description": "规则的人类可读描述"
			}
		},
		"required": ["tag", "outbound"]
	}`),
	"remove_route_rule": json.RawMessage(`{
		"type": "object",
		"properties": {
			"tag": {
				"type": "string",
				"description": "要删除的规则标识（不含 _agent: 前缀，自动补全）"
			}
		},
		"required": ["tag"]
	}`),
	"list_route_rules": json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
	"import_subscription": json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "订阅链接 URL"
			},
			"name": {
				"type": "string",
				"description": "订阅名称（可选，留空用 URL host 自动生成）"
			}
		},
		"required": ["url"]
	}`),
	"list_subscriptions": json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
	"update_subscription": json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {
				"type": "string",
				"description": "订阅 ID（list_subscriptions 返回的第一列）。留空则更新全部订阅"
			}
		},
		"required": []
	}`),
	"remove_subscription": json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {
				"type": "string",
				"description": "要删除的订阅 ID。若未提供，tool 会先列出候选让用户选择"
			}
		},
		"required": []
	}`),
	"create_chain": json.RawMessage(`{
		"type": "object",
		"properties": {
			"tag": {
				"type": "string",
				"description": "链路出口的新 outbound tag，会自动加 _agent: 前缀。例如 jp-hk"
			},
			"nodes": {
				"type": "array",
				"items": {"type": "string"},
				"description": "节点 tag 顺序列表 entry→exit，至少 2 个。例 [\"JP-1\",\"HK-1\"] 表示流量经 JP-1 → HK-1 → target。节点 tag 须与 get_node_pool 返回的 tag 一致",
				"minItems": 2
			},
			"activate": {
				"type": "boolean",
				"description": "默认 true: 创建后立即把 proxy-group selector 切到该 chain 让流量真正经过。false 仅创建不切换（仅在用户明确要求 '只建不切' 时用）。绝大多数场景不要设为 false。"
			}
		},
		"required": ["tag", "nodes"]
	}`),
	"get_dns_config": json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
	"set_dns_config": json.RawMessage(`{
		"type": "object",
		"properties": {
			"mode": {
				"type": "string",
				"enum": ["secure", "split", "local"],
				"description": "secure=DoT via proxy (anti-leak); split=direct/proxy split; local=direct UDP only"
			}
		},
		"required": ["mode"]
	}`),
	"start_vpn": json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
	"stop_vpn": json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
	"vpn_status": json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
	"set_per_app_vpn": json.RawMessage(`{
		"type": "object",
		"properties": {
			"mode": {
				"type": "string",
				"enum": ["allow", "deny", "off"],
				"description": "allow=白名单(只有列表内 App 走代理); deny=黑名单(列表内 App 直连其他走代理); off=关闭过滤所有 App 走代理"
			},
			"packages": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Android 包名列表. 常见: Chrome=com.android.chrome, 微信=com.tencent.mm, Telegram=org.telegram.messenger, YouTube=com.google.android.youtube, Twitter/X=com.twitter.android, TikTok=com.zhiliaoapp.musically. mode=off 时可省略"
			}
		},
		"required": ["mode"]
	}`),
	"get_per_app_vpn": json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
	"setup_app_chain": json.RawMessage(`{
		"type": "object",
		"properties": {
			"app_pkg": {
				"type": "string",
				"description": "Android 包名, 例 com.android.chrome / com.tencent.mm / com.google.android.youtube。 流量将限定到该 App"
			},
			"nodes": {
				"type": "array",
				"items": {"type": "string"},
				"minItems": 2,
				"description": "链路节点 tag 顺序列表 entry→exit, 至少 2 个。 内部会先用 test_latency 校验全活, 死节点直接 fail-fast 不会浪费写盘"
			},
			"chain_tag": {
				"type": "string",
				"description": "链路出口 outbound 自定义 tag (会自动加 _agent: 前缀)。 留空则用 app_pkg 自动生成 (com.android.chrome → com-android-chrome-chain)"
			}
		},
		"required": ["app_pkg", "nodes"]
	}`),
	"diagnose_connectivity": json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
	"import_and_activate_subscription": json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "订阅 URL (http/https) 或单节点 URI (ss/vmess/vless/trojan/...)。 必填"
			},
			"name": {
				"type": "string",
				"description": "可选, 订阅显示名。 留空时自动从 URL host 派生"
			},
			"pick_fastest": {
				"type": "boolean",
				"description": "可选, 默认 true。 true=测速选最快节点切 selector; false=直接选第一个 tag (跳过测速, 适合用户只想'先把节点导进来')"
			}
		},
		"required": ["url"]
	}`),
	"switch_to_fastest_node": json.RawMessage(`{
		"type": "object",
		"properties": {
			"region_match": {
				"type": "array",
				"items": {"type": "string"},
				"description": "可选, 节点 tag substring 白名单 (大小写不敏感, 任一命中即视为候选)。 \"切到日本最快\" 传 [\"JP\",\"Tokyo\",\"日本\"]; \"切到美国最快\" 传 [\"US\",\"San-Jose\",\"美国\"]; 留空 = 全节点比较"
			},
			"skip_vpn_ensure": {
				"type": "boolean",
				"description": "可选, 默认 false。 false=VPN 没启动时自动拉起再测; true=直接测 (仅在你确定 VPN 已开时用)"
			}
		},
		"required": []
	}`),
}

// ConvertToolsToSchema 将内部 ToolDef 转为 OpenAI API 的 tools 参数格式（全量，向后兼容）
func ConvertToolsToSchema(tools map[string]*tool.ToolDef) []Tool {
	var result []Tool
	for name, td := range tools {
		// 不暴露 snapshot 和 rollback
		if name == "snapshot" || name == "rollback" {
			continue
		}
		schema, ok := toolParameterSchemas[name]
		if !ok {
			continue
		}
		result = append(result, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        name,
				Description: td.Description,
				Parameters:  schema,
			},
		})
	}
	return result
}

// ConvertToolsForRole 按角色白名单过滤，只返回该角色允许使用的 tool schema
func ConvertToolsForRole(tools map[string]*tool.ToolDef, allowedTools []string) []Tool {
	allowed := make(map[string]bool, len(allowedTools))
	for _, name := range allowedTools {
		allowed[name] = true
	}

	var result []Tool
	for name, td := range tools {
		if !allowed[name] {
			continue
		}
		schema, ok := toolParameterSchemas[name]
		if !ok {
			continue
		}
		result = append(result, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        name,
				Description: td.Description,
				Parameters:  schema,
			},
		})
	}
	return result
}
