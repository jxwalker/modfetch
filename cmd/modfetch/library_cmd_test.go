package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestHandleLibrarySyncPushHTTPTargetWithBearerToken(t *testing.T) {
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
		DownloadURL: "https://example.com/http-pushed.gguf",
		Dest:        filepath.Join(dir, "cfg", "downloads", "http-pushed.gguf"),
		ModelName:   "HTTP Pushed Model",
		Source:      "direct",
	}); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	t.Setenv("MODFETCH_TEST_SYNC_TOKEN", "secret-token")
	var received bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Errorf("unexpected authorization header %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("unexpected content-type %q", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode pushed catalog: %v", err)
		}
		if payload["app"] != "modfetch" {
			t.Errorf("unexpected catalog app %v", payload["app"])
		}
		models, ok := payload["models"].([]any)
		if !ok || len(models) != 1 {
			t.Errorf("unexpected catalog models %v", payload["models"])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	out := captureStdout(t, func() {
		if err := handleLibrary(context.Background(), []string{"sync", "push", "--config", cfgPath, "--target", server.URL, "--token-env", "MODFETCH_TEST_SYNC_TOKEN", "--json"}); err != nil {
			t.Fatalf("library sync push HTTP: %v", err)
		}
	})
	if !received {
		t.Fatal("expected HTTP sync target to receive a request")
	}
	var result librarySyncPushResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode sync push result: %v\n%s", err, out)
	}
	if result.Method != http.MethodPut || result.Status != "201 Created" || !result.Authenticated || result.Models != 1 {
		t.Fatalf("unexpected sync push result: %+v", result)
	}
}

func TestHandleLibrarySyncPushHTTPDryRunDoesNotContactTarget(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeLibraryConfig(t, filepath.Join(dir, "cfg"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry-run should not contact HTTP target")
	}))
	defer server.Close()

	if err := handleLibrary(context.Background(), []string{"sync", "push", "--config", cfgPath, "--target", server.URL, "--dry-run"}); err != nil {
		t.Fatalf("library sync push HTTP dry-run: %v", err)
	}
}

func TestHandleLibrarySyncRejectsUnsupportedTarget(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeLibraryConfig(t, filepath.Join(dir, "cfg"))

	err := handleLibrary(context.Background(), []string{"sync", "push", "--config", cfgPath, "--target", "s3://example/catalog.json"})
	if err == nil || !strings.Contains(err.Error(), "unsupported sync target scheme") {
		t.Fatalf("expected unsupported target scheme error, got %v", err)
	}
}

func TestHandleLibrarySyncPullHTTPTarget(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeLibraryConfig(t, filepath.Join(dir, "cfg"))
	payload, err := json.Marshal(map[string]any{
		"app":             "modfetch",
		"catalog_version": 1,
		"models": []map[string]any{{
			"metadata": map[string]any{
				"download_url": "https://example.com/http-synced.gguf",
				"dest":         filepath.Join(dir, "downloads", "http-synced.gguf"),
				"model_name":   "HTTP Synced Model",
				"source":       "direct",
				"favorite":     true,
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("unexpected accept header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	if err := handleLibrary(context.Background(), []string{"sync", "pull", "--config", cfgPath, "--target", server.URL, "--dry-run"}); err != nil {
		t.Fatalf("library sync pull HTTP dry-run: %v", err)
	}
	db, err := state.Open(&config.Config{General: config.General{
		DataRoot:     filepath.Join(dir, "cfg", "data"),
		DownloadRoot: filepath.Join(dir, "cfg", "downloads"),
	}})
	if err != nil {
		t.Fatalf("open db after dry-run: %v", err)
	}
	if meta, err := db.GetMetadata("https://example.com/http-synced.gguf"); !errors.Is(err, sql.ErrNoRows) || meta != nil {
		t.Fatalf("dry-run should not write HTTP metadata, meta=%+v err=%v", meta, err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db after dry-run: %v", err)
	}

	if err := handleLibrary(context.Background(), []string{"sync", "pull", "--config", cfgPath, "--target", server.URL}); err != nil {
		t.Fatalf("library sync pull HTTP: %v", err)
	}
	db, err = state.Open(&config.Config{General: config.General{
		DataRoot:     filepath.Join(dir, "cfg", "data"),
		DownloadRoot: filepath.Join(dir, "cfg", "downloads"),
	}})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	meta, err := db.GetMetadata("https://example.com/http-synced.gguf")
	if err != nil {
		t.Fatalf("get HTTP synced metadata: %v", err)
	}
	if meta.ModelName != "HTTP Synced Model" || !meta.Favorite {
		t.Fatalf("unexpected HTTP metadata: %+v", meta)
	}
}

func TestHandleLibrarySyncPullHTTPTargetWithBearerToken(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeLibraryConfig(t, filepath.Join(dir, "cfg"))
	payload, err := json.Marshal(map[string]any{
		"app":             "modfetch",
		"catalog_version": 1,
		"models": []map[string]any{{
			"metadata": map[string]any{
				"download_url": "https://example.com/auth-http-synced.gguf",
				"dest":         filepath.Join(dir, "downloads", "auth-http-synced.gguf"),
				"model_name":   "Auth HTTP Synced Model",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MODFETCH_TEST_SYNC_TOKEN", "secret-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Errorf("unexpected authorization header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	if err := handleLibrary(context.Background(), []string{"sync", "pull", "--config", cfgPath, "--target", server.URL, "--token-env", "MODFETCH_TEST_SYNC_TOKEN"}); err != nil {
		t.Fatalf("library sync pull HTTP with token: %v", err)
	}
	db, err := state.Open(&config.Config{General: config.General{
		DataRoot:     filepath.Join(dir, "cfg", "data"),
		DownloadRoot: filepath.Join(dir, "cfg", "downloads"),
	}})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	meta, err := db.GetMetadata("https://example.com/auth-http-synced.gguf")
	if err != nil {
		t.Fatalf("get HTTP synced metadata: %v", err)
	}
	if meta.ModelName != "Auth HTTP Synced Model" {
		t.Fatalf("unexpected HTTP metadata: %+v", meta)
	}
}

func TestHandleLibrarySyncPullHTTPStatusError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeLibraryConfig(t, filepath.Join(dir, "cfg"))
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	err := handleLibrary(context.Background(), []string{"sync", "pull", "--config", cfgPath, "--target", server.URL})
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("expected HTTP 404 sync error, got %v", err)
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
