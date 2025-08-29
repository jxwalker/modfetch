package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeFileName strips directory components and disallowed characters.
// Keep it conservative: replace '/' and '\\' with '_', trim spaces, and ensure non-empty.
func SafeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	if name == "" { return "download" }
	return name
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
		if os.IsNotExist(err) { return path, nil }
		return "", err
	}
	if strings.TrimSpace(versionHint) != "" {
		cand := filepath.Join(dir, fmt.Sprintf("%s (v%s)%s", name, versionHint, ext))
		if _, err := os.Stat(cand); os.IsNotExist(err) { return cand, nil }
	}
	for i := 2; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
		if _, err := os.Stat(cand); os.IsNotExist(err) { return cand, nil }
	}
}
