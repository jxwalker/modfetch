package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/state"
)

func TestHandleTUISnapshotJSONUsesRealState(t *testing.T) {
	cfgPath := seedTUISnapshotState(t)

	out := captureStdout(t, func() {
		if err := handleTUI(context.Background(), []string{"--config", cfgPath, "--snapshot", "--json"}); err != nil {
			t.Fatalf("tui snapshot json: %v", err)
		}
	})

	var snap tuiSnapshot
	if err := json.Unmarshal([]byte(out), &snap); err != nil {
		t.Fatalf("decode snapshot JSON: %v\n%s", err, out)
	}
	if snap.Downloads.Total != 4 || snap.Downloads.Active != 1 || snap.Downloads.Pending != 1 || snap.Downloads.Completed != 1 || snap.Downloads.Failed != 1 {
		t.Fatalf("unexpected download summary: %+v", snap.Downloads)
	}
	if snap.Downloads.ErrorLike != 1 || snap.Downloads.ByStatus["error"] != 1 || snap.Downloads.ByStatus["complete"] != 1 {
		t.Fatalf("unexpected download status counts: %+v", snap.Downloads)
	}
	if snap.Library.Total != 2 || snap.Library.Favorites != 1 {
		t.Fatalf("unexpected library summary: %+v", snap.Library)
	}
	if snap.Library.BySource["huggingface"] != 1 || snap.Library.BySource["modelscope"] != 1 {
		t.Fatalf("unexpected library sources: %+v", snap.Library.BySource)
	}
	if snap.Library.ByType["llm"] != 1 || snap.Library.ByType["checkpoint"] != 1 {
		t.Fatalf("unexpected library types: %+v", snap.Library.ByType)
	}
}

func TestHandleTUISnapshotTextUsesRealState(t *testing.T) {
	cfgPath := seedTUISnapshotState(t)

	out := captureStdout(t, func() {
		if err := handleTUI(context.Background(), []string{"--config", cfgPath, "--snapshot"}); err != nil {
			t.Fatalf("tui snapshot text: %v", err)
		}
	})

	for _, want := range []string{
		"TUI snapshot: downloads=4 active=1 pending=1 completed=1 failed=1 error_like=1 library=2 favorites=1",
		"Download statuses: complete=1 error=1 pending=1 running=1",
		"Library sources: huggingface=1 modelscope=1",
		"Library types: checkpoint=1 llm=1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("snapshot text missing %q:\n%s", want, out)
		}
	}
}

func TestHandleTUISnapshotMissingConfigReturnsCLIError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yml")
	err := handleTUI(context.Background(), []string{"--config", missing, "--snapshot"})
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func seedTUISnapshotState(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "cfg")
	cfgPath := writeLibraryConfig(t, root)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	db, err := state.Open(cfg)
	if err != nil {
		t.Fatalf("open state db: %v", err)
	}
	defer func() { _ = db.Close() }()

	downloads := []state.DownloadRow{
		{URL: "https://example.com/pending.gguf", Dest: filepath.Join(root, "downloads", "pending.gguf"), Status: "pending"},
		{URL: "https://example.com/running.gguf", Dest: filepath.Join(root, "downloads", "running.gguf"), Status: "running"},
		{URL: "https://example.com/complete.gguf", Dest: filepath.Join(root, "downloads", "complete.gguf"), Status: "complete"},
		{URL: "https://example.com/error.gguf", Dest: filepath.Join(root, "downloads", "error.gguf"), Status: "error", LastError: "network"},
	}
	for _, row := range downloads {
		if err := db.UpsertDownload(row); err != nil {
			t.Fatalf("seed download %s: %v", row.URL, err)
		}
	}

	metadata := []*state.ModelMetadata{
		{
			DownloadURL: "https://example.com/complete.gguf",
			Dest:        filepath.Join(root, "downloads", "complete.gguf"),
			ModelName:   "Complete Model",
			Source:      "huggingface",
			ModelType:   "LLM",
			Favorite:    true,
		},
		{
			DownloadURL: "https://example.com/checkpoint.safetensors",
			Dest:        filepath.Join(root, "downloads", "checkpoint.safetensors"),
			ModelName:   "Checkpoint Model",
			Source:      "modelscope",
			ModelType:   "Checkpoint",
		},
	}
	for _, meta := range metadata {
		if err := db.UpsertMetadata(meta); err != nil {
			t.Fatalf("seed metadata %s: %v", meta.DownloadURL, err)
		}
	}

	if err := os.MkdirAll(cfg.General.DownloadRoot, 0o755); err != nil {
		t.Fatalf("create download root: %v", err)
	}
	return cfgPath
}
