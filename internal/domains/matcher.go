package domains

import "strings"

type Matcher struct {
	domains map[string]struct{}
}

func New(values []string) *Matcher {
	domainSet := make(map[string]struct{}, len(values))
	for _, value := range values {
		domain := normalize(value)
		if domain != "" {
			domainSet[domain] = struct{}{}
		}
	}
	return &Matcher{domains: domainSet}
}

func (matcher *Matcher) Match(value string) bool {
	domain := normalize(value)
	for domain != "" {
		if _, ok := matcher.domains[domain]; ok {
			return true
		}
		separator := strings.IndexByte(domain, '.')
		if separator < 0 {
			return false
		}
		domain = domain[separator+1:]
	}
	return false
}

func normalize(value string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
}
