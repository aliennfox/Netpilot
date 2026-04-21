package subscription

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ConvertToSingboxOutbound 将 NodeConfig 转为 sing-box outbound 配置
// 信息条目（IsInfoEntry=true）不转换，返回 nil, nil
func ConvertToSingboxOutbound(node NodeConfig) (map[string]interface{}, error) {
	if node.IsInfoEntry {
		return nil, nil
	}

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
	case "hysteria":
		return convertHysteria1(node, tag)
	case "tuic":
		return convertTuic(node, tag)
	case "anytls":
		return convertAnyTLS(node, tag)
	case "shadowtls":
		return convertShadowTLS(node, tag)
	case "wireguard":
		return convertWireGuard(node, tag)
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

func convertWireGuard(node NodeConfig, tag string) (map[string]interface{}, error) {
	priv := node.Extra["private_key"]
	peer := node.Extra["peer_public_key"]
	if node.Server == "" || node.Port == 0 || priv == "" || peer == "" {
		return nil, fmt.Errorf("WireGuard 节点缺少必要字段: server=%s port=%d private_key=%v peer_public_key=%v",
			node.Server, node.Port, priv != "", peer != "")
	}
	ob := map[string]interface{}{
		"type":            "wireguard",
		"tag":             tag,
		"server":          node.Server,
		"server_port":     node.Port,
		"private_key":     priv,
		"peer_public_key": peer,
	}
	if addr := node.Extra["local_address"]; addr != "" {
		// 支持多地址，逗号分隔
		var list []interface{}
		for _, a := range strings.Split(addr, ",") {
			a = strings.TrimSpace(a)
			if a != "" {
				list = append(list, a)
			}
		}
		ob["local_address"] = list
	}
	if psk := node.Extra["pre_shared_key"]; psk != "" {
		ob["pre_shared_key"] = psk
	}
	if mtu := node.Extra["mtu"]; mtu != "" {
		if n, err := strconv.Atoi(mtu); err == nil {
			ob["mtu"] = n
		}
	}
	if reserved := node.Extra["reserved"]; reserved != "" {
		var list []interface{}
		for _, p := range strings.Split(reserved, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				list = append(list, n)
			}
		}
		if len(list) > 0 {
			ob["reserved"] = list
		}
	}
	return ob, nil
}

// convertTuic 将 TUIC v5 节点转为 sing-box outbound
// Schema 依据: sing-box v1.13.8 `option/tuic.go:TUICOutboundOptions`
func convertTuic(node NodeConfig, tag string) (map[string]interface{}, error) {
	if node.Server == "" || node.Port == 0 || node.UUID == "" {
		return nil, fmt.Errorf("TUIC 节点缺少必要字段: server=%s port=%d uuid=%s", node.Server, node.Port, node.UUID)
	}
	ob := map[string]interface{}{
		"type":        "tuic",
		"tag":         tag,
		"server":      node.Server,
		"server_port": node.Port,
		"uuid":        node.UUID,
	}
	if node.Password != "" {
		ob["password"] = node.Password
	}
	if cc := node.Extra["congestion_control"]; cc != "" {
		ob["congestion_control"] = cc
	}
	if urm := node.Extra["udp_relay_mode"]; urm != "" {
		ob["udp_relay_mode"] = urm
	}
	tls := map[string]interface{}{"enabled": true}
	if node.SNI != "" {
		tls["server_name"] = node.SNI
	}
	if node.Extra["allow_insecure"] == "true" {
		tls["insecure"] = true
	}
	if node.Extra["disable_sni"] == "true" {
		tls["disable_sni"] = true
	}
	if alpn := node.Extra["alpn"]; alpn != "" {
		tls["alpn"] = splitCSV(alpn)
	}
	ob["tls"] = tls
	return ob, nil
}

// convertHysteria1 将 Hysteria v1 节点转为 sing-box outbound
// Schema 依据: sing-box v1.13.8 `option/hysteria.go:HysteriaOutboundOptions`
// 注意: Hysteria v1 的 auth 字段有两种:bytes 版 `auth` 和 字符串版 `auth_str`,
// 字符串 URL 传递的一律 auth_str 更合适(base64 版极少出现在 URI)。
func convertHysteria1(node NodeConfig, tag string) (map[string]interface{}, error) {
	if node.Server == "" || node.Port == 0 {
		return nil, fmt.Errorf("Hysteria 节点缺少必要字段: server=%s port=%d", node.Server, node.Port)
	}
	ob := map[string]interface{}{
		"type":        "hysteria",
		"tag":         tag,
		"server":      node.Server,
		"server_port": node.Port,
	}
	if auth := node.Extra["auth"]; auth != "" {
		ob["auth_str"] = auth
	}
	if up := node.Extra["up_mbps"]; up != "" {
		if n, err := strconv.Atoi(up); err == nil && n > 0 {
			ob["up_mbps"] = n
		}
	}
	if down := node.Extra["down_mbps"]; down != "" {
		if n, err := strconv.Atoi(down); err == nil && n > 0 {
			ob["down_mbps"] = n
		}
	}
	// Hysteria v1 的 obfs 是对称混淆密码(单字符串),非 hy2 的 {type,password} 结构
	if obfs := node.Extra["obfs_param"]; obfs != "" {
		ob["obfs"] = obfs
	} else if obfs := node.Extra["obfs"]; obfs != "" && obfs != "xplus" {
		// fallback: 部分机场把实际密码塞在 obfs 字段
		ob["obfs"] = obfs
	}
	if hop := node.Extra["hop_ports"]; hop != "" {
		// sing-box server_ports 格式: ["12000:14000","15001"]
		ob["server_ports"] = splitCSV(hop)
	}
	tls := map[string]interface{}{"enabled": true}
	if node.SNI != "" {
		tls["server_name"] = node.SNI
	}
	if node.Extra["insecure"] == "true" {
		tls["insecure"] = true
	}
	if alpn := node.Extra["alpn"]; alpn != "" {
		tls["alpn"] = splitCSV(alpn)
	}
	ob["tls"] = tls
	return ob, nil
}

// convertAnyTLS 将 AnyTLS 节点转为 sing-box outbound
// Schema: sing-box v1.13.8 `option/anytls.go:AnyTLSOutboundOptions`
func convertAnyTLS(node NodeConfig, tag string) (map[string]interface{}, error) {
	if node.Server == "" || node.Port == 0 || node.Password == "" {
		return nil, fmt.Errorf("AnyTLS 节点缺少必要字段: server=%s port=%d", node.Server, node.Port)
	}
	ob := map[string]interface{}{
		"type":        "anytls",
		"tag":         tag,
		"server":      node.Server,
		"server_port": node.Port,
		"password":    node.Password,
	}
	tls := map[string]interface{}{"enabled": true}
	if node.SNI != "" {
		tls["server_name"] = node.SNI
	}
	if node.Extra["insecure"] == "true" {
		tls["insecure"] = true
	}
	if fp := node.Extra["fingerprint"]; fp != "" {
		tls["utls"] = map[string]interface{}{
			"enabled":     true,
			"fingerprint": fp,
		}
	}
	if alpn := node.Extra["alpn"]; alpn != "" {
		tls["alpn"] = splitCSV(alpn)
	}
	ob["tls"] = tls
	return ob, nil
}

// convertShadowTLS 将 ShadowTLS 节点转为 sing-box outbound
// Schema: sing-box v1.13.8 `option/shadowtls.go:ShadowTLSOutboundOptions`
// 注意: 生产环境 ShadowTLS 通常作为 SS 的 detour 前置;本函数只产出 shadowtls 单独
// outbound, 链式编排留给用户 (create_chain tool 或 Clash YAML 订阅本身自带链式描述)
func convertShadowTLS(node NodeConfig, tag string) (map[string]interface{}, error) {
	if node.Server == "" || node.Port == 0 {
		return nil, fmt.Errorf("ShadowTLS 节点缺少必要字段: server=%s port=%d", node.Server, node.Port)
	}
	version := 3
	if v := node.Extra["version"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 3 {
			version = n
		}
	}
	ob := map[string]interface{}{
		"type":        "shadowtls",
		"tag":         tag,
		"server":      node.Server,
		"server_port": node.Port,
		"version":     version,
	}
	if node.Password != "" {
		ob["password"] = node.Password
	}
	tls := map[string]interface{}{"enabled": true}
	if node.SNI != "" {
		tls["server_name"] = node.SNI
	}
	if node.Extra["insecure"] == "true" {
		tls["insecure"] = true
	}
	if fp := node.Extra["fingerprint"]; fp != "" {
		tls["utls"] = map[string]interface{}{
			"enabled":     true,
			"fingerprint": fp,
		}
	}
	if alpn := node.Extra["alpn"]; alpn != "" {
		tls["alpn"] = splitCSV(alpn)
	}
	ob["tls"] = tls
	return ob, nil
}

// splitCSV 把 "a,b,c" 或 "a b c" 或换行分隔拆成 []interface{}(sing-box JSON slice 语义)
func splitCSV(s string) []interface{} {
	var list []interface{}
	// 按逗号 / 换行 / 空白分割, 三者都容忍
	sep := func(r rune) bool { return r == ',' || r == '\n' || r == ' ' || r == '\r' }
	for _, part := range strings.FieldsFunc(s, sep) {
		part = strings.TrimSpace(part)
		if part != "" {
			list = append(list, part)
		}
	}
	return list
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

// protocolSuffixRegex 匹配节点名末尾的协议类型后缀，如 "(Hysteria2)", "(SS)", " [VMess]" 等
var protocolSuffixRegex = regexp.MustCompile(`(?i)\s*[\(\[]\s*(hysteria2?|hy2?|ss|shadowsocks|trojan|vmess|vless|wireguard|wg|tuic|anytls|shadowtls|stls)\s*[\)\]]\s*$`)

// tagCleanRegex 清理 tag 中不允许的字符（只保留中日韩文字、英文字母、数字、连字符）
var tagCleanRegex = regexp.MustCompile(`[^a-zA-Z0-9\-\p{Han}\p{Katakana}\p{Hiragana}\p{Hangul}]`)

// SanitizeTag 清理节点名作为 sing-box tag（保留中日韩文字、字母数字和连字符）
func SanitizeTag(name string) string {
	// 去除前后空白
	name = strings.TrimSpace(name)
	// 去除协议类型后缀
	name = protocolSuffixRegex.ReplaceAllString(name, "")
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

// SummarizeNodes 统计真实节点类型（排除信息条目）
func SummarizeNodes(nodes []NodeConfig) string {
	counts := map[string]int{}
	for _, n := range nodes {
		if !n.IsInfoEntry {
			counts[n.Type]++
		}
	}
	var parts []string
	order := []string{"shadowsocks", "trojan", "vmess", "vless", "hysteria2", "hysteria", "tuic", "anytls", "shadowtls", "wireguard"}
	labels := map[string]string{
		"shadowsocks": "SS",
		"trojan":      "Trojan",
		"vmess":       "VMess",
		"vless":       "VLess",
		"hysteria2":   "Hy2",
		"hysteria":    "Hy",
		"tuic":        "TUIC",
		"anytls":      "AnyTLS",
		"shadowtls":   "ShadowTLS",
		"wireguard":   "WG",
	}
	for _, t := range order {
		if c, ok := counts[t]; ok && c > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", labels[t], c))
		}
	}
	return strings.Join(parts, ", ")
}

// FormatInfoEntries 提取信息条目的名称，用 " | " 拼接
func FormatInfoEntries(nodes []NodeConfig) string {
	var names []string
	for _, n := range nodes {
		if n.IsInfoEntry {
			names = append(names, n.Name)
		}
	}
	return strings.Join(names, " | ")
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
