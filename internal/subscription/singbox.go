package subscription

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// sing-box 原生 JSON 订阅解析 (M12)
//
// 格式参考: https://sing-box.sagernet.org/configuration/
//   {
//     "outbounds": [
//       { "type": "vless", "tag": "HK-01", "server": "...", "server_port": 443, "uuid": "...", ... },
//       { "type": "selector", "tag": "proxy-group", "outbounds": ["HK-01", ...] }
//       ...
//     ]
//   }
//
// 我们只提取"可拨出"节点类型 outbound。 Selector / urltest / direct / block / dns 等
// 控制流 outbound 跳过(这些通常由 overlay 生成或已在 base config 里)

type singboxRoot struct {
	Outbounds []map[string]interface{} `json:"outbounds"`
}

// ParseSingBoxJSON 从 sing-box JSON 订阅中抽出节点列表
func ParseSingBoxJSON(data []byte) ([]NodeConfig, error) {
	var root singboxRoot
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("sing-box JSON 解析失败: %w", err)
	}
	if len(root.Outbounds) == 0 {
		return nil, fmt.Errorf("sing-box JSON 无 outbounds 数组或为空")
	}

	var nodes []NodeConfig
	for i, ob := range root.Outbounds {
		node, err := singboxOutboundToNode(ob)
		if err != nil {
			tag := sbStr(ob, "tag")
			fmt.Printf("  ⚠️  sing-box outbound[%d] %q 跳过: %v\n", i, tag, err)
			continue
		}
		if node.Type == "" {
			continue // skip 类
		}
		node.IsInfoEntry = isInfoEntry(node.Name)
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("sing-box JSON 未解析到任何可拨出节点")
	}
	return nodes, nil
}

func singboxOutboundToNode(ob map[string]interface{}) (NodeConfig, error) {
	typ := sbStr(ob, "type")
	// 直接忽略的控制流 outbound
	switch typ {
	case "direct", "block", "dns", "selector", "urltest", "":
		return NodeConfig{}, nil // Type="" → 上层 skip
	}

	node := NodeConfig{
		Type:   typ,
		Name:   sbStr(ob, "tag"),
		Server: sbStr(ob, "server"),
		Port:   sbInt(ob, "server_port"),
		Extra:  map[string]string{},
	}

	tls := sbMap(ob, "tls")
	if tls != nil && sbBool(tls, "enabled") {
		node.TLS = true
		if sn := sbStr(tls, "server_name"); sn != "" {
			node.SNI = sn
		}
		if sbBool(tls, "insecure") {
			node.Extra["insecure"] = "true"
		}
		if utls := sbMap(tls, "utls"); utls != nil {
			if fp := sbStr(utls, "fingerprint"); fp != "" {
				node.Extra["fingerprint"] = fp
			}
		}
		if reality := sbMap(tls, "reality"); reality != nil && sbBool(reality, "enabled") {
			if pk := sbStr(reality, "public_key"); pk != "" {
				node.Extra["public_key"] = pk
			}
			if sid := sbStr(reality, "short_id"); sid != "" {
				node.Extra["short_id"] = sid
			}
		}
	}

	// Transport (ws / grpc / http)
	if t := sbMap(ob, "transport"); t != nil {
		switch sbStr(t, "type") {
		case "ws":
			node.Network = "ws"
			node.Path = sbStr(t, "path")
			if h := sbMap(t, "headers"); h != nil {
				node.Host = sbStr(h, "Host")
				if node.Host == "" {
					node.Host = sbStr(h, "host")
				}
			}
		case "grpc":
			node.Network = "grpc"
			if sn := sbStr(t, "service_name"); sn != "" {
				node.Extra["service_name"] = sn
			}
		case "http":
			node.Network = "h2"
			node.Path = sbStr(t, "path")
		}
	}

	switch typ {
	case "shadowsocks":
		node.Method = sbStr(ob, "method")
		node.Password = sbStr(ob, "password")
	case "trojan":
		node.Password = sbStr(ob, "password")
		node.TLS = true
	case "vmess":
		node.UUID = sbStr(ob, "uuid")
		node.AlterId = sbInt(ob, "alter_id")
	case "vless":
		node.UUID = sbStr(ob, "uuid")
		if flow := sbStr(ob, "flow"); flow != "" {
			node.Extra["flow"] = flow
		}
	case "hysteria2":
		node.Password = sbStr(ob, "password")
		node.TLS = true
		if obfs := sbMap(ob, "obfs"); obfs != nil {
			node.Extra["obfs"] = sbStr(obfs, "type")
			node.Extra["obfs_password"] = sbStr(obfs, "password")
		}
	case "hysteria":
		if a := sbStr(ob, "auth_str"); a != "" {
			node.Extra["auth"] = a
		}
		if up := sbInt(ob, "up_mbps"); up > 0 {
			node.Extra["up_mbps"] = strconv.Itoa(up)
		}
		if down := sbInt(ob, "down_mbps"); down > 0 {
			node.Extra["down_mbps"] = strconv.Itoa(down)
		}
		if obfs := sbStr(ob, "obfs"); obfs != "" {
			node.Extra["obfs"] = obfs
		}
		node.TLS = true
	case "tuic":
		node.UUID = sbStr(ob, "uuid")
		node.Password = sbStr(ob, "password")
		if cc := sbStr(ob, "congestion_control"); cc != "" {
			node.Extra["congestion_control"] = cc
		}
		if urm := sbStr(ob, "udp_relay_mode"); urm != "" {
			node.Extra["udp_relay_mode"] = urm
		}
		node.TLS = true
	case "anytls":
		node.Password = sbStr(ob, "password")
		node.TLS = true
	case "shadowtls":
		node.Password = sbStr(ob, "password")
		if v := sbInt(ob, "version"); v > 0 {
			node.Extra["version"] = strconv.Itoa(v)
		}
		node.TLS = true
	case "wireguard":
		node.Extra["private_key"] = sbStr(ob, "private_key")
		node.Extra["peer_public_key"] = sbStr(ob, "peer_public_key")
		if psk := sbStr(ob, "pre_shared_key"); psk != "" {
			node.Extra["pre_shared_key"] = psk
		}
		if addrs, ok := ob["local_address"].([]interface{}); ok && len(addrs) > 0 {
			var first string
			for _, a := range addrs {
				first = fmt.Sprintf("%v", a)
				break
			}
			node.Extra["local_address"] = first
		}
		if mtu := sbInt(ob, "mtu"); mtu > 0 {
			node.Extra["mtu"] = strconv.Itoa(mtu)
		}
	default:
		return NodeConfig{}, fmt.Errorf("sing-box outbound 类型暂未映射: %s", typ)
	}

	if node.Server == "" || node.Port == 0 {
		return NodeConfig{}, fmt.Errorf("outbound 缺 server/server_port")
	}
	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", node.Server, node.Port)
	}
	return node, nil
}

// --- JSON 取值辅助 ---

func sbStr(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func sbInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		if n, err := strconv.Atoi(t); err == nil {
			return n
		}
	}
	return 0
}

func sbBool(m map[string]interface{}, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func sbMap(m map[string]interface{}, key string) map[string]interface{} {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if mm, ok := v.(map[string]interface{}); ok {
		return mm
	}
	return nil
}
