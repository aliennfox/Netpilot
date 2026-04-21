package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

type SingBoxAdapter struct {
	baseURL    string
	configPath string
	httpClient *http.Client
}

func NewSingBoxAdapter(baseURL string) *SingBoxAdapter {
	// 缺 scheme 的 host:port (如 Android 传的 "127.0.0.1:9090") 会被 url.Parse
	// 当成 scheme:opaque, http.Get 直接报 "first path segment in URL cannot
	// contain colon"。统一补 http:// 让两端调用者兼容。
	addr := strings.TrimRight(baseURL, "/")
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	return &SingBoxAdapter{
		baseURL: addr,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetConfigPath 返回当前使用的配置文件路径
func (a *SingBoxAdapter) GetConfigPath() string {
	return a.configPath
}

// SetConfigPath 设置当前使用的配置文件路径
func (a *SingBoxAdapter) SetConfigPath(path string) {
	a.configPath = path
}

// --- Implemented methods (Phase 1) ---

func (a *SingBoxAdapter) GetProxies() ([]ProxyInfo, error) {
	resp, err := a.httpClient.Get(a.baseURL + "/proxies")
	if err != nil {
		return nil, fmt.Errorf("clash API unreachable: %w", err)
	}
	defer resp.Body.Close()

	var raw struct {
		Proxies map[string]json.RawMessage `json:"proxies"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode proxies response: %w", err)
	}

	var proxies []ProxyInfo
	for tag, data := range raw.Proxies {
		var p struct {
			Type string `json:"type"`
			Now  string `json:"now"`
		}
		json.Unmarshal(data, &p)
		proxies = append(proxies, ProxyInfo{
			Tag:  tag,
			Type: p.Type,
		})
	}
	return proxies, nil
}

func (a *SingBoxAdapter) GetProxyGroup(groupTag string) (*ProxyGroup, error) {
	resp, err := a.httpClient.Get(a.baseURL + "/proxies/" + url.PathEscape(groupTag))
	if err != nil {
		return nil, fmt.Errorf("clash API unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("proxy group %q not found", groupTag)
	}

	var raw struct {
		Type string   `json:"type"`
		Now  string   `json:"now"`
		All  []string `json:"all"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode proxy group response: %w", err)
	}

	// 拉一次 /proxies 拿每个成员的 type/alive/history, 让 UI 能显示 latency 和节点类型。
	// Clash API 结构: {"proxies": {"tag": {type, name, alive, history:[{delay}]}}}。
	detailByTag := a.fetchProxyDetails()

	group := &ProxyGroup{
		Tag:  groupTag,
		Type: raw.Type,
		Now:  raw.Now,
	}
	for _, tag := range raw.All {
		info := ProxyInfo{Tag: tag, GroupTag: groupTag}
		if d, ok := detailByTag[tag]; ok {
			info.Type = d.Type
			info.Alive = d.Alive
			info.Latency = d.Latency
		}
		group.All = append(group.All, info)
	}
	return group, nil
}

// proxyDetail 解析 Clash API /proxies 单条 proxy 的相关字段。
type proxyDetail struct {
	Type    string
	Alive   bool
	Latency int
}

// fetchProxyDetails 拉一次 /proxies 并把有用字段整理成 tag -> detail。
// 失败时返回空 map, 调用方继续走 (只是 latency/alive 为空)。
func (a *SingBoxAdapter) fetchProxyDetails() map[string]proxyDetail {
	resp, err := a.httpClient.Get(a.baseURL + "/proxies")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var envelope struct {
		Proxies map[string]struct {
			Type    string `json:"type"`
			Alive   bool   `json:"alive"`
			History []struct {
				Delay int `json:"delay"`
			} `json:"history"`
		} `json:"proxies"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil
	}
	out := make(map[string]proxyDetail, len(envelope.Proxies))
	for tag, p := range envelope.Proxies {
		d := proxyDetail{Type: p.Type, Alive: p.Alive}
		// 取最新一次延迟测试 (Clash 约定 history 末尾最新)
		if n := len(p.History); n > 0 {
			d.Latency = p.History[n-1].Delay
		}
		out[tag] = d
	}
	return out
}

func (a *SingBoxAdapter) SetActiveProxy(groupTag, proxyTag string) error {
	bodyMap := map[string]string{"name": proxyTag}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, a.baseURL+"/proxies/"+url.PathEscape(groupTag), bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("clash API unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		var errBody struct {
			Message string `json:"message"`
		}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return fmt.Errorf("switch proxy failed (HTTP %d): %s", resp.StatusCode, errBody.Message)
	}
	return nil
}

func (a *SingBoxAdapter) TestLatency(proxyTag string, testURL string, timeout time.Duration) (int, error) {
	reqURL := fmt.Sprintf("%s/proxies/%s/delay?url=%s&timeout=%d",
		a.baseURL, url.PathEscape(proxyTag), url.QueryEscape(testURL), timeout.Milliseconds())

	// Use a longer HTTP timeout for latency tests since the server needs time to probe
	client := &http.Client{Timeout: timeout + 2*time.Second}
	resp, err := client.Get(reqURL)
	if err != nil {
		return 0, fmt.Errorf("latency test request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Message string `json:"message"`
		}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return 0, fmt.Errorf("latency test failed (HTTP %d): %s", resp.StatusCode, errBody.Message)
	}

	var result struct {
		Delay int `json:"delay"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode latency response: %w", err)
	}
	return result.Delay, nil
}

func (a *SingBoxAdapter) GetConnections() ([]ConnectionInfo, error) {
	resp, err := a.httpClient.Get(a.baseURL + "/connections")
	if err != nil {
		return nil, fmt.Errorf("clash API unreachable: %w", err)
	}
	defer resp.Body.Close()

	var raw struct {
		Connections []struct {
			ID       string `json:"id"`
			Metadata struct {
				Host        string `json:"host"`
				Destination string `json:"destinationIP"`
				Process     string `json:"process"`
				Network     string `json:"network"`
			} `json:"metadata"`
			Upload   int64    `json:"upload"`
			Download int64    `json:"download"`
			Start    string   `json:"start"`
			Chains   []string `json:"chains"`
			Rule     string   `json:"rule"`
		} `json:"connections"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode connections response: %w", err)
	}

	var conns []ConnectionInfo
	for _, c := range raw.Connections {
		dest := c.Metadata.Host
		if dest == "" {
			dest = c.Metadata.Destination
		}
		chain := ""
		if len(c.Chains) > 0 {
			chain = strings.Join(c.Chains, " → ")
		}
		conns = append(conns, ConnectionInfo{
			ID:          c.ID,
			Destination: dest,
			Protocol:    c.Metadata.Network,
			ProcessName: c.Metadata.Process,
			Upload:      c.Upload,
			Download:    c.Download,
			StartTime:   c.Start,
			Chain:       chain,
			Rule:        c.Rule,
		})
	}
	return conns, nil
}

func (a *SingBoxAdapter) GetLogs(level string, lines int) ([]LogEntry, error) {
	reqURL := a.baseURL + "/logs"
	if level != "" {
		reqURL += "?level=" + level
	}

	// The /logs endpoint is streaming (only delivers new entries, no backlog).
	// We open it with a short context deadline to collect whatever arrives.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create logs request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		// Timeout is expected — the endpoint streams indefinitely.
		// Return empty if we simply timed out with no data.
		return nil, nil
	}
	defer resp.Body.Close()

	var logs []LogEntry
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() && len(logs) < lines {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			logs = append(logs, LogEntry{Type: "info", Payload: line})
			continue
		}
		logs = append(logs, entry)
	}
	return logs, nil
}

// --- Not implemented in Phase 1 ---

func (a *SingBoxAdapter) Start(configPath string) error {
	return fmt.Errorf("not implemented: Start")
}
func (a *SingBoxAdapter) Stop() error {
	return fmt.Errorf("not implemented: Stop")
}

// Reload 通过重启 sing-box 进程重载配置
// sing-box v1.13+ 的 Clash API PUT /configs 不支持切换到不同配置文件，
// 因此使用 pkill + 重新启动的方式
//
// 嵌入式模式 (Android/iOS, #M12 双轨) 下 sing-box 不作为独立二进制运行, 而是
// 编进 libbox 由 Kotlin/Swift 驱动。此时 exec.LookPath("sing-box") 会失败, 返回
// nil 让 Apply() 链成功 —— merged.json 已经写盘, 平台层负责触发 libbox 重载。
func (a *SingBoxAdapter) Reload() error {
	if a.configPath == "" {
		return fmt.Errorf("未设置配置文件路径，无法重载")
	}

	if _, err := exec.LookPath("sing-box"); err != nil {
		// 嵌入模式: merged.json 已写, 平台层自行重载 libbox
		return nil
	}

	// 停止现有 sing-box 进程
	_ = exec.Command("pkill", "-x", "sing-box").Run()
	time.Sleep(500 * time.Millisecond)

	// 启动新 sing-box 进程
	cmd := exec.Command("sing-box", "run", "-c", a.configPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 sing-box 失败: %w", err)
	}

	// 等待 Clash API 就绪
	for i := 0; i < 40; i++ {
		time.Sleep(200 * time.Millisecond)
		resp, err := http.Get(a.baseURL + "/proxies")
		if err == nil {
			resp.Body.Close()
			return nil
		}
	}
	return fmt.Errorf("sing-box 重启后 Clash API 未就绪 (超时 8s)")
}

func (a *SingBoxAdapter) IsRunning() bool { return false }
func (a *SingBoxAdapter) GetCurrentConfig() ([]byte, error) {
	return nil, fmt.Errorf("not implemented: GetCurrentConfig")
}
func (a *SingBoxAdapter) PatchConfig(patch []byte) error {
	return fmt.Errorf("not implemented: PatchConfig")
}
func (a *SingBoxAdapter) ReplaceConfig(config []byte) error {
	return fmt.Errorf("not implemented: ReplaceConfig")
}
func (a *SingBoxAdapter) ValidateConfig(config []byte) error {
	return fmt.Errorf("not implemented: ValidateConfig")
}
func (a *SingBoxAdapter) TestLatencyBatch(tags []string, testURL string, timeout time.Duration) ([]LatencyResult, error) {
	return nil, fmt.Errorf("not implemented: TestLatencyBatch")
}
func (a *SingBoxAdapter) CloseConnection(id string) error {
	return fmt.Errorf("not implemented: CloseConnection")
}
func (a *SingBoxAdapter) GetTrafficStats() (*TrafficStats, error) {
	return nil, fmt.Errorf("not implemented: GetTrafficStats")
}
func (a *SingBoxAdapter) SubscribeLogs(level string) (<-chan LogEntry, func()) {
	ch := make(chan LogEntry)
	close(ch)
	return ch, func() {}
}
func (a *SingBoxAdapter) QueryDNS(domain string) (*DNSResult, error) {
	return nil, fmt.Errorf("not implemented: QueryDNS")
}
func (a *SingBoxAdapter) OnNetworkChanged() {}
