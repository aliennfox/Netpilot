package template

import (
	"strings"
)

// TemplateStore 管理分流模板
type TemplateStore struct {
	templates map[string]*Template
}

func NewTemplateStore() *TemplateStore {
	s := &TemplateStore{
		templates: make(map[string]*Template),
	}
	// 加载内置模板
	for _, t := range builtinTemplates {
		s.templates[t.ID] = t
	}
	return s
}

// Get 按 ID 获取模板
func (s *TemplateStore) Get(id string) *Template {
	return s.templates[id]
}

// Search 按关键词搜索模板（返回所有匹配的）
func (s *TemplateStore) Search(keyword string) []*Template {
	keyword = strings.ToLower(keyword)
	var results []*Template
	for _, t := range s.templates {
		if strings.Contains(strings.ToLower(t.Name), keyword) ||
			strings.Contains(strings.ToLower(t.ID), keyword) {
			results = append(results, t)
			continue
		}
		for _, kw := range t.Keywords {
			if strings.Contains(keyword, strings.ToLower(kw)) ||
				strings.Contains(strings.ToLower(kw), keyword) {
				results = append(results, t)
				break
			}
		}
	}
	return results
}

// List 返回所有模板
func (s *TemplateStore) List() []*Template {
	var all []*Template
	for _, t := range s.templates {
		all = append(all, t)
	}
	return all
}
