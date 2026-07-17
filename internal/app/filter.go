package app

import "strings"

type toolFilter struct{ include, exclude []string }

func newToolFilter(include, exclude string) (toolFilter, error) {
	i, err := parsePatterns(include, true)
	if err != nil {
		return toolFilter{}, err
	}
	e, err := parsePatterns(exclude, false)
	return toolFilter{i, e}, err
}
func parsePatterns(raw string, include bool) ([]string, error) {
	var patterns []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			patterns = append(patterns, p)
		}
	}
	if include && len(patterns) == 0 {
		return []string{"*"}, nil
	}
	return patterns, nil
}
func (f toolFilter) allowed(name string) bool {
	for _, p := range f.exclude {
		if wildcardMatch(p, name) {
			return false
		}
	}
	for _, p := range f.include {
		if wildcardMatch(p, name) {
			return true
		}
	}
	return false
}
func wildcardMatch(pattern, value string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == value
	}
	if !strings.HasPrefix(value, parts[0]) {
		return false
	}
	value = value[len(parts[0]):]
	for _, p := range parts[1 : len(parts)-1] {
		at := strings.Index(value, p)
		if at < 0 {
			return false
		}
		value = value[at+len(p):]
	}
	return strings.HasSuffix(value, parts[len(parts)-1])
}
