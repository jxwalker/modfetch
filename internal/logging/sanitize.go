package logging

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var (
	bearerTokenPattern = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]+`)
	urlPattern         = regexp.MustCompile(`https?://[^\s<>"']+`)
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

// SanitizeText redacts bearer tokens and sanitizes HTTP URLs embedded in log text.
func SanitizeText(raw string) string {
	s := bearerTokenPattern.ReplaceAllString(raw, "Bearer [REDACTED]")
	return urlPattern.ReplaceAllStringFunc(s, func(match string) string {
		trailing := ""
		for len(match) > 0 {
			last := match[len(match)-1]
			if !strings.ContainsRune(".,);]", rune(last)) {
				break
			}
			trailing = string(last) + trailing
			match = match[:len(match)-1]
		}
		return SanitizeURL(match) + trailing
	})
}

// SanitizeError returns an error with sensitive text redacted while preserving nil.
func SanitizeError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(SanitizeText(err.Error()))
}
