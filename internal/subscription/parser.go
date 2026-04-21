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
	Name     string
	Type     string // "shadowsocks" | "trojan" | "vmess" | "vless" | "hysteria" | "hysteria2" | "wireguard" | "tuic" | "anytls" | "shadowtls"
	Server   string
	Port     int
	Password string // ss / trojan / hysteria2 / anytls / shadowtls / tuic-token-fallback
	Method   string // ss 加密方式
	UUID     string // vmess / vless / tuic
	AlterId  int    // vmess
	Network  string // vmess/vless 传输层 (ws, tcp, grpc...)
	TLS      bool
	SNI      string
	Path     string // ws path
	Host     string // ws host
	// Extra 保留协议特有字段,避免主结构膨胀。按协议惯用 key:
	//   VLESS Reality: public_key / short_id / fingerprint / flow / spider_x
	//   WireGuard: private_key / peer_public_key / pre_shared_key / local_address / mtu / reserved
	//   Hysteria2: insecure / obfs / obfs_password / hop_ports
	//   Hysteria v1: auth / alpn / obfs / obfs_param / up_mbps / down_mbps / protocol / insecure / hop_ports
	//   TUIC v5: congestion_control / udp_relay_mode / alpn / allow_insecure / disable_sni / token
	//   AnyTLS: insecure / fingerprint / alpn
	//   ShadowTLS: version / handshake_server / handshake_port / fingerprint
	Extra       map[string]string
	IsInfoEntry bool // 机场塞的信息条目（套餐到期、剩余流量等），非真实节点
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

// ParseSubscription 解析订阅内容。 支持三种输入格式(按优先级探测):
//  1. Clash / Clash.Meta / Mihomo YAML (顶层 `proxies:` key)
//  2. sing-box native JSON (顶层 `{`, 含 outbounds 数组, M12 支持)
//  3. Base64 编码的 URI 列表 (默认兜底, 兼容 v2ray-subscribe 生态)
func ParseSubscription(raw string) ([]NodeConfig, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("订阅内容为空")
	}

	// 1. Clash YAML 优先 —— 这是 Karing / Clash Meta 生态的主流订阅格式
	if IsClashYAML(trimmed) {
		return ParseClashYAML([]byte(trimmed))
	}

	// 2. sing-box native JSON —— 顶层 `{` 且含 outbounds 数组
	if strings.HasPrefix(trimmed, "{") {
		if nodes, err := ParseSingBoxJSON([]byte(trimmed)); err == nil {
			return nodes, nil
		}
		// fallthrough: 可能是"意外以 { 开头的 base64",继续尝试 base64 路径
	}

	// 3. Base64 URI 列表 (原主路径)
	decoded := tryBase64Decode(trimmed)
	if decoded == "" {
		// 再尝试当作明文 URI 列表(每行一条)
		decoded = trimmed
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
	case strings.HasPrefix(line, "hysteria://"):
		return parseHysteria1(line)
	case strings.HasPrefix(line, "tuic://"):
		return parseTuic(line)
	case strings.HasPrefix(line, "anytls://"):
		return parseAnyTLS(line)
	case strings.HasPrefix(line, "shadowtls://"):
		return parseShadowTLS(line)
	case strings.HasPrefix(line, "wireguard://") || strings.HasPrefix(line, "wg://"):
		return parseWireGuard(line)
	default:
		return NodeConfig{}, fmt.Errorf("不支持的协议: %s", truncate(line, 20))
	}
}

// parseSS 解析 ss:// URI
// 格式: ss://BASE64(method:password)@server:port[?plugin=...][#name]  (SIP002)
// 或:   ss://BASE64(method:password@server:port)[?plugin=...][#name]
//
// 关键:SIP002 允许在 server:port 之后接 `?plugin=obfs-local;obfs=tls`
// 等查询参数。早期实现把 `port?plugin=...` 一起送进 parseHostPort 导致
// 端口解析失败、整条节点丢失 (#M17)。现在先把 `?query` 剥出来,再交给
// parseHostPort,并把 query 参数收进 node.Extra 供上层消费。
func parseSS(uri string) (NodeConfig, error) {
	node := NodeConfig{Type: "shadowsocks", Extra: map[string]string{}}

	// 提取名称 (#name) — 必须先于 query,因为 name 在最末尾
	if idx := strings.LastIndex(uri, "#"); idx != -1 {
		node.Name = decodeURIComponent(uri[idx+1:])
		uri = uri[:idx]
	}

	body := strings.TrimPrefix(uri, "ss://")

	// 剥离 query string (含 plugin / type 等),避免 server:port 解析受污染
	queryStr := ""
	if idx := strings.Index(body, "?"); idx != -1 {
		queryStr = body[idx+1:]
		body = body[:idx]
	}

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

	if queryStr != "" {
		params := parseQuery(queryStr)
		for k, v := range params {
			node.Extra[k] = v
		}
		if t, ok := params["type"]; ok {
			node.Network = t
		}
		if plugin, ok := params["plugin"]; ok {
			node.Extra["plugin"] = decodeURIComponent(plugin)
		}
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

// parseTuic 解析 tuic:// URI (TUIC v5)
// 格式: tuic://UUID[:TOKEN]@server:port?sni=&congestion_control=&udp_relay_mode=&alpn=&allow_insecure=&disable_sni=#name
//
// 参考:
//   - NekoBox `~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/fmt/tuic/TuicFmt.kt:parseTuic`
//   - sing-box TUIC outbound: https://sing-box.sagernet.org/configuration/outbound/tuic/
//
// 关键细节: userinfo 形如 `uuid:token`, 二者均 URL-decoded。也支持 "uuid@host" + 把
// token 放 query 参数里的变体(部分机场),但主流是冒号分隔。
func parseTuic(uri string) (NodeConfig, error) {
	node := NodeConfig{Type: "tuic", TLS: true, Extra: map[string]string{}}

	// 名称
	if idx := strings.LastIndex(uri, "#"); idx != -1 {
		node.Name = decodeURIComponent(uri[idx+1:])
		uri = uri[:idx]
	}

	body := strings.TrimPrefix(uri, "tuic://")

	queryStr := ""
	if idx := strings.Index(body, "?"); idx != -1 {
		queryStr = body[idx+1:]
		body = body[:idx]
	}
	body = strings.TrimRight(body, "/")

	atIdx := strings.LastIndex(body, "@")
	if atIdx == -1 {
		return node, fmt.Errorf("tuic URI 无 @")
	}
	userinfo := decodeURIComponent(body[:atIdx])
	server, port, err := parseHostPort(body[atIdx+1:])
	if err != nil {
		return node, err
	}
	node.Server = server
	node.Port = port

	if colon := strings.Index(userinfo, ":"); colon != -1 {
		node.UUID = userinfo[:colon]
		node.Password = userinfo[colon+1:]
	} else {
		node.UUID = userinfo
	}

	if queryStr != "" {
		params := parseQuery(queryStr)
		if sni, ok := params["sni"]; ok && sni != "" {
			node.SNI = decodeURIComponent(sni)
		}
		if cc, ok := params["congestion_control"]; ok && cc != "" {
			node.Extra["congestion_control"] = cc
		}
		if urm, ok := params["udp_relay_mode"]; ok && urm != "" {
			node.Extra["udp_relay_mode"] = urm
		}
		if alpn, ok := params["alpn"]; ok && alpn != "" {
			node.Extra["alpn"] = decodeURIComponent(alpn)
		}
		if ai, ok := params["allow_insecure"]; ok && (ai == "1" || ai == "true") {
			node.Extra["allow_insecure"] = "true"
		}
		if ds, ok := params["disable_sni"]; ok && (ds == "1" || ds == "true") {
			node.Extra["disable_sni"] = "true"
		}
		// 非标变体: token 放 query
		if tok, ok := params["password"]; ok && node.Password == "" && tok != "" {
			node.Password = decodeURIComponent(tok)
		}
		if tok, ok := params["token"]; ok && node.Password == "" && tok != "" {
			node.Password = decodeURIComponent(tok)
		}
	}

	if node.UUID == "" {
		return node, fmt.Errorf("tuic URI 缺 uuid")
	}
	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", node.Server, node.Port)
	}
	return node, nil
}

// parseHysteria1 解析 hysteria:// URI (Hysteria v1, 非 hy2)
// 格式: hysteria://host:port?auth=&peer=&insecure=&upmbps=&downmbps=&alpn=&obfs=&obfsParam=&protocol=&mport=#name
//
// 关键区别 vs Hysteria2:
//   - auth 在 query 而非 userinfo
//   - up_mbps / down_mbps 必填
//   - obfs 是 "xplus" 等,obfsParam 才是密码
//
// 参考: NekoBox `io/nekohasekai/sagernet/fmt/hysteria/HysteriaFmt.kt:parseHysteria1`
func parseHysteria1(uri string) (NodeConfig, error) {
	node := NodeConfig{Type: "hysteria", TLS: true, Extra: map[string]string{}}

	if idx := strings.LastIndex(uri, "#"); idx != -1 {
		node.Name = decodeURIComponent(uri[idx+1:])
		uri = uri[:idx]
	}

	body := strings.TrimPrefix(uri, "hysteria://")
	queryStr := ""
	if idx := strings.Index(body, "?"); idx != -1 {
		queryStr = body[idx+1:]
		body = body[:idx]
	}
	body = strings.TrimRight(body, "/")

	// Hysteria v1 URI 不一定有 @: 通常是 host:port 开头
	hostPart := body
	if atIdx := strings.LastIndex(body, "@"); atIdx != -1 {
		// 某些变体把 auth 塞 userinfo
		node.Extra["auth"] = decodeURIComponent(body[:atIdx])
		hostPart = body[atIdx+1:]
	}
	server, port, err := parseHostPort(hostPart)
	if err != nil {
		return node, err
	}
	node.Server = server
	node.Port = port

	if queryStr != "" {
		params := parseQuery(queryStr)
		if mport, ok := params["mport"]; ok && mport != "" {
			node.Extra["hop_ports"] = mport
		}
		if peer, ok := params["peer"]; ok && peer != "" {
			node.SNI = decodeURIComponent(peer)
		}
		if auth, ok := params["auth"]; ok && auth != "" {
			node.Extra["auth"] = decodeURIComponent(auth)
		}
		if alpn, ok := params["alpn"]; ok && alpn != "" {
			node.Extra["alpn"] = decodeURIComponent(alpn)
		}
		if obfs, ok := params["obfs"]; ok && obfs != "" {
			node.Extra["obfs"] = obfs
		}
		if op, ok := params["obfsParam"]; ok && op != "" {
			node.Extra["obfs_param"] = decodeURIComponent(op)
		}
		if up, ok := params["upmbps"]; ok && up != "" {
			node.Extra["up_mbps"] = up
		}
		if down, ok := params["downmbps"]; ok && down != "" {
			node.Extra["down_mbps"] = down
		}
		if proto, ok := params["protocol"]; ok && proto != "" {
			node.Extra["protocol"] = proto
		}
		if ins, ok := params["insecure"]; ok && (ins == "1" || ins == "true") {
			node.Extra["insecure"] = "true"
		}
	}

	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", node.Server, node.Port)
	}
	return node, nil
}

// parseAnyTLS 解析 anytls:// URI
// 格式: anytls://password@server:port?sni=&insecure=&fp=#name
// 规范:  https://github.com/anytls/anytls-go/blob/main/docs/uri_scheme.md
//
// 参考: NekoBox `moe/matsuri/nb4a/proxy/anytls/AnyTLSFmt.kt:parseAnytls`
func parseAnyTLS(uri string) (NodeConfig, error) {
	node := NodeConfig{Type: "anytls", TLS: true, Extra: map[string]string{}}

	if idx := strings.LastIndex(uri, "#"); idx != -1 {
		node.Name = decodeURIComponent(uri[idx+1:])
		uri = uri[:idx]
	}

	body := strings.TrimPrefix(uri, "anytls://")
	queryStr := ""
	if idx := strings.Index(body, "?"); idx != -1 {
		queryStr = body[idx+1:]
		body = body[:idx]
	}
	body = strings.TrimRight(body, "/")

	atIdx := strings.LastIndex(body, "@")
	if atIdx == -1 {
		return node, fmt.Errorf("anytls URI 无 @")
	}
	node.Password = decodeURIComponent(body[:atIdx])
	server, port, err := parseHostPort(body[atIdx+1:])
	if err != nil {
		return node, err
	}
	node.Server = server
	node.Port = port

	if queryStr != "" {
		params := parseQuery(queryStr)
		if sni, ok := params["sni"]; ok && sni != "" {
			node.SNI = decodeURIComponent(sni)
		}
		if ins, ok := params["insecure"]; ok && (ins == "1" || ins == "true") {
			node.Extra["insecure"] = "true"
		}
		if fp, ok := params["fp"]; ok && fp != "" {
			node.Extra["fingerprint"] = fp
		}
		if alpn, ok := params["alpn"]; ok && alpn != "" {
			node.Extra["alpn"] = decodeURIComponent(alpn)
		}
	}

	if node.Password == "" {
		return node, fmt.Errorf("anytls URI 缺 password")
	}
	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", node.Server, node.Port)
	}
	return node, nil
}

// parseShadowTLS 解析 shadowtls:// URI
// 格式: shadowtls://password@server:port?version=3&host=handshake.sni&fp=chrome#name
//
// 注意: NekoBox 本身不提供 shadowtls URI 解析(只有设置 UI + 内部 Bean 往 sing-box
// 写配置)。 sing-box `shadowtls` outbound 官方文档:
//
//	https://sing-box.sagernet.org/configuration/outbound/shadowtls/
//
// 真实世界部署 90% 走 Clash YAML(M11 覆盖)或 sing-box JSON(M12 覆盖), 独立 URI
// 粘贴场景少见。这里按"shadowtls 作为一条独立 URI"的合理约定实现,如果生态里冒出
// 别的非标格式,按需在 Extra 增字段即可。
func parseShadowTLS(uri string) (NodeConfig, error) {
	node := NodeConfig{Type: "shadowtls", TLS: true, Extra: map[string]string{}}

	if idx := strings.LastIndex(uri, "#"); idx != -1 {
		node.Name = decodeURIComponent(uri[idx+1:])
		uri = uri[:idx]
	}

	body := strings.TrimPrefix(uri, "shadowtls://")
	queryStr := ""
	if idx := strings.Index(body, "?"); idx != -1 {
		queryStr = body[idx+1:]
		body = body[:idx]
	}
	body = strings.TrimRight(body, "/")

	atIdx := strings.LastIndex(body, "@")
	if atIdx == -1 {
		return node, fmt.Errorf("shadowtls URI 无 @")
	}
	node.Password = decodeURIComponent(body[:atIdx])
	server, port, err := parseHostPort(body[atIdx+1:])
	if err != nil {
		return node, err
	}
	node.Server = server
	node.Port = port

	if queryStr != "" {
		params := parseQuery(queryStr)
		if v, ok := params["version"]; ok && v != "" {
			node.Extra["version"] = v
		}
		if host, ok := params["host"]; ok && host != "" {
			// "host" 或 "sni" 均指 handshake 服务器 SNI
			node.SNI = decodeURIComponent(host)
		} else if sni, ok := params["sni"]; ok && sni != "" {
			node.SNI = decodeURIComponent(sni)
		}
		if ins, ok := params["insecure"]; ok && (ins == "1" || ins == "true") {
			node.Extra["insecure"] = "true"
		}
		if fp, ok := params["fp"]; ok && fp != "" {
			node.Extra["fingerprint"] = fp
		}
		if alpn, ok := params["alpn"]; ok && alpn != "" {
			node.Extra["alpn"] = decodeURIComponent(alpn)
		}
	}

	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", node.Server, node.Port)
	}
	// version 缺省为 3 (ShadowTLS v3 是当前主流)
	if _, ok := node.Extra["version"]; !ok {
		node.Extra["version"] = "3"
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
