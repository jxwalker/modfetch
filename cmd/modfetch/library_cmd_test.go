package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/state"
)

func TestHandleLibraryExportImport(t *testing.T) {
	dir := t.TempDir()
	sourceCfg := writeLibraryConfig(t, filepath.Join(dir, "source"))
	source, err := state.Open(&config.Config{General: config.General{
		DataRoot:     filepath.Join(dir, "source", "data"),
		DownloadRoot: filepath.Join(dir, "source", "downloads"),
	}})
	if err != nil {
		t.Fatalf("open source db: %v", err)
	}
	if err := source.UpsertMetadata(&state.ModelMetadata{
		DownloadURL: "https://example.com/model.gguf",
		Dest:        filepath.Join(dir, "source", "downloads", "model.gguf"),
		ModelName:   "CLI Model",
		Source:      "direct",
		Favorite:    true,
	}); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	if err := source.Close(); err != nil {
		t.Fatalf("close source db: %v", err)
	}

	catalogPath := filepath.Join(dir, "catalog.json")
	if err := handleLibrary(context.Background(), []string{"export", "--config", sourceCfg, "--output", catalogPath}); err != nil {
		t.Fatalf("library export: %v", err)
	}
	if _, err := os.Stat(catalogPath); err != nil {
		t.Fatalf("expected catalog output: %v", err)
	}

	targetCfg := writeLibraryConfig(t, filepath.Join(dir, "target"))
	if err := handleLibrary(context.Background(), []string{"import", "--config", targetCfg, "--input", catalogPath, "--dry-run"}); err != nil {
		t.Fatalf("library import dry-run: %v", err)
	}
	if err := handleLibrary(context.Background(), []string{"import", "--config", targetCfg, "--input", catalogPath}); err != nil {
		t.Fatalf("library import: %v", err)
	}

	target, err := state.Open(&config.Config{General: config.General{
		DataRoot:     filepath.Join(dir, "target", "data"),
		DownloadRoot: filepath.Join(dir, "target", "downloads"),
	}})
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer func() { _ = target.Close() }()
	meta, err := target.GetMetadata("https://example.com/model.gguf")
	if err != nil {
		t.Fatalf("get imported metadata: %v", err)
	}
	if meta.ModelName != "CLI Model" || !meta.Favorite {
		t.Fatalf("unexpected imported metadata: %+v", meta)
	}
}

func writeLibraryConfig(t *testing.T, root string) string {
	t.Helper()
	cfgPath := filepath.Join(root, "config.yml")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "version: 1\n" +
		"general:\n" +
		"  data_root: " + filepath.Join(root, "data") + "\n" +
		"  download_root: " + filepath.Join(root, "downloads") + "\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}
