// Package netcheck 做网络自检, 用户"连不上"时一键诊断。
//
// 不追求完整覆盖, 只给 5 条用户能看懂的"分段证据":
//  1. VPN 状态       (tun 在不在跑 + agent 是否就绪)
//  2. 当前出口节点   (节点 tag + 延迟 ms, 或 "未设")
//  3. DNS 解析       (能不能解析到 IP)
//  4. HTTP 连通性    (能不能 GET 一个标志性 URL, 每个并发, 1.5s 超时)
//  5. 规则 / 节点数  (overlay 中有多少节点 / 规则)
//
// 报告结构做成 JSON 数组, UI 层按行渲染, 每行 Level = "ok" | "warn" | "fail" + Msg。
package netcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Line 一条诊断行。
type Line struct {
	Section string `json:"section"` // "VPN" / "节点" / "DNS" / "HTTP" / "规则"
	Level   string `json:"level"`   // "ok" | "warn" | "fail"
	Msg     string `json:"msg"`
	Detail  string `json:"detail,omitempty"` // 可选的额外技术细节, UI 上可折叠展示
}

// Report 一次完整自检结果。
type Report struct {
	TS        int64  `json:"ts"` // Unix ms
	Lines     []Line `json:"lines"`
	ElapsedMs int64  `json:"elapsed_ms"`
}

// Options 运行时注入的能力和参数。
type Options struct {
	// VpnRunning 返回 TUN 是否在跑
	VpnRunning func() bool
	// AgentReady LLM agent 是否就绪 (有 apiKey)
	AgentReady func() bool
	// CurrentNode 返回当前出口节点 tag + 延迟 ms, 拿不到返回空字符串
	CurrentNode func() (tag string, latencyMs int, err error)
	// NodeCount 订阅 + overlay 中的节点总数
	NodeCount func() int
	// RuleCount overlay 中的路由规则数
	RuleCount func() int

	// HTTPTargets 连通性探测目标。 留空走默认 (ipinfo.io / baidu.com / google.com)。
	HTTPTargets []string
	// DNSTargets 解析探测目标。 留空走默认 (www.google.com / www.baidu.com)
	DNSTargets []string
	// HTTPTimeout 单次 HTTP 请求超时, 默认 2s
	HTTPTimeout time.Duration
	// DNSTimeout 单次 DNS 查询超时, 默认 1.5s
	DNSTimeout time.Duration
}

// Run 跑一次完整自检。 context 可被上层 cancel (UI 退出 section)。
func Run(ctx context.Context, opts Options) Report {
	if opts.HTTPTargets == nil {
		opts.HTTPTargets = []string{"https://www.baidu.com", "https://www.google.com", "https://ipinfo.io/ip"}
	}
	if opts.DNSTargets == nil {
		opts.DNSTargets = []string{"www.google.com", "www.baidu.com"}
	}
	if opts.HTTPTimeout == 0 {
		opts.HTTPTimeout = 2 * time.Second
	}
	if opts.DNSTimeout == 0 {
		opts.DNSTimeout = 1500 * time.Millisecond
	}

	start := time.Now()
	var lines []Line

	// 1. VPN 状态
	lines = append(lines, checkVpn(opts)...)

	// 2. 当前节点
	lines = append(lines, checkCurrentNode(opts))

	// 3. 规则 / 节点数
	lines = append(lines, checkCounts(opts))

	// 4. DNS + 5. HTTP 并发跑 (都是 IO 阻塞, 串行没意义)
	var wg sync.WaitGroup
	var dnsLines, httpLines []Line
	var mu sync.Mutex

	wg.Add(2)
	go func() {
		defer wg.Done()
		l := checkDNS(ctx, opts)
		mu.Lock()
		dnsLines = l
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		l := checkHTTP(ctx, opts)
		mu.Lock()
		httpLines = l
		mu.Unlock()
	}()
	wg.Wait()

	lines = append(lines, dnsLines...)
	lines = append(lines, httpLines...)

	return Report{
		TS:        time.Now().UnixMilli(),
		Lines:     lines,
		ElapsedMs: time.Since(start).Milliseconds(),
	}
}

func checkVpn(opts Options) []Line {
	var out []Line
	vpn := false
	if opts.VpnRunning != nil {
		vpn = opts.VpnRunning()
	}
	if vpn {
		out = append(out, Line{Section: "VPN", Level: "ok", Msg: "TUN 运行中"})
	} else {
		out = append(out, Line{Section: "VPN", Level: "fail", Msg: "TUN 未启动", Detail: "VPN 未跑, 下面 HTTP/DNS 探测会走手机原生网络, 反映不了 Pilotty 的实际链路"})
	}
	agent := false
	if opts.AgentReady != nil {
		agent = opts.AgentReady()
	}
	if agent {
		out = append(out, Line{Section: "VPN", Level: "ok", Msg: "Agent 已配置 (LLM apiKey)"})
	} else {
		out = append(out, Line{Section: "VPN", Level: "warn", Msg: "Agent 未配置 (Chat 高级功能降级)", Detail: "Settings → Agent · LLM 里填 apiKey 即可"})
	}
	return out
}

func checkCurrentNode(opts Options) Line {
	if opts.CurrentNode == nil {
		return Line{Section: "节点", Level: "warn", Msg: "当前节点信息不可用"}
	}
	tag, ms, err := opts.CurrentNode()
	if err != nil {
		return Line{Section: "节点", Level: "fail", Msg: "查询当前节点失败", Detail: err.Error()}
	}
	if tag == "" {
		return Line{Section: "节点", Level: "warn", Msg: "未设置出口节点", Detail: "Nodes tab 选一个节点再自检"}
	}
	if ms <= 0 {
		return Line{Section: "节点", Level: "warn", Msg: fmt.Sprintf("当前节点: %s · 未测速", tag), Detail: "VPN 启动后到 Nodes tab 触发测速"}
	}
	level := "ok"
	switch {
	case ms > 1500:
		level = "warn"
	case ms > 4000:
		level = "fail"
	}
	return Line{Section: "节点", Level: level, Msg: fmt.Sprintf("当前节点: %s · %dms", tag, ms)}
}

func checkCounts(opts Options) Line {
	nc, rc := 0, 0
	if opts.NodeCount != nil {
		nc = opts.NodeCount()
	}
	if opts.RuleCount != nil {
		rc = opts.RuleCount()
	}
	// nc=0 有两种可能: (1) 用户没导过节点 = 真缺;
	// (2) VPN 未启动, Clash API 不可达, mobile.NodeCount 回退 0 但 overlay 里其实有节点。
	// 第二种只影响"显示"不影响功能, 所以 warn 不 fail。 由调用方提供 overlay fallback 更准,
	// 这里保守降级避免误导用户 "没节点要导入"。
	if nc == 0 {
		return Line{Section: "规则", Level: "warn", Msg: "节点数不可用", Detail: "VPN 未启动时 Clash API 不可达, 启动后重新自检可看到真实数量"}
	}
	return Line{Section: "规则", Level: "ok", Msg: fmt.Sprintf("节点 %d 个 · 规则 %d 条", nc, rc)}
}

func checkDNS(ctx context.Context, opts Options) []Line {
	var out []Line
	resolver := net.Resolver{PreferGo: false} // 走系统 resolver, 经 VPN 时就是 pilotty 的 sing-box
	for _, target := range opts.DNSTargets {
		cctx, cancel := context.WithTimeout(ctx, opts.DNSTimeout)
		start := time.Now()
		ips, err := resolver.LookupHost(cctx, target)
		cancel()
		dur := time.Since(start).Milliseconds()
		if err != nil {
			out = append(out, Line{
				Section: "DNS",
				Level:   "fail",
				Msg:     fmt.Sprintf("%s → 解析失败 (%dms)", target, dur),
				Detail:  err.Error(),
			})
			continue
		}
		if len(ips) == 0 {
			out = append(out, Line{Section: "DNS", Level: "warn", Msg: fmt.Sprintf("%s → 无结果 (%dms)", target, dur)})
			continue
		}
		// 展示第一个 IP + 总数
		first := ips[0]
		extra := ""
		if len(ips) > 1 {
			extra = fmt.Sprintf(" (+%d)", len(ips)-1)
		}
		level := "ok"
		if dur > 1000 {
			level = "warn"
		}
		out = append(out, Line{
			Section: "DNS",
			Level:   level,
			Msg:     fmt.Sprintf("%s → %s%s · %dms", target, first, extra, dur),
		})
	}
	return out
}

func checkHTTP(ctx context.Context, opts Options) []Line {
	type result struct {
		line Line
		idx  int
	}
	ch := make(chan result, len(opts.HTTPTargets))
	client := &http.Client{Timeout: opts.HTTPTimeout}
	for i, target := range opts.HTTPTargets {
		go func(i int, target string) {
			cctx, cancel := context.WithTimeout(ctx, opts.HTTPTimeout)
			defer cancel()
			req, err := http.NewRequestWithContext(cctx, http.MethodGet, target, nil)
			if err != nil {
				ch <- result{idx: i, line: Line{Section: "HTTP", Level: "fail", Msg: fmt.Sprintf("%s → 构造请求失败", shortURL(target)), Detail: err.Error()}}
				return
			}
			start := time.Now()
			resp, err := client.Do(req)
			dur := time.Since(start).Milliseconds()
			if err != nil {
				ch <- result{idx: i, line: Line{
					Section: "HTTP",
					Level:   "fail",
					Msg:     fmt.Sprintf("%s → 连接失败 (%dms)", shortURL(target), dur),
					Detail:  err.Error(),
				}}
				return
			}
			_ = resp.Body.Close()
			level := "ok"
			if resp.StatusCode >= 400 {
				level = "warn"
			}
			ch <- result{idx: i, line: Line{
				Section: "HTTP",
				Level:   level,
				Msg:     fmt.Sprintf("%s → HTTP %d · %dms", shortURL(target), resp.StatusCode, dur),
			}}
		}(i, target)
	}
	results := make([]Line, len(opts.HTTPTargets))
	for i := 0; i < len(opts.HTTPTargets); i++ {
		r := <-ch
		results[r.idx] = r.line
	}
	return results
}

func shortURL(u string) string {
	s := strings.TrimPrefix(u, "https://")
	s = strings.TrimPrefix(s, "http://")
	return s
}
