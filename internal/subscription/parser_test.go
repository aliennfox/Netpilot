package subscription

import (
	"encoding/base64"
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
