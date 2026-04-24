package state

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func testDownloadDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})
	return db
}

func TestClearChunksAndUpsertDownload(t *testing.T) {
	db := testDownloadDB(t)

	row := DownloadRow{URL: "https://example.com/a", Dest: "/tmp/a.bin", Status: "running"}
	if err := db.UpsertDownload(row); err != nil {
		t.Fatalf("upsert download: %v", err)
	}
	if err := db.UpsertChunk(ChunkRow{URL: row.URL, Dest: row.Dest, Index: 0, Start: 0, End: 9, Size: 10, Status: "complete"}); err != nil {
		t.Fatalf("upsert chunk: %v", err)
	}

	row.Status = "canceled"
	row.LastError = "user canceled"
	if err := db.ClearChunksAndUpsertDownload(row); err != nil {
		t.Fatalf("clear chunks and upsert: %v", err)
	}

	chunks, err := db.ListChunks(row.URL, row.Dest)
	if err != nil {
		t.Fatalf("list chunks: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("expected chunks to be cleared, got %d", len(chunks))
	}

	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one download row, got %d", len(rows))
	}
	if rows[0].Status != "canceled" || rows[0].LastError != "user canceled" {
		t.Fatalf("unexpected row after transaction: %+v", rows[0])
	}
}

func TestDeleteDownloadAndChunks(t *testing.T) {
	db := testDownloadDB(t)

	row := DownloadRow{URL: "https://example.com/a", Dest: "/tmp/a.bin", Status: "pending"}
	if err := db.UpsertDownload(row); err != nil {
		t.Fatalf("upsert download: %v", err)
	}
	if err := db.UpsertChunk(ChunkRow{URL: row.URL, Dest: row.Dest, Index: 0, Start: 0, End: 9, Size: 10, Status: "pending"}); err != nil {
		t.Fatalf("upsert chunk: %v", err)
	}

	if err := db.DeleteDownloadAndChunks(row.URL, row.Dest); err != nil {
		t.Fatalf("delete download and chunks: %v", err)
	}

	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected download row to be deleted, got %d", len(rows))
	}
	chunks, err := db.ListChunks(row.URL, row.Dest)
	if err != nil {
		t.Fatalf("list chunks: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("expected chunks to be deleted, got %d", len(chunks))
	}
}

func TestReplaceDownloadURL(t *testing.T) {
	db := testDownloadDB(t)

	oldURL := "hf://owner/repo/file.gguf"
	dest := "/tmp/file.gguf"
	if err := db.UpsertDownload(DownloadRow{URL: oldURL, Dest: dest, Status: "pending"}); err != nil {
		t.Fatalf("upsert original: %v", err)
	}

	newURL := "https://cdn.example.com/file.gguf"
	if err := db.ReplaceDownloadURL(oldURL, DownloadRow{URL: newURL, Dest: dest, Status: "pending"}); err != nil {
		t.Fatalf("replace URL: %v", err)
	}

	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly one row after replace, got %d", len(rows))
	}
	if rows[0].URL != newURL {
		t.Fatalf("expected resolved URL %q, got %q", newURL, rows[0].URL)
	}

	var count int
	if err := db.SQL.QueryRow(`SELECT COUNT(*) FROM downloads WHERE url=? AND dest=?`, oldURL, dest).Scan(&count); err != nil && err != sql.ErrNoRows {
		t.Fatalf("count old row: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected old row to be removed, got %d", count)
	}
}
