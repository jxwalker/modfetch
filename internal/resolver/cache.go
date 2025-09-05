package resolver

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
)

type cacheEntry struct {
	Resolved  Resolved `json:"resolved"`
	UpdatedAt int64    `json:"updated_at"`
}

var cacheMu sync.Mutex

func cacheFilePath(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", errors.New("nil config")
	}
	root := strings.TrimSpace(cfg.General.DataRoot)
	if root == "" {
		return "", errors.New("general.data_root required")
	}
	return filepath.Join(root, "resolver-cache.json"), nil
}

func cacheTTL(cfg *config.Config) time.Duration {
	ttl := 24 * time.Hour
	if cfg != nil && cfg.Resolver.CacheTTLHours > 0 {
		ttl = time.Duration(cfg.Resolver.CacheTTLHours) * time.Hour
	}
	return ttl
}

func loadCache(path string) (map[string]cacheEntry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]cacheEntry{}, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return map[string]cacheEntry{}, nil
	}
	var m map[string]cacheEntry
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]cacheEntry{}, err
	}
	return m, nil
}

func saveCache(path string, m map[string]cacheEntry) error {
	tmp := path + ".tmp"
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func cacheGet(cfg *config.Config, uri string) (*Resolved, bool, error) {
	path, err := cacheFilePath(cfg)
	if err != nil {
		return nil, false, err
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	m, err := loadCache(path)
	if err != nil {
		return nil, false, err
	}
	ce, ok := m[uri]
	if !ok {
		return nil, false, nil
	}
	ttl := cacheTTL(cfg)
	if ttl > 0 && time.Since(time.Unix(ce.UpdatedAt, 0)) > ttl {
		delete(m, uri)
		_ = saveCache(path, m)
		return nil, false, nil
	}
	return &ce.Resolved, true, nil
}

func cacheSet(cfg *config.Config, uri string, res *Resolved) error {
	path, err := cacheFilePath(cfg)
	if err != nil {
		return err
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	m, err := loadCache(path)
	if err != nil {
		return err
	}
	m[uri] = cacheEntry{Resolved: *res, UpdatedAt: time.Now().Unix()}
	return saveCache(path, m)
}

func cacheDelete(cfg *config.Config, uri string) error {
	path, err := cacheFilePath(cfg)
	if err != nil {
		return err
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	m, err := loadCache(path)
	if err != nil {
		return err
	}
	delete(m, uri)
	return saveCache(path, m)
}

// ClearCache removes all resolver cache entries.
func ClearCache(cfg *config.Config) error {
	path, err := cacheFilePath(cfg)
	if err != nil {
		return err
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
