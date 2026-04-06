package monitor

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
)

// ConnectionMonitor 轮询 Clash API 获取连接信息
type ConnectionMonitor struct {
	adapter  engine.EngineAdapter
	interval time.Duration
	stopCh   chan struct{}
	// 上一次快照，用于计算速率
	prevConns map[string]engine.ConnectionInfo
	prevTime  time.Time
}

// NewConnectionMonitor 创建连接监控器
func NewConnectionMonitor(adapter engine.EngineAdapter, interval time.Duration) *ConnectionMonitor {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &ConnectionMonitor{
		adapter:   adapter,
		interval:  interval,
		prevConns: map[string]engine.ConnectionInfo{},
	}
}

// RunLive 进入实时连接监控模式，阻塞直到用户按 q 或 Ctrl+C
func (m *ConnectionMonitor) RunLive() {
	m.stopCh = make(chan struct{})

	// 设置终端为 raw 模式以捕获单个按键
	oldState, err := makeRaw(os.Stdin.Fd())
	if err != nil {
		fmt.Printf("无法进入 raw 模式: %v\n", err)
		fmt.Println("回退到单次快照模式...")
		m.printSnapshot()
		return
	}
	defer restore(os.Stdin.Fd(), oldState)

	// 启动按键监听
	keyCh := make(chan byte, 1)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			select {
			case keyCh <- buf[0]:
			default:
			}
		}
	}()

	fmt.Print("进入实时连接监控（按 q 退出）\n")

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// 立即显示一次
	m.renderLive()

	for {
		select {
		case <-ticker.C:
			m.renderLive()
		case key := <-keyCh:
			if key == 'q' || key == 'Q' || key == 3 { // q or Ctrl+C
				fmt.Print("\033[2J\033[H") // 清屏
				fmt.Println("已退出实时监控。")
				return
			}
		case <-m.stopCh:
			return
		}
	}
}

// Stop 停止监控
func (m *ConnectionMonitor) Stop() {
	if m.stopCh != nil {
		select {
		case <-m.stopCh:
		default:
			close(m.stopCh)
		}
	}
}

// printSnapshot 打印一次连接快照（非实时模式）
func (m *ConnectionMonitor) printSnapshot() {
	conns, err := m.adapter.GetConnections()
	if err != nil {
		fmt.Printf("获取连接失败: %v\n", err)
		return
	}
	if len(conns) == 0 {
		fmt.Println("当前没有活跃连接。")
		return
	}
	fmt.Printf("活跃连接: %d\n", len(conns))
	fmt.Println(formatConnectionTable(conns, nil))
}

// renderLive 清屏并重新绘制实时连接表
func (m *ConnectionMonitor) renderLive() {
	conns, err := m.adapter.GetConnections()
	if err != nil {
		return
	}

	// 计算速率
	now := time.Now()
	speeds := map[string][2]float64{} // id → [upload_speed, download_speed]
	elapsed := now.Sub(m.prevTime).Seconds()
	if elapsed > 0 && len(m.prevConns) > 0 {
		for _, c := range conns {
			if prev, ok := m.prevConns[c.ID]; ok {
				upSpeed := float64(c.Upload-prev.Upload) / elapsed
				downSpeed := float64(c.Download-prev.Download) / elapsed
				speeds[c.ID] = [2]float64{upSpeed, downSpeed}
			}
		}
	}

	// 更新快照
	m.prevConns = map[string]engine.ConnectionInfo{}
	for _, c := range conns {
		m.prevConns[c.ID] = c
	}
	m.prevTime = now

	// 计算总速率
	var totalUp, totalDown float64
	for _, s := range speeds {
		totalUp += s[0]
		totalDown += s[1]
	}

	// 清屏 + 光标归位
	fmt.Print("\033[2J\033[H")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("活跃连接: \033[36m%d\033[0m | ↑ \033[32m%s\033[0m | ↓ \033[33m%s\033[0m\n",
		len(conns), FormatSpeed(totalUp), FormatSpeed(totalDown))
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if len(conns) == 0 {
		fmt.Println("  （无活跃连接）")
	} else {
		fmt.Print(formatConnectionTable(conns, speeds))
	}

	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  按 \033[36mq\033[0m 退出 | 刷新间隔 %v\n", m.interval)
}

// formatConnectionTable 格式化连接表
func formatConnectionTable(conns []engine.ConnectionInfo, speeds map[string][2]float64) string {
	// 按下载量排序
	sort.Slice(conns, func(i, j int) bool {
		return conns[i].Download > conns[j].Download
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  %-30s %-6s %-14s %8s %8s\n",
		"目标", "协议", "节点", "↑", "↓"))

	for _, c := range conns {
		dest := c.Destination
		if len(dest) > 30 {
			dest = dest[:27] + "..."
		}

		// 提取链路中的出站节点（最后一个非空链路）
		node := extractNode(c.Chain)
		if len(node) > 14 {
			node = node[:11] + "..."
		}

		proto := strings.ToUpper(c.Protocol)

		var upStr, downStr string
		if speeds != nil {
			if s, ok := speeds[c.ID]; ok {
				upStr = FormatSpeed(s[0])
				downStr = FormatSpeed(s[1])
			} else {
				upStr = FormatBytes(c.Upload)
				downStr = FormatBytes(c.Download)
			}
		} else {
			upStr = FormatBytes(c.Upload)
			downStr = FormatBytes(c.Download)
		}

		sb.WriteString(fmt.Sprintf("  %-30s %-6s %-14s %8s %8s\n",
			dest, proto, node, upStr, downStr))
	}
	return sb.String()
}

// extractNode 从 chain 中提取出站节点名
func extractNode(chain string) string {
	if chain == "" {
		return "-"
	}
	// chain 格式: "美国A → proxy-group" — 取第一个
	parts := strings.Split(chain, " → ")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return chain
}

// FormatBytes 格式化字节数
func FormatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
}

// FormatSpeed 格式化速率
func FormatSpeed(bytesPerSec float64) string {
	return FormatBytes(int64(bytesPerSec)) + "/s"
}
