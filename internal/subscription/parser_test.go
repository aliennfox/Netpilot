package subscription

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

// TestParseSS 覆盖 Known Issue #M17 的回归:SIP002 query string 存在时,
// 早期实现把 `server:port?plugin=...` 整体送进 parseHostPort,strconv.Atoi
// 失败后整条节点被 parseLine 的 switch 外层吞掉 → 用户订阅丢节点。
//
// 三种常见格式覆盖:
//  1. 纯粹 SIP002: ss://BASE64(method:password)@host:port#name
//  2. 带 plugin query: ss://BASE64(method:password)@host:port?plugin=...&type=tcp#name
//  3. 旧式全 base64: ss://BASE64(method:password@host:port)#name
func TestParseSS(t *testing.T) {
	userInfoB64 := func(s string) string {
		return base64.RawStdEncoding.EncodeToString([]byte(s))
	}
	allB64 := func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	}

	tests := []struct {
		name        string
		uri         string
		wantServer  string
		wantPort    int
		wantMethod  string
		wantPwd     string
		wantNetwork string
		wantNameHas string
		wantErr     bool
	}{
		{
			name:        "sip002 basic",
			uri:         "ss://" + userInfoB64("aes-256-gcm:pass123") + "@1.2.3.4:8388#HK-01",
			wantServer:  "1.2.3.4",
			wantPort:    8388,
			wantMethod:  "aes-256-gcm",
			wantPwd:     "pass123",
			wantNameHas: "HK-01",
		},
		{
			name:        "sip002 with type=tcp query (regression #M17)",
			uri:         "ss://" + userInfoB64("aes-256-gcm:pass123") + "@example.com:443?type=tcp#JP-02",
			wantServer:  "example.com",
			wantPort:    443,
			wantMethod:  "aes-256-gcm",
			wantPwd:     "pass123",
			wantNetwork: "tcp",
			wantNameHas: "JP-02",
		},
		{
			name:        "sip002 with plugin query",
			uri:         "ss://" + userInfoB64("chacha20-ietf-poly1305:abc") + "@10.0.0.1:8080?plugin=obfs-local%3Bobfs%3Dtls#SG-03",
			wantServer:  "10.0.0.1",
			wantPort:    8080,
			wantMethod:  "chacha20-ietf-poly1305",
			wantPwd:     "abc",
			wantNameHas: "SG-03",
		},
		{
			name:        "legacy all-base64 form",
			uri:         "ss://" + allB64("aes-256-gcm:secret@node.example.com:8388") + "#US-04",
			wantServer:  "node.example.com",
			wantPort:    8388,
			wantMethod:  "aes-256-gcm",
			wantPwd:     "secret",
			wantNameHas: "US-04",
		},
		{
			name:    "malformed missing port",
			uri:     "ss://" + userInfoB64("aes-256-gcm:pass") + "@host-only",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSS(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (node=%+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Server != tc.wantServer {
				t.Errorf("server: want %q got %q", tc.wantServer, got.Server)
			}
			if got.Port != tc.wantPort {
				t.Errorf("port: want %d got %d", tc.wantPort, got.Port)
			}
			if got.Method != tc.wantMethod {
				t.Errorf("method: want %q got %q", tc.wantMethod, got.Method)
			}
			if got.Password != tc.wantPwd {
				t.Errorf("password: want %q got %q", tc.wantPwd, got.Password)
			}
			if tc.wantNetwork != "" && got.Network != tc.wantNetwork {
				t.Errorf("network: want %q got %q", tc.wantNetwork, got.Network)
			}
			if tc.wantNameHas != "" && got.Name != tc.wantNameHas {
				t.Errorf("name: want %q got %q", tc.wantNameHas, got.Name)
			}
		})
	}
}

// TestParseTuic 覆盖 TUIC v5 URI (M10a)
func TestParseTuic(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		wantServer string
		wantPort   int
		wantUUID   string
		wantPwd    string
		wantSNI    string
		wantCC     string
		wantName   string
		wantErr    bool
	}{
		{
			name:       "uuid:token userinfo",
			uri:        "tuic://11111111-2222-3333-4444-555555555555:secret@tuic.example.com:443?sni=tuic.example.com&congestion_control=bbr&udp_relay_mode=native&alpn=h3#HK-TUIC",
			wantServer: "tuic.example.com",
			wantPort:   443,
			wantUUID:   "11111111-2222-3333-4444-555555555555",
			wantPwd:    "secret",
			wantSNI:    "tuic.example.com",
			wantCC:     "bbr",
			wantName:   "HK-TUIC",
		},
		{
			name:       "uuid-only userinfo, token in query",
			uri:        "tuic://uuid-a@1.2.3.4:12345?token=querytoken&sni=cdn.example.com#JP",
			wantServer: "1.2.3.4",
			wantPort:   12345,
			wantUUID:   "uuid-a",
			wantPwd:    "querytoken",
			wantSNI:    "cdn.example.com",
			wantName:   "JP",
		},
		{
			name:    "missing @ should fail",
			uri:     "tuic://uuid-only.example.com:443",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTuic(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Server != tc.wantServer {
				t.Errorf("server: want %q got %q", tc.wantServer, got.Server)
			}
			if got.Port != tc.wantPort {
				t.Errorf("port: want %d got %d", tc.wantPort, got.Port)
			}
			if got.UUID != tc.wantUUID {
				t.Errorf("uuid: want %q got %q", tc.wantUUID, got.UUID)
			}
			if got.Password != tc.wantPwd {
				t.Errorf("password: want %q got %q", tc.wantPwd, got.Password)
			}
			if tc.wantSNI != "" && got.SNI != tc.wantSNI {
				t.Errorf("sni: want %q got %q", tc.wantSNI, got.SNI)
			}
			if tc.wantCC != "" && got.Extra["congestion_control"] != tc.wantCC {
				t.Errorf("cc: want %q got %q", tc.wantCC, got.Extra["congestion_control"])
			}
			if tc.wantName != "" && got.Name != tc.wantName {
				t.Errorf("name: want %q got %q", tc.wantName, got.Name)
			}
		})
	}
}

// TestConvertTuic 确保 TUIC outbound 的必填字段和 tls 结构正确
func TestConvertTuic(t *testing.T) {
	node := NodeConfig{
		Type:     "tuic",
		Name:     "HK-TUIC",
		Server:   "tuic.example.com",
		Port:     443,
		UUID:     "uuid-xyz",
		Password: "pw",
		SNI:      "tuic.example.com",
		TLS:      true,
		Extra: map[string]string{
			"congestion_control": "bbr",
			"udp_relay_mode":     "native",
			"alpn":               "h3",
			"allow_insecure":     "true",
		},
	}
	ob, err := ConvertToSingboxOutbound(node)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if ob["type"] != "tuic" || ob["uuid"] != "uuid-xyz" || ob["password"] != "pw" {
		t.Errorf("bad base fields: %+v", ob)
	}
	if ob["congestion_control"] != "bbr" || ob["udp_relay_mode"] != "native" {
		t.Errorf("bad tuic fields: %+v", ob)
	}
	tls, ok := ob["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing tls: %+v", ob)
	}
	if tls["server_name"] != "tuic.example.com" || tls["insecure"] != true {
		t.Errorf("bad tls: %+v", tls)
	}
	if alpn, ok := tls["alpn"].([]interface{}); !ok || len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("bad alpn: %+v", tls["alpn"])
	}
}

// TestParseHysteria1 覆盖 Hysteria v1 URI (M10b)
// 参考 NekoBox: hysteria://host:port?auth=&peer=&insecure=&upmbps=&downmbps=&alpn=&obfs=&obfsParam=#name
func TestParseHysteria1(t *testing.T) {
	uri := "hysteria://hy1.example.com:36712?auth=mypass&peer=cloudfront.example.com&insecure=1&upmbps=50&downmbps=200&alpn=h3&obfs=xplus&obfsParam=obfspw#HK-Hy1"
	got, err := parseHysteria1(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Server != "hy1.example.com" || got.Port != 36712 {
		t.Errorf("host/port: %+v", got)
	}
	if got.Extra["auth"] != "mypass" {
		t.Errorf("auth: %q", got.Extra["auth"])
	}
	if got.SNI != "cloudfront.example.com" {
		t.Errorf("sni: %q", got.SNI)
	}
	if got.Extra["up_mbps"] != "50" || got.Extra["down_mbps"] != "200" {
		t.Errorf("bw: up=%q down=%q", got.Extra["up_mbps"], got.Extra["down_mbps"])
	}
	if got.Extra["obfs"] != "xplus" || got.Extra["obfs_param"] != "obfspw" {
		t.Errorf("obfs: %+v", got.Extra)
	}
	if got.Extra["insecure"] != "true" {
		t.Errorf("insecure: %q", got.Extra["insecure"])
	}
	if got.Name != "HK-Hy1" {
		t.Errorf("name: %q", got.Name)
	}
}

// TestConvertHysteria1 验证 sing-box outbound 必填字段(auth_str / up_mbps / down_mbps / tls)
func TestConvertHysteria1(t *testing.T) {
	node := NodeConfig{
		Type: "hysteria", Name: "HK-Hy1", Server: "hy1.example.com", Port: 36712, TLS: true,
		SNI: "cloudfront.example.com",
		Extra: map[string]string{
			"auth": "mypass", "up_mbps": "50", "down_mbps": "200",
			"alpn": "h3", "obfs_param": "obfspw", "insecure": "true",
		},
	}
	ob, err := ConvertToSingboxOutbound(node)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if ob["type"] != "hysteria" {
		t.Errorf("type: %v", ob["type"])
	}
	if ob["auth_str"] != "mypass" {
		t.Errorf("auth_str: %v", ob["auth_str"])
	}
	if ob["up_mbps"] != 50 || ob["down_mbps"] != 200 {
		t.Errorf("bw: up=%v down=%v", ob["up_mbps"], ob["down_mbps"])
	}
	if ob["obfs"] != "obfspw" {
		t.Errorf("obfs: %v", ob["obfs"])
	}
	tls := ob["tls"].(map[string]interface{})
	if tls["insecure"] != true || tls["server_name"] != "cloudfront.example.com" {
		t.Errorf("tls: %+v", tls)
	}
}

// TestParseAnyTLS 覆盖 AnyTLS URI (M10c)
// 规范: https://github.com/anytls/anytls-go/blob/main/docs/uri_scheme.md
func TestParseAnyTLS(t *testing.T) {
	uri := "anytls://mypassword@anytls.example.com:443?sni=anytls.example.com&insecure=0&fp=chrome#AT-01"
	got, err := parseAnyTLS(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Server != "anytls.example.com" || got.Port != 443 {
		t.Errorf("host/port: %+v", got)
	}
	if got.Password != "mypassword" {
		t.Errorf("password: %q", got.Password)
	}
	if got.SNI != "anytls.example.com" {
		t.Errorf("sni: %q", got.SNI)
	}
	if got.Extra["fingerprint"] != "chrome" {
		t.Errorf("fp: %q", got.Extra["fingerprint"])
	}
	if got.Extra["insecure"] == "true" {
		t.Errorf("insecure should be false, got %q", got.Extra["insecure"])
	}
}

func TestConvertAnyTLS(t *testing.T) {
	node := NodeConfig{
		Type: "anytls", Name: "AT-01", Server: "anytls.example.com", Port: 443,
		Password: "mypassword", SNI: "anytls.example.com", TLS: true,
		Extra: map[string]string{"fingerprint": "chrome", "alpn": "h2,http/1.1"},
	}
	ob, err := ConvertToSingboxOutbound(node)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if ob["type"] != "anytls" || ob["password"] != "mypassword" {
		t.Errorf("base: %+v", ob)
	}
	tls := ob["tls"].(map[string]interface{})
	utls, ok := tls["utls"].(map[string]interface{})
	if !ok || utls["fingerprint"] != "chrome" {
		t.Errorf("utls: %+v", tls["utls"])
	}
	alpn, ok := tls["alpn"].([]interface{})
	if !ok || len(alpn) != 2 {
		t.Errorf("alpn: %+v", tls["alpn"])
	}
}

// TestParseShadowTLS 覆盖 ShadowTLS URI (M10d)
// 注: NekoBox 本身没提供 shadowtls:// URI 解析;该格式是按 sing-box outbound schema
// 约定的"合理产物", 真实世界更多通过 Clash YAML / sing-box JSON 传递
func TestParseShadowTLS(t *testing.T) {
	uri := "shadowtls://base64pw@stls.example.com:443?version=3&host=cdn.example.com&fp=chrome#HK-STLS"
	got, err := parseShadowTLS(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Server != "stls.example.com" || got.Port != 443 {
		t.Errorf("host/port: %+v", got)
	}
	if got.Password != "base64pw" {
		t.Errorf("password: %q", got.Password)
	}
	if got.SNI != "cdn.example.com" {
		t.Errorf("sni: %q", got.SNI)
	}
	if got.Extra["version"] != "3" {
		t.Errorf("version: %q", got.Extra["version"])
	}
}

func TestConvertShadowTLS(t *testing.T) {
	node := NodeConfig{
		Type: "shadowtls", Name: "HK-STLS", Server: "stls.example.com", Port: 443,
		Password: "base64pw", SNI: "cdn.example.com", TLS: true,
		Extra: map[string]string{"version": "3", "fingerprint": "chrome"},
	}
	ob, err := ConvertToSingboxOutbound(node)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if ob["type"] != "shadowtls" || ob["version"] != 3 || ob["password"] != "base64pw" {
		t.Errorf("base: %+v", ob)
	}
}

// TestParseVLessReality_Vision 回归确保 VLESS + Reality + Vision 所有字段被捕获
// 且 converter 把 flow / public_key / short_id / fingerprint 正确写入 outbound
func TestParseVLessReality_Vision(t *testing.T) {
	uri := "vless://11111111-2222-3333-4444-555555555555@reality.example.com:443" +
		"?security=reality&sni=www.microsoft.com&type=tcp&flow=xtls-rprx-vision" +
		"&fp=chrome&pbk=rr-pubkey-base64&sid=abcd1234&spx=%2F#HK-Reality"
	got, err := parseVLess(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Extra["flow"] != "xtls-rprx-vision" {
		t.Errorf("flow: %q", got.Extra["flow"])
	}
	if got.Extra["public_key"] != "rr-pubkey-base64" {
		t.Errorf("pbk: %q", got.Extra["public_key"])
	}
	if got.Extra["short_id"] != "abcd1234" {
		t.Errorf("sid: %q", got.Extra["short_id"])
	}
	if got.Extra["fingerprint"] != "chrome" {
		t.Errorf("fp: %q", got.Extra["fingerprint"])
	}
	if got.Extra["spider_x"] != "/" {
		t.Errorf("spider_x: %q", got.Extra["spider_x"])
	}

	ob, err := ConvertToSingboxOutbound(got)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if ob["flow"] != "xtls-rprx-vision" {
		t.Errorf("outbound.flow: %v", ob["flow"])
	}
	tls := ob["tls"].(map[string]interface{})
	reality, ok := tls["reality"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing reality: %+v", tls)
	}
	if reality["public_key"] != "rr-pubkey-base64" || reality["short_id"] != "abcd1234" {
		t.Errorf("reality: %+v", reality)
	}
	utls, ok := tls["utls"].(map[string]interface{})
	if !ok || utls["fingerprint"] != "chrome" {
		t.Errorf("utls: %+v", tls["utls"])
	}
	// spider_x 当前 sing-box v1.13.8 OutboundRealityOptions 无该字段, 刻意不写出 —— 保留在
	// NodeConfig.Extra 里即可, 若未来 sing-box 加字段再 write-through
	if _, has := reality["spider_x"]; has {
		t.Errorf("sing-box v1.13.8 的 OutboundRealityOptions 无 spider_x 字段, converter 不应写出: %+v", reality)
	}
}

// TestParseSS_QueryPreservedInExtra 确保 plugin 等参数保留到 Extra,
// 给后续 converter.go 消费 (尽管当前 SS outbound 没用到, 保留可扩展性)。
func TestParseSS_QueryPreservedInExtra(t *testing.T) {
	userInfoB64 := base64.RawStdEncoding.EncodeToString([]byte("aes-256-gcm:pass"))
	uri := "ss://" + userInfoB64 + "@1.1.1.1:443?plugin=obfs-local%3Bobfs%3Dtls&type=tcp#X"

	got, err := parseSS(uri)
	if err != nil {
		t.Fatalf("parseSS: %v", err)
	}
	if got.Extra["plugin"] == "" {
		t.Errorf("plugin should be preserved in Extra, got %+v", got.Extra)
	}
	if got.Extra["type"] != "tcp" {
		t.Errorf("type should be preserved in Extra, got %q", got.Extra["type"])
	}
}

// TestConvertWireGuardEndpoint 锁 #M25 修复: WG converter 产新 endpoint schema
// (peers[] 数组, server/port/peer_public_key 合并进 peer, 顶层 address/private_key/mtu),
// 不再产老 outbound schema (local_address/peer_public_key)。
func TestConvertWireGuardEndpoint(t *testing.T) {
	node := NodeConfig{
		Type:   "wireguard",
		Name:   "WG-JP",
		Server: "1.2.3.4",
		Port:   51820,
		Extra: map[string]string{
			"private_key":                   "privkey_base64==",
			"peer_public_key":               "pubkey_base64==",
			"local_address":                 "10.0.0.2/32,fd00::2/128",
			"pre_shared_key":                "psk_base64==",
			"mtu":                           "1408",
			"reserved":                      "1,2,3",
			"persistent_keepalive_interval": "25",
		},
	}
	ep, err := convertWireGuard(node, "WG-JP")
	if err != nil {
		t.Fatalf("convertWireGuard: %v", err)
	}

	// 顶层
	if ep["type"] != "wireguard" || ep["tag"] != "WG-JP" {
		t.Errorf("type/tag wrong: %+v", ep)
	}
	if ep["private_key"] != "privkey_base64==" {
		t.Errorf("private_key missing")
	}
	if ep["mtu"] != 1408 {
		t.Errorf("mtu=%v, want 1408", ep["mtu"])
	}
	// address 是 interface{} slice
	addrs, ok := ep["address"].([]interface{})
	if !ok || len(addrs) != 2 {
		t.Errorf("address: %+v", ep["address"])
	}

	// 老 schema 字段一定不能出现 (会被 sing-box 1.13+ 拒)
	if _, has := ep["server"]; has {
		t.Errorf("server should NOT be at endpoint top level in new schema: %+v", ep)
	}
	if _, has := ep["server_port"]; has {
		t.Errorf("server_port should NOT be at endpoint top level: %+v", ep)
	}
	if _, has := ep["peer_public_key"]; has {
		t.Errorf("peer_public_key should NOT be at endpoint top level: %+v", ep)
	}
	if _, has := ep["local_address"]; has {
		t.Errorf("local_address should be renamed to 'address': %+v", ep)
	}

	// peers 数组
	peers, ok := ep["peers"].([]interface{})
	if !ok || len(peers) != 1 {
		t.Fatalf("peers: %+v", ep["peers"])
	}
	peer, ok := peers[0].(map[string]interface{})
	if !ok {
		t.Fatalf("peer not map: %+v", peers[0])
	}
	if peer["address"] != "1.2.3.4" {
		t.Errorf("peer.address=%v, want 1.2.3.4", peer["address"])
	}
	if peer["port"] != 51820 {
		t.Errorf("peer.port=%v, want 51820", peer["port"])
	}
	if peer["public_key"] != "pubkey_base64==" {
		t.Errorf("peer.public_key missing")
	}
	if peer["pre_shared_key"] != "psk_base64==" {
		t.Errorf("peer.pre_shared_key missing")
	}
	if peer["persistent_keepalive_interval"] != 25 {
		t.Errorf("peer.keepalive=%v, want 25", peer["persistent_keepalive_interval"])
	}
	if ai, ok := peer["allowed_ips"].([]interface{}); !ok || len(ai) != 2 {
		t.Errorf("peer.allowed_ips: %+v", peer["allowed_ips"])
	}
	if res, ok := peer["reserved"].([]interface{}); !ok || len(res) != 3 {
		t.Errorf("peer.reserved: %+v", peer["reserved"])
	}
}

// TestMergeConfigs_WireguardGoesToEndpoints 锁 merger 层: wireguard type 分流到
// merged.json.endpoints, 不进 merged.json.outbounds。 selector 仍拿到 tag 引用。
func TestMergeConfigs_WireguardGoesToEndpoints(t *testing.T) {
	t.Skip("overlay package test, see internal/overlay/merger_test.go (scaffold)")
}

// TestConvertWireGuardAddressCIDRNormalize 锁 #M25 追加修: Clash `ip:` 字段习惯
// 给裸 IP, 但 sing-box endpoint.address 要 netip.Prefix (CIDR)。 converter 必须
// 自动补 /32 (v4) / /128 (v6), 否则 sing-box check 报 `no '/'` 直接挂整份配置。
func TestConvertWireGuardAddressCIDRNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []interface{}
	}{
		{"v4 bare", "10.0.0.2", []interface{}{"10.0.0.2/32"}},
		{"v6 bare", "fd00::2", []interface{}{"fd00::2/128"}},
		{"mixed bare", "10.0.0.2,fd00::2", []interface{}{"10.0.0.2/32", "fd00::2/128"}},
		{"already cidr v4", "10.0.0.2/24", []interface{}{"10.0.0.2/24"}},
		{"already cidr v6", "fd00::2/64", []interface{}{"fd00::2/64"}},
		{"mixed cidr and bare", "10.0.0.2/32,fd00::2", []interface{}{"10.0.0.2/32", "fd00::2/128"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node := NodeConfig{
				Type: "wireguard", Name: "WG", Server: "1.2.3.4", Port: 51820,
				Extra: map[string]string{
					"private_key":     "pk==",
					"peer_public_key": "pub==",
					"local_address":   c.in,
				},
			}
			ep, err := convertWireGuard(node, "WG")
			if err != nil {
				t.Fatalf("convertWireGuard: %v", err)
			}
			got, ok := ep["address"].([]interface{})
			if !ok {
				t.Fatalf("address not slice: %+v", ep["address"])
			}
			if len(got) != len(c.want) {
				t.Fatalf("len=%d want %d, got=%v", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("addr[%d]=%v want %v", i, got[i], c.want[i])
				}
			}
		})
	}
}

// TestParseSSH Phase 9 A2
func TestParseSSH(t *testing.T) {
	uri := "ssh://alice:pass123@10.0.0.5:2222?host_key_algorithms=ssh-ed25519%3Brsa-sha2-256&client_version=SSH-2.0-go#bastion-jp"
	got, err := parseSSH(uri)
	if err != nil {
		t.Fatalf("parseSSH: %v", err)
	}
	if got.Server != "10.0.0.5" || got.Port != 2222 {
		t.Errorf("server/port: %s:%d", got.Server, got.Port)
	}
	if got.Extra["user"] != "alice" || got.Password != "pass123" {
		t.Errorf("user/pass: %+v", got)
	}
	if got.Extra["host_key_algorithms"] != "ssh-ed25519;rsa-sha2-256" {
		t.Errorf("host_key_algorithms: %q", got.Extra["host_key_algorithms"])
	}
	if got.Extra["client_version"] != "SSH-2.0-go" {
		t.Errorf("client_version: %q", got.Extra["client_version"])
	}
	if got.Name != "bastion-jp" {
		t.Errorf("name: %q", got.Name)
	}
}

// TestConvertSSH Phase 9 A2
func TestConvertSSH(t *testing.T) {
	node := NodeConfig{
		Type: "ssh", Server: "host.lan", Port: 22, Password: "p",
		Extra: map[string]string{
			"user":                "ops",
			"private_key_path":    "/data/key",
			"host_key":            "ed25519 AAAA;rsa AAAB",
			"host_key_algorithms": "ssh-ed25519;rsa-sha2-512",
		},
	}
	ob, err := convertSSH(node, "ssh-host")
	if err != nil {
		t.Fatal(err)
	}
	if ob["type"] != "ssh" || ob["server"] != "host.lan" || ob["server_port"] != 22 {
		t.Errorf("base: %+v", ob)
	}
	if ob["user"] != "ops" || ob["password"] != "p" {
		t.Errorf("auth: %+v", ob)
	}
	if ob["private_key_path"] != "/data/key" {
		t.Errorf("private_key_path: %+v", ob)
	}
	// host_key 应被 ; 切成 []string
	hk, ok := ob["host_key"].([]interface{})
	if !ok || len(hk) != 2 {
		t.Errorf("host_key should be 2-elem list: %+v", ob["host_key"])
	}
	hka, ok := ob["host_key_algorithms"].([]interface{})
	if !ok || len(hka) != 2 {
		t.Errorf("host_key_algorithms should be 2-elem list: %+v", ob["host_key_algorithms"])
	}
}

// TestParseNaive Phase 9 A3
func TestParseNaive(t *testing.T) {
	uri := "naive+https://user:pass@jp.example.com:443?sni=example.com#JP-Naive"
	got, err := parseNaive(uri)
	if err != nil {
		t.Fatalf("parseNaive: %v", err)
	}
	if got.Server != "jp.example.com" || got.Port != 443 {
		t.Errorf("server/port: %s:%d", got.Server, got.Port)
	}
	if got.Extra["username"] != "user" || got.Password != "pass" {
		t.Errorf("auth: %+v", got)
	}
	if got.SNI != "example.com" {
		t.Errorf("sni: %q", got.SNI)
	}
	if got.Name != "JP-Naive" {
		t.Errorf("name: %q", got.Name)
	}
}

// TestConvertNaive Phase 9 A3
func TestConvertNaive(t *testing.T) {
	node := NodeConfig{
		Type: "naive", Server: "jp.example.com", Port: 443,
		Password: "pwd", SNI: "cdn.example.com",
		Extra: map[string]string{"username": "bob"},
	}
	ob, err := convertNaive(node, "JP-1")
	if err != nil {
		t.Fatal(err)
	}
	if ob["type"] != "naive" {
		t.Errorf("type: %+v", ob)
	}
	if ob["username"] != "bob" || ob["password"] != "pwd" {
		t.Errorf("auth: %+v", ob)
	}
	tls, ok := ob["tls"].(map[string]interface{})
	if !ok || tls["enabled"] != true || tls["server_name"] != "cdn.example.com" {
		t.Errorf("tls: %+v", tls)
	}
}

// TestParseSocks 覆盖 socks/socks5/socks4/socks4a URI 解析,含 v2rayN base64 兜底
func TestParseSocks(t *testing.T) {
	userInfoB64 := base64.StdEncoding.EncodeToString([]byte("alice:secret"))
	tests := []struct {
		name     string
		uri      string
		server   string
		port     int
		user     string
		password string
		version  string
		label    string
		wantErr  bool
	}{
		{
			name:     "socks5 plain auth",
			uri:      "socks5://alice:secret@1.2.3.4:1080#Home-SOCKS",
			server:   "1.2.3.4",
			port:     1080,
			user:     "alice",
			password: "secret",
			version:  "5",
			label:    "Home-SOCKS",
		},
		{
			name:    "socks no auth, bare host:port",
			uri:     "socks://10.0.0.1:1080#Local",
			server:  "10.0.0.1",
			port:    1080,
			version: "5",
			label:   "Local",
		},
		{
			name:     "socks v2rayN base64 userinfo",
			uri:      "socks://" + userInfoB64 + "@proxy.example.com:1080#v2rayN-style",
			server:   "proxy.example.com",
			port:     1080,
			user:     "alice",
			password: "secret",
			version:  "5",
			label:    "v2rayN-style",
		},
		{
			name:    "socks4 version detection",
			uri:     "socks4://bob@legacy.lan:1080#Legacy",
			server:  "legacy.lan",
			port:    1080,
			user:    "bob",
			version: "4",
			label:   "Legacy",
		},
		{
			name:    "socks4a version detection",
			uri:     "socks4a://remote.lan:1080",
			server:  "remote.lan",
			port:    1080,
			version: "4a",
		},
		{
			name:    "missing port",
			uri:     "socks5://host-only",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSocks(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Server != tc.server || got.Port != tc.port {
				t.Errorf("host/port: want %s:%d got %s:%d", tc.server, tc.port, got.Server, got.Port)
			}
			if got.Extra["username"] != tc.user {
				t.Errorf("user: want %q got %q", tc.user, got.Extra["username"])
			}
			if got.Password != tc.password {
				t.Errorf("password: want %q got %q", tc.password, got.Password)
			}
			if got.Extra["version"] != tc.version {
				t.Errorf("version: want %q got %q", tc.version, got.Extra["version"])
			}
			if tc.label != "" && got.Name != tc.label {
				t.Errorf("name: want %q got %q", tc.label, got.Name)
			}
		})
	}
}

// TestConvertSocks 验证 sing-box outbound 必填字段 + version default
func TestConvertSocks(t *testing.T) {
	// v5 with auth
	node := NodeConfig{
		Type: "socks", Server: "1.2.3.4", Port: 1080,
		Password: "secret", Extra: map[string]string{"username": "alice", "version": "5"},
	}
	ob, err := ConvertToSingboxOutbound(node)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if ob["type"] != "socks" || ob["server"] != "1.2.3.4" || ob["server_port"] != 1080 {
		t.Errorf("base: %+v", ob)
	}
	if ob["version"] != "5" || ob["username"] != "alice" || ob["password"] != "secret" {
		t.Errorf("auth: %+v", ob)
	}

	// v4a without auth, default version applied
	node2 := NodeConfig{
		Type: "socks", Server: "remote.lan", Port: 1080,
		Extra: map[string]string{"version": "4a"},
	}
	ob2, err := ConvertToSingboxOutbound(node2)
	if err != nil {
		t.Fatalf("convert2: %v", err)
	}
	if ob2["version"] != "4a" {
		t.Errorf("v4a: %+v", ob2)
	}
	if _, has := ob2["username"]; has {
		t.Errorf("no auth should not emit username field: %+v", ob2)
	}

	// version defaulted to 5 when Extra 缺失
	node3 := NodeConfig{Type: "socks", Server: "x", Port: 1, Extra: map[string]string{}}
	ob3, err := ConvertToSingboxOutbound(node3)
	if err != nil {
		t.Fatalf("convert3: %v", err)
	}
	if ob3["version"] != "5" {
		t.Errorf("default version should be 5, got %+v", ob3["version"])
	}
}

// TestParseHTTP 覆盖 http:// / https:// 代理 URI,含 userinfo + sni query
func TestParseHTTP(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		server   string
		port     int
		user     string
		password string
		tls      bool
		sni      string
		label    string
		wantErr  bool
	}{
		{
			name:   "http no auth",
			uri:    "http://proxy.corp:8080#Corp",
			server: "proxy.corp",
			port:   8080,
			label:  "Corp",
		},
		{
			name:     "http with auth",
			uri:      "http://alice:secret@10.0.0.2:3128#LAN",
			server:   "10.0.0.2",
			port:     3128,
			user:     "alice",
			password: "secret",
			label:    "LAN",
		},
		{
			name:     "https TLS on",
			uri:      "https://alice:secret@proxy.example.com:443#HTTPS-Proxy",
			server:   "proxy.example.com",
			port:     443,
			user:     "alice",
			password: "secret",
			tls:      true,
			label:    "HTTPS-Proxy",
		},
		{
			name:   "https with sni query",
			uri:    "https://proxy.example.com:443?sni=cdn.example.com#SNI-override",
			server: "proxy.example.com",
			port:   443,
			tls:    true,
			sni:    "cdn.example.com",
			label:  "SNI-override",
		},
		{
			name:    "missing port",
			uri:     "http://host-only",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHTTP(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Server != tc.server || got.Port != tc.port {
				t.Errorf("host/port: want %s:%d got %s:%d", tc.server, tc.port, got.Server, got.Port)
			}
			if got.Extra["username"] != tc.user {
				t.Errorf("user: want %q got %q", tc.user, got.Extra["username"])
			}
			if got.Password != tc.password {
				t.Errorf("password: want %q got %q", tc.password, got.Password)
			}
			if got.TLS != tc.tls {
				t.Errorf("tls: want %v got %v", tc.tls, got.TLS)
			}
			if got.SNI != tc.sni {
				t.Errorf("sni: want %q got %q", tc.sni, got.SNI)
			}
			if tc.label != "" && got.Name != tc.label {
				t.Errorf("name: want %q got %q", tc.label, got.Name)
			}
		})
	}
}

// TestConvertHTTP 验证 sing-box http outbound: TLS 段仅在 node.TLS 时出现
func TestConvertHTTP(t *testing.T) {
	// 明文 http
	plain := NodeConfig{
		Type: "http", Server: "proxy.corp", Port: 8080,
		Password: "pw", Extra: map[string]string{"username": "alice"},
	}
	ob, err := ConvertToSingboxOutbound(plain)
	if err != nil {
		t.Fatalf("convert plain: %v", err)
	}
	if ob["type"] != "http" || ob["server"] != "proxy.corp" || ob["server_port"] != 8080 {
		t.Errorf("base: %+v", ob)
	}
	if ob["username"] != "alice" || ob["password"] != "pw" {
		t.Errorf("auth: %+v", ob)
	}
	if _, has := ob["tls"]; has {
		t.Errorf("明文 http 不应有 tls 块: %+v", ob)
	}

	// https 带 SNI
	tls := NodeConfig{
		Type: "http", Server: "proxy.example.com", Port: 443,
		Password: "pw", SNI: "cdn.example.com", TLS: true,
		Extra: map[string]string{"username": "alice", "insecure": "true"},
	}
	ob2, err := ConvertToSingboxOutbound(tls)
	if err != nil {
		t.Fatalf("convert tls: %v", err)
	}
	tlsMap, ok := ob2["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tls map: %+v", ob2)
	}
	if tlsMap["enabled"] != true || tlsMap["server_name"] != "cdn.example.com" || tlsMap["insecure"] != true {
		t.Errorf("tls: %+v", tlsMap)
	}

	// https 无 SNI 时 server_name 回落到 server
	tls2 := NodeConfig{Type: "http", Server: "proxy.example.com", Port: 443, TLS: true, Extra: map[string]string{}}
	ob3, err := ConvertToSingboxOutbound(tls2)
	if err != nil {
		t.Fatalf("convert tls2: %v", err)
	}
	if tm := ob3["tls"].(map[string]interface{}); tm["server_name"] != "proxy.example.com" {
		t.Errorf("server_name fallback: %+v", tm)
	}
}

// TestParseLine_UnsupportedSkipsGracefully Phase 9 A4: 不支持协议在 parser 层被拒, ParseSubscription 不崩
func TestParseLine_UnsupportedSkipsGracefully(t *testing.T) {
	// base64 一条 ssr + 一条合法 ss 的订阅, 期望保留 ss 跳过 ssr
	// 注意: tryBase64Decode 会 decode, 顶层订阅不加 base64 也行 (明文 URI 列表)
	content := strings.Join([]string{
		"ssr://fake-ssr",
		"ss://" + base64.RawStdEncoding.EncodeToString([]byte("aes-256-gcm:pass")) + "@1.1.1.1:443#TestSS",
		"trojan-go://fake-trojan-go",
		"mieru://fake-mieru",
		"juicity://fake-juicity",
	}, "\n")

	nodes, err := ParseSubscription(content)
	if err != nil {
		t.Fatalf("ParseSubscription: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node (ss), got %d: %+v", len(nodes), nodes)
	}
	if nodes[0].Type != "shadowsocks" || nodes[0].Name != "TestSS" {
		t.Errorf("survivor node wrong: %+v", nodes[0])
	}
}

// ============================================================================
// Trojan
// ============================================================================

func TestParseTrojan(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantServer  string
		wantPort    int
		wantPwd     string
		wantTLS     bool
		wantSNI     string
		wantNetwork string
		wantPath    string
		wantHost    string
		wantNameHas string
		wantErr     bool
	}{
		{
			name:        "basic tls",
			uri:         "trojan://pass123@trojan.example.com:443?sni=trojan.example.com#HK-Trojan",
			wantServer:  "trojan.example.com",
			wantPort:    443,
			wantPwd:     "pass123",
			wantTLS:     true,
			wantSNI:     "trojan.example.com",
			wantNameHas: "HK-Trojan",
		},
		{
			name:        "websocket transport",
			uri:         "trojan://pw@1.2.3.4:443?type=ws&host=cdn.example.com&path=%2Fws&sni=cdn.example.com#JP-WS",
			wantServer:  "1.2.3.4",
			wantPort:    443,
			wantPwd:     "pw",
			wantTLS:     true,
			wantSNI:     "cdn.example.com",
			wantNetwork: "ws",
			wantHost:    "cdn.example.com",
			wantNameHas: "JP-WS",
		},
		{
			name:        "sni falls back to host when omitted",
			uri:         "trojan://pw@5.6.7.8:443?type=ws&host=fallback.example.com#X",
			wantServer:  "5.6.7.8",
			wantPort:    443,
			wantPwd:     "pw",
			wantTLS:     true,
			wantSNI:     "fallback.example.com",
			wantNetwork: "ws",
			wantHost:    "fallback.example.com",
		},
		{
			name:    "missing @ should fail",
			uri:     "trojan://no-at-here.example.com:443",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTrojan(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Server != tc.wantServer {
				t.Errorf("server: want %q got %q", tc.wantServer, got.Server)
			}
			if got.Port != tc.wantPort {
				t.Errorf("port: want %d got %d", tc.wantPort, got.Port)
			}
			if got.Password != tc.wantPwd {
				t.Errorf("password: want %q got %q", tc.wantPwd, got.Password)
			}
			if got.TLS != tc.wantTLS {
				t.Errorf("tls: want %v got %v", tc.wantTLS, got.TLS)
			}
			if tc.wantSNI != "" && got.SNI != tc.wantSNI {
				t.Errorf("sni: want %q got %q", tc.wantSNI, got.SNI)
			}
			if tc.wantNetwork != "" && got.Network != tc.wantNetwork {
				t.Errorf("network: want %q got %q", tc.wantNetwork, got.Network)
			}
			if tc.wantHost != "" && got.Host != tc.wantHost {
				t.Errorf("host: want %q got %q", tc.wantHost, got.Host)
			}
			if tc.wantNameHas != "" && got.Name != tc.wantNameHas {
				t.Errorf("name: want %q got %q", tc.wantNameHas, got.Name)
			}
		})
	}
}

func TestConvertTrojan(t *testing.T) {
	t.Run("tls basic", func(t *testing.T) {
		node := NodeConfig{Type: "trojan", Server: "trojan.example.com", Port: 443, Password: "pass123", TLS: true, SNI: "trojan.example.com"}
		ob, err := convertTrojan(node, "T1")
		if err != nil {
			t.Fatalf("convert: %v", err)
		}
		if ob["type"] != "trojan" || ob["tag"] != "T1" || ob["server"] != "trojan.example.com" || ob["server_port"] != 443 || ob["password"] != "pass123" {
			t.Errorf("base fields wrong: %+v", ob)
		}
		tls, ok := ob["tls"].(map[string]interface{})
		if !ok {
			t.Fatalf("tls block missing: %+v", ob)
		}
		if tls["enabled"] != true || tls["server_name"] != "trojan.example.com" {
			t.Errorf("tls fields: %+v", tls)
		}
	})

	t.Run("ws transport", func(t *testing.T) {
		node := NodeConfig{Type: "trojan", Server: "1.2.3.4", Port: 443, Password: "pw", TLS: true, SNI: "cdn.example.com", Network: "ws", Path: "/ws", Host: "cdn.example.com"}
		ob, err := convertTrojan(node, "T2")
		if err != nil {
			t.Fatalf("convert: %v", err)
		}
		tr, ok := ob["transport"].(map[string]interface{})
		if !ok {
			t.Fatalf("missing transport: %+v", ob)
		}
		if tr["type"] != "ws" || tr["path"] != "/ws" {
			t.Errorf("transport fields: %+v", tr)
		}
		hdrs, ok := tr["headers"].(map[string]interface{})
		if !ok || hdrs["Host"] != "cdn.example.com" {
			t.Errorf("headers: %+v", tr["headers"])
		}
	})

	t.Run("missing password rejected", func(t *testing.T) {
		_, err := convertTrojan(NodeConfig{Type: "trojan", Server: "x", Port: 443}, "X")
		if err == nil {
			t.Fatalf("expected error on empty password")
		}
	})
}

// ============================================================================
// VMess
// ============================================================================

func TestParseVMess(t *testing.T) {
	mkVMess := func(obj map[string]interface{}) string {
		raw, _ := json.Marshal(obj)
		return "vmess://" + base64.StdEncoding.EncodeToString(raw)
	}

	tests := []struct {
		name        string
		uri         string
		wantServer  string
		wantPort    int
		wantUUID    string
		wantNetwork string
		wantTLS     bool
		wantSNI     string
		wantNameHas string
		wantErr     bool
	}{
		{
			name: "ws + tls",
			uri: mkVMess(map[string]interface{}{
				"v": "2", "ps": "HK-VMess", "add": "vmess.example.com", "port": "443",
				"id": "uuid-vmess", "aid": 0, "net": "ws", "host": "vmess.example.com",
				"path": "/path", "tls": "tls", "sni": "vmess.example.com",
			}),
			wantServer:  "vmess.example.com",
			wantPort:    443,
			wantUUID:    "uuid-vmess",
			wantNetwork: "ws",
			wantTLS:     true,
			wantSNI:     "vmess.example.com",
			wantNameHas: "HK-VMess",
		},
		{
			name: "tcp no tls, sni falls back to host when tls=tls but sni omitted (skipped: tls=none so SNI stays empty)",
			uri: mkVMess(map[string]interface{}{
				"ps": "JP", "add": "1.2.3.4", "port": 8080, "id": "u-jp", "net": "tcp", "tls": "",
			}),
			wantServer: "1.2.3.4",
			wantPort:   8080,
			wantUUID:   "u-jp",
			wantTLS:    false,
		},
		{
			name:    "non-base64 should fail",
			uri:     "vmess://not-base64-!@#$%",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseVMess(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Server != tc.wantServer {
				t.Errorf("server: want %q got %q", tc.wantServer, got.Server)
			}
			if got.Port != tc.wantPort {
				t.Errorf("port: want %d got %d", tc.wantPort, got.Port)
			}
			if got.UUID != tc.wantUUID {
				t.Errorf("uuid: want %q got %q", tc.wantUUID, got.UUID)
			}
			if tc.wantNetwork != "" && got.Network != tc.wantNetwork {
				t.Errorf("network: want %q got %q", tc.wantNetwork, got.Network)
			}
			if got.TLS != tc.wantTLS {
				t.Errorf("tls: want %v got %v", tc.wantTLS, got.TLS)
			}
			if tc.wantSNI != "" && got.SNI != tc.wantSNI {
				t.Errorf("sni: want %q got %q", tc.wantSNI, got.SNI)
			}
			if tc.wantNameHas != "" && got.Name != tc.wantNameHas {
				t.Errorf("name: want %q got %q", tc.wantNameHas, got.Name)
			}
		})
	}
}

func TestConvertVMess(t *testing.T) {
	t.Run("ws + tls", func(t *testing.T) {
		node := NodeConfig{Type: "vmess", Server: "vmess.example.com", Port: 443, UUID: "u1", AlterId: 0,
			TLS: true, SNI: "vmess.example.com", Network: "ws", Path: "/path", Host: "vmess.example.com"}
		ob, err := convertVMess(node, "V1")
		if err != nil {
			t.Fatalf("convert: %v", err)
		}
		if ob["uuid"] != "u1" || ob["security"] != "auto" || ob["alter_id"] != 0 {
			t.Errorf("base fields: %+v", ob)
		}
		tls, _ := ob["tls"].(map[string]interface{})
		if tls == nil || tls["enabled"] != true || tls["server_name"] != "vmess.example.com" {
			t.Errorf("tls: %+v", tls)
		}
		tr, _ := ob["transport"].(map[string]interface{})
		if tr == nil || tr["type"] != "ws" || tr["path"] != "/path" {
			t.Errorf("transport: %+v", tr)
		}
	})

	t.Run("missing uuid rejected", func(t *testing.T) {
		_, err := convertVMess(NodeConfig{Type: "vmess", Server: "x", Port: 1}, "X")
		if err == nil {
			t.Fatalf("expected error on empty uuid")
		}
	})
}

// ============================================================================
// VLESS (普通 — Reality 在 TestParseVLessReality_Vision 已锁)
// ============================================================================

func TestParseVLess(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantServer  string
		wantPort    int
		wantUUID    string
		wantNetwork string
		wantTLS     bool
		wantSNI     string
		wantSvcName string
		wantNameHas string
		wantErr     bool
	}{
		{
			name:        "tls basic",
			uri:         "vless://uuid-1@vless.example.com:443?security=tls&sni=vless.example.com&type=tcp#HK-VLess",
			wantServer:  "vless.example.com",
			wantPort:    443,
			wantUUID:    "uuid-1",
			wantNetwork: "tcp",
			wantTLS:     true,
			wantSNI:     "vless.example.com",
			wantNameHas: "HK-VLess",
		},
		{
			name:        "grpc with serviceName",
			uri:         "vless://uuid-2@grpc.example.com:443?security=tls&sni=grpc.example.com&type=grpc&serviceName=mygrpc#JP-gRPC",
			wantServer:  "grpc.example.com",
			wantPort:    443,
			wantUUID:    "uuid-2",
			wantNetwork: "grpc",
			wantTLS:     true,
			wantSNI:     "grpc.example.com",
			wantSvcName: "mygrpc",
		},
		{
			name:    "missing @ should fail",
			uri:     "vless://no-at.example.com:443",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseVLess(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Server != tc.wantServer || got.Port != tc.wantPort || got.UUID != tc.wantUUID {
				t.Errorf("base: server=%q port=%d uuid=%q", got.Server, got.Port, got.UUID)
			}
			if got.Network != tc.wantNetwork {
				t.Errorf("network: want %q got %q", tc.wantNetwork, got.Network)
			}
			if got.TLS != tc.wantTLS {
				t.Errorf("tls: want %v got %v", tc.wantTLS, got.TLS)
			}
			if tc.wantSNI != "" && got.SNI != tc.wantSNI {
				t.Errorf("sni: want %q got %q", tc.wantSNI, got.SNI)
			}
			if tc.wantSvcName != "" && got.Extra["service_name"] != tc.wantSvcName {
				t.Errorf("service_name: want %q got %q", tc.wantSvcName, got.Extra["service_name"])
			}
			if tc.wantNameHas != "" && got.Name != tc.wantNameHas {
				t.Errorf("name: want %q got %q", tc.wantNameHas, got.Name)
			}
		})
	}
}

func TestConvertVLess_NormalTLS(t *testing.T) {
	// 普通 TLS,不带 reality —— reality 路径已被 TestParseVLessReality_Vision 覆盖
	node := NodeConfig{Type: "vless", Server: "vless.example.com", Port: 443, UUID: "u1",
		TLS: true, SNI: "vless.example.com", Network: "tcp"}
	ob, err := convertVLess(node, "V")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if ob["uuid"] != "u1" {
		t.Errorf("uuid: %v", ob["uuid"])
	}
	tls, _ := ob["tls"].(map[string]interface{})
	if tls == nil || tls["enabled"] != true || tls["server_name"] != "vless.example.com" {
		t.Errorf("tls: %+v", tls)
	}
	if _, has := tls["reality"]; has {
		t.Errorf("reality should NOT be present without public_key: %+v", tls)
	}
	if _, has := tls["utls"]; has {
		t.Errorf("utls should NOT be present without fingerprint: %+v", tls)
	}
}

// ============================================================================
// Hysteria2
// ============================================================================

func TestParseHysteria2(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantServer  string
		wantPort    int
		wantPwd     string
		wantSNI     string
		wantInsec   string
		wantObfs    string
		wantObfsPwd string
		wantNameHas string
		wantErr     bool
	}{
		{
			name:        "hysteria2 scheme + obfs salamander",
			uri:         "hysteria2://pass123@hy2.example.com:8443?sni=hy2.example.com&insecure=1&obfs=salamander&obfs-password=mysalt#HK-Hy2",
			wantServer:  "hy2.example.com",
			wantPort:    8443,
			wantPwd:     "pass123",
			wantSNI:     "hy2.example.com",
			wantInsec:   "true",
			wantObfs:    "salamander",
			wantObfsPwd: "mysalt",
			wantNameHas: "HK-Hy2",
		},
		{
			name:        "hy2 alias",
			uri:         "hy2://abc@1.2.3.4:443?sni=1.2.3.4#JP-Hy",
			wantServer:  "1.2.3.4",
			wantPort:    443,
			wantPwd:     "abc",
			wantSNI:     "1.2.3.4",
			wantNameHas: "JP-Hy",
		},
		{
			name:       "trailing slash tolerated",
			uri:        "hysteria2://p@example.com:443/?sni=example.com#X",
			wantServer: "example.com",
			wantPort:   443,
			wantPwd:    "p",
			wantSNI:    "example.com",
		},
		{
			name:    "missing @ should fail",
			uri:     "hysteria2://no-at.example.com:443",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHysteria2(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Server != tc.wantServer || got.Port != tc.wantPort || got.Password != tc.wantPwd {
				t.Errorf("base: server=%q port=%d pwd=%q", got.Server, got.Port, got.Password)
			}
			if !got.TLS {
				t.Errorf("hysteria2 should always be TLS")
			}
			if tc.wantSNI != "" && got.SNI != tc.wantSNI {
				t.Errorf("sni: want %q got %q", tc.wantSNI, got.SNI)
			}
			if tc.wantInsec != "" && got.Extra["insecure"] != tc.wantInsec {
				t.Errorf("insecure: want %q got %q", tc.wantInsec, got.Extra["insecure"])
			}
			if tc.wantObfs != "" && got.Extra["obfs"] != tc.wantObfs {
				t.Errorf("obfs: want %q got %q", tc.wantObfs, got.Extra["obfs"])
			}
			if tc.wantObfsPwd != "" && got.Extra["obfs_password"] != tc.wantObfsPwd {
				t.Errorf("obfs_password: want %q got %q", tc.wantObfsPwd, got.Extra["obfs_password"])
			}
			if tc.wantNameHas != "" && got.Name != tc.wantNameHas {
				t.Errorf("name: want %q got %q", tc.wantNameHas, got.Name)
			}
		})
	}
}

func TestConvertHysteria2(t *testing.T) {
	t.Run("basic + insecure", func(t *testing.T) {
		node := NodeConfig{Type: "hysteria2", Server: "hy2.example.com", Port: 8443, Password: "pass",
			TLS: true, SNI: "hy2.example.com", Extra: map[string]string{"insecure": "true"}}
		ob, err := convertHysteria2(node, "H1")
		if err != nil {
			t.Fatalf("convert: %v", err)
		}
		if ob["type"] != "hysteria2" || ob["password"] != "pass" {
			t.Errorf("base: %+v", ob)
		}
		tls, _ := ob["tls"].(map[string]interface{})
		if tls == nil || tls["server_name"] != "hy2.example.com" || tls["insecure"] != true {
			t.Errorf("tls: %+v", tls)
		}
		if _, has := ob["obfs"]; has {
			t.Errorf("no obfs configured but obfs block present: %+v", ob)
		}
	})

	t.Run("salamander obfs", func(t *testing.T) {
		node := NodeConfig{Type: "hysteria2", Server: "x", Port: 1, Password: "p",
			Extra: map[string]string{"obfs": "salamander", "obfs_password": "mysalt"}}
		ob, err := convertHysteria2(node, "H2")
		if err != nil {
			t.Fatalf("convert: %v", err)
		}
		obfs, _ := ob["obfs"].(map[string]interface{})
		if obfs == nil || obfs["type"] != "salamander" || obfs["password"] != "mysalt" {
			t.Errorf("obfs: %+v", obfs)
		}
	})

	t.Run("missing password rejected", func(t *testing.T) {
		_, err := convertHysteria2(NodeConfig{Type: "hysteria2", Server: "x", Port: 1}, "X")
		if err == nil {
			t.Fatalf("expected error on empty password")
		}
	})
}

// ============================================================================
// WireGuard URI (TestConvertWireGuardEndpoint 已覆盖 converter, 这里只测 parser URI)
// ============================================================================

func TestParseWireGuard(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantServer  string
		wantPort    int
		wantPriv    string
		wantPeer    string
		wantAddr    string
		wantMTU     string
		wantReserve string
		wantNameHas string
		wantErr     bool
	}{
		{
			// reserved 字段 parser 当前不做 URL decode (parser.go:843), 真实订阅都用裸逗号; 用 encoded "%2C"
			// 会原样塞到 Extra 让 converter 拆解失败。这里锁定真实行为。
			name:        "wireguard scheme full params",
			uri:         "wireguard://privkey-base64@wg.example.com:51820?publickey=peer-pub&presharedkey=psk&address=10.0.0.2%2F32&mtu=1420&reserved=0,0,0#WG-JP",
			wantServer:  "wg.example.com",
			wantPort:    51820,
			wantPriv:    "privkey-base64",
			wantPeer:    "peer-pub",
			wantAddr:    "10.0.0.2/32",
			wantMTU:     "1420",
			wantReserve: "0,0,0",
			wantNameHas: "WG-JP",
		},
		{
			name:        "wg alias + URL-encoded private key",
			uri:         "wg://" + url.QueryEscape("priv+with/slash=") + "@1.2.3.4:51820?public_key=pkey&ip=10.7.0.1#X",
			wantServer:  "1.2.3.4",
			wantPort:    51820,
			wantPriv:    "priv+with/slash=",
			wantPeer:    "pkey",
			wantAddr:    "10.7.0.1",
			wantNameHas: "X",
		},
		{
			name:    "missing @ should fail",
			uri:     "wireguard://no-at.example.com:51820",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseWireGuard(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Server != tc.wantServer || got.Port != tc.wantPort {
				t.Errorf("base: server=%q port=%d", got.Server, got.Port)
			}
			if got.Extra["private_key"] != tc.wantPriv {
				t.Errorf("private_key: want %q got %q", tc.wantPriv, got.Extra["private_key"])
			}
			if got.Extra["peer_public_key"] != tc.wantPeer {
				t.Errorf("peer_public_key: want %q got %q", tc.wantPeer, got.Extra["peer_public_key"])
			}
			if tc.wantAddr != "" && got.Extra["local_address"] != tc.wantAddr {
				t.Errorf("local_address: want %q got %q", tc.wantAddr, got.Extra["local_address"])
			}
			if tc.wantMTU != "" && got.Extra["mtu"] != tc.wantMTU {
				t.Errorf("mtu: want %q got %q", tc.wantMTU, got.Extra["mtu"])
			}
			if tc.wantReserve != "" && got.Extra["reserved"] != tc.wantReserve {
				t.Errorf("reserved: want %q got %q", tc.wantReserve, got.Extra["reserved"])
			}
			if tc.wantNameHas != "" && got.Name != tc.wantNameHas {
				t.Errorf("name: want %q got %q", tc.wantNameHas, got.Name)
			}
		})
	}
}
