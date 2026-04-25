package agent

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestHistory_AddTrimsToMaxEntries 超过上限丢最旧的
func TestHistory_AddTrimsToMaxEntries(t *testing.T) {
	h := NewConversationHistory(3)
	h.Add("user", "msg1", "agent")
	h.Add("assistant", "reply1", "agent")
	h.Add("user", "msg2", "agent")
	h.Add("assistant", "reply2", "agent")
	h.Add("user", "msg3", "agent")

	if got := h.Len(); got != 3 {
		t.Errorf("Len: want 3 (trimmed), got %d", got)
	}
	recent := h.GetRecent(10)
	if len(recent) != 3 {
		t.Fatalf("GetRecent: want 3, got %d", len(recent))
	}
	// 最旧的 msg1 / reply1 应该被丢, 最新 3 条是 msg2 / reply2 / msg3
	if recent[0].Content != "msg2" || recent[1].Content != "reply2" || recent[2].Content != "msg3" {
		t.Errorf("trim kept wrong slice: %+v", []string{recent[0].Content, recent[1].Content, recent[2].Content})
	}
}

// TestHistory_GetRecentDefensiveCopy GetRecent 返回副本, 改它不污染内部 state
func TestHistory_GetRecentDefensiveCopy(t *testing.T) {
	h := NewConversationHistory(10)
	h.Add("user", "original", "agent")

	recent := h.GetRecent(10)
	if len(recent) != 1 {
		t.Fatalf("setup: want 1 entry, got %d", len(recent))
	}
	recent[0].Content = "MUTATED"

	// 内部 state 应未被污染
	again := h.GetRecent(10)
	if again[0].Content == "MUTATED" {
		t.Errorf("GetRecent should return defensive copy, internal state was mutated")
	}
}

// TestHistory_GetRecentEdgeN n<=0 / n > len 的边界
func TestHistory_GetRecentEdgeN(t *testing.T) {
	h := NewConversationHistory(10)
	if got := h.GetRecent(5); got != nil {
		t.Errorf("empty history n=5 should return nil, got %+v", got)
	}
	h.Add("user", "x", "agent")
	if got := h.GetRecent(0); got != nil {
		t.Errorf("n=0 should return nil, got %+v", got)
	}
	if got := h.GetRecent(-1); got != nil {
		t.Errorf("n<0 should return nil, got %+v", got)
	}
	if got := h.GetRecent(99); len(got) != 1 {
		t.Errorf("n>len should clamp to len, got %d", len(got))
	}
}

// TestHistory_ToMessagesContext_Empty 空 history 返回空字符串
func TestHistory_ToMessagesContext_Empty(t *testing.T) {
	h := NewConversationHistory(10)
	if got := h.ToMessagesContext(); got != "" {
		t.Errorf("empty history should produce empty context, got %q", got)
	}
}

// TestHistory_ToMessagesContext_FormatsRoles user / assistant 都有 prefix
func TestHistory_ToMessagesContext_FormatsRoles(t *testing.T) {
	h := NewConversationHistory(10)
	h.Add("user", "你好", "agent")
	h.Add("assistant", "你好,有什么可以帮你?", "agent")

	got := h.ToMessagesContext()
	if !strings.Contains(got, "## 最近对话记录") {
		t.Errorf("missing header: %q", got)
	}
	if !strings.Contains(got, "用户: 你好") {
		t.Errorf("missing user line: %q", got)
	}
	if !strings.Contains(got, "助手: 你好,有什么可以帮你?") {
		t.Errorf("missing assistant line: %q", got)
	}
}

// TestHistory_ToMessagesContext_TruncatesPerEntry 单条用户消息按 200 rune 截, 助手 300 rune
func TestHistory_ToMessagesContext_TruncatesPerEntry(t *testing.T) {
	h := NewConversationHistory(10)
	longUser := strings.Repeat("中", 250)      // 250 中文 rune
	longAssistant := strings.Repeat("文", 350) // 350 中文 rune
	h.Add("user", longUser, "agent")
	h.Add("assistant", longAssistant, "agent")

	got := h.ToMessagesContext()
	// user 截到 200 + "..."
	if !strings.Contains(got, "...") {
		t.Errorf("expected truncation marker, got %q", got)
	}
	// 用户那行不能含全 250 个中文
	if strings.Contains(got, strings.Repeat("中", 250)) {
		t.Errorf("user message should be truncated to 200 runes")
	}
	// 助手那行不能含全 350 个中文
	if strings.Contains(got, strings.Repeat("文", 350)) {
		t.Errorf("assistant message should be truncated to 300 runes")
	}
}

// TestHistory_ToMessagesContext_TotalLengthCap 累计超 2000 字符的多条消息会被截断
func TestHistory_ToMessagesContext_TotalLengthCap(t *testing.T) {
	h := NewConversationHistory(20)
	// 写 10 条助手长消息 (每条截到 300 rune ~= 900 byte UTF-8), 累加会超过 2000
	for i := 0; i < 10; i++ {
		h.Add("assistant", strings.Repeat("文", 300), "agent")
	}
	got := h.ToMessagesContext()
	// 实际 byte 长度应在 2000 字符附近 (header 约 30 byte + 若干条 ~903 byte/条)
	if len(got) > 2200 {
		t.Errorf("ToMessagesContext should cap around 2000 bytes, got %d bytes", len(got))
	}
	// 但起码包含 header + 第一条
	if !strings.Contains(got, "## 最近对话记录") {
		t.Errorf("header missing: %q", got[:100])
	}
}

// TestHistory_ToMessagesContext_OnlyLast10 最多取最近 10 条 (即使 history 里有 20)
func TestHistory_ToMessagesContext_OnlyLast10(t *testing.T) {
	h := NewConversationHistory(50)
	for i := 0; i < 20; i++ {
		h.Add("user", strings.Repeat("a", 5), "agent")
	}
	got := h.ToMessagesContext()
	// 数 "用户:" 出现的次数, 应该不超过 10 (但加 header 检查避免 prefix 误算)
	count := strings.Count(got, "用户: ")
	if count > 10 {
		t.Errorf("ToMessagesContext should take at most 10 entries, got %d 用户 lines", count)
	}
}

// TestHistory_FormatDisplay 格式化输出含时间戳 + role + source 标记
func TestHistory_FormatDisplay(t *testing.T) {
	h := NewConversationHistory(10)
	if got := h.FormatDisplay(); got != "暂无对话记录。" {
		t.Errorf("empty: %q", got)
	}

	h.Add("user", "你好", "local")
	h.Add("assistant", "回复", "agent")

	got := h.FormatDisplay()
	if !strings.Contains(got, "用户") || !strings.Contains(got, "助手") {
		t.Errorf("missing role labels: %q", got)
	}
	if !strings.Contains(got, "[本地]") || !strings.Contains(got, "[Agent]") {
		t.Errorf("missing source markers: %q", got)
	}
}

// TestHistory_Clear 清空后 Len=0
func TestHistory_Clear(t *testing.T) {
	h := NewConversationHistory(10)
	h.Add("user", "x", "agent")
	h.Add("assistant", "y", "agent")
	h.Clear()
	if got := h.Len(); got != 0 {
		t.Errorf("Clear should empty history, got Len=%d", got)
	}
	// Add 仍然能正常工作
	h.Add("user", "z", "agent")
	if got := h.Len(); got != 1 {
		t.Errorf("Add after Clear should work, got Len=%d", got)
	}
}

// TestHistory_SaveAndLoad 持久化 round-trip: SaveToFile + LoadFromFile 内容一致
func TestHistory_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	h1 := NewConversationHistory(10)
	h1.Add("user", "msg1", "agent")
	h1.Add("assistant", "reply1", "local")
	h1.Add("user", "msg2", "agent")

	if err := h1.SaveToFile(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	h2 := NewConversationHistory(10)
	if err := h2.LoadFromFile(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if h2.Len() != 3 {
		t.Errorf("loaded len: want 3, got %d", h2.Len())
	}
	got := h2.GetRecent(10)
	if got[0].Content != "msg1" || got[2].Content != "msg2" {
		t.Errorf("loaded content wrong: %+v",
			[]string{got[0].Content, got[1].Content, got[2].Content})
	}
	if got[1].Source != "local" {
		t.Errorf("Source field should persist, got %q", got[1].Source)
	}
}

// TestHistory_LoadFromFile_NoFile 文件不存在返回 nil (首次启动)
func TestHistory_LoadFromFile_NoFile(t *testing.T) {
	h := NewConversationHistory(10)
	err := h.LoadFromFile("/nonexistent/path/history.json")
	if err != nil {
		t.Errorf("missing file should return nil err (first launch), got %v", err)
	}
}

// TestHistory_LoadFromFile_TrimsToMax 加载时超出 maxEntries 截到上限
func TestHistory_LoadFromFile_TrimsToMax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	// h1 maxEntries=20, 写 15 条
	h1 := NewConversationHistory(20)
	for i := 0; i < 15; i++ {
		h1.Add("user", "x", "agent")
	}
	if err := h1.SaveToFile(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	// h2 maxEntries=5, 加载应截到 5 (保留最新 5 条)
	h2 := NewConversationHistory(5)
	if err := h2.LoadFromFile(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := h2.Len(); got != 5 {
		t.Errorf("loaded should be trimmed to maxEntries=5, got %d", got)
	}
}

// TestTruncateRunes 中文按 rune 截, 不破坏 UTF-8
func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		in     string
		maxLen int
		want   string
	}{
		{"短", 10, "短"},
		{"abcdef", 3, "abc..."},
		{"中文测试很长很长", 4, "中文测试..."},
		{"", 5, ""},
	}
	for _, tc := range tests {
		got := truncateRunes(tc.in, tc.maxLen)
		if got != tc.want {
			t.Errorf("truncateRunes(%q, %d) = %q, want %q", tc.in, tc.maxLen, got, tc.want)
		}
	}
}
