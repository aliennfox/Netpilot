package observe

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// LogEntry 一条运行时日志条目 (捕获自 Go log.Print/log.Println 等)。
// Level/Tag 目前是空 —— Go 标准 log 不带 level 概念; 未来可加 zap/slog 替换底层,
// 再扩字段。 当前 UI 只需要 ts + msg 够用。
type LogEntry struct {
	Ts    int64  `json:"ts"`              // unix 毫秒
	Level string `json:"level,omitempty"` // "I"|"W"|"E"|"D"; 默认 "I"
	Tag   string `json:"tag,omitempty"`   // 可选 tag (来自 log prefix 首段)
	Msg   string `json:"msg"`
}

// LogRing 固定容量 ring buffer, 线程安全, io.Writer 兼容。
type LogRing struct {
	mu       sync.Mutex
	entries  []LogEntry
	capacity int
	next     int
	filled   bool
}

// DefaultLogRing 进程级单例 (NewClient 初始化时 Attach)。
var DefaultLogRing = NewLogRing(500)

func NewLogRing(capacity int) *LogRing {
	if capacity <= 0 {
		capacity = 500
	}
	return &LogRing{
		entries:  make([]LogEntry, capacity),
		capacity: capacity,
	}
}

// Write 实现 io.Writer, 被 log.SetOutput 调用, 每个日志 record 进来一次 (含前缀 + newline)。
func (r *LogRing) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	if msg == "" {
		return len(p), nil
	}
	r.Append(LogEntry{
		Ts:    time.Now().UnixMilli(),
		Level: "I",
		Msg:   msg,
	})
	return len(p), nil
}

// Append 显式追加 (例如 telemetry 桥接 tool 调用日志可用)。
func (r *LogRing) Append(e LogEntry) {
	if e.Ts == 0 {
		e.Ts = time.Now().UnixMilli()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[r.next] = e
	r.next = (r.next + 1) % r.capacity
	if r.next == 0 {
		r.filled = true
	}
}

// Recent 返回最近 n 条 (时间序升序, 最新在最后)。 n<=0 或超容量 → 全量。
func (r *LogRing) Recent(n int) []LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	size := r.capacity
	if !r.filled {
		size = r.next
	}
	if n <= 0 || n > size {
		n = size
	}
	out := make([]LogEntry, 0, n)
	start := r.next - n
	if start < 0 {
		start += r.capacity
	}
	for i := 0; i < n; i++ {
		idx := (start + i) % r.capacity
		if r.entries[idx].Ts > 0 {
			out = append(out, r.entries[idx])
		}
	}
	return out
}

// RecentJSON 方便 gomobile 层透传。
func (r *LogRing) RecentJSON(n int) string {
	b, _ := json.Marshal(r.Recent(n))
	return string(b)
}

// AttachDefault 把 Go 标准 log 的输出复制到 DefaultLogRing, 保留原有 stderr 输出。
// NewClient 启动时调一次即可, 幂等 (靠 sync.Once)。
var attachOnce sync.Once

func AttachDefault() {
	attachOnce.Do(func() {
		log.SetOutput(io.MultiWriter(os.Stderr, DefaultLogRing))
		// 加一条首条日志, 方便 UI 侧判断是否接上
		log.Println("[observe] log ring attached, capacity=500")
	})
}
