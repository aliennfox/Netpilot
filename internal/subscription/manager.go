package subscription

import (
	"fmt"
	"sync"
	"time"

	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
)

// SubscriptionManager 编排订阅的增删改查和自动更新
type SubscriptionManager struct {
	store   *SubscriptionStore
	overlay *overlay.ConfigOverlay
	adapter engine.EngineAdapter
	stopCh  chan struct{}
	stopMu  sync.Mutex
}

// NewSubscriptionManager 创建订阅管理器
func NewSubscriptionManager(store *SubscriptionStore, ov *overlay.ConfigOverlay, adapter engine.EngineAdapter) *SubscriptionManager {
	return &SubscriptionManager{
		store:   store,
		overlay: ov,
		adapter: adapter,
	}
}

// AddSubscription 添加并导入一个新订阅，返回节点数和订阅信息摘要
func (m *SubscriptionManager) AddSubscription(name, url string) (int, string, error) {
	// 检查是否已存在
	if existing := m.store.FindByURL(url); existing != nil {
		// URL 已存在，执行更新
		count, summary, err := m.UpdateSubscription(existing.ID)
		if err != nil {
			return 0, "", err
		}
		return count, fmt.Sprintf("订阅 %s (%s) 已更新\n%s", existing.Name, existing.ID, summary), nil
	}

	// 下载并解析
	fmt.Println("正在下载订阅...")
	nodes, err := FetchAndParse(url)
	if err != nil {
		return 0, "", fmt.Errorf("订阅解析失败: %v", err)
	}

	// 自动命名
	if name == "" {
		name = inferName(url, nodes)
	}

	// 保存到 store（先注册，后面更新 tags）
	sub, err := m.store.Add(name, url)
	if err != nil {
		return 0, "", err
	}

	// 转换并导入
	outbounds, tags, skipped := m.convertNodes(nodes)
	if len(outbounds) == 0 {
		_ = m.store.Remove(sub.ID)
		return 0, "", fmt.Errorf("没有有效的节点可导入")
	}

	// 写入 overlay
	if err := m.overlay.AddOutboundsBatch(outbounds); err != nil {
		_ = m.store.Remove(sub.ID)
		return 0, "", fmt.Errorf("写入 overlay 失败: %v", err)
	}

	// 重载
	if err := m.overlay.Apply(m.adapter); err != nil {
		return 0, "", fmt.Errorf("应用配置失败: %v", err)
	}

	// 更新 store 元数据
	sub.Tags = tags
	sub.NodeCount = len(outbounds)
	sub.LastUpdate = time.Now()
	_ = m.store.Update(sub)

	summary := buildImportSummary(nodes, len(outbounds), skipped, sub)
	return len(outbounds), summary, nil
}

// RemoveSubscription 删除订阅及其所有节点
func (m *SubscriptionManager) RemoveSubscription(id string) (string, error) {
	sub := m.store.Get(id)
	if sub == nil {
		return "", fmt.Errorf("订阅 %q 不存在", id)
	}

	// 从 overlay 删除该订阅的节点
	if len(sub.Tags) > 0 {
		if err := m.overlay.RemoveOutboundsByTags(sub.Tags); err != nil {
			return "", fmt.Errorf("删除节点失败: %v", err)
		}
	}

	// 重载
	if err := m.overlay.Apply(m.adapter); err != nil {
		return "", fmt.Errorf("应用配置失败: %v", err)
	}

	name := sub.Name
	count := sub.NodeCount

	// 从 store 删除
	if err := m.store.Remove(id); err != nil {
		return "", err
	}

	return fmt.Sprintf("已删除订阅: %s（移除 %d 个节点）\n配置已重载。", name, count), nil
}

// UpdateSubscription 更新指定订阅，返回新节点数和摘要
func (m *SubscriptionManager) UpdateSubscription(id string) (int, string, error) {
	sub := m.store.Get(id)
	if sub == nil {
		return 0, "", fmt.Errorf("订阅 %q 不存在", id)
	}

	// 下载并解析
	nodes, err := FetchAndParse(sub.URL)
	if err != nil {
		return 0, "", fmt.Errorf("下载失败: %v", err)
	}

	// 转换
	outbounds, newTags, _ := m.convertNodes(nodes)
	if len(outbounds) == 0 {
		return 0, "", fmt.Errorf("没有有效的节点")
	}

	// 删除旧节点
	if len(sub.Tags) > 0 {
		_ = m.overlay.RemoveOutboundsByTags(sub.Tags)
	}

	// 导入新节点
	if err := m.overlay.AddOutboundsBatch(outbounds); err != nil {
		return 0, "", fmt.Errorf("写入 overlay 失败: %v", err)
	}

	// 重载
	if err := m.overlay.Apply(m.adapter); err != nil {
		return 0, "", fmt.Errorf("应用配置失败: %v", err)
	}

	oldCount := sub.NodeCount
	sub.Tags = newTags
	sub.NodeCount = len(outbounds)
	sub.LastUpdate = time.Now()
	_ = m.store.Update(sub)

	diff := len(outbounds) - oldCount
	diffStr := "无变化"
	if diff > 0 {
		diffStr = fmt.Sprintf("+%d", diff)
	} else if diff < 0 {
		diffStr = fmt.Sprintf("%d", diff)
	}

	return len(outbounds), fmt.Sprintf("%s: %d→%d 节点（%s）", sub.Name, oldCount, len(outbounds), diffStr), nil
}

// UpdateAll 更新所有订阅，返回汇总
func (m *SubscriptionManager) UpdateAll() string {
	subs := m.store.List()
	if len(subs) == 0 {
		return "没有已保存的订阅。"
	}

	var lines []string
	for _, sub := range subs {
		_, summary, err := m.UpdateSubscription(sub.ID)
		if err != nil {
			lines = append(lines, fmt.Sprintf("[更新] %s: 失败 (%v)", sub.Name, err))
		} else {
			lines = append(lines, fmt.Sprintf("[更新] %s", summary))
		}
	}
	result := ""
	for _, l := range lines {
		result += l + "\n"
	}
	result += "全部更新完成。"
	return result
}

// StartAutoUpdate 启动后台自动更新
func (m *SubscriptionManager) StartAutoUpdate() {
	m.stopMu.Lock()
	defer m.stopMu.Unlock()

	if m.stopCh != nil {
		return // 已在运行
	}
	m.stopCh = make(chan struct{})

	go func() {
		// 首次等 60 秒再开始检查（避免启动时立即更新）
		select {
		case <-time.After(60 * time.Second):
		case <-m.stopCh:
			return
		}

		ticker := time.NewTicker(1 * time.Minute) // 每分钟检查一次
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.checkAndUpdate()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// StopAutoUpdate 停止后台自动更新
func (m *SubscriptionManager) StopAutoUpdate() {
	m.stopMu.Lock()
	defer m.stopMu.Unlock()

	if m.stopCh != nil {
		close(m.stopCh)
		m.stopCh = nil
	}
}

// Store 返回底层 store（供 action 层读取订阅列表）
func (m *SubscriptionManager) Store() *SubscriptionStore {
	return m.store
}

// checkAndUpdate 检查哪些订阅到了更新时间
func (m *SubscriptionManager) checkAndUpdate() {
	subs := m.store.List()
	for _, sub := range subs {
		if !sub.AutoUpdate {
			continue
		}
		interval := time.Duration(sub.Interval) * time.Minute
		if interval <= 0 {
			interval = 60 * time.Minute
		}
		if time.Since(sub.LastUpdate) < interval {
			continue
		}
		_, summary, err := m.UpdateSubscription(sub.ID)
		if err != nil {
			fmt.Printf("\033[33m[自动更新] %s: 更新失败 (%v)\033[0m\n", sub.Name, err)
		} else {
			fmt.Printf("\033[33m[自动更新] %s\033[0m\n", summary)
		}
	}
}

// convertNodes 将 NodeConfig 列表转为 outbound 和 tag 列表
func (m *SubscriptionManager) convertNodes(nodes []NodeConfig) ([]map[string]interface{}, []string, int) {
	var outbounds []map[string]interface{}
	var tags []string
	var skipped int
	tagCount := map[string]int{}

	for _, node := range nodes {
		ob, err := ConvertToSingboxOutbound(node)
		if err != nil {
			fmt.Printf("  ⚠️  跳过节点 %s: %v\n", node.Name, err)
			skipped++
			continue
		}
		if ob == nil {
			continue // 信息条目
		}
		tag, _ := ob["tag"].(string)
		tagCount[tag]++
		if tagCount[tag] > 1 {
			ob["tag"] = fmt.Sprintf("%s-%d", tag, tagCount[tag])
			tag = ob["tag"].(string)
		}
		outbounds = append(outbounds, ob)
		tags = append(tags, tag)
	}
	return outbounds, tags, skipped
}

// buildImportSummary 构建导入结果摘要
func buildImportSummary(nodes []NodeConfig, imported, skipped int, sub *Subscription) string {
	summary := SummarizeNodes(nodes)
	msg := fmt.Sprintf("成功导入 %d 个节点 (%s)", imported, summary)
	if skipped > 0 {
		msg += fmt.Sprintf("，跳过 %d 个无效节点", skipped)
	}
	msg += "\n已合并配置，sing-box 已重载。"
	if info := FormatInfoEntries(nodes); info != "" {
		msg += fmt.Sprintf("\n📋 订阅信息: %s", info)
	}
	msg += fmt.Sprintf("\n已保存订阅: %s (%s)", sub.Name, sub.ID)
	return msg
}

// inferName 从 URL 或节点信息推断订阅名称
func inferName(url string, nodes []NodeConfig) string {
	// 尝试从信息条目中找到官网名
	for _, n := range nodes {
		if n.IsInfoEntry && containsAny(n.Name, "官网", "www.", ".com") {
			return extractSiteName(n.Name)
		}
	}
	// 用节点数 + 类型作为名称
	summary := SummarizeNodes(nodes)
	if summary != "" {
		return fmt.Sprintf("订阅(%s)", summary)
	}
	return "未命名订阅"
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func extractSiteName(info string) string {
	// "官网www.平民.com" → "平民"
	// 简单提取，不完美但够用
	for _, prefix := range []string{"官网", "官网:"} {
		if idx := findSubstring(info, prefix); idx >= 0 {
			rest := info[idx+len(prefix):]
			// 去掉 www. 前缀和 .com 后缀
			rest = trimPrefix(rest, "www.")
			if dotIdx := findSubstring(rest, "."); dotIdx > 0 {
				return rest[:dotIdx]
			}
			if rest != "" {
				return rest
			}
		}
	}
	return "未命名订阅"
}

func findSubstring(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
