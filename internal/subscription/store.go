package subscription

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Subscription 表示一个订阅源
type Subscription struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	URL      string    `json:"url"`
	NodeCount int      `json:"node_count"`
	Tags     []string  `json:"tags"`             // 该订阅导入的 outbound tag 列表
	LastUpdate time.Time `json:"last_update"`
	AutoUpdate bool     `json:"auto_update"`
	Interval   int      `json:"interval_minutes"` // 自动更新间隔，默认 60
}

// SubscriptionStore 管理订阅的持久化存储
type SubscriptionStore struct {
	Subscriptions []Subscription `json:"subscriptions"`
	filePath      string
	mu            sync.RWMutex
}

// NewSubscriptionStore 创建订阅存储
func NewSubscriptionStore(dataDir string) *SubscriptionStore {
	return &SubscriptionStore{
		filePath: filepath.Join(dataDir, "subscriptions.json"),
	}
}

// Load 从文件加载订阅数据
func (s *SubscriptionStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.Subscriptions = nil
			return nil
		}
		return fmt.Errorf("读取订阅文件失败: %w", err)
	}
	return json.Unmarshal(data, &s.Subscriptions)
}

// Save 保存订阅数据到文件
func (s *SubscriptionStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *SubscriptionStore) saveLocked() error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	data, err := json.MarshalIndent(s.Subscriptions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// Add 添加一个新订阅
func (s *SubscriptionStore) Add(name, url string) (*Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查 URL 是否已存在
	for _, sub := range s.Subscriptions {
		if sub.URL == url {
			return &sub, fmt.Errorf("该订阅已存在: %s (%s)", sub.Name, sub.ID)
		}
	}

	id := s.nextID()
	sub := Subscription{
		ID:         id,
		Name:       name,
		URL:        url,
		AutoUpdate: true,
		Interval:   60,
	}
	s.Subscriptions = append(s.Subscriptions, sub)
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return &s.Subscriptions[len(s.Subscriptions)-1], nil
}

// Remove 按 ID 删除订阅
func (s *SubscriptionStore) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var kept []Subscription
	found := false
	for _, sub := range s.Subscriptions {
		if sub.ID == id {
			found = true
			continue
		}
		kept = append(kept, sub)
	}
	if !found {
		return fmt.Errorf("订阅 %q 不存在", id)
	}
	s.Subscriptions = kept
	return s.saveLocked()
}

// List 列出所有订阅
func (s *SubscriptionStore) List() []Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Subscription, len(s.Subscriptions))
	copy(result, s.Subscriptions)
	return result
}

// Get 按 ID 获取订阅
func (s *SubscriptionStore) Get(id string) *Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.Subscriptions {
		if s.Subscriptions[i].ID == id {
			sub := s.Subscriptions[i]
			return &sub
		}
	}
	return nil
}

// FindByURL 按 URL 查找订阅
func (s *SubscriptionStore) FindByURL(url string) *Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.Subscriptions {
		if s.Subscriptions[i].URL == url {
			sub := s.Subscriptions[i]
			return &sub
		}
	}
	return nil
}

// Update 更新订阅元数据（tags、node_count、last_update 等）
func (s *SubscriptionStore) Update(sub *Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Subscriptions {
		if s.Subscriptions[i].ID == sub.ID {
			s.Subscriptions[i] = *sub
			return s.saveLocked()
		}
	}
	return fmt.Errorf("订阅 %q 不存在", sub.ID)
}

// nextID 生成下一个订阅 ID: sub-01, sub-02, ...
func (s *SubscriptionStore) nextID() string {
	maxNum := 0
	for _, sub := range s.Subscriptions {
		var num int
		if _, err := fmt.Sscanf(sub.ID, "sub-%d", &num); err == nil && num > maxNum {
			maxNum = num
		}
	}
	return fmt.Sprintf("sub-%02d", maxNum+1)
}
