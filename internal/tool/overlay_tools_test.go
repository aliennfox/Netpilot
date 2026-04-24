package tool

import "testing"

func TestNormalizeDomainSuffixes_LeadingDot(t *testing.T) {
	cases := []struct {
		name   string
		in     []string
		want   []string
		hasErr bool
	}{
		{"leading dot stripped", []string{".netflix.com"}, []string{"netflix.com"}, false},
		{"nekobox clash style", []string{".google.com", ".youtube.com"}, []string{"google.com", "youtube.com"}, false},
		{"mixed with and without dot", []string{".netflix.com", "nflxvideo.net"}, []string{"netflix.com", "nflxvideo.net"}, false},
		{"dedupe after strip", []string{".netflix.com", "netflix.com"}, []string{"netflix.com"}, false},
		{"whitespace trimmed", []string{"  .github.com  "}, []string{"github.com"}, false},
		{"empty input ok", []string{}, []string{}, false},
		{"empty element skipped", []string{".", "  "}, []string{}, false},
		{"subdomain ok", []string{"api.openai.com"}, []string{"api.openai.com"}, false},
		{"invalid rejected", []string{"bad _space.com"}, nil, true},
		{"too long rejected", []string{makeStr(260) + ".com"}, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeDomainSuffixes(tc.in)
			if tc.hasErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !equalSlice(got, tc.want) {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestValidateDomains_StrictRejectsLeadingDot(t *testing.T) {
	// validateDomains 用于精确匹配,保持严格:前导点必须被拒
	if err := validateDomains([]string{".netflix.com"}); err == nil {
		t.Fatal("validateDomains should reject leading-dot form for exact match")
	}
	if err := validateDomains([]string{"netflix.com"}); err != nil {
		t.Fatalf("validateDomains should accept bare FQDN: %v", err)
	}
}

func makeStr(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
