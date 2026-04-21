package subscription

import (
	"testing"
)

// TestIsClashYAML 检测 Clash 订阅探测函数 (M11)
func TestIsClashYAML(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"顶层 proxies key", "proxies:\n  - name: foo\n", true},
		{"有注释 + 顶层 proxies", "# a clash conf\nproxies:\n  - name: foo\n", true},
		{"proxy-providers", "proxy-providers:\n  default: {}\n", true},
		{"不是 YAML", "ss://base64===", false},
		{"v2ray base64 订阅(碰巧含 proxies: 字样)", "cHJveGllczogZm9vCg==", false},
		{"空", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsClashYAML(tc.raw); got != tc.want {
				t.Errorf("%q: want %v got %v", tc.raw, tc.want, got)
			}
		})
	}
}

// TestParseClashYAML 覆盖主流协议类型在 Clash YAML 里的映射 (M11)
func TestParseClashYAML(t *testing.T) {
	yaml := `
proxies:
  - name: "SS-HK"
    type: ss
    server: ss.example.com
    port: 8388
    cipher: aes-256-gcm
    password: ss-pass
  - name: "VLESS-Reality-JP"
    type: vless
    server: reality.example.com
    port: 443
    uuid: 11111111-2222-3333-4444-555555555555
    network: tcp
    flow: xtls-rprx-vision
    tls: true
    servername: www.microsoft.com
    client-fingerprint: chrome
    reality-opts:
      public-key: rr-pubkey-base64
      short-id: abcd1234
  - name: "Trojan-SG"
    type: trojan
    server: trojan.example.com
    port: 443
    password: trojan-pw
    sni: trojan.example.com
    network: ws
    ws-opts:
      path: /trojan
      headers:
        Host: trojan.example.com
  - name: "Hy2-US"
    type: hysteria2
    server: hy2.example.com
    port: 443
    password: hy2-pw
    sni: hy2.example.com
    obfs: salamander
    obfs-password: obfspw
  - name: "TUIC-HK"
    type: tuic
    server: tuic.example.com
    port: 443
    uuid: tuic-uuid
    password: tuic-pw
    sni: tuic.example.com
    congestion-controller: bbr
    udp-relay-mode: native
    alpn: [h3]
  - name: "AnyTLS-01"
    type: anytls
    server: any.example.com
    port: 443
    password: any-pw
    sni: any.example.com
    client-fingerprint: chrome
  - name: "WG-01"
    type: wireguard
    server: wg.example.com
    port: 51820
    private-key: privkey
    public-key: peerkey
    ip: 10.0.0.2/32
    mtu: 1420
  - name: "SSR 不支持"
    type: ssr
    server: x
    port: 1
`
	nodes, err := ParseClashYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// 8 条里 SSR 被 skip, 剩 7 条
	if len(nodes) != 7 {
		t.Fatalf("want 7 nodes, got %d: %+v", len(nodes), nodes)
	}

	// 分别验证关键字段
	byName := map[string]NodeConfig{}
	for _, n := range nodes {
		byName[n.Name] = n
	}

	if n := byName["SS-HK"]; n.Method != "aes-256-gcm" || n.Password != "ss-pass" || n.Port != 8388 {
		t.Errorf("SS-HK: %+v", n)
	}

	vless := byName["VLESS-Reality-JP"]
	if vless.UUID == "" || vless.Extra["flow"] != "xtls-rprx-vision" {
		t.Errorf("VLESS flow: %+v", vless)
	}
	if vless.Extra["public_key"] != "rr-pubkey-base64" || vless.Extra["short_id"] != "abcd1234" {
		t.Errorf("VLESS reality: %+v", vless.Extra)
	}
	if vless.Extra["fingerprint"] != "chrome" {
		t.Errorf("VLESS client-fingerprint: %+v", vless.Extra)
	}
	if !vless.TLS {
		t.Errorf("VLESS reality 应触发 TLS=true")
	}

	trojan := byName["Trojan-SG"]
	if trojan.Network != "ws" || trojan.Path != "/trojan" || trojan.Host != "trojan.example.com" {
		t.Errorf("Trojan ws-opts: %+v", trojan)
	}

	hy2 := byName["Hy2-US"]
	if hy2.Extra["obfs"] != "salamander" || hy2.Extra["obfs_password"] != "obfspw" {
		t.Errorf("Hy2 obfs: %+v", hy2.Extra)
	}

	tuic := byName["TUIC-HK"]
	if tuic.Extra["congestion_control"] != "bbr" || tuic.Extra["udp_relay_mode"] != "native" {
		t.Errorf("TUIC fields: %+v", tuic.Extra)
	}
	if tuic.Extra["alpn"] != "h3" {
		t.Errorf("TUIC alpn: %q", tuic.Extra["alpn"])
	}

	wg := byName["WG-01"]
	if wg.Extra["private_key"] != "privkey" || wg.Extra["peer_public_key"] != "peerkey" {
		t.Errorf("WG keys: %+v", wg.Extra)
	}
	if wg.Extra["local_address"] != "10.0.0.2/32" || wg.Extra["mtu"] != "1420" {
		t.Errorf("WG addr/mtu: %+v", wg.Extra)
	}
}

// TestParseSubscription_ClashDispatch 端到端验证 ParseSubscription 能自动路由到 Clash
func TestParseSubscription_ClashDispatch(t *testing.T) {
	yaml := "proxies:\n  - { name: foo, type: ss, server: a.com, port: 8388, cipher: aes-256-gcm, password: pw }\n"
	nodes, err := ParseSubscription(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Type != "shadowsocks" || nodes[0].Method != "aes-256-gcm" {
		t.Errorf("bad dispatch: %+v", nodes)
	}
}

// TestParseSingBoxJSON 覆盖 sing-box native JSON 订阅 (M12)
func TestParseSingBoxJSON(t *testing.T) {
	data := `{
  "outbounds": [
    { "type": "direct", "tag": "direct" },
    { "type": "selector", "tag": "proxy", "outbounds": ["HK-01"] },
    {
      "type": "vless",
      "tag": "HK-01",
      "server": "reality.example.com",
      "server_port": 443,
      "uuid": "11111111-2222-3333-4444-555555555555",
      "flow": "xtls-rprx-vision",
      "tls": {
        "enabled": true,
        "server_name": "www.microsoft.com",
        "utls": {"enabled": true, "fingerprint": "chrome"},
        "reality": {"enabled": true, "public_key": "pk", "short_id": "sid"}
      }
    },
    {
      "type": "tuic",
      "tag": "JP-TUIC",
      "server": "tuic.example.com",
      "server_port": 443,
      "uuid": "tuic-uuid",
      "password": "tuic-pw",
      "congestion_control": "bbr",
      "udp_relay_mode": "native",
      "tls": {"enabled": true, "server_name": "tuic.example.com"}
    }
  ]
}`
	nodes, err := ParseSingBoxJSON([]byte(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("want 2 wallet-outbound nodes (direct/selector 被 skip), got %d: %+v", len(nodes), nodes)
	}

	byName := map[string]NodeConfig{}
	for _, n := range nodes {
		byName[n.Name] = n
	}

	vless := byName["HK-01"]
	if vless.Type != "vless" || vless.UUID == "" {
		t.Errorf("vless: %+v", vless)
	}
	if vless.Extra["flow"] != "xtls-rprx-vision" {
		t.Errorf("flow: %+v", vless.Extra)
	}
	if vless.Extra["public_key"] != "pk" || vless.Extra["short_id"] != "sid" {
		t.Errorf("reality: %+v", vless.Extra)
	}
	if vless.Extra["fingerprint"] != "chrome" {
		t.Errorf("utls fp: %+v", vless.Extra)
	}

	tuic := byName["JP-TUIC"]
	if tuic.Type != "tuic" || tuic.UUID != "tuic-uuid" || tuic.Password != "tuic-pw" {
		t.Errorf("tuic: %+v", tuic)
	}
	if tuic.Extra["congestion_control"] != "bbr" {
		t.Errorf("cc: %q", tuic.Extra["congestion_control"])
	}
}

// TestParseSubscription_SingBoxDispatch 验证 ParseSubscription 路由到 sing-box JSON
func TestParseSubscription_SingBoxDispatch(t *testing.T) {
	json := `{"outbounds":[{"type":"trojan","tag":"T1","server":"t.example.com","server_port":443,"password":"pw","tls":{"enabled":true,"server_name":"t.example.com"}}]}`
	nodes, err := ParseSubscription(json)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Type != "trojan" {
		t.Errorf("bad dispatch: %+v", nodes)
	}
}
