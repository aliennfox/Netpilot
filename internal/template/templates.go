package template

import "github.com/foxnetpilot/netpilot/internal/overlay"

// ProxyPlaceholder 在应用模板时替换为实际最快代理节点
const ProxyPlaceholder = "__BEST_PROXY__"

// Template 是一个预置分流模板
type Template struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Keywords    []string                 `json:"keywords"`
	Rules       []overlay.RouteRule      `json:"rules"`
	Outbounds   []map[string]interface{} `json:"outbounds,omitempty"`
}

var builtinTemplates = []*Template{
	{
		ID:          "netflix",
		Name:        "Netflix 流媒体",
		Description: "Netflix 及相关 CDN 域名走代理",
		Keywords:    []string{"netflix", "奈飞", "网飞"},
		Rules: []overlay.RouteRule{
			{
				Tag:          "_agent:netflix",
				DomainSuffix: []string{".netflix.com", ".nflxvideo.net", ".nflxso.net", ".nflxext.com"},
				Outbound:     ProxyPlaceholder,
				Description:  "Netflix 流媒体流量",
				Source:       "template:netflix",
			},
		},
	},
	{
		ID:          "ai_services",
		Name:        "AI 服务",
		Description: "OpenAI、Anthropic、Midjourney 等 AI 服务走代理",
		Keywords:    []string{"ai", "chatgpt", "openai", "claude", "midjourney"},
		Rules: []overlay.RouteRule{
			{
				Tag:          "_agent:ai-services",
				DomainSuffix: []string{".openai.com", ".anthropic.com", ".midjourney.com", ".claude.ai"},
				Outbound:     ProxyPlaceholder,
				Description:  "AI 服务流量",
				Source:       "template:ai_services",
			},
		},
	},
	{
		ID:          "social_media",
		Name:        "社交媒体",
		Description: "Twitter/X、Instagram、Telegram 等走代理",
		Keywords:    []string{"社交", "推特", "twitter", "instagram", "telegram", "tg"},
		Rules: []overlay.RouteRule{
			{
				Tag:          "_agent:social-media",
				DomainSuffix: []string{".twitter.com", ".x.com", ".instagram.com", ".telegram.org", ".t.me"},
				Outbound:     ProxyPlaceholder,
				Description:  "社交媒体流量",
				Source:       "template:social_media",
			},
		},
	},
	{
		ID:          "google",
		Name:        "Google 服务",
		Description: "Google、YouTube、Gmail 等走代理",
		Keywords:    []string{"google", "谷歌", "youtube", "gmail"},
		Rules: []overlay.RouteRule{
			{
				Tag:          "_agent:google",
				DomainSuffix: []string{".google.com", ".googleapis.com", ".youtube.com", ".gmail.com", ".gstatic.com"},
				Outbound:     ProxyPlaceholder,
				Description:  "Google 服务流量",
				Source:       "template:google",
			},
		},
	},
}
