package resolver

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
)

func testCacheConfig(root string, ttlHours int) *config.Config {
	return &config.Config{
		General:  config.General{DataRoot: root},
		Resolver: config.ResolverConf{CacheTTLHours: ttlHours},
	}
}

func TestResolverCacheRoundTripDeleteAndClear(t *testing.T) {
	cfg := testCacheConfig(t.TempDir(), 24)
	uri := "hf://owner/repo/model.gguf"
	want := &Resolved{
		URL:               "https://example.com/model.gguf",
		Headers:           map[string]string{"Authorization": "Bearer token"},
		SuggestedFilename: "model.gguf",
		RepoOwner:         "owner",
		RepoName:          "repo",
	}

	if err := cacheSet(cfg, uri, want); err != nil {
		t.Fatalf("cacheSet: %v", err)
	}
	got, ok, err := cacheGet(cfg, uri)
	if err != nil {
		t.Fatalf("cacheGet: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.URL != want.URL || got.Headers["Authorization"] != want.Headers["Authorization"] ||
		got.SuggestedFilename != want.SuggestedFilename || got.RepoOwner != want.RepoOwner || got.RepoName != want.RepoName {
		t.Fatalf("unexpected cached result: %+v", got)
	}

	if err := cacheDelete(cfg, uri); err != nil {
		t.Fatalf("cacheDelete: %v", err)
	}
	if _, ok, err := cacheGet(cfg, uri); err != nil {
		t.Fatalf("cacheGet after delete: %v", err)
	} else if ok {
		t.Fatal("expected cache miss after delete")
	}

	if err := cacheSet(cfg, uri, want); err != nil {
		t.Fatalf("cacheSet after delete: %v", err)
	}
	if err := ClearCache(cfg); err != nil {
		t.Fatalf("ClearCache: %v", err)
	}
	path, err := cacheFilePath(cfg)
	if err != nil {
		t.Fatalf("cacheFilePath: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected cache file to be removed, stat err=%v", err)
	}
	if err := ClearCache(cfg); err != nil {
		t.Fatalf("ClearCache missing file: %v", err)
	}
}

func TestResolverCacheExpiresEntries(t *testing.T) {
	cfg := testCacheConfig(t.TempDir(), 1)
	uri := "civitai://model/123"
	path, err := cacheFilePath(cfg)
	if err != nil {
		t.Fatalf("cacheFilePath: %v", err)
	}
	old := time.Now().Add(-2 * time.Hour).Unix()
	if err := saveCache(path, map[string]cacheEntry{
		uri: {Resolved: Resolved{URL: "https://example.com/old.bin"}, UpdatedAt: old},
	}); err != nil {
		t.Fatalf("saveCache: %v", err)
	}

	if _, ok, err := cacheGet(cfg, uri); err != nil {
		t.Fatalf("cacheGet: %v", err)
	} else if ok {
		t.Fatal("expected expired entry to miss")
	}
	entries, err := loadCache(path)
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	if _, ok := entries[uri]; ok {
		t.Fatal("expected expired entry to be removed")
	}
}

func TestResolverCachePathValidationAndLoadErrors(t *testing.T) {
	if _, err := cacheFilePath(nil); err == nil {
		t.Fatal("expected nil config error")
	}
	if _, err := cacheFilePath(&config.Config{}); err == nil {
		t.Fatal("expected empty data_root error")
	}

	tmp := t.TempDir()
	empty := filepath.Join(tmp, "empty.json")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write empty cache: %v", err)
	}
	if entries, err := loadCache(empty); err != nil {
		t.Fatalf("load empty cache: %v", err)
	} else if len(entries) != 0 {
		t.Fatalf("expected empty cache, got %+v", entries)
	}

	invalid := filepath.Join(tmp, "invalid.json")
	if err := os.WriteFile(invalid, []byte("{"), 0o644); err != nil {
		t.Fatalf("write invalid cache: %v", err)
	}
	if _, err := loadCache(invalid); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}
