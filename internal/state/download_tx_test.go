package state

import (
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
	dest := filepath.Join(t.TempDir(), "file.gguf")
	if err := db.UpsertDownload(DownloadRow{URL: oldURL, Dest: dest, Status: "pending"}); err != nil {
		t.Fatalf("upsert original: %v", err)
	}
	if err := db.UpsertChunk(ChunkRow{URL: oldURL, Dest: dest, Index: 0, Start: 0, End: 9, Size: 10, Status: "pending"}); err != nil {
		t.Fatalf("upsert original chunk: %v", err)
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
	if err := db.SQL.QueryRow(`SELECT COUNT(*) FROM downloads WHERE url=? AND dest=?`, oldURL, dest).Scan(&count); err != nil {
		t.Fatalf("count old row: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected old row to be removed, got %d", count)
	}
	chunks, err := db.ListChunks(oldURL, dest)
	if err != nil {
		t.Fatalf("list old chunks: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("expected old chunks to be removed, got %d", len(chunks))
	}
}

func TestDeleteDownloadsAndChunksByDest(t *testing.T) {
	db := testDownloadDB(t)

	dest := filepath.Join(t.TempDir(), "file.gguf")
	for _, url := range []string{"https://example.com/a", "https://cdn.example.com/a"} {
		if err := db.UpsertDownload(DownloadRow{URL: url, Dest: dest, Status: "error"}); err != nil {
			t.Fatalf("upsert download: %v", err)
		}
		if err := db.UpsertChunk(ChunkRow{URL: url, Dest: dest, Index: 0, Start: 0, End: 9, Size: 10, Status: "dirty"}); err != nil {
			t.Fatalf("upsert chunk: %v", err)
		}
	}

	if err := db.DeleteDownloadsAndChunksByDest(dest); err != nil {
		t.Fatalf("delete by dest: %v", err)
	}

	var count int
	if err := db.SQL.QueryRow(`SELECT COUNT(*) FROM downloads WHERE dest=?`, dest).Scan(&count); err != nil {
		t.Fatalf("count downloads: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected downloads to be removed, got %d", count)
	}
	if err := db.SQL.QueryRow(`SELECT COUNT(*) FROM chunks WHERE dest=?`, dest).Scan(&count); err != nil {
		t.Fatalf("count chunks: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected chunks to be removed, got %d", count)
	}
}

func TestReplaceDownloadDest(t *testing.T) {
	db := testDownloadDB(t)

	url := "https://example.com/model.bin"
	oldDest := filepath.Join(t.TempDir(), "model.bin")
	newDest := "s3://models/model.bin"
	if err := db.UpsertDownload(DownloadRow{URL: url, Dest: oldDest, Status: "complete", ActualSHA256: "abc"}); err != nil {
		t.Fatalf("upsert download: %v", err)
	}
	if err := db.UpsertChunk(ChunkRow{URL: url, Dest: oldDest, Index: 0, Start: 0, End: 3, Size: 4, Status: "complete"}); err != nil {
		t.Fatalf("upsert chunk: %v", err)
	}

	if err := db.ReplaceDownloadDest(url, oldDest, newDest); err != nil {
		t.Fatalf("replace dest: %v", err)
	}

	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 1 || rows[0].Dest != newDest {
		t.Fatalf("expected download dest %q, got %+v", newDest, rows)
	}
	chunks, err := db.ListChunks(url, newDest)
	if err != nil {
		t.Fatalf("list chunks: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected chunk to move to new dest, got %d", len(chunks))
	}
}
