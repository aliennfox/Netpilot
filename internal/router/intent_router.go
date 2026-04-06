package router

import (
	"strings"
	"unicode"
)

type RoutingResult struct {
	Matched    bool
	ActionID   string
	Params     map[string]string
	Confidence float32
}

type IntentRouter struct {
	keywords map[string][]string
}

func NewIntentRouter() *IntentRouter {
	return &IntentRouter{
		keywords: KeywordMap,
	}
}

func (r *IntentRouter) Route(input string) RoutingResult {
	normalized := normalize(input)

	type candidate struct {
		actionID   string
		hitCount   int
		longestHit int // length of the longest matched keyword
	}

	var best candidate

	for actionID, keywords := range r.keywords {
		hits := 0
		longest := 0
		for _, kw := range keywords {
			kwLower := strings.ToLower(kw)
			if strings.Contains(normalized, kwLower) {
				hits++
				if len(kwLower) > longest {
					longest = len(kwLower)
				}
			}
		}
		if hits == 0 {
			continue
		}
		// Pick: most hits wins; tie-break by longest keyword (more specific)
		if hits > best.hitCount || (hits == best.hitCount && longest > best.longestHit) {
			best = candidate{actionID: actionID, hitCount: hits, longestHit: longest}
		}
	}

	if best.hitCount == 0 {
		return RoutingResult{Matched: false}
	}

	// 如果输入含指代词，说明用户在引用上下文，Local Engine 无法处理，交给 Agent
	if containsReferenceWord(normalized) {
		return RoutingResult{Matched: false}
	}

	return RoutingResult{
		Matched:    true,
		ActionID:   best.actionID,
		Params:     map[string]string{},
		Confidence: 1.0,
	}
}

// referenceWords 是上下文指代词，出现时说明用户在引用之前的对话内容，
// Local Engine 没有上下文能力，应交给 Agent 处理。
var referenceWords = []string{
	"第一个", "第二个", "第三个",
	"那个", "这个", "它",
	"上面的", "刚才的", "之前的",
	"你推荐的", "你说的",
}

func containsReferenceWord(input string) bool {
	for _, w := range referenceWords {
		if strings.Contains(input, w) {
			return true
		}
	}
	return false
}

// normalize lowercases the input and strips punctuation.
// Chinese characters are preserved as-is (no segmentation needed — we use substring matching).
func normalize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsPunct(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
