package state

import (
	"path/filepath"
	"testing"
)

func TestTransferHistoryUpsertBestAndList(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.UpsertTransferHistory(TransferHistoryRow{
		Host:        "Example.COM",
		Tool:        "modfetch",
		Connections: 4,
		ChunkSizeMB: 8,
		AvgBPS:      100,
		LastStatus:  "complete",
	}); err != nil {
		t.Fatalf("upsert slow: %v", err)
	}
	if err := db.UpsertTransferHistory(TransferHistoryRow{
		Host:        "example.com",
		Tool:        "modfetch",
		Connections: 8,
		ChunkSizeMB: 16,
		AvgBPS:      200,
		LastStatus:  "complete",
	}); err != nil {
		t.Fatalf("upsert fast: %v", err)
	}
	if err := db.UpsertTransferHistory(TransferHistoryRow{
		Host:        "example.com",
		Tool:        "modfetch",
		Connections: 16,
		ChunkSizeMB: 64,
		AvgBPS:      500,
		RateLimited: true,
		LastStatus:  "complete",
	}); err != nil {
		t.Fatalf("upsert rate limited: %v", err)
	}

	best, ok, err := db.BestTransferHistory("EXAMPLE.com", "modfetch")
	if err != nil {
		t.Fatalf("best: %v", err)
	}
	if !ok {
		t.Fatal("expected best history")
	}
	if best.Connections != 8 || best.ChunkSizeMB != 16 {
		t.Fatalf("best = %+v, want connections=8 chunk=16", best)
	}

	rows, err := db.ListTransferHistory()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows len = %d, want 3", len(rows))
	}
}
