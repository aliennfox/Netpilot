package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- 构造器 ---

// TestNewSingBoxAdapter_BaseURLNormalization #M14: 裸 host:port 自动补 http://
func TestNewSingBoxAdapter_BaseURLNormalization(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"127.0.0.1:9090", "http://127.0.0.1:9090"},
		{"http://127.0.0.1:9090", "http://127.0.0.1:9090"},
		{"https://api.example.com", "https://api.example.com"},
		{"127.0.0.1:9090/", "http://127.0.0.1:9090"}, // 尾斜杠也要剥
	}
	for _, tc := range cases {
		a := NewSingBoxAdapter(tc.in)
		if a.baseURL != tc.want {
			t.Errorf("NewSingBoxAdapter(%q).baseURL = %q, want %q", tc.in, a.baseURL, tc.want)
		}
	}
}

// TestNewSingBoxAdapter_ConfigPath GetConfigPath/SetConfigPath round-trip
func TestNewSingBoxAdapter_ConfigPath(t *testing.T) {
	a := NewSingBoxAdapter("127.0.0.1:9090")
	if a.GetConfigPath() != "" {
		t.Errorf("default configPath should be empty, got %q", a.GetConfigPath())
	}
	a.SetConfigPath("/tmp/x.json")
	if a.GetConfigPath() != "/tmp/x.json" {
		t.Errorf("SetConfigPath round-trip failed: got %q", a.GetConfigPath())
	}
}

// --- GetProxies ---

func TestGetProxies_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxies" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"proxies":{
			"HK-1":{"type":"shadowsocks","now":""},
			"proxy-group":{"type":"Selector","now":"HK-1"}
		}}`))
	}))
	defer srv.Close()

	a := NewSingBoxAdapter(srv.URL)
	proxies, err := a.GetProxies()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(proxies) != 2 {
		t.Fatalf("want 2 proxies, got %d", len(proxies))
	}
	tagSeen := map[string]string{}
	for _, p := range proxies {
		tagSeen[p.Tag] = p.Type
	}
	if tagSeen["HK-1"] != "shadowsocks" {
		t.Errorf("HK-1 type: %q", tagSeen["HK-1"])
	}
	if tagSeen["proxy-group"] != "Selector" {
		t.Errorf("proxy-group type: %q", tagSeen["proxy-group"])
	}
}

func TestGetProxies_NetworkError(t *testing.T) {
	a := NewSingBoxAdapter("http://127.0.0.1:1") // refused
	a.httpClient.Timeout = 200 * time.Millisecond
	_, err := a.GetProxies()
	if err == nil || !strings.Contains(err.Error(), "VPN 未启动") {
		t.Errorf("expect 'VPN 未启动' wrap, got %v", err)
	}
}

func TestGetProxies_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()
	a := NewSingBoxAdapter(srv.URL)
	_, err := a.GetProxies()
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("expect decode err, got %v", err)
	}
}

// --- GetProxyGroup ---

func TestGetProxyGroup_HappyWithDetails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/proxies/proxy-group":
			_, _ = w.Write([]byte(`{"type":"Selector","now":"HK-1","all":["HK-1","JP-1"]}`))
		case "/proxies":
			_, _ = w.Write([]byte(`{"proxies":{
				"HK-1":{"type":"shadowsocks","alive":true,"history":[{"delay":120}]},
				"JP-1":{"type":"vless","alive":false,"history":[]}
			}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := NewSingBoxAdapter(srv.URL)
	g, err := a.GetProxyGroup("proxy-group")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if g.Tag != "proxy-group" || g.Type != "Selector" || g.Now != "HK-1" {
		t.Errorf("group meta: %+v", g)
	}
	if len(g.All) != 2 {
		t.Fatalf("want 2 members, got %d", len(g.All))
	}
	hk := g.All[0]
	if hk.Tag != "HK-1" || hk.Type != "shadowsocks" || hk.Alive != true || hk.Latency != 120 {
		t.Errorf("HK-1 detail: %+v", hk)
	}
	jp := g.All[1]
	if jp.Latency != 0 || jp.Alive != false {
		t.Errorf("JP-1 should have no history → latency=0, alive=false: %+v", jp)
	}
}

func TestGetProxyGroup_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	a := NewSingBoxAdapter(srv.URL)
	_, err := a.GetProxyGroup("missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expect 'not found', got %v", err)
	}
}

func TestGetProxyGroup_PathEscape(t *testing.T) {
	// 中文 / 空格 tag 必须 url.PathEscape, 否则路径里出现 raw byte 报 4xx。
	// net/http 服务端会把 r.URL.Path 解码, 看原始 wire-form 要看 RequestURI
	var rawURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.RequestURI, "/proxies/") && r.RequestURI != "/proxies" {
			rawURI = r.RequestURI
		}
		_, _ = w.Write([]byte(`{"type":"Selector","now":"","all":[]}`))
	}))
	defer srv.Close()
	a := NewSingBoxAdapter(srv.URL)
	_, err := a.GetProxyGroup("代理 组")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(rawURI, "%") {
		t.Errorf("PathEscape should percent-encode CJK/space, got raw URI: %q", rawURI)
	}
}

// --- SetActiveProxy ---

func TestSetActiveProxy_Success(t *testing.T) {
	var seen struct {
		method, path, ct string
		body             string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.method = r.Method
		seen.path = r.URL.Path
		seen.ct = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		seen.body = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	a := NewSingBoxAdapter(srv.URL)
	if err := a.SetActiveProxy("proxy-group", "HK-1"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if seen.method != "PUT" {
		t.Errorf("method: %q", seen.method)
	}
	if seen.path != "/proxies/proxy-group" {
		t.Errorf("path: %q", seen.path)
	}
	if seen.ct != "application/json" {
		t.Errorf("content-type: %q", seen.ct)
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(seen.body), &parsed); err != nil || parsed["name"] != "HK-1" {
		t.Errorf("body should be {\"name\":\"HK-1\"}: %q", seen.body)
	}
}

func TestSetActiveProxy_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"node not in group"}`))
	}))
	defer srv.Close()
	a := NewSingBoxAdapter(srv.URL)
	err := a.SetActiveProxy("proxy-group", "ZZ-1")
	if err == nil || !strings.Contains(err.Error(), "switch proxy failed") || !strings.Contains(err.Error(), "node not in group") {
		t.Errorf("expect HTTP 400 wrap with body, got %v", err)
	}
}

func TestSetActiveProxy_AcceptsHTTP200(t *testing.T) {
	// 真 sing-box 返回 204 但有些实现 (老版本 / mocks) 返回 200, 都接受
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	a := NewSingBoxAdapter(srv.URL)
	if err := a.SetActiveProxy("proxy-group", "HK-1"); err != nil {
		t.Errorf("HTTP 200 should also be accepted: %v", err)
	}
}

// --- TestLatency ---

func TestTestLatency_HappyPath(t *testing.T) {
	var seen struct {
		path, query string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.path = r.URL.Path
		seen.query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"delay":234}`))
	}))
	defer srv.Close()

	a := NewSingBoxAdapter(srv.URL)
	delay, err := a.TestLatency("HK-1", "https://www.gstatic.com/generate_204", 3*time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if delay != 234 {
		t.Errorf("want 234, got %d", delay)
	}
	if seen.path != "/proxies/HK-1/delay" {
		t.Errorf("path: %q", seen.path)
	}
	// query 包含 url= 和 timeout=
	if !strings.Contains(seen.query, "url=") || !strings.Contains(seen.query, "timeout=3000") {
		t.Errorf("query missing fields: %q", seen.query)
	}
}

func TestTestLatency_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = w.Write([]byte(`{"message":"timeout"}`))
	}))
	defer srv.Close()
	a := NewSingBoxAdapter(srv.URL)
	_, err := a.TestLatency("HK-1", "x", 1*time.Second)
	if err == nil || !strings.Contains(err.Error(), "504") || !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expect 504 + body, got %v", err)
	}
}

func TestTestLatency_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not-json`))
	}))
	defer srv.Close()
	a := NewSingBoxAdapter(srv.URL)
	_, err := a.TestLatency("HK-1", "x", 1*time.Second)
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("expect decode err, got %v", err)
	}
}

// --- GetConnections ---

func TestGetConnections_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"connections":[
			{"id":"abc","metadata":{"host":"api.example.com","destinationIP":"1.2.3.4","process":"chrome","network":"tcp"},
			 "upload":100,"download":200,"start":"2026-04-25T00:00:00Z","chains":["proxy-group","HK-1"],"rule":""}
		]}`))
	}))
	defer srv.Close()

	a := NewSingBoxAdapter(srv.URL)
	conns, err := a.GetConnections()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("want 1, got %d", len(conns))
	}
	c := conns[0]
	if c.Destination != "api.example.com" {
		t.Errorf("host should win over destinationIP: %q", c.Destination)
	}
	if c.Chain != "proxy-group → HK-1" {
		t.Errorf("chain join: %q", c.Chain)
	}
	if c.ProcessName != "chrome" || c.Protocol != "tcp" {
		t.Errorf("metadata: %+v", c)
	}
}

func TestGetConnections_HostFallbackToIP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"connections":[
			{"id":"x","metadata":{"host":"","destinationIP":"8.8.8.8","network":"udp"},"upload":0,"download":0,"chains":[]}
		]}`))
	}))
	defer srv.Close()
	a := NewSingBoxAdapter(srv.URL)
	conns, err := a.GetConnections()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if conns[0].Destination != "8.8.8.8" {
		t.Errorf("host empty → fallback to destinationIP, got %q", conns[0].Destination)
	}
	if conns[0].Chain != "" {
		t.Errorf("empty chains → empty chain, got %q", conns[0].Chain)
	}
}

// --- CloseConnection ---

func TestCloseConnection_EmptyID(t *testing.T) {
	a := NewSingBoxAdapter("http://127.0.0.1:1")
	if err := a.CloseConnection(""); err == nil || !strings.Contains(err.Error(), "为空") {
		t.Errorf("expect empty id err, got %v", err)
	}
}

func TestCloseConnection_Success(t *testing.T) {
	var seen struct {
		method, path string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.method, seen.path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	a := NewSingBoxAdapter(srv.URL)
	if err := a.CloseConnection("uuid-xyz"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if seen.method != "DELETE" || seen.path != "/connections/uuid-xyz" {
		t.Errorf("expect DELETE /connections/uuid-xyz, got %s %s", seen.method, seen.path)
	}
}

func TestCloseConnection_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	a := NewSingBoxAdapter(srv.URL)
	err := a.CloseConnection("id")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expect 500 wrap, got %v", err)
	}
}

// --- GetTrafficStats ---

func TestGetTrafficStats_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connections" {
			t.Errorf("expect /connections, got %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"uploadTotal":12345,"downloadTotal":67890,"connections":[]}`))
	}))
	defer srv.Close()
	a := NewSingBoxAdapter(srv.URL)
	stats, err := a.GetTrafficStats()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if stats.Upload != 12345 || stats.Download != 67890 {
		t.Errorf("stats: %+v", stats)
	}
}

func TestGetTrafficStats_NetworkError(t *testing.T) {
	a := NewSingBoxAdapter("http://127.0.0.1:1")
	a.httpClient.Timeout = 200 * time.Millisecond
	_, err := a.GetTrafficStats()
	if err == nil || !strings.Contains(err.Error(), "VPN 未启动") {
		t.Errorf("expect VPN 未启动 wrap, got %v", err)
	}
}

// --- Reload ---

func TestReload_NoConfigPath(t *testing.T) {
	a := NewSingBoxAdapter("http://127.0.0.1:1")
	err := a.Reload()
	if err == nil || !strings.Contains(err.Error(), "未设置配置文件路径") {
		t.Errorf("expect missing-configPath err, got %v", err)
	}
}

// --- IsRunning + 未实现方法 ---

func TestIsRunning_AlwaysFalse(t *testing.T) {
	// #L2: stub 永久 false (双轨决策, 不实现)
	a := NewSingBoxAdapter("x")
	if a.IsRunning() {
		t.Error("IsRunning is documented stub, should always be false")
	}
}

func TestUnimplementedMethods_ReturnError(t *testing.T) {
	// 一组未实现方法都应该返回 not implemented, 防止哪天误"实现"了忘了改契约
	a := NewSingBoxAdapter("x")
	cases := []struct {
		name string
		fn   func() error
	}{
		{"Start", func() error { return a.Start("c.json") }},
		{"Stop", a.Stop},
		{"PatchConfig", func() error { return a.PatchConfig(nil) }},
		{"ReplaceConfig", func() error { return a.ReplaceConfig(nil) }},
		{"ValidateConfig", func() error { return a.ValidateConfig(nil) }},
		{"QueryDNS", func() error { _, e := a.QueryDNS("x"); return e }},
		{"GetCurrentConfig", func() error { _, e := a.GetCurrentConfig(); return e }},
	}
	for _, tc := range cases {
		err := tc.fn()
		if err == nil || !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("%s should return 'not implemented', got %v", tc.name, err)
		}
	}
}

// --- 双服务集成: 完整 group 详情链路 ---

// TestEndToEnd_GroupDetailsAfterSwitch SetActiveProxy 切完节点, 重新拉 group 应该看到新的 Now
func TestEndToEnd_GroupDetailsAfterSwitch(t *testing.T) {
	current := "HK-1"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/proxies/"):
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			current = body["name"]
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/proxies/proxy-group":
			_, _ = fmt.Fprintf(w, `{"type":"Selector","now":%q,"all":["HK-1","JP-1"]}`, current)
		case r.URL.Path == "/proxies":
			_, _ = w.Write([]byte(`{"proxies":{"HK-1":{"type":"ss"},"JP-1":{"type":"vless"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := NewSingBoxAdapter(srv.URL)
	if err := a.SetActiveProxy("proxy-group", "JP-1"); err != nil {
		t.Fatalf("switch: %v", err)
	}
	g, err := a.GetProxyGroup("proxy-group")
	if err != nil {
		t.Fatalf("read group: %v", err)
	}
	if g.Now != "JP-1" {
		t.Errorf("after switch, Now should be JP-1, got %q", g.Now)
	}
}
