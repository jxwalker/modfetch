package main

import (
	"context"
	"encoding/json"
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
	if strings.Contains(out, "secret") || strings.Contains(out, "token=") {
		t.Fatalf("dry-run JSON leaked sensitive URL material: %s", out)
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
