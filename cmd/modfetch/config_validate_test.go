package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigValidateStrictRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	cfg := "version: 1\n" +
		"general:\n" +
		"  data_root: " + filepath.Join(dir, "data") + "\n" +
		"  download_root: " + filepath.Join(dir, "downloads") + "\n" +
		"  download_rooot: typo\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := handleConfig(context.Background(), []string{"validate", "--config", cfgPath}); err != nil {
		t.Fatalf("non-strict validate should pass, got %v", err)
	}
	err := handleConfig(context.Background(), []string{"validate", "--config", cfgPath, "--strict"})
	if err == nil || !strings.Contains(err.Error(), "download_rooot") {
		t.Fatalf("expected strict unknown-field error, got %v", err)
	}
}
