package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestHandleLibraryImportJSONReturnsErrorOnConflicts(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeLibraryConfig(t, filepath.Join(dir, "cfg"))
	db, err := state.Open(&config.Config{General: config.General{
		DataRoot:     filepath.Join(dir, "cfg", "data"),
		DownloadRoot: filepath.Join(dir, "cfg", "downloads"),
	}})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.UpsertMetadata(&state.ModelMetadata{
		DownloadURL: "https://example.com/existing.gguf",
		Dest:        "/models/shared.gguf",
		ModelName:   "Existing",
	}); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	payload, err := json.Marshal(map[string]any{
		"app":             "modfetch",
		"catalog_version": 1,
		"models": []map[string]any{{
			"metadata": map[string]any{
				"download_url": "https://example.com/incoming.gguf",
				"dest":         "/models/shared.gguf",
				"model_name":   "Incoming",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(dir, "conflict.json")
	if err := os.WriteFile(catalogPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	err = handleLibrary(context.Background(), []string{"import", "--config", cfgPath, "--input", catalogPath, "--json"})
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected JSON import conflict error, got %v", err)
	}
}

func TestHandleLibraryScanConfiguredDirectory(t *testing.T) {
	dir := t.TempDir()
	cfgRoot := filepath.Join(dir, "cfg")
	cfgPath := writeLibraryConfig(t, cfgRoot)
	downloadRoot := filepath.Join(cfgRoot, "downloads")
	if err := os.MkdirAll(downloadRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(downloadRoot, "cli-model.gguf")
	if err := os.WriteFile(modelPath, []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := handleLibrary(context.Background(), []string{"scan", "--config", cfgPath, "--workers", "2", "--no-progress"}); err != nil {
		t.Fatalf("library scan: %v", err)
	}

	db, err := state.Open(&config.Config{General: config.General{
		DataRoot:     filepath.Join(cfgRoot, "data"),
		DownloadRoot: downloadRoot,
	}})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	meta, err := db.GetMetadataByDest(modelPath)
	if err != nil {
		t.Fatalf("get scanned metadata: %v", err)
	}
	if meta == nil || meta.Source != "local" || meta.ModelName != "cli" {
		t.Fatalf("unexpected scanned metadata: %+v", meta)
	}
}

func TestHandleLibraryScanRepairStale(t *testing.T) {
	dir := t.TempDir()
	cfgRoot := filepath.Join(dir, "cfg")
	cfgPath := writeLibraryConfig(t, cfgRoot)
	downloadRoot := filepath.Join(cfgRoot, "downloads")
	if err := os.MkdirAll(downloadRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	missingPath := filepath.Join(downloadRoot, "missing.gguf")

	db, err := state.Open(&config.Config{General: config.General{
		DataRoot:     filepath.Join(cfgRoot, "data"),
		DownloadRoot: downloadRoot,
	}})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.UpsertMetadata(&state.ModelMetadata{
		DownloadURL: "file://" + missingPath,
		Dest:        missingPath,
		ModelName:   "missing",
		Source:      "local",
	}); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	if err := handleLibrary(context.Background(), []string{"scan", "--config", cfgPath, "--repair-stale", "--json"}); err != nil {
		t.Fatalf("library scan repair: %v", err)
	}

	db, err = state.Open(&config.Config{General: config.General{
		DataRoot:     filepath.Join(cfgRoot, "data"),
		DownloadRoot: downloadRoot,
	}})
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer func() { _ = db.Close() }()
	if meta, err := db.GetMetadata("file://" + missingPath); !errors.Is(err, sql.ErrNoRows) || meta != nil {
		t.Fatalf("stale metadata should be removed, meta=%+v err=%v", meta, err)
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
