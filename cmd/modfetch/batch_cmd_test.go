package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/batch"
)

func TestBatchImport_FromTextURLs_NoNetworkRequired(t *testing.T) {
	// Prepare temp dirs/files
	d := t.TempDir()
	cfgPath := filepath.Join(d, "config.yaml")
	downloadRoot := filepath.Join(d, "dlroot")
	if err := os.MkdirAll(downloadRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "version: 1\n" +
		"general:\n" +
		"  data_root: " + strings.ReplaceAll(d, "\\", "\\\\") + "\n" +
		"  download_root: " + strings.ReplaceAll(downloadRoot, "\\", "\\\\") + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	inPath := filepath.Join(d, "input.txt")
	// Use simple URLs; importer tolerates probe failures
	input := strings.Join([]string{
		"https://example.com/a.bin",
		"https://example.com/b.bin",
		"https://example.com/path/c.bin",
	}, "\n")
	if err := os.WriteFile(inPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(d, "batch.yaml")
	args := []string{
		"--config", cfgPath,
		"--input", inPath,
		"--output", outPath,
		"--dest-dir", downloadRoot,
		"--sha-mode", "none",
	}
	if err := handleBatchImport(context.Background(), args); err != nil {
		t.Fatalf("batch import failed: %v", err)
	}

	bf, err := batch.Load(outPath)
	if err != nil {
		t.Fatalf("load batch: %v", err)
	}
	if bf.Version != 1 {
		t.Fatalf("bad version: %d", bf.Version)
	}
	if len(bf.Jobs) != 3 {
		t.Fatalf("expected 3 jobs; got %d", len(bf.Jobs))
	}
	for i, j := range bf.Jobs {
		if !strings.HasPrefix(j.Dest, downloadRoot) {
			t.Fatalf("job %d dest not under dest-dir: %s", i, j.Dest)
		}
		if strings.TrimSpace(j.URI) == "" {
			t.Fatalf("job %d empty uri", i)
		}
	}
}
