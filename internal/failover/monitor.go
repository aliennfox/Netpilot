// Package failover 实现节点健康监控与自动故障切换。
//
// 工作流程：
//  1. 周期性探测当前活跃节点的延迟（HTTP HEAD via Clash API /proxies/:tag/delay）
//  2. 连续失败次数达到阈值 → 触发 failover
//  3. failover 时对 group 内全部节点逐一探测，挑选延迟最低的健康节点切换
//  4. 切换成功后清零失败计数；切换失败仅记录，不抛 panic
//
// 设计目标：
//   - 默认关闭，调用方显式 Start
//   - 调用方负责持有 ctx；Stop() 幂等
//   - 与 Tool Pipeline 解耦：直接走 EngineAdapter，避免快照/审计噪音
//   - 单次只处理一个 group（默认 "proxy-group"）
package failover

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// Config 故障切换配置。零值即默认值。
type Config struct {
	Group     string        // 监控的代理组 tag；默认 "proxy-group"
	TestURL   string        // 延迟测试 URL；默认 https://www.gstatic.com/generate_204
	Interval  time.Duration // 探测间隔；默认 30s
	Timeout   time.Duration // 单次探测超时；默认 5s
	FailLimit int           // 连续失败多少次后触发切换；默认 3
	Cooldown  time.Duration // 两次 failover 之间的最小冷却；默认 60s
}

func (c *Config) applyDefaults() {
	if c.Group == "" {
		c.Group = "proxy-group"
	}
	if c.TestURL == "" {
		c.TestURL = "https://www.gstatic.com/generate_204"
	}
	if c.Interval <= 0 {
		c.Interval = 30 * time.Second
	}
	if c.Timeout <= 0 {
		c.Timeout = 5 * time.Second
	}
	if c.FailLimit <= 0 {
		c.FailLimit = 3
	}
	if c.Cooldown <= 0 {
		c.Cooldown = 60 * time.Second
	}
}

// Status 表示监控当前状态快照。
type Status struct {
	Running          bool      `json:"running"`
	Group            string    `json:"group"`
	CurrentNode      string    `json:"current_node"`
	ConsecutiveFails int       `json:"consecutive_fails"`
	LastSwitchAt     time.Time `json:"last_switch_at,omitempty"`
	LastSwitchTo     string    `json:"last_switch_to,omitempty"`
	LastError        string    `json:"last_error,omitempty"`
	TotalSwitches    int       `json:"total_switches"`
	TotalProbes      int       `json:"total_probes"`
}

// Monitor 是后台健康监控器。线程安全。
type Monitor struct {
	adapter engine.EngineAdapter
	cfg     Config

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
	status  Status
}

// New 构造一个监控器。不会立即启动。
func New(adapter engine.EngineAdapter, cfg Config) *Monitor {
	cfg.applyDefaults()
	return &Monitor{
		adapter: adapter,
		cfg:     cfg,
		status:  Status{Group: cfg.Group},
	}
}

// Start 启动后台 goroutine。重复调用安全（只启动一次）。
func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.running = true
	m.status.Running = true
	m.mu.Unlock()
	go m.loop(ctx)
}

// Stop 停止后台 goroutine。幂等。
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	if m.cancel != nil {
		m.cancel()
	}
	m.running = false
	m.status.Running = false
}

// Status 返回当前监控状态快照。
func (m *Monitor) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.status
	return s
}

// SetConfig 在线更新配置（不会重启 loop，下一个 tick 生效）。
func (m *Monitor) SetConfig(cfg Config) {
	cfg.applyDefaults()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	m.status.Group = cfg.Group
}

func (m *Monitor) loop(ctx context.Context) {
	// 启动后立即跑一次，避免等待 interval
	m.tick(ctx)
	t := time.NewTicker(m.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.tick(ctx)
		}
	}
}

func (m *Monitor) tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	m.mu.Lock()
	cfg := m.cfg
	m.status.TotalProbes++
	m.mu.Unlock()

	g, err := m.adapter.GetProxyGroup(cfg.Group)
	if err != nil || g == nil || g.Now == "" {
		m.recordFail(fmt.Sprintf("group lookup failed: %v", err))
		return
	}
	m.mu.Lock()
	m.status.CurrentNode = g.Now
	m.mu.Unlock()

	latency, err := m.adapter.TestLatency(g.Now, cfg.TestURL, cfg.Timeout)
	if err != nil || latency <= 0 {
		msg := "timeout"
		if err != nil {
			msg = err.Error()
		}
		m.recordFail(msg)

		m.mu.Lock()
		fails := m.status.ConsecutiveFails
		lastSwitch := m.status.LastSwitchAt
		m.mu.Unlock()

		if fails >= cfg.FailLimit && time.Since(lastSwitch) >= cfg.Cooldown {
			m.failover(ctx, g)
		}
		return
	}
	// 健康：清零失败计数
	m.mu.Lock()
	m.status.ConsecutiveFails = 0
	m.status.LastError = ""
	m.mu.Unlock()
}

func (m *Monitor) recordFail(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.ConsecutiveFails++
	m.status.LastError = reason
}

// failover 在 group 内挑选最佳健康节点并切换。
func (m *Monitor) failover(ctx context.Context, g *engine.ProxyGroup) {
	if ctx.Err() != nil {
		return
	}
	m.mu.Lock()
	cfg := m.cfg
	m.mu.Unlock()

	type cand struct {
		tag     string
		latency int
	}
	cands := make([]cand, 0, len(g.All))
	for _, p := range g.All {
		if p.Tag == g.Now {
			continue
		}
		// 跳过明显的 selector / 内置 outbound
		if p.Type == "Selector" || p.Type == "URLTest" || p.Type == "selector" || p.Type == "urltest" {
			continue
		}
		l, err := m.adapter.TestLatency(p.Tag, cfg.TestURL, cfg.Timeout)
		if err != nil || l <= 0 {
			continue
		}
		cands = append(cands, cand{p.Tag, l})
	}
	if len(cands) == 0 {
		m.mu.Lock()
		m.status.LastError = "no healthy node available"
		m.mu.Unlock()
		return
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].latency < cands[j].latency })
	best := cands[0]

	if err := m.adapter.SetActiveProxy(cfg.Group, best.tag); err != nil {
		m.mu.Lock()
		m.status.LastError = fmt.Sprintf("switch failed: %v", err)
		m.mu.Unlock()
		return
	}
	m.mu.Lock()
	m.status.ConsecutiveFails = 0
	m.status.LastSwitchAt = time.Now()
	m.status.LastSwitchTo = best.tag
	m.status.TotalSwitches++
	m.status.LastError = ""
	m.mu.Unlock()
}
