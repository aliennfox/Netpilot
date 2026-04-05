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
}

// ConvertToolsToSchema 将内部 ToolDef 转为 OpenAI API 的 tools 参数格式
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
