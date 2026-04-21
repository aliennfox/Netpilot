package agent

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ConversationHistory 管理对话历史，支持摘要注入到 system prompt。
// 不持久化——进程退出即清空。
type ConversationHistory struct {
	mu         sync.Mutex
	messages   []HistoryEntry
	maxEntries int // 最多保留的消息条数（一问一答 = 2 条）
}

// HistoryEntry 单条对话记录
type HistoryEntry struct {
	Role      string // "user" 或 "assistant"
	Content   string
	Timestamp time.Time
	Source    string // "local" (Local Engine) 或 "agent" (LLM Agent)
}

// NewConversationHistory 创建对话历史管理器
func NewConversationHistory(maxEntries int) *ConversationHistory {
	return &ConversationHistory{
		messages:   make([]HistoryEntry, 0, maxEntries),
		maxEntries: maxEntries,
	}
}

// Add 添加一条对话记录
func (h *ConversationHistory) Add(role, content, source string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, HistoryEntry{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		Source:    source,
	})
	// 超过上限则丢弃最旧的
	if len(h.messages) > h.maxEntries {
		h.messages = h.messages[len(h.messages)-h.maxEntries:]
	}
}

// GetRecent 返回最近 n 条消息
func (h *ConversationHistory) GetRecent(n int) []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	if n <= 0 || len(h.messages) == 0 {
		return nil
	}
	if n > len(h.messages) {
		n = len(h.messages)
	}
	// 返回副本，避免外部持有引用导致竞态
	result := make([]HistoryEntry, n)
	copy(result, h.messages[len(h.messages)-n:])
	return result
}

// ToMessagesContext 将最近对话压缩为摘要文本，用于注入 system prompt。
// 最多取最近 10 条消息（5 轮对话），总长度不超过 2000 字符。
func (h *ConversationHistory) ToMessagesContext() string {
	recent := h.GetRecent(10) // 最近 5 轮（10 条消息）
	if len(recent) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 最近对话记录\n")

	for _, entry := range recent {
		var line string
		if entry.Role == "user" {
			line = fmt.Sprintf("用户: %s\n", truncateRunes(entry.Content, 200))
		} else {
			line = fmt.Sprintf("助手: %s\n", truncateRunes(entry.Content, 300))
		}
		// 如果加上这行会超过 2000 字符，停止
		if sb.Len()+len(line) > 2000 {
			break
		}
		sb.WriteString(line)
	}

	return sb.String()
}

// FormatDisplay 格式化对话历史用于终端显示（/history 命令）
func (h *ConversationHistory) FormatDisplay() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.messages) == 0 {
		return "暂无对话记录。"
	}

	var sb strings.Builder
	for _, entry := range h.messages {
		ts := entry.Timestamp.Format("15:04")
		role := "用户"
		if entry.Role == "assistant" {
			role = "助手"
		}
		source := ""
		if entry.Source == "local" {
			source = " [本地]"
		} else if entry.Source == "agent" {
			source = " [Agent]"
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s%s: %s\n", ts, role, source, truncateRunes(entry.Content, 80)))
	}
	return sb.String()
}

// Clear 清空所有对话历史
func (h *ConversationHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = h.messages[:0]
}

// Len 返回当前消息条数
func (h *ConversationHistory) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.messages)
}

// truncateRunes 按 rune 截断字符串（正确处理中文）
func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}
