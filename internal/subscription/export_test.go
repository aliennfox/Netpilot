package subscription

import (
	"encoding/base64"
	"strings"
	"testing"
)

// TestOutboundToURI_RoundTrip 验证 6 个主流协议 parseLine → convert → OutboundToURI
// 往返不 panic 且能产出非空 URI。 严格字段级 round-trip (URI = original) 不在范围内,
// 因为 tag 可能被 SanitizeTag 改名、query 参数顺序未必一致, 但核心身份字段必须保留。
func TestOutboundToURI_RoundTrip(t *testing.T) {
	cases := []struct {
		name       string
		uri        string
		wantScheme string
	}{
		{
			name:       "ss sip002",
			uri:        "ss://YWVzLTI1Ni1nY206dGVzdA==@1.2.3.4:8388#ss-test",
			wantScheme: "ss://",
		},
		{
			name:       "trojan",
			uri:        "trojan://password123@example.com:443?sni=example.com#trojan-test",
			wantScheme: "trojan://",
		},
		{
			name:       "vmess",
			uri:        "vmess://eyJ2IjoiMiIsInBzIjoidm1lc3MtdGVzdCIsImFkZCI6IjEuMi4zLjQiLCJwb3J0IjoiNDQzIiwiaWQiOiJhYWFhYWFhYS1iYmJiLWNjY2MtZGRkZC1lZWVlZWVlZWVlZWUiLCJhaWQiOiIwIiwibmV0IjoidGNwIiwidHlwZSI6Im5vbmUiLCJob3N0IjoiIiwicGF0aCI6IiIsInRscyI6InRscyJ9",
			wantScheme: "vmess://",
		},
		{
			name:       "vless",
			uri:        "vless://aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee@example.com:443?security=tls&sni=example.com&type=tcp#vless-test",
			wantScheme: "vless://",
		},
		{
			name:       "hysteria2",
			uri:        "hysteria2://auth@example.com:443?sni=example.com#hy2-test",
			wantScheme: "hysteria2://",
		},
		{
			name:       "tuic",
			uri:        "tuic://aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:pass@example.com:443?sni=example.com#tuic-test",
			wantScheme: "tuic://",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := parseLine(tc.uri)
			if err != nil {
				t.Fatalf("parseLine: %v", err)
			}
			ob, err := ConvertToSingboxOutbound(node)
			if err != nil {
				t.Fatalf("ConvertToSingboxOutbound: %v", err)
			}
			if ob == nil {
				t.Fatal("ob is nil")
			}
			uri, err := OutboundToURI(ob)
			if err != nil {
				t.Fatalf("OutboundToURI: %v", err)
			}
			if !strings.HasPrefix(uri, tc.wantScheme) {
				t.Errorf("got %q, want prefix %q", uri, tc.wantScheme)
			}
			// 核心身份字段必须在 URI 里可见 (vmess 字段在 base64 body 内, 先解码)
			searchSpace := uri
			if strings.HasPrefix(uri, "vmess://") {
				b64 := strings.TrimPrefix(uri, "vmess://")
				if decoded, err := base64.StdEncoding.DecodeString(b64); err == nil {
					searchSpace = string(decoded)
				}
			}
			if node.Server != "" && !strings.Contains(searchSpace, node.Server) {
				t.Errorf("server %q missing from uri %q", node.Server, uri)
			}
		})
	}
}

func TestOutboundToURI_Unsupported(t *testing.T) {
	ob := map[string]interface{}{
		"type":        "wireguard",
		"tag":         "wg-test",
		"server":      "1.2.3.4",
		"server_port": float64(51820),
	}
	_, err := OutboundToURI(ob)
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
	if !strings.Contains(err.Error(), "不支持") {
		t.Errorf("unexpected err: %v", err)
	}
}

func TestOutboundToURI_NilAndEmpty(t *testing.T) {
	if _, err := OutboundToURI(nil); err == nil {
		t.Fatal("expected error on nil")
	}
	if _, err := OutboundToURI(map[string]interface{}{"type": "shadowsocks"}); err == nil {
		t.Fatal("expected error on missing fields")
	}
}
