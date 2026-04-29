package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
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

func TestHandleLibrarySyncPushPullFileTarget(t *testing.T) {
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
		DownloadURL: "https://example.com/synced.gguf",
		Dest:        filepath.Join(dir, "source", "downloads", "synced.gguf"),
		ModelName:   "Synced Model",
		Source:      "direct",
		Favorite:    true,
	}); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	if err := source.Close(); err != nil {
		t.Fatalf("close source db: %v", err)
	}

	targetPath := filepath.Join(dir, "remote", "catalog with space.json")
	targetURI := fileURI(targetPath)
	if err := handleLibrary(context.Background(), []string{"sync", "push", "--config", sourceCfg, "--target", targetURI}); err != nil {
		t.Fatalf("library sync push: %v", err)
	}
	if err := handleLibrary(context.Background(), []string{"sync", "push", "--config", sourceCfg, "--target", targetURI}); err != nil {
		t.Fatalf("library sync push replacing existing target: %v", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected sync target catalog: %v", err)
	}

	targetCfg := writeLibraryConfig(t, filepath.Join(dir, "target"))
	if err := handleLibrary(context.Background(), []string{"sync", "pull", "--config", targetCfg, "--target", targetURI, "--dry-run"}); err != nil {
		t.Fatalf("library sync pull dry-run: %v", err)
	}
	targetDB, err := state.Open(&config.Config{General: config.General{
		DataRoot:     filepath.Join(dir, "target", "data"),
		DownloadRoot: filepath.Join(dir, "target", "downloads"),
	}})
	if err != nil {
		t.Fatalf("open target db after dry-run: %v", err)
	}
	if meta, err := targetDB.GetMetadata("https://example.com/synced.gguf"); !errors.Is(err, sql.ErrNoRows) || meta != nil {
		t.Fatalf("dry-run should not write metadata, meta=%+v err=%v", meta, err)
	}
	if err := targetDB.Close(); err != nil {
		t.Fatalf("close target db after dry-run: %v", err)
	}

	if err := handleLibrary(context.Background(), []string{"sync", "pull", "--config", targetCfg, "--target", targetURI}); err != nil {
		t.Fatalf("library sync pull: %v", err)
	}
	targetDB, err = state.Open(&config.Config{General: config.General{
		DataRoot:     filepath.Join(dir, "target", "data"),
		DownloadRoot: filepath.Join(dir, "target", "downloads"),
	}})
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer func() { _ = targetDB.Close() }()
	meta, err := targetDB.GetMetadata("https://example.com/synced.gguf")
	if err != nil {
		t.Fatalf("get synced metadata: %v", err)
	}
	if meta.ModelName != "Synced Model" || !meta.Favorite {
		t.Fatalf("unexpected synced metadata: %+v", meta)
	}
}

func TestHandleLibrarySyncPushDryRunDoesNotWriteTarget(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeLibraryConfig(t, filepath.Join(dir, "cfg"))
	targetPath := filepath.Join(dir, "remote", "catalog.json")

	if err := handleLibrary(context.Background(), []string{"sync", "push", "--config", cfgPath, "--target", fileURI(targetPath), "--dry-run"}); err != nil {
		t.Fatalf("library sync push dry-run: %v", err)
	}
	if _, err := os.Stat(targetPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run should not write target, err=%v", err)
	}
}

func TestHandleLibrarySyncRejectsUnsupportedTarget(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeLibraryConfig(t, filepath.Join(dir, "cfg"))

	err := handleLibrary(context.Background(), []string{"sync", "push", "--config", cfgPath, "--target", "https://example.com/catalog.json"})
	if err == nil || !strings.Contains(err.Error(), "unsupported sync target scheme") {
		t.Fatalf("expected unsupported target scheme error, got %v", err)
	}
}

func TestFileSyncTargetPathTreatsDriveLetterAsPlainPath(t *testing.T) {
	target := `C:\backup\catalog.json`
	got, err := fileSyncTargetPath(target)
	if err != nil {
		t.Fatalf("fileSyncTargetPath: %v", err)
	}
	if want := filepath.Clean(target); got != want {
		t.Fatalf("fileSyncTargetPath() = %q, want %q", got, want)
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

func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: path}).String()
}
