package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/state"
)

func setupRecoveryModel(t *testing.T) (*Model, *state.DB, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := state.NewDB(dbPath)
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	cfg := &config.Config{
		General: config.General{
			DownloadRoot: tmpDir,
		},
	}
	model := New(cfg, db, "test-version").(*Model)
	return model, db, tmpDir
}

func TestDlDoneMsgClearsRunningStateOnSuccess(t *testing.T) {
	model, db, _ := setupRecoveryModel(t)
	defer func() { _ = db.Close() }()

	key := "https://example.com/model.bin|/tmp/model.bin"
	model.running[key] = func() {}
	model.retrying[key] = model.lastRefresh

	updated, _ := model.Update(dlDoneMsg{
		url:  "https://example.com/model.bin",
		dest: "/tmp/model.bin",
		path: "/tmp/model.bin",
	})
	m := updated.(*Model)

	if _, ok := m.running[key]; ok {
		t.Fatal("running key still present after success")
	}
	if _, ok := m.retrying[key]; ok {
		t.Fatal("retrying key still present after success")
	}
}

func TestDlDoneMsgClearsRunningStateOnFailure(t *testing.T) {
	model, db, _ := setupRecoveryModel(t)
	defer func() { _ = db.Close() }()

	key := "https://example.com/model.bin|/tmp/model.bin"
	model.running[key] = func() {}

	updated, _ := model.Update(dlDoneMsg{
		url:  "https://example.com/model.bin",
		dest: "/tmp/model.bin",
		err:  context.Canceled,
	})
	m := updated.(*Model)

	if _, ok := m.running[key]; ok {
		t.Fatal("running key still present after failure")
	}
}

func TestRecoverCmdReconcilesFinalizedRunningRow(t *testing.T) {
	model, db, tmpDir := setupRecoveryModel(t)
	defer func() { _ = db.Close() }()

	dest := filepath.Join(tmpDir, "model.bin")
	if err := os.WriteFile(dest, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write dest: %v", err)
	}
	if err := os.WriteFile(dest+".sha256", []byte("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824  model.bin\n"), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
	if err := db.UpsertDownload(state.DownloadRow{
		URL:    "https://example.com/model.bin",
		Dest:   dest,
		Size:   5,
		Status: "running",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	msg := model.recoverCmd()().(recoverRowsMsg)
	if len(msg.rows) != 0 {
		t.Fatalf("expected no rows to recover, got %d", len(msg.rows))
	}
	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Status != "complete" {
		t.Fatalf("status = %q, want complete", rows[0].Status)
	}
}

func TestRecoverCmdDoesNotResumeHoldRows(t *testing.T) {
	model, db, tmpDir := setupRecoveryModel(t)
	defer func() { _ = db.Close() }()

	dest := filepath.Join(tmpDir, "hold.bin")
	if err := db.UpsertDownload(state.DownloadRow{
		URL:    "https://example.com/hold.bin",
		Dest:   dest,
		Status: "hold",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	msg := model.recoverCmd()().(recoverRowsMsg)
	if len(msg.rows) != 0 {
		t.Fatalf("expected no hold rows to auto-recover, got %d", len(msg.rows))
	}
}
