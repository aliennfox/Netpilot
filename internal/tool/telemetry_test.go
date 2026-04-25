package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestTelemetry_LogAndRecent Log 后 Recent 立即可见, in-memory ring 行为正确
func TestTelemetry_LogAndRecent(t *testing.T) {
	tl := NewTelemetryLogger(t.TempDir())

	tl.Log(TelemetryEntry{Tool: "switch_node", Success: true, DurationMs: 100, Timestamp: time.Now()})
	tl.Log(TelemetryEntry{Tool: "set_per_app_vpn", Success: true, DurationMs: 50, Timestamp: time.Now()})

	got := tl.Recent(10)
	if len(got) != 2 {
		t.Fatalf("Recent: want 2 entries, got %d", len(got))
	}
	if got[0].Tool != "switch_node" || got[1].Tool != "set_per_app_vpn" {
		t.Errorf("order/contents wrong: %+v", got)
	}
}

// TestTelemetry_RecentTrimsToRequest Recent(n) 限定返回数量, n > len 时不 panic 而是返回全部
func TestTelemetry_RecentTrimsToRequest(t *testing.T) {
	tl := NewTelemetryLogger(t.TempDir())
	for i := 0; i < 5; i++ {
		tl.Log(TelemetryEntry{Tool: "x", Success: true, Timestamp: time.Now()})
	}

	if got := len(tl.Recent(3)); got != 3 {
		t.Errorf("Recent(3): want 3, got %d", got)
	}
	if got := len(tl.Recent(100)); got != 5 {
		t.Errorf("Recent(100) with only 5 entries: want 5, got %d", got)
	}
}

// TestTelemetry_RecentReturnsLatest Recent 返回最新的 n 条 (尾部), 不是头部
func TestTelemetry_RecentReturnsLatest(t *testing.T) {
	tl := NewTelemetryLogger(t.TempDir())
	for i := 0; i < 5; i++ {
		tl.Log(TelemetryEntry{Tool: "tool" + string(rune('A'+i)), Success: true, Timestamp: time.Now()})
	}

	got := tl.Recent(2)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	// 最后两条是 toolD / toolE
	if got[0].Tool != "toolD" || got[1].Tool != "toolE" {
		t.Errorf("Recent should return latest, got %+v", []string{got[0].Tool, got[1].Tool})
	}
}

// TestTelemetry_InMemoryRingTrim ring 上限 100, 写 105 条只保留最近 100
func TestTelemetry_InMemoryRingTrim(t *testing.T) {
	tl := NewTelemetryLogger(t.TempDir())
	for i := 0; i < 105; i++ {
		tl.Log(TelemetryEntry{Tool: "x", Success: true, Timestamp: time.Now()})
	}
	if got := len(tl.Recent(200)); got != 100 {
		t.Errorf("ring trim: write 105 expect ring=100, got %d", got)
	}
}

// TestTelemetry_PersistsToJSONL 写入后 telemetry.jsonl 真实落盘
func TestTelemetry_PersistsToJSONL(t *testing.T) {
	dir := t.TempDir()
	tl := NewTelemetryLogger(dir)

	tl.Log(TelemetryEntry{Tool: "switch_node", Success: true, Timestamp: time.Now()})
	tl.Log(TelemetryEntry{Tool: "set_per_app_vpn", Success: false, Error: "denied", Timestamp: time.Now()})

	data, err := os.ReadFile(filepath.Join(dir, "telemetry.jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("jsonl line count: want 2 got %d (raw: %q)", len(lines), string(data))
	}
	if !strings.Contains(lines[0], `"tool":"switch_node"`) {
		t.Errorf("first line missing tool: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"error":"denied"`) {
		t.Errorf("second line missing error: %s", lines[1])
	}
}

// TestTelemetry_LoadRecentFromDisk 跨进程重启场景: 写一波关闭 logger, 新建 logger
// 用 LoadRecentFromDisk 应能拿到全部 entries (in-memory ring 是空的)
func TestTelemetry_LoadRecentFromDisk(t *testing.T) {
	dir := t.TempDir()

	// 第一次会话: 写 5 条
	tl1 := NewTelemetryLogger(dir)
	for i := 0; i < 5; i++ {
		tl1.Log(TelemetryEntry{Tool: "tool" + string(rune('A'+i)), Success: true, Timestamp: time.Now()})
	}

	// 模拟重启: 新 logger, in-memory ring 必空
	tl2 := NewTelemetryLogger(dir)
	if got := tl2.Recent(10); len(got) != 0 {
		t.Fatalf("fresh logger ring should be empty, got %d", len(got))
	}

	// LoadRecentFromDisk 拉历史
	disk := tl2.LoadRecentFromDisk(10)
	if len(disk) != 5 {
		t.Fatalf("load 10, want 5 (only 5 written), got %d", len(disk))
	}
	if disk[0].Tool != "toolA" || disk[4].Tool != "toolE" {
		t.Errorf("disk order wrong: first=%q last=%q", disk[0].Tool, disk[4].Tool)
	}
}

// TestTelemetry_LoadRecentFromDisk_RingBuffer LoadRecentFromDisk(n) 只返回最后 n 条,
// 历史 100 条 + n=10 → 最后 10 条 (toolP-toolY 等)
func TestTelemetry_LoadRecentFromDisk_RingBuffer(t *testing.T) {
	dir := t.TempDir()
	tl := NewTelemetryLogger(dir)

	// 写 50 条, tool 名是 tool0 ... tool49
	for i := 0; i < 50; i++ {
		tl.Log(TelemetryEntry{Tool: "tool", Success: true, DurationMs: int64(i), Timestamp: time.Now()})
	}

	disk := tl.LoadRecentFromDisk(10)
	if len(disk) != 10 {
		t.Fatalf("LoadRecentFromDisk(10): want 10, got %d", len(disk))
	}
	// 最后 10 条 DurationMs 应该是 40..49
	if disk[0].DurationMs != 40 || disk[9].DurationMs != 49 {
		t.Errorf("ring buffer kept wrong slice: first=%d last=%d (want 40..49)",
			disk[0].DurationMs, disk[9].DurationMs)
	}
}

// TestTelemetry_LoadRecentFromDisk_NoFile 文件不存在返回 nil 而不是 panic / error
func TestTelemetry_LoadRecentFromDisk_NoFile(t *testing.T) {
	tl := NewTelemetryLogger(t.TempDir())
	got := tl.LoadRecentFromDisk(10)
	if got != nil && len(got) != 0 {
		t.Errorf("missing file should yield nil/empty, got %+v", got)
	}
}

// TestTelemetry_LoadRecentFromDisk_NSentinel n <= 0 返回 nil
func TestTelemetry_LoadRecentFromDisk_NSentinel(t *testing.T) {
	tl := NewTelemetryLogger(t.TempDir())
	tl.Log(TelemetryEntry{Tool: "x", Success: true, Timestamp: time.Now()})

	if got := tl.LoadRecentFromDisk(0); got != nil {
		t.Errorf("n=0 should return nil, got %+v", got)
	}
	if got := tl.LoadRecentFromDisk(-1); got != nil {
		t.Errorf("n<0 should return nil, got %+v", got)
	}
}

// TestTelemetry_LoadRecentFromDisk_SkipMalformed 损坏行不让其它条目失活
func TestTelemetry_LoadRecentFromDisk_SkipMalformed(t *testing.T) {
	dir := t.TempDir()
	tl := NewTelemetryLogger(dir)

	tl.Log(TelemetryEntry{Tool: "good1", Success: true, Timestamp: time.Now()})
	// 手动 append 损坏行
	logFile := filepath.Join(dir, "telemetry.jsonl")
	f, _ := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("{this is not valid json\n")
	f.WriteString("\n") // 空行
	f.Close()
	tl.Log(TelemetryEntry{Tool: "good2", Success: true, Timestamp: time.Now()})

	got := tl.LoadRecentFromDisk(10)
	if len(got) != 2 {
		t.Fatalf("malformed lines should be skipped, expect 2 valid entries, got %d", len(got))
	}
	if got[0].Tool != "good1" || got[1].Tool != "good2" {
		t.Errorf("kept entries wrong: %+v", []string{got[0].Tool, got[1].Tool})
	}
}
