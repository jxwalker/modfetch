package util

import (
	"fmt"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

// SafeFileName returns a conservative, cross-platform-safe filename.
// It trims spaces, preserves the extension, and replaces any rune not in
// [A-Za-z0-9._-] with '-'. It also collapses duplicate '-' and trims leading/trailing
// separators. Falls back to "download" when empty after cleaning.
func SafeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "download"
	}
	// Preserve extension while cleaning base
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	var b strings.Builder
	prevDash := false
	for _, r := range base {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.'
		if ok {
			b.WriteRune(r)
			prevDash = false
		} else {
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	clean := b.String()
	clean = strings.Trim(clean, "-.")
	if clean == "" {
		clean = "download"
	}
	return clean + ext
}

// URLPathBase extracts the last element of the URL path, ignoring query and fragment.
// If parsing fails or the path is empty, it falls back to a reasonable default ("download").
func URLPathBase(u string) string {
	s := strings.TrimSpace(u)
	if s == "" {
		return "download"
	}
	if pu, err := url.Parse(s); err == nil && pu != nil {
		p := pu.Path
		b := pathpkg.Base(p)
		if b != "" && b != "/" && b != "." {
			return b
		}
		// URL parsed but had no usable path segment
		return "download"
	}
	// Fallback: strip query/fragment manually then use filepath.Base
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	b := filepath.Base(s)
	if b == "" || b == "/" || b == "." {
		return "download"
	}
	return b
}

// UniquePath returns a unique path inside dir for the given base filename.
// If a file already exists, it first tries adding a version hint " (v<versionHint>)"
// before the extension (when versionHint != ""). Then it tries numeric suffixes
// " (2)", " (3)", etc., before the extension.
func UniquePath(dir, base, versionHint string) (string, error) {
	base = SafeFileName(base)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	path := filepath.Join(dir, base)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return path, nil
		}
		return "", err
	}
	if strings.TrimSpace(versionHint) != "" {
		cand := filepath.Join(dir, fmt.Sprintf("%s (v%s)%s", name, versionHint, ext))
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand, nil
		}
	}
	for i := 2; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand, nil
		}
	}
}
