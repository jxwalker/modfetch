package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyScanDirDoesNotRequireConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("MODFETCH_CONFIG", "")

	scanDir := filepath.Join(home, "models")
	if err := os.MkdirAll(scanDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := handleVerify(context.Background(), []string{"--scan-dir", scanDir, "--safetensors-deep"})
	if err != nil {
		t.Fatalf("verify scan-dir without config: %v", err)
	}
}
