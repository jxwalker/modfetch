package util

import "strings"

// ExpandPattern replaces tokens in the form {token} with values from the map.
// Unknown tokens are left as-is. Nil or empty maps produce the original pattern.
func ExpandPattern(pattern string, tokens map[string]string) string {
	p := pattern
	if strings.TrimSpace(p) == "" {
		return ""
	}
	for k, v := range tokens {
		p = strings.ReplaceAll(p, "{"+k+"}", strings.TrimSpace(v))
	}
	return p
}
