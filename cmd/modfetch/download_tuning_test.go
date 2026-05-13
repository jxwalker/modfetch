package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestDownloadDryRunReportsTuning(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	cfgBody := "version: 1\n" +
		"general:\n" +
		"  data_root: " + filepath.Join(tmp, "data") + "\n" +
		"  download_root: " + filepath.Join(tmp, "downloads") + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var runErr error
	out := captureStdout(t, func() {
		runErr = handleDownload(context.Background(), []string{
			"--config", cfgPath,
			"--url", "starter://gpt2-config",
			"--dry-run",
			"--summary-json",
			"--profile", "large-model",
			"--connections", "20",
			"--chunk-size-mb", "32",
		})
	})
	if runErr != nil {
		t.Fatalf("download dry-run: %v", runErr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode dry-run JSON: %v\n%s", err, out)
	}
	if got["profile"] != "large-model" {
		t.Fatalf("profile = %v, want large-model", got["profile"])
	}
	if got["connections"] != float64(20) {
		t.Fatalf("connections = %v, want 20", got["connections"])
	}
	if got["chunk_size_mode"] != "fixed" {
		t.Fatalf("chunk_size_mode = %v, want fixed", got["chunk_size_mode"])
	}
	if got["chunk_size_mb"] != float64(32) {
		t.Fatalf("chunk_size_mb = %v, want 32", got["chunk_size_mb"])
	}
}

func TestDownloadDryRunSanitizesResolverURI(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	cfgBody := "version: 1\n" +
		"general:\n" +
		"  data_root: " + filepath.Join(tmp, "data") + "\n" +
		"  download_root: " + filepath.Join(tmp, "downloads") + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var runErr error
	out := captureStdout(t, func() {
		runErr = handleDownload(context.Background(), []string{
			"--config", cfgPath,
			"--url", "https://user:secret@example.com/model.gguf?token=secret&x=1",
			"--dry-run",
			"--summary-json",
		})
	})
	if runErr != nil {
		t.Fatalf("download dry-run: %v", runErr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode dry-run JSON: %v\n%s", err, out)
	}
	if got["resolver_uri"] != "https://example.com/model.gguf" {
		t.Fatalf("resolver_uri = %v, want sanitized URL", got["resolver_uri"])
	}
	if got["chunk_size_mode"] != "auto" {
		t.Fatalf("chunk_size_mode = %v, want auto", got["chunk_size_mode"])
	}
	if _, ok := got["chunk_size_mb"]; ok {
		t.Fatalf("chunk_size_mb should be omitted in auto mode: %v", got["chunk_size_mb"])
	}
	chunkRange, ok := got["chunk_size_range_mb"].(map[string]any)
	if !ok {
		t.Fatalf("chunk_size_range_mb = %T, want object", got["chunk_size_range_mb"])
	}
	if chunkRange["min"] != float64(1) || chunkRange["max"] != float64(64) {
		t.Fatalf("chunk_size_range_mb = %v, want min=1 max=64", chunkRange)
	}
	if strings.Contains(out, "secret") || strings.Contains(out, "token=") {
		t.Fatalf("dry-run JSON leaked sensitive URL material: %s", out)
	}
}

func TestDownloadDryRunAutoTunesLargeRangeCapableObject(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "2147483648")
		if r.Method == http.MethodHead {
			return
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte{0})
	}))
	defer ts.Close()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	cfgBody := "version: 1\n" +
		"general:\n" +
		"  data_root: " + filepath.Join(tmp, "data") + "\n" +
		"  download_root: " + filepath.Join(tmp, "downloads") + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var runErr error
	out := captureStdout(t, func() {
		runErr = handleDownload(context.Background(), []string{
			"--config", cfgPath,
			"--url", ts.URL + "/large.gguf",
			"--dry-run",
			"--summary-json",
		})
	})
	if runErr != nil {
		t.Fatalf("download dry-run: %v", runErr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode dry-run JSON: %v\n%s", err, out)
	}
	if got["profile"] != "large-model" {
		t.Fatalf("profile = %v, want large-model", got["profile"])
	}
	if got["profile_source"] != "auto" {
		t.Fatalf("profile_source = %v, want auto", got["profile_source"])
	}
	if got["connections"] != float64(16) {
		t.Fatalf("connections = %v, want 16", got["connections"])
	}
	if got["chunk_size_mb"] != float64(64) {
		t.Fatalf("chunk_size_mb = %v, want 64", got["chunk_size_mb"])
	}
}

func TestAdaptiveTuningReturnsErrorForEmptyProbeMetadata(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	cfg := &config.Config{}
	decision, err := maybeApplyAdaptiveDownloadTuning(context.Background(), cfg, ts.URL+"/empty.gguf", nil, "", 0, 0)
	if err == nil {
		t.Fatalf("expected empty probe metadata error, got decision=%+v", decision)
	}
}

func TestApplyDownloadTuningLargeModelProfile(t *testing.T) {
	cfg := &config.Config{}
	cfg.Concurrency.PerFileChunks = 4
	cfg.Concurrency.PerHostRequests = 4
	cfg.Concurrency.ChunkSizeMB = 8

	if err := applyDownloadTuning(cfg, "large-model", 0, 0); err != nil {
		t.Fatalf("apply tuning: %v", err)
	}
	if cfg.Concurrency.PerFileChunks != 16 {
		t.Fatalf("per_file_chunks = %d, want 16", cfg.Concurrency.PerFileChunks)
	}
	if cfg.Concurrency.PerHostRequests != 16 {
		t.Fatalf("per_host_requests = %d, want 16", cfg.Concurrency.PerHostRequests)
	}
	if cfg.Concurrency.ChunkSizeMB != 64 {
		t.Fatalf("chunk_size_mb = %d, want 64", cfg.Concurrency.ChunkSizeMB)
	}
}

func TestApplyDownloadTuningExplicitOverridesProfile(t *testing.T) {
	cfg := &config.Config{}
	cfg.Concurrency.PerFileChunks = 4
	cfg.Concurrency.PerHostRequests = 4
	cfg.Concurrency.ChunkSizeMB = 8

	if err := applyDownloadTuning(cfg, "large-model", 24, 32); err != nil {
		t.Fatalf("apply tuning: %v", err)
	}
	if cfg.Concurrency.PerFileChunks != 24 {
		t.Fatalf("per_file_chunks = %d, want 24", cfg.Concurrency.PerFileChunks)
	}
	if cfg.Concurrency.PerHostRequests != 24 {
		t.Fatalf("per_host_requests = %d, want 24", cfg.Concurrency.PerHostRequests)
	}
	if cfg.Concurrency.ChunkSizeMB != 32 {
		t.Fatalf("chunk_size_mb = %d, want 32", cfg.Concurrency.ChunkSizeMB)
	}
}

func TestApplyDownloadTuningRejectsInvalidValues(t *testing.T) {
	for _, tt := range []struct {
		name        string
		profile     string
		connections int
		chunkSizeMB int
		want        string
	}{
		{name: "bad profile", profile: "turbo", want: "unknown download profile"},
		{name: "negative connections", connections: -1, want: "--connections"},
		{name: "negative chunk size", chunkSizeMB: -1, want: "--chunk-size-mb"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := applyDownloadTuning(&config.Config{}, tt.profile, tt.connections, tt.chunkSizeMB)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestEffectiveChunkSizeReportsAutoWhenUnset(t *testing.T) {
	mode, size := effectiveChunkSize(&config.Config{})
	if mode != "auto" || size != 0 {
		t.Fatalf("effectiveChunkSize = (%q, %d), want (auto, 0)", mode, size)
	}

	cfg := &config.Config{}
	cfg.Concurrency.ChunkSizeMB = 32
	mode, size = effectiveChunkSize(cfg)
	if mode != "fixed" || size != 32 {
		t.Fatalf("effectiveChunkSize = (%q, %d), want (fixed, 32)", mode, size)
	}
}
