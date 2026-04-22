// Package observe 维护 Pilotty 运行时的轻量可观测状态 (ring buffer 型),
// 供 mobile 层导出给 UI 层做流量图 / 日志窗展示。
//
// 设计: 不做持久化, 进程退出即丢。 目的是让 UI 能零开销订阅最近 N 秒/条数据。
package observe

import (
	"encoding/json"
	"sync"
	"time"
)

// TrafficPoint 一秒采样点: 当前秒内上行/下行字节数的累计 (非增量, Clash API 原值)。
type TrafficPoint struct {
	Ts       int64 `json:"t"`         // unix 秒
	Upload   int64 `json:"up"`        // 累计 upload bytes
	Download int64 `json:"down"`      // 累计 download bytes
	UpRate   int64 `json:"up_rate"`   // 当前秒比上一秒的增量 (bytes/s)
	DownRate int64 `json:"down_rate"` // 同上
}

// TrafficRing 固定容量 ring buffer, 线程安全。
type TrafficRing struct {
	mu       sync.Mutex
	points   []TrafficPoint
	capacity int
	next     int // 下一个写入位置
	filled   bool
}

func NewTrafficRing(capacity int) *TrafficRing {
	if capacity <= 0 {
		capacity = 60
	}
	return &TrafficRing{
		points:   make([]TrafficPoint, capacity),
		capacity: capacity,
	}
}

// Append 推入一条新采样。 自动计算 UpRate/DownRate (基于前一条)。
func (r *TrafficRing) Append(upload, download int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prev := r.lastLocked()
	pt := TrafficPoint{
		Ts:       time.Now().Unix(),
		Upload:   upload,
		Download: download,
	}
	if prev != nil {
		if delta := upload - prev.Upload; delta > 0 {
			pt.UpRate = delta
		}
		if delta := download - prev.Download; delta > 0 {
			pt.DownRate = delta
		}
	}
	r.points[r.next] = pt
	r.next = (r.next + 1) % r.capacity
	if r.next == 0 {
		r.filled = true
	}
}

// lastLocked 返回最新一条 (caller 持锁)。
func (r *TrafficRing) lastLocked() *TrafficPoint {
	if !r.filled && r.next == 0 {
		return nil
	}
	prevIdx := r.next - 1
	if prevIdx < 0 {
		prevIdx = r.capacity - 1
	}
	if r.points[prevIdx].Ts == 0 {
		return nil
	}
	cp := r.points[prevIdx]
	return &cp
}

// Recent 返回最近 n 条 (时间序升序, 最新在最后)。 n<=0 或 n>buffer 则返回全量。
func (r *TrafficRing) Recent(n int) []TrafficPoint {
	r.mu.Lock()
	defer r.mu.Unlock()
	size := r.capacity
	if !r.filled {
		size = r.next
	}
	if n <= 0 || n > size {
		n = size
	}
	out := make([]TrafficPoint, 0, n)
	// 从最老的位置开始读 n 条
	start := r.next - n
	if start < 0 {
		start += r.capacity
	}
	for i := 0; i < n; i++ {
		idx := (start + i) % r.capacity
		if r.points[idx].Ts > 0 {
			out = append(out, r.points[idx])
		}
	}
	return out
}

// Clear 清空 buffer (VPN 停止后可选用, 避免显示旧数据)。
func (r *TrafficRing) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.points = make([]TrafficPoint, r.capacity)
	r.next = 0
	r.filled = false
}

// RecentJSON 返回最近 n 条的 JSON 数组表示, 方便 gomobile 层直接透传。
func (r *TrafficRing) RecentJSON(n int) string {
	pts := r.Recent(n)
	b, _ := json.Marshal(pts)
	return string(b)
}
