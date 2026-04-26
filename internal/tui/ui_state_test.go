package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/state"
)

func TestLoadUIStateRestoresCompactFalse(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "ui_state_v2.json"), []byte(`{"compact":false}`), 0o644); err != nil {
		t.Fatalf("write ui state: %v", err)
	}
	db, err := state.NewDB(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	defer func() { _ = db.Close() }()

	cfg := &config.Config{
		General: config.General{DataRoot: tmpDir, DownloadRoot: tmpDir},
		UI:      config.UIOptions{Compact: true},
	}
	New(cfg, db, "test")

	if cfg.UI.Compact {
		t.Fatal("expected persisted compact=false to override config compact=true")
	}
}
