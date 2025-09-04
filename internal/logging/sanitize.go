package logging

import (
	"net/url"
	"strings"
)

// SanitizeURL removes userinfo and query params for logging to avoid leaking secrets.
// Returns the URL without userinfo and query, preserving scheme, host, and path.
func SanitizeURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	u, err := url.Parse(s)
	if err != nil {
		return s
	}
	// If no scheme and no authority, keep as-is to avoid percent-encoding spaces
	if u.Scheme == "" && !strings.Contains(s, "://") {
		return s
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
