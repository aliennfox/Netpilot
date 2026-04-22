package subscription

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Clash / Clash.Meta / Mihomo YAML 订阅解析。
//
// 参考:
//   - Clash.Meta 配置文档: https://wiki.metacubex.one/config/proxies/
//   - NekoBox 实现: ~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/group/RawUpdater.kt:227-243
//     (SnakeYAML 检测 `proxies:` 顶层 key → 遍历 map → 分 type 映射到 Bean)
//
// 本文件覆盖的协议类型:
//   ss (shadowsocks), vmess, vless, trojan, hysteria2, hysteria (v1), tuic, anytls,
//   shadowtls, wireguard, socks5/socks, http(s)
// 明确不覆盖(对应 gap doc W11-W15):
//   ssr, mieru, naive, ssh, trojan-go

// clashRoot 只 Unmarshal proxies 数组, 其余 Clash 全局配置(rules/dns/...)我们不关心
type clashRoot struct {
	Proxies []map[string]interface{} `yaml:"proxies"`
}

// IsClashYAML 检测原文是否为 Clash 订阅格式。
//
// 判定逻辑:整份文本里存在无缩进的 `proxies:` 或 `proxy-providers:` 顶层 key,
// 同时文本看起来像 YAML(首个非空非注释行必须是合法顶层键,不能是 base64/URI/JSON)。
//
// 不能只靠 `strings.Contains(raw, "proxies:")`: base64 解码后的 URI 列表或某些
// Trojan/VLESS URL 里碰巧含该字节序列会误伤。 所以要求"YAML 形状 + 存在 proxies 顶层 key"。
func IsClashYAML(raw string) bool {
	firstKeyChecked := false
	hasProxies := false
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// 第一个非空非注释行必须像 YAML 顶层:"key:" 开头或 "key: value"
		if !firstKeyChecked {
			firstKeyChecked = true
			if !looksLikeYAMLToplevelKey(line) {
				return false
			}
		}
		// 无缩进 + 含冒号的顶层 key
		if strings.HasPrefix(line, "proxies:") || strings.HasPrefix(line, "proxy-providers:") {
			hasProxies = true
			return true
		}
	}
	return hasProxies
}

// looksLikeYAMLToplevelKey 判断一行是否像顶层 YAML key(用于拒绝 base64 / URI / JSON)
func looksLikeYAMLToplevelKey(line string) bool {
	// JSON 开头 `{` → 不是 Clash YAML
	if strings.HasPrefix(strings.TrimLeft(line, " \t"), "{") {
		return false
	}
	// 无缩进时首字符必须是字母 / `-`(YAML 列表首项)
	if len(line) == 0 {
		return false
	}
	c := line[0]
	if c == ' ' || c == '\t' {
		// 有缩进的行不是顶层 key, 但可能只是 yaml 子层; 不足以否定整个文件
		return true
	}
	// 顶层必须是 key: 语法, 最起码包含冒号
	if !strings.Contains(line, ":") {
		return false
	}
	// 字母/数字/连字符/下划线开头都接受
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
		return true
	}
	return false
}

// ParseClashYAML 解析 Clash / Clash.Meta YAML, 返回 NodeConfig 列表
func ParseClashYAML(data []byte) ([]NodeConfig, error) {
	var root clashRoot
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("Clash YAML 解析失败: %w", err)
	}
	if len(root.Proxies) == 0 {
		// 兜底: 某些订阅把 proxies 嵌在 proxy-providers 里, 当前不解析此路径
		return nil, fmt.Errorf("Clash YAML 未找到 proxies 数组或为空")
	}

	var nodes []NodeConfig
	for i, p := range root.Proxies {
		node, err := clashProxyToNode(p)
		if err != nil {
			// 单条失败不中断整包, 但记录 stderr(与 base64 URI 列表行为一致)
			name := clashStr(p, "name")
			fmt.Printf("  ⚠️  Clash proxy[%d] %q 跳过: %v\n", i, name, err)
			continue
		}
		node.IsInfoEntry = isInfoEntry(node.Name)
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("Clash YAML 解析 0 个有效节点")
	}
	return nodes, nil
}

// clashProxyToNode 分 type dispatch
func clashProxyToNode(p map[string]interface{}) (NodeConfig, error) {
	typ := clashStr(p, "type")
	switch typ {
	case "ss", "shadowsocks":
		return clashSS(p)
	case "vmess":
		return clashVMess(p)
	case "vless":
		return clashVLess(p)
	case "trojan":
		return clashTrojan(p)
	case "hysteria2", "hy2":
		return clashHysteria2(p)
	case "hysteria":
		return clashHysteria1(p)
	case "tuic":
		return clashTuic(p)
	case "anytls":
		return clashAnyTLS(p)
	case "shadowtls":
		return clashShadowTLS(p)
	case "wireguard", "wg":
		return clashWireGuard(p)
	case "ssh":
		return clashSSH(p)
	case "naive":
		return clashNaive(p)
	case "socks5", "socks":
		return NodeConfig{}, fmt.Errorf("socks 暂未实现 (S12)")
	case "http", "https":
		return NodeConfig{}, fmt.Errorf("http(s) 暂未实现 (S12)")
	case "ssr":
		return NodeConfig{}, errProtocolUnsupported("ssr")
	case "mieru", "trojan-go", "juicity":
		return NodeConfig{}, errProtocolUnsupported(typ)
	default:
		return NodeConfig{}, fmt.Errorf("未知 Clash type: %s", typ)
	}
}

// errProtocolUnsupported Phase 9 A4: 上游 sing-box 不原生支持 + 咱们本阶段不魔改 → 清晰跳过。
func errProtocolUnsupported(proto string) error {
	return fmt.Errorf("协议 %s 本版本暂不支持 (sing-box 上游无原生实现, 跳过)", proto)
}

// clashSSH Clash YAML: type=ssh → NodeConfig (Phase 9 A2)
func clashSSH(p map[string]interface{}) (NodeConfig, error) {
	node := NodeConfig{
		Type:     "ssh",
		Name:     clashStr(p, "name"),
		Server:   clashStr(p, "server"),
		Port:     clashInt(p, "port"),
		Password: clashStr(p, "password"),
		Extra:    map[string]string{},
	}
	if user := clashStr(p, "username"); user != "" {
		node.Extra["user"] = user
	}
	for _, k := range []string{"private_key", "private_key_path", "private_key_passphrase",
		"host_key", "host_key_algorithms", "client_version"} {
		if v := clashStr(p, k); v != "" {
			node.Extra[k] = v
		}
	}
	return node, nil
}

// clashNaive Clash YAML: type=naive → NodeConfig (Phase 9 A3)
func clashNaive(p map[string]interface{}) (NodeConfig, error) {
	node := NodeConfig{
		Type:     "naive",
		Name:     clashStr(p, "name"),
		Server:   clashStr(p, "server"),
		Port:     clashInt(p, "port"),
		Password: clashStr(p, "password"),
		SNI:      clashStr(p, "sni"),
		Extra:    map[string]string{"tls": "true"},
	}
	if user := clashStr(p, "username"); user != "" {
		node.Extra["username"] = user
	}
	return node, nil
}

// --- 各类型映射 --- //

func clashSS(p map[string]interface{}) (NodeConfig, error) {
	node := NodeConfig{
		Type:     "shadowsocks",
		Name:     clashStr(p, "name"),
		Server:   clashStr(p, "server"),
		Port:     clashInt(p, "port"),
		Method:   clashStr(p, "cipher"),
		Password: clashStr(p, "password"),
		Extra:    map[string]string{},
	}
	if node.Server == "" || node.Port == 0 {
		return node, fmt.Errorf("ss server/port 缺失")
	}
	// 插件信息透传 (sing-box SS outbound 当前 converter 没用, 保留用于未来)
	if plugin := clashStr(p, "plugin"); plugin != "" {
		node.Extra["plugin"] = plugin
	}
	return node, nil
}

func clashVMess(p map[string]interface{}) (NodeConfig, error) {
	node := NodeConfig{
		Type:    "vmess",
		Name:    clashStr(p, "name"),
		Server:  clashStr(p, "server"),
		Port:    clashInt(p, "port"),
		UUID:    clashStr(p, "uuid"),
		AlterId: clashInt(p, "alterId"),
		Network: clashStr(p, "network"),
		TLS:     clashBool(p, "tls"),
		Extra:   map[string]string{},
	}
	if sni := clashStr(p, "servername"); sni != "" {
		node.SNI = sni
	} else if sni := clashStr(p, "sni"); sni != "" {
		node.SNI = sni
	}
	extractWSOpts(p, &node)
	extractGRPCOpts(p, &node)
	return node, nil
}

func clashVLess(p map[string]interface{}) (NodeConfig, error) {
	node := NodeConfig{
		Type:    "vless",
		Name:    clashStr(p, "name"),
		Server:  clashStr(p, "server"),
		Port:    clashInt(p, "port"),
		UUID:    clashStr(p, "uuid"),
		Network: clashStr(p, "network"),
		TLS:     clashBool(p, "tls"),
		Extra:   map[string]string{},
	}
	if sni := clashStr(p, "servername"); sni != "" {
		node.SNI = sni
	} else if sni := clashStr(p, "sni"); sni != "" {
		node.SNI = sni
	}
	if flow := clashStr(p, "flow"); flow != "" {
		node.Extra["flow"] = flow
	}
	if fp := clashStr(p, "client-fingerprint"); fp != "" {
		node.Extra["fingerprint"] = fp
	}
	// reality-opts: { public-key, short-id }
	if opts := clashMap(p, "reality-opts"); opts != nil {
		if pk := clashStr(opts, "public-key"); pk != "" {
			node.Extra["public_key"] = pk
			node.TLS = true
		}
		if sid := clashStr(opts, "short-id"); sid != "" {
			node.Extra["short_id"] = sid
		}
	}
	extractWSOpts(p, &node)
	extractGRPCOpts(p, &node)
	return node, nil
}

func clashTrojan(p map[string]interface{}) (NodeConfig, error) {
	node := NodeConfig{
		Type:     "trojan",
		Name:     clashStr(p, "name"),
		Server:   clashStr(p, "server"),
		Port:     clashInt(p, "port"),
		Password: clashStr(p, "password"),
		SNI:      clashStr(p, "sni"),
		TLS:      true,
		Network:  clashStr(p, "network"),
		Extra:    map[string]string{},
	}
	extractWSOpts(p, &node)
	extractGRPCOpts(p, &node)
	if clashBool(p, "skip-cert-verify") {
		node.Extra["insecure"] = "true"
	}
	return node, nil
}

func clashHysteria2(p map[string]interface{}) (NodeConfig, error) {
	node := NodeConfig{
		Type:     "hysteria2",
		Name:     clashStr(p, "name"),
		Server:   clashStr(p, "server"),
		Port:     clashInt(p, "port"),
		Password: clashStr(p, "password"),
		SNI:      clashStr(p, "sni"),
		TLS:      true,
		Extra:    map[string]string{},
	}
	if obfs := clashStr(p, "obfs"); obfs != "" {
		node.Extra["obfs"] = obfs
	}
	if pw := clashStr(p, "obfs-password"); pw != "" {
		node.Extra["obfs_password"] = pw
	}
	if clashBool(p, "skip-cert-verify") {
		node.Extra["insecure"] = "true"
	}
	return node, nil
}

func clashHysteria1(p map[string]interface{}) (NodeConfig, error) {
	node := NodeConfig{
		Type: "hysteria", Name: clashStr(p, "name"), Server: clashStr(p, "server"),
		Port: clashInt(p, "port"), TLS: true, Extra: map[string]string{},
	}
	if sni := clashStr(p, "sni"); sni != "" {
		node.SNI = sni
	}
	if auth := clashStr(p, "auth-str"); auth != "" {
		node.Extra["auth"] = auth
	} else if auth := clashStr(p, "auth_str"); auth != "" {
		node.Extra["auth"] = auth
	}
	if up := clashInt(p, "up"); up > 0 {
		node.Extra["up_mbps"] = strconv.Itoa(up)
	}
	if up := clashStr(p, "up"); up != "" && node.Extra["up_mbps"] == "" {
		node.Extra["up_mbps"] = stripMbps(up)
	}
	if down := clashInt(p, "down"); down > 0 {
		node.Extra["down_mbps"] = strconv.Itoa(down)
	}
	if down := clashStr(p, "down"); down != "" && node.Extra["down_mbps"] == "" {
		node.Extra["down_mbps"] = stripMbps(down)
	}
	if obfs := clashStr(p, "obfs"); obfs != "" {
		node.Extra["obfs"] = obfs
	}
	if alpn := clashList(p, "alpn"); len(alpn) > 0 {
		node.Extra["alpn"] = strings.Join(alpn, ",")
	}
	if clashBool(p, "skip-cert-verify") {
		node.Extra["insecure"] = "true"
	}
	return node, nil
}

func clashTuic(p map[string]interface{}) (NodeConfig, error) {
	node := NodeConfig{
		Type: "tuic", Name: clashStr(p, "name"), Server: clashStr(p, "server"),
		Port: clashInt(p, "port"),
		UUID: clashStr(p, "uuid"), Password: clashStr(p, "password"),
		SNI: clashStr(p, "sni"), TLS: true, Extra: map[string]string{},
	}
	// Clash Meta 用 congestion-controller 或 congestion_control
	if cc := clashStr(p, "congestion-controller"); cc != "" {
		node.Extra["congestion_control"] = cc
	} else if cc := clashStr(p, "congestion_control"); cc != "" {
		node.Extra["congestion_control"] = cc
	}
	if urm := clashStr(p, "udp-relay-mode"); urm != "" {
		node.Extra["udp_relay_mode"] = urm
	}
	if alpn := clashList(p, "alpn"); len(alpn) > 0 {
		node.Extra["alpn"] = strings.Join(alpn, ",")
	}
	if clashBool(p, "skip-cert-verify") {
		node.Extra["allow_insecure"] = "true"
	}
	if clashBool(p, "disable-sni") {
		node.Extra["disable_sni"] = "true"
	}
	return node, nil
}

func clashAnyTLS(p map[string]interface{}) (NodeConfig, error) {
	node := NodeConfig{
		Type: "anytls", Name: clashStr(p, "name"), Server: clashStr(p, "server"),
		Port:     clashInt(p, "port"),
		Password: clashStr(p, "password"), SNI: clashStr(p, "sni"),
		TLS: true, Extra: map[string]string{},
	}
	if fp := clashStr(p, "client-fingerprint"); fp != "" {
		node.Extra["fingerprint"] = fp
	}
	if alpn := clashList(p, "alpn"); len(alpn) > 0 {
		node.Extra["alpn"] = strings.Join(alpn, ",")
	}
	if clashBool(p, "skip-cert-verify") {
		node.Extra["insecure"] = "true"
	}
	return node, nil
}

func clashShadowTLS(p map[string]interface{}) (NodeConfig, error) {
	// Clash.Meta 里 shadowtls 通常作为 SS 的 plugin, 但也有顶层写法。 这里处理顶层。
	node := NodeConfig{
		Type: "shadowtls", Name: clashStr(p, "name"),
		Server: clashStr(p, "server"), Port: clashInt(p, "port"),
		Password: clashStr(p, "password"), SNI: clashStr(p, "sni"),
		TLS: true, Extra: map[string]string{},
	}
	if v := clashInt(p, "version"); v > 0 {
		node.Extra["version"] = strconv.Itoa(v)
	} else {
		node.Extra["version"] = "3"
	}
	if fp := clashStr(p, "client-fingerprint"); fp != "" {
		node.Extra["fingerprint"] = fp
	}
	return node, nil
}

func clashWireGuard(p map[string]interface{}) (NodeConfig, error) {
	node := NodeConfig{
		Type: "wireguard", Name: clashStr(p, "name"),
		Server: clashStr(p, "server"), Port: clashInt(p, "port"),
		Extra: map[string]string{},
	}
	if pk := clashStr(p, "private-key"); pk != "" {
		node.Extra["private_key"] = pk
	}
	if pk := clashStr(p, "public-key"); pk != "" {
		node.Extra["peer_public_key"] = pk
	}
	if psk := clashStr(p, "preshared-key"); psk != "" {
		node.Extra["pre_shared_key"] = psk
	}
	if ip := clashStr(p, "ip"); ip != "" {
		node.Extra["local_address"] = ip
	}
	if mtu := clashInt(p, "mtu"); mtu > 0 {
		node.Extra["mtu"] = strconv.Itoa(mtu)
	}
	if reserved := clashList(p, "reserved"); len(reserved) > 0 {
		node.Extra["reserved"] = strings.Join(reserved, ",")
	}
	return node, nil
}

// --- YAML 取值辅助(兼容 int / string / bool 的 yaml.v3 反序列化) --- //

func clashStr(p map[string]interface{}, key string) string {
	v, ok := p[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func clashInt(p map[string]interface{}, key string) int {
	v, ok := p[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
	}
	return 0
}

func clashBool(p map[string]interface{}, key string) bool {
	v, ok := p[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		b, _ := strconv.ParseBool(t)
		return b
	case int:
		return t != 0
	}
	return false
}

func clashMap(p map[string]interface{}, key string) map[string]interface{} {
	v, ok := p[key]
	if !ok || v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	// yaml.v3 默认反序列化嵌套 map 为 map[string]interface{},但也兜底处理一下
	if m, ok := v.(map[interface{}]interface{}); ok {
		out := map[string]interface{}{}
		for k, val := range m {
			out[fmt.Sprintf("%v", k)] = val
		}
		return out
	}
	return nil
}

func clashList(p map[string]interface{}, key string) []string {
	v, ok := p[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []interface{}:
		var out []string
		for _, item := range t {
			out = append(out, fmt.Sprintf("%v", item))
		}
		return out
	case []string:
		return t
	case string:
		if t != "" {
			return []string{t}
		}
	}
	return nil
}

// extractWSOpts 把 Clash ws-opts 映射到 NodeConfig.Path/Host
func extractWSOpts(p map[string]interface{}, node *NodeConfig) {
	if node.Network != "ws" {
		return
	}
	opts := clashMap(p, "ws-opts")
	if opts == nil {
		return
	}
	if path := clashStr(opts, "path"); path != "" {
		node.Path = path
	}
	if headers := clashMap(opts, "headers"); headers != nil {
		if host := clashStr(headers, "Host"); host != "" {
			node.Host = host
		} else if host := clashStr(headers, "host"); host != "" {
			node.Host = host
		}
	}
}

// extractGRPCOpts 把 Clash grpc-opts.grpc-service-name 映射到 NodeConfig.Extra["service_name"]
func extractGRPCOpts(p map[string]interface{}, node *NodeConfig) {
	if node.Network != "grpc" {
		return
	}
	opts := clashMap(p, "grpc-opts")
	if opts == nil {
		return
	}
	if sn := clashStr(opts, "grpc-service-name"); sn != "" {
		if node.Extra == nil {
			node.Extra = map[string]string{}
		}
		node.Extra["service_name"] = sn
	}
}

// stripMbps "100 Mbps" / "100mbps" / "100" → "100"
func stripMbps(s string) string {
	s = strings.TrimSpace(s)
	for _, suffix := range []string{" Mbps", "Mbps", " mbps", "mbps", "M", "m"} {
		s = strings.TrimSuffix(s, suffix)
	}
	return strings.TrimSpace(s)
}
