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
			if strings.Contains(normalized, kw) {
				hits++
				if len(kw) > longest {
					longest = len(kw)
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

	return RoutingResult{
		Matched:    true,
		ActionID:   best.actionID,
		Params:     map[string]string{},
		Confidence: 1.0,
	}
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
