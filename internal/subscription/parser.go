package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// NodeConfig 是解析后的通用节点配置
type NodeConfig struct {
	Name        string
	Type        string // "shadowsocks", "trojan", "vmess", "vless", "hysteria2", "wireguard"
	Server      string
	Port        int
	Password    string // ss, trojan, hysteria2
	Method      string // ss 加密方式
	UUID        string // vmess, vless
	AlterId     int    // vmess
	Network     string // vmess/vless 传输层 (ws, tcp, grpc...)
	TLS         bool
	SNI         string
	Path        string            // ws path
	Host        string            // ws host
	Extra       map[string]string // 其他未分类参数
	IsInfoEntry bool              // 机场塞的信息条目（套餐到期、剩余流量等），非真实节点
}

// infoKeywords 机场在订阅中塞的信息条目关键词
var infoKeywords = []string{
	"套餐", "到期", "剩余流量", "官网", "重置", "过期", "距离", "连不上的时候",
}

// isInfoEntry 判断节点名是否为信息条目
func isInfoEntry(name string) bool {
	for _, kw := range infoKeywords {
		if strings.Contains(name, kw) {
			return true
		}
	}
	return false
}

// FetchAndParse 下载订阅链接并解析节点列表（向后兼容包装）
func FetchAndParse(subURL string) ([]NodeConfig, error) {
	nodes, _, err := FetchAndParseWithInfo(subURL)
	return nodes, err
}

// FetchAndParseWithInfo 下载并解析订阅，同时提取 subscription-userinfo header
func FetchAndParseWithInfo(subURL string) ([]NodeConfig, *UserInfo, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("GET", subURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Shadowrocket/1900 CFNetwork/1410.0.3 Darwin/22.6.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("下载订阅失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("订阅返回 HTTP %d", resp.StatusCode)
	}

	// 提取 subscription-userinfo header
	// 机场常见返回头之一：subscription-userinfo / Subscription-Userinfo
	var info *UserInfo
	if h := resp.Header.Get("subscription-userinfo"); h != "" {
		info = ParseUserInfoHeader(h)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("读取响应失败: %w", err)
	}

	nodes, err := ParseSubscription(string(body))
	return nodes, info, err
}

// ParseUserInfoHeader 解析 "upload=1; download=2; total=10; expire=1700000000"
func ParseUserInfoHeader(header string) *UserInfo {
	info := &UserInfo{}
	any := false
	for _, part := range strings.Split(header, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "upload":
			info.Upload = val
			any = true
		case "download":
			info.Download = val
			any = true
		case "total":
			info.Total = val
			any = true
		case "expire":
			info.Expire = val
			any = true
		}
	}
	if !any {
		return nil
	}
	return info
}

// ParseSubscription 解析 Base64 编码的订阅内容
func ParseSubscription(raw string) ([]NodeConfig, error) {
	// 尝试 Base64 解码（标准和 URL-safe）
	decoded := tryBase64Decode(strings.TrimSpace(raw))
	if decoded == "" {
		return nil, fmt.Errorf("Base64 解码失败")
	}

	var nodes []NodeConfig
	lines := strings.Split(decoded, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		node, err := parseLine(line)
		if err != nil {
			fmt.Printf("  ⚠️  跳过无法解析的行: %s (%v)\n", truncate(line, 60), err)
			continue
		}
		node.IsInfoEntry = isInfoEntry(node.Name)
		nodes = append(nodes, node)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("未解析到任何有效节点")
	}
	return nodes, nil
}

// tryBase64Decode 尝试标准和 URL-safe Base64 解码
func tryBase64Decode(s string) string {
	// 补齐 padding
	padded := s
	if m := len(padded) % 4; m != 0 {
		padded += strings.Repeat("=", 4-m)
	}

	// 标准 Base64
	if decoded, err := base64.StdEncoding.DecodeString(padded); err == nil {
		return string(decoded)
	}
	// URL-safe Base64
	if decoded, err := base64.URLEncoding.DecodeString(padded); err == nil {
		return string(decoded)
	}
	// 无 padding 版本
	if decoded, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return string(decoded)
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return string(decoded)
	}
	return ""
}

func parseLine(line string) (NodeConfig, error) {
	switch {
	case strings.HasPrefix(line, "ss://"):
		return parseSS(line)
	case strings.HasPrefix(line, "trojan://"):
		return parseTrojan(line)
	case strings.HasPrefix(line, "vmess://"):
		return parseVMess(line)
	case strings.HasPrefix(line, "vless://"):
		return parseVLess(line)
	case strings.HasPrefix(line, "hysteria2://") || strings.HasPrefix(line, "hy2://"):
		return parseHysteria2(line)
	case strings.HasPrefix(line, "wireguard://") || strings.HasPrefix(line, "wg://"):
		return parseWireGuard(line)
	default:
		return NodeConfig{}, fmt.Errorf("不支持的协议: %s", truncate(line, 20))
	}
}

// parseSS 解析 ss:// URI
// 格式: ss://BASE64(method:password)@server:port#name
// 或:   ss://BASE64(method:password@server:port)#name
func parseSS(uri string) (NodeConfig, error) {
	node := NodeConfig{Type: "shadowsocks", Extra: map[string]string{}}

	// 提取名称 (#name)
	if idx := strings.LastIndex(uri, "#"); idx != -1 {
		node.Name = decodeURIComponent(uri[idx+1:])
		uri = uri[:idx]
	}

	body := strings.TrimPrefix(uri, "ss://")

	// 尝试格式1: BASE64(method:password)@server:port
	if atIdx := strings.LastIndex(body, "@"); atIdx != -1 {
		userInfo := tryBase64Decode(body[:atIdx])
		if userInfo == "" {
			userInfo = body[:atIdx] // 可能没编码
		}
		serverPart := body[atIdx+1:]

		if colonIdx := strings.Index(userInfo, ":"); colonIdx != -1 {
			node.Method = userInfo[:colonIdx]
			node.Password = userInfo[colonIdx+1:]
		} else {
			return node, fmt.Errorf("无法解析 SS userinfo: %s", userInfo)
		}

		server, port, err := parseHostPort(serverPart)
		if err != nil {
			return node, fmt.Errorf("无法解析 SS server: %w", err)
		}
		node.Server = server
		node.Port = port
	} else {
		// 格式2: BASE64(method:password@server:port)
		decoded := tryBase64Decode(body)
		if decoded == "" {
			return node, fmt.Errorf("无法 Base64 解码 SS URI")
		}
		atIdx := strings.LastIndex(decoded, "@")
		if atIdx == -1 {
			return node, fmt.Errorf("SS 解码后无 @: %s", decoded)
		}
		userInfo := decoded[:atIdx]
		serverPart := decoded[atIdx+1:]

		if colonIdx := strings.Index(userInfo, ":"); colonIdx != -1 {
			node.Method = userInfo[:colonIdx]
			node.Password = userInfo[colonIdx+1:]
		}

		server, port, err := parseHostPort(serverPart)
		if err != nil {
			return node, err
		}
		node.Server = server
		node.Port = port
	}

	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", node.Server, node.Port)
	}
	return node, nil
}

// parseTrojan 解析 trojan:// URI
// 格式: trojan://password@server:port?参数#name
func parseTrojan(uri string) (NodeConfig, error) {
	node := NodeConfig{Type: "trojan", TLS: true, Extra: map[string]string{}}

	// 提取名称
	if idx := strings.LastIndex(uri, "#"); idx != -1 {
		node.Name = decodeURIComponent(uri[idx+1:])
		uri = uri[:idx]
	}

	body := strings.TrimPrefix(uri, "trojan://")

	// 提取查询参数
	queryStr := ""
	if idx := strings.Index(body, "?"); idx != -1 {
		queryStr = body[idx+1:]
		body = body[:idx]
	}

	// password@server:port
	atIdx := strings.LastIndex(body, "@")
	if atIdx == -1 {
		return node, fmt.Errorf("trojan URI 无 @")
	}
	node.Password = body[:atIdx]

	server, port, err := parseHostPort(body[atIdx+1:])
	if err != nil {
		return node, err
	}
	node.Server = server
	node.Port = port

	// 解析查询参数
	if queryStr != "" {
		params := parseQuery(queryStr)
		if sni, ok := params["sni"]; ok {
			node.SNI = sni
		}
		if host, ok := params["host"]; ok {
			node.Host = host
		}
		if path, ok := params["path"]; ok {
			node.Path = path
		}
		if t, ok := params["type"]; ok {
			node.Network = t
		}
		if sec, ok := params["security"]; ok && sec == "none" {
			node.TLS = false
		}
		if alpn, ok := params["alpn"]; ok {
			node.Extra["alpn"] = alpn
		}
	}

	if node.SNI == "" && node.Host != "" {
		node.SNI = node.Host
	}
	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", node.Server, node.Port)
	}
	return node, nil
}

// parseVMess 解析 vmess:// URI
// 格式: vmess://BASE64(JSON)
func parseVMess(uri string) (NodeConfig, error) {
	node := NodeConfig{Type: "vmess", Extra: map[string]string{}}

	body := strings.TrimPrefix(uri, "vmess://")
	decoded := tryBase64Decode(body)
	if decoded == "" {
		return node, fmt.Errorf("无法 Base64 解码 VMess URI")
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(decoded), &obj); err != nil {
		return node, fmt.Errorf("VMess JSON 解析失败: %w", err)
	}

	node.Name = jsonStr(obj, "ps")
	node.Server = jsonStr(obj, "add")
	node.Port = jsonInt(obj, "port")
	node.UUID = jsonStr(obj, "id")
	node.AlterId = jsonInt(obj, "aid")
	node.Network = jsonStr(obj, "net")
	node.Host = jsonStr(obj, "host")
	node.Path = jsonStr(obj, "path")

	if tls := jsonStr(obj, "tls"); tls == "tls" {
		node.TLS = true
	}
	if sni := jsonStr(obj, "sni"); sni != "" {
		node.SNI = sni
	} else if node.TLS && node.Host != "" {
		node.SNI = node.Host
	}

	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", node.Server, node.Port)
	}
	return node, nil
}

// parseVLess 解析 vless:// URI
// 格式: vless://uuid@server:port?参数#name
func parseVLess(uri string) (NodeConfig, error) {
	node := NodeConfig{Type: "vless", Extra: map[string]string{}}

	// 提取名称
	if idx := strings.LastIndex(uri, "#"); idx != -1 {
		node.Name = decodeURIComponent(uri[idx+1:])
		uri = uri[:idx]
	}

	body := strings.TrimPrefix(uri, "vless://")

	// 提取查询参数
	queryStr := ""
	if idx := strings.Index(body, "?"); idx != -1 {
		queryStr = body[idx+1:]
		body = body[:idx]
	}

	// uuid@server:port
	atIdx := strings.Index(body, "@")
	if atIdx == -1 {
		return node, fmt.Errorf("vless URI 无 @")
	}
	node.UUID = body[:atIdx]

	server, port, err := parseHostPort(body[atIdx+1:])
	if err != nil {
		return node, err
	}
	node.Server = server
	node.Port = port

	if queryStr != "" {
		params := parseQuery(queryStr)
		if t, ok := params["type"]; ok {
			node.Network = t
		}
		if sec, ok := params["security"]; ok && (sec == "tls" || sec == "reality") {
			node.TLS = true
		}
		if sni, ok := params["sni"]; ok {
			node.SNI = sni
		}
		if host, ok := params["host"]; ok {
			node.Host = host
		}
		if path, ok := params["path"]; ok {
			node.Path = decodeURIComponent(path)
		}
		if sn, ok := params["serviceName"]; ok && sn != "" {
			node.Extra["service_name"] = sn
		}
		if flow, ok := params["flow"]; ok {
			node.Extra["flow"] = flow
		}
		if fp, ok := params["fp"]; ok {
			node.Extra["fingerprint"] = fp
		}
		if pbk, ok := params["pbk"]; ok {
			node.Extra["public_key"] = pbk
		}
		if sid, ok := params["sid"]; ok {
			node.Extra["short_id"] = sid
		}
		if spx, ok := params["spx"]; ok {
			node.Extra["spider_x"] = decodeURIComponent(spx)
		}
	}

	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", node.Server, node.Port)
	}
	return node, nil
}

// parseHysteria2 解析 hysteria2:// 或 hy2:// URI
// 格式: hysteria2://password@server:port/?参数#name
func parseHysteria2(uri string) (NodeConfig, error) {
	node := NodeConfig{Type: "hysteria2", TLS: true, Extra: map[string]string{}}

	// 提取名称
	if idx := strings.LastIndex(uri, "#"); idx != -1 {
		node.Name = decodeURIComponent(uri[idx+1:])
		uri = uri[:idx]
	}

	body := strings.TrimPrefix(uri, "hysteria2://")
	body = strings.TrimPrefix(body, "hy2://")

	// 提取查询参数
	queryStr := ""
	if idx := strings.Index(body, "?"); idx != -1 {
		queryStr = body[idx+1:]
		body = body[:idx]
	}

	// password@server:port — 去掉尾部斜杠
	body = strings.TrimRight(body, "/")
	atIdx := strings.LastIndex(body, "@")
	if atIdx == -1 {
		return node, fmt.Errorf("hysteria2 URI 无 @")
	}
	node.Password = body[:atIdx]

	server, port, err := parseHostPort(body[atIdx+1:])
	if err != nil {
		return node, err
	}
	node.Server = server
	node.Port = port

	if queryStr != "" {
		params := parseQuery(queryStr)
		if sni, ok := params["sni"]; ok && sni != "" {
			node.SNI = sni
		}
		if insecure, ok := params["insecure"]; ok && insecure == "1" {
			node.Extra["insecure"] = "true"
		}
		if obfs, ok := params["obfs"]; ok && obfs != "" {
			node.Extra["obfs"] = obfs
		}
		if obfsPw, ok := params["obfs-password"]; ok && obfsPw != "" {
			node.Extra["obfs_password"] = obfsPw
		}
		if mport, ok := params["mport"]; ok && mport != "" {
			node.Extra["hop_ports"] = mport
		}
	}

	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", node.Server, node.Port)
	}
	return node, nil
}

// parseWireGuard 解析 wireguard:// / wg:// URI
// 格式: wireguard://privateKey@server:port?publickey=xxx&presharedkey=xxx&address=10.0.0.2/32&mtu=1420&reserved=0,0,0#name
func parseWireGuard(uri string) (NodeConfig, error) {
	node := NodeConfig{Type: "wireguard", Extra: map[string]string{}}

	if idx := strings.LastIndex(uri, "#"); idx != -1 {
		node.Name = decodeURIComponent(uri[idx+1:])
		uri = uri[:idx]
	}

	body := strings.TrimPrefix(uri, "wireguard://")
	body = strings.TrimPrefix(body, "wg://")

	queryStr := ""
	if idx := strings.Index(body, "?"); idx != -1 {
		queryStr = body[idx+1:]
		body = body[:idx]
	}
	body = strings.TrimRight(body, "/")

	atIdx := strings.LastIndex(body, "@")
	if atIdx == -1 {
		return node, fmt.Errorf("wireguard URI 无 @")
	}
	// privateKey 可能是 URL-encoded base64
	node.Extra["private_key"] = decodeURIComponent(body[:atIdx])

	server, port, err := parseHostPort(body[atIdx+1:])
	if err != nil {
		return node, err
	}
	node.Server = server
	node.Port = port

	if queryStr != "" {
		params := parseQuery(queryStr)
		if pk, ok := params["publickey"]; ok && pk != "" {
			node.Extra["peer_public_key"] = decodeURIComponent(pk)
		}
		if pk, ok := params["public_key"]; ok && pk != "" {
			node.Extra["peer_public_key"] = decodeURIComponent(pk)
		}
		if psk, ok := params["presharedkey"]; ok && psk != "" {
			node.Extra["pre_shared_key"] = decodeURIComponent(psk)
		}
		if addr, ok := params["address"]; ok && addr != "" {
			node.Extra["local_address"] = decodeURIComponent(addr)
		}
		if addr, ok := params["ip"]; ok && addr != "" && node.Extra["local_address"] == "" {
			node.Extra["local_address"] = decodeURIComponent(addr)
		}
		if mtu, ok := params["mtu"]; ok && mtu != "" {
			node.Extra["mtu"] = mtu
		}
		if reserved, ok := params["reserved"]; ok && reserved != "" {
			node.Extra["reserved"] = reserved
		}
	}

	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", node.Server, node.Port)
	}
	return node, nil
}

// --- helpers ---

func parseHostPort(s string) (string, int, error) {
	// 处理 IPv6: [::1]:port
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end == -1 {
			return "", 0, fmt.Errorf("IPv6 格式错误: %s", s)
		}
		host := s[1:end]
		portStr := ""
		if end+1 < len(s) && s[end+1] == ':' {
			portStr = s[end+2:]
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return host, 0, fmt.Errorf("端口解析失败: %s", portStr)
		}
		return host, port, nil
	}

	// 普通 host:port — 从右侧找 : 避免 IPv6 误切
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s, 0, fmt.Errorf("无端口: %s", s)
	}
	host := s[:idx]
	port, err := strconv.Atoi(s[idx+1:])
	if err != nil {
		return host, 0, fmt.Errorf("端口解析失败: %s", s[idx+1:])
	}
	return host, port, nil
}

func parseQuery(q string) map[string]string {
	result := map[string]string{}
	pairs := strings.Split(q, "&")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

func decodeURIComponent(s string) string {
	decoded, err := url.QueryUnescape(s)
	if err != nil {
		return s
	}
	return decoded
}

func jsonStr(obj map[string]interface{}, key string) string {
	v, ok := obj[key]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.Itoa(int(val))
	default:
		return fmt.Sprintf("%v", val)
	}
}

func jsonInt(obj map[string]interface{}, key string) int {
	v, ok := obj[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n]) + "..."
	}
	return s
}
