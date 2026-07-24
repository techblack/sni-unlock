package domains

import "testing"

func TestMatcher(t *testing.T) {
	matcher := New([]string{"netflix.com", "Example.ORG."})
	tests := map[string]bool{
		"netflix.com":         true,
		"www.netflix.com.":    true,
		"example.org":         true,
		"notnetflix.com":      false,
		"netflix.com.example": false,
	}
	for domain, expected := range tests {
		if actual := matcher.Match(domain); actual != expected {
			t.Fatalf("Match(%q) = %v, want %v", domain, actual, expected)
		}
	}
}
