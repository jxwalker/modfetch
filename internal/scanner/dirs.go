package scanner

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/jxwalker/modfetch/internal/config"
)

// ConfiguredDirectories returns the library scan roots implied by config.
func ConfiguredDirectories(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	var dirs []string
	seen := map[string]bool{}
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		dirs = append(dirs, path)
	}

	add(cfg.General.DownloadRoot)
	appNames := make([]string, 0, len(cfg.Placement.Apps))
	for name := range cfg.Placement.Apps {
		appNames = append(appNames, name)
	}
	sort.Strings(appNames)
	for _, name := range appNames {
		app := cfg.Placement.Apps[name]
		base := strings.TrimSpace(app.Base)
		add(base)

		pathKeys := make([]string, 0, len(app.Paths))
		for key := range app.Paths {
			pathKeys = append(pathKeys, key)
		}
		sort.Strings(pathKeys)
		for _, key := range pathKeys {
			path := strings.TrimSpace(app.Paths[key])
			if path == "" {
				continue
			}
			if base == "" || filepath.IsAbs(path) {
				add(path)
				continue
			}
			add(filepath.Join(base, path))
		}
	}
	return filterRedundantDirs(dirs)
}

func filterRedundantDirs(dirs []string) []string {
	filtered := make([]string, 0, len(dirs))
	for i, dir := range dirs {
		redundant := false
		for j, other := range dirs {
			if i == j {
				continue
			}
			if pathsEquivalent(dir, other) {
				if j < i {
					redundant = true
					break
				}
				continue
			}
			if pathWithinDirs(dir, []string{other}) {
				redundant = true
				break
			}
		}
		if !redundant {
			filtered = append(filtered, dir)
		}
	}
	return filtered
}

func pathsEquivalent(a, b string) bool {
	for _, aVariant := range pathVariants(a) {
		for _, bVariant := range pathVariants(b) {
			if aVariant == bVariant {
				return true
			}
		}
	}
	return false
}
