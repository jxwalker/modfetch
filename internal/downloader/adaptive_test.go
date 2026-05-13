package downloader

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/jxwalker/modfetch/internal/state"
)

func TestInitialAdaptiveLimitUsesTransferHistory(t *testing.T) {
	db, err := state.NewDB(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.UpsertTransferHistory(state.TransferHistoryRow{
		Host:        "example.com",
		Tool:        "modfetch",
		Connections: 12,
		ChunkSizeMB: 64,
		AvgBPS:      1024,
		LastStatus:  "complete",
	}); err != nil {
		t.Fatalf("upsert history: %v", err)
	}

	if got := initialAdaptiveLimit(db, "example.com", 16); got != 12 {
		t.Fatalf("initial limit = %d, want 12", got)
	}
	if got := initialAdaptiveLimit(db, "example.com", 8); got != 8 {
		t.Fatalf("clamped initial limit = %d, want 8", got)
	}
	if got := initialAdaptiveLimit(db, "missing.example", 16); got != 4 {
		t.Fatalf("default ramp start = %d, want 4", got)
	}

	db2, err := state.NewDB(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("state2: %v", err)
	}
	defer func() { _ = db2.Close() }()
	if err := db2.UpsertTransferHistory(state.TransferHistoryRow{
		Host:        "limited.example",
		Tool:        "modfetch",
		Connections: 12,
		AvgBPS:      2048,
		RateLimited: true,
		LastStatus:  "complete",
	}); err != nil {
		t.Fatalf("upsert limited history: %v", err)
	}
	if got := initialAdaptiveLimit(db2, "limited.example", 16); got != 6 {
		t.Fatalf("rate-limited initial limit = %d, want 6", got)
	}
}

func TestAdaptiveProgressReaderReportsBytes(t *testing.T) {
	var observed int64
	r := &adaptiveProgressReader{
		r: bytes.NewBufferString("abcdef"),
		onBytes: func(n int64) {
			observed += n
		},
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "abcdef" {
		t.Fatalf("read body = %q", got)
	}
	if observed != 6 {
		t.Fatalf("observed bytes = %d, want 6", observed)
	}
}
