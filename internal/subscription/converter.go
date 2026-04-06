package subscription

import (
	"fmt"
	"regexp"
	"strings"
)

// ConvertToSingboxOutbound 将 NodeConfig 转为 sing-box outbound 配置
func ConvertToSingboxOutbound(node NodeConfig) (map[string]interface{}, error) {
	tag := SanitizeTag(node.Name)
	if tag == "" {
		tag = fmt.Sprintf("%s-%s-%d", node.Type, node.Server, node.Port)
	}

	switch node.Type {
	case "shadowsocks":
		return convertSS(node, tag)
	case "trojan":
		return convertTrojan(node, tag)
	case "vmess":
		return convertVMess(node, tag)
	case "vless":
		return convertVLess(node, tag)
	case "hysteria2":
		return convertHysteria2(node, tag)
	default:
		return nil, fmt.Errorf("不支持的节点类型: %s", node.Type)
	}
}

func convertSS(node NodeConfig, tag string) (map[string]interface{}, error) {
	if node.Server == "" || node.Port == 0 || node.Method == "" || node.Password == "" {
		return nil, fmt.Errorf("SS 节点缺少必要字段: server=%s port=%d method=%s", node.Server, node.Port, node.Method)
	}
	ob := map[string]interface{}{
		"type":        "shadowsocks",
		"tag":         tag,
		"server":      node.Server,
		"server_port": node.Port,
		"method":      node.Method,
		"password":    node.Password,
	}
	return ob, nil
}

func convertTrojan(node NodeConfig, tag string) (map[string]interface{}, error) {
	if node.Server == "" || node.Port == 0 || node.Password == "" {
		return nil, fmt.Errorf("Trojan 节点缺少必要字段: server=%s port=%d", node.Server, node.Port)
	}
	ob := map[string]interface{}{
		"type":        "trojan",
		"tag":         tag,
		"server":      node.Server,
		"server_port": node.Port,
		"password":    node.Password,
	}
	if node.TLS {
		tls := map[string]interface{}{
			"enabled": true,
		}
		if node.SNI != "" {
			tls["server_name"] = node.SNI
		}
		ob["tls"] = tls
	}
	addTransport(ob, node)
	return ob, nil
}

func convertVMess(node NodeConfig, tag string) (map[string]interface{}, error) {
	if node.Server == "" || node.Port == 0 || node.UUID == "" {
		return nil, fmt.Errorf("VMess 节点缺少必要字段: server=%s port=%d uuid=%s", node.Server, node.Port, node.UUID)
	}
	ob := map[string]interface{}{
		"type":        "vmess",
		"tag":         tag,
		"server":      node.Server,
		"server_port": node.Port,
		"uuid":        node.UUID,
		"alter_id":    node.AlterId,
		"security":    "auto",
	}
	if node.TLS {
		tls := map[string]interface{}{
			"enabled": true,
		}
		if node.SNI != "" {
			tls["server_name"] = node.SNI
		}
		ob["tls"] = tls
	}
	addTransport(ob, node)
	return ob, nil
}

func convertVLess(node NodeConfig, tag string) (map[string]interface{}, error) {
	if node.Server == "" || node.Port == 0 || node.UUID == "" {
		return nil, fmt.Errorf("VLess 节点缺少必要字段: server=%s port=%d uuid=%s", node.Server, node.Port, node.UUID)
	}
	ob := map[string]interface{}{
		"type":        "vless",
		"tag":         tag,
		"server":      node.Server,
		"server_port": node.Port,
		"uuid":        node.UUID,
	}
	if flow, ok := node.Extra["flow"]; ok && flow != "" {
		ob["flow"] = flow
	}
	if node.TLS {
		tls := map[string]interface{}{
			"enabled": true,
		}
		if node.SNI != "" {
			tls["server_name"] = node.SNI
		}
		// REALITY 支持
		if pbk, ok := node.Extra["public_key"]; ok && pbk != "" {
			reality := map[string]interface{}{
				"enabled":    true,
				"public_key": pbk,
			}
			if sid, ok := node.Extra["short_id"]; ok {
				reality["short_id"] = sid
			}
			tls["reality"] = reality
		}
		if fp, ok := node.Extra["fingerprint"]; ok && fp != "" {
			tls["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": fp,
			}
		}
		ob["tls"] = tls
	}
	addTransport(ob, node)
	return ob, nil
}

func convertHysteria2(node NodeConfig, tag string) (map[string]interface{}, error) {
	if node.Server == "" || node.Port == 0 || node.Password == "" {
		return nil, fmt.Errorf("Hysteria2 节点缺少必要字段: server=%s port=%d", node.Server, node.Port)
	}
	ob := map[string]interface{}{
		"type":        "hysteria2",
		"tag":         tag,
		"server":      node.Server,
		"server_port": node.Port,
		"password":    node.Password,
	}
	tls := map[string]interface{}{
		"enabled": true,
	}
	if node.SNI != "" {
		tls["server_name"] = node.SNI
	}
	if node.Extra["insecure"] == "true" {
		tls["insecure"] = true
	}
	ob["tls"] = tls

	if obfs, ok := node.Extra["obfs"]; ok && obfs != "" {
		obfsCfg := map[string]interface{}{
			"type": obfs,
		}
		if pw, ok := node.Extra["obfs_password"]; ok {
			obfsCfg["password"] = pw
		}
		ob["obfs"] = obfsCfg
	}
	return ob, nil
}

// addTransport 添加传输层配置（ws, grpc, h2）
func addTransport(ob map[string]interface{}, node NodeConfig) {
	switch node.Network {
	case "ws":
		transport := map[string]interface{}{
			"type": "ws",
		}
		if node.Path != "" {
			transport["path"] = node.Path
		}
		if node.Host != "" {
			transport["headers"] = map[string]interface{}{
				"Host": node.Host,
			}
		}
		ob["transport"] = transport
	case "grpc":
		transport := map[string]interface{}{
			"type": "grpc",
		}
		// gRPC service_name: 优先用 Extra["service_name"]（来自 serviceName 参数），其次用 Path
		if sn, ok := node.Extra["service_name"]; ok && sn != "" {
			transport["service_name"] = sn
		} else if node.Path != "" {
			transport["service_name"] = node.Path
		}
		ob["transport"] = transport
	case "h2":
		transport := map[string]interface{}{
			"type": "http",
		}
		if node.Path != "" {
			transport["path"] = node.Path
		}
		if node.Host != "" {
			transport["host"] = []interface{}{node.Host}
		}
		ob["transport"] = transport
	}
	// tcp 不需要额外传输层配置
}

// tagCleanRegex 清理 tag 中不允许的字符
var tagCleanRegex = regexp.MustCompile(`[^\w\-\.\p{Han}\p{Katakana}\p{Hiragana}\p{Hangul}]`)

// SanitizeTag 清理节点名作为 sing-box tag（保留中日韩文字、字母数字和连字符）
func SanitizeTag(name string) string {
	// 去除前后空白
	name = strings.TrimSpace(name)
	// 替换不允许的字符为连字符
	name = tagCleanRegex.ReplaceAllString(name, "-")
	// 合并连续连字符
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	// 去除首尾连字符
	name = strings.Trim(name, "-")
	if name == "" {
		return ""
	}
	return name
}

// SummarizeNodes 统计节点类型
func SummarizeNodes(nodes []NodeConfig) string {
	counts := map[string]int{}
	for _, n := range nodes {
		counts[n.Type]++
	}
	var parts []string
	order := []string{"shadowsocks", "trojan", "vmess", "vless", "hysteria2"}
	labels := map[string]string{"shadowsocks": "SS", "trojan": "Trojan", "vmess": "VMess", "vless": "VLess", "hysteria2": "Hy2"}
	for _, t := range order {
		if c, ok := counts[t]; ok && c > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", labels[t], c))
		}
	}
	return strings.Join(parts, ", ")
}

// DeduplicateByTag 按 tag 去重，后出现的覆盖先出现的
func DeduplicateByTag(outbounds []map[string]interface{}) []map[string]interface{} {
	seen := map[string]int{}
	var result []map[string]interface{}
	for _, ob := range outbounds {
		tag, _ := ob["tag"].(string)
		if idx, exists := seen[tag]; exists {
			result[idx] = ob // 覆盖
		} else {
			seen[tag] = len(result)
			result = append(result, ob)
		}
	}
	return result
}
