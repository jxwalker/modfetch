package state

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestChunkLifecycleTxAndDirectUpdates(t *testing.T) {
	db := testDownloadDB(t)
	url := "https://example.com/model.bin"
	dest := filepath.Join(t.TempDir(), "model.bin")

	if err := db.WithTx(func(tx *sql.Tx) error {
		if err := db.UpsertChunkTx(tx, ChunkRow{URL: url, Dest: dest, Index: 1, Start: 10, End: 19, Size: 10, SHA256: "abc", Status: "pending"}); err != nil {
			return err
		}
		if err := db.UpsertChunkTx(tx, ChunkRow{URL: url, Dest: dest, Index: 0, Start: 0, End: 9, Size: 10, Status: "running"}); err != nil {
			return err
		}
		if err := db.UpdateChunkStatusTx(tx, url, dest, 1, "complete"); err != nil {
			return err
		}
		return db.UpdateChunkSHATx(tx, url, dest, 0, "def")
	}); err != nil {
		t.Fatalf("chunk tx lifecycle: %v", err)
	}

	chunks, err := db.ListChunks(url, dest)
	if err != nil {
		t.Fatalf("list chunks: %v", err)
	}
	if len(chunks) != 2 || chunks[0].Index != 0 || chunks[1].Index != 1 {
		t.Fatalf("expected chunks ordered by index, got %+v", chunks)
	}
	if chunks[0].SHA256 != "def" || chunks[1].Status != "complete" {
		t.Fatalf("unexpected chunk state: %+v", chunks)
	}

	if err := db.UpsertChunk(ChunkRow{URL: url, Dest: dest, Index: 1, Start: 10, End: 19, Size: 10, Status: "dirty"}); err != nil {
		t.Fatalf("upsert chunk with empty sha: %v", err)
	}
	chunks, err = db.ListChunks(url, dest)
	if err != nil {
		t.Fatalf("list chunks after upsert: %v", err)
	}
	if chunks[1].SHA256 != "abc" || chunks[1].Status != "dirty" {
		t.Fatalf("expected sha preservation and status update, got %+v", chunks[1])
	}

	if err := db.UpdateChunkStatus(url, dest, 1, "complete"); err != nil {
		t.Fatalf("direct status update: %v", err)
	}
	if err := db.UpdateChunkSHA(url, dest, 1, "ghi"); err != nil {
		t.Fatalf("direct sha update: %v", err)
	}
	if err := db.DeleteChunks(url, dest); err != nil {
		t.Fatalf("delete chunks: %v", err)
	}
	chunks, err = db.ListChunks(url, dest)
	if err != nil {
		t.Fatalf("list chunks after delete: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("expected chunks deleted, got %+v", chunks)
	}
}

func TestDownloadStatusRetriesAndDelete(t *testing.T) {
	db := testDownloadDB(t)
	row := DownloadRow{URL: "https://example.com/a", Dest: "/tmp/a.bin", Status: "pending"}
	if err := db.UpsertDownload(row); err != nil {
		t.Fatalf("upsert download: %v", err)
	}
	if err := db.IncDownloadRetries(row.URL, row.Dest, 2); err != nil {
		t.Fatalf("increment retries: %v", err)
	}
	if err := db.IncDownloadRetries(row.URL, row.Dest, 0); err != nil {
		t.Fatalf("zero retry increment should be a no-op: %v", err)
	}
	if err := db.UpdateDownloadStatus(row.URL, row.Dest, "error", "network"); err != nil {
		t.Fatalf("update status: %v", err)
	}
	if err := db.UpdateDownloadStatus("missing", row.Dest, "error", "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for missing update, got %v", err)
	}

	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 1 || rows[0].Retries != 2 || rows[0].Status != "error" || rows[0].LastError != "network" {
		t.Fatalf("unexpected download row: %+v", rows)
	}

	if err := db.DeleteDownload(row.URL, row.Dest); err != nil {
		t.Fatalf("delete download: %v", err)
	}
	rows, err = db.ListDownloads()
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected download deleted, got %+v", rows)
	}
}

func TestHostCapsListDeleteClearAndNilDB(t *testing.T) {
	db := testDownloadDB(t)
	if err := db.UpsertHostCaps("b.example", false, true); err != nil {
		t.Fatalf("upsert b: %v", err)
	}
	if err := db.UpsertHostCaps("a.example", true, false); err != nil {
		t.Fatalf("upsert a: %v", err)
	}

	caps, err := db.ListHostCaps()
	if err != nil {
		t.Fatalf("list host caps: %v", err)
	}
	if len(caps) != 2 || caps[0].Host != "a.example" || !caps[0].HeadOK || caps[0].AcceptRanges {
		t.Fatalf("unexpected listed host caps: %+v", caps)
	}

	if err := db.DeleteHostCaps("a.example"); err != nil {
		t.Fatalf("delete host cap: %v", err)
	}
	if _, ok, err := db.GetHostCaps("a.example"); err != nil || ok {
		t.Fatalf("expected deleted host cap to be missing, ok=%v err=%v", ok, err)
	}
	if err := db.ClearHostCaps(); err != nil {
		t.Fatalf("clear host caps: %v", err)
	}
	caps, err = db.ListHostCaps()
	if err != nil {
		t.Fatalf("list after clear: %v", err)
	}
	if len(caps) != 0 {
		t.Fatalf("expected empty host caps, got %+v", caps)
	}

	var nilDB *DB
	if err := nilDB.InitHostCapsTable(); err == nil {
		t.Fatal("expected nil db init error")
	}
	if err := nilDB.UpsertHostCaps("example.com", true, true); err == nil {
		t.Fatal("expected nil db upsert error")
	}
	if _, _, err := nilDB.GetHostCaps("example.com"); err == nil {
		t.Fatal("expected nil db get error")
	}
	if _, err := nilDB.ListHostCaps(); err == nil {
		t.Fatal("expected nil db list error")
	}
	if err := nilDB.DeleteHostCaps("example.com"); err == nil {
		t.Fatal("expected nil db delete error")
	}
	if err := nilDB.ClearHostCaps(); err == nil {
		t.Fatal("expected nil db clear error")
	}
}

func TestGetMetadataByDestAndUsage(t *testing.T) {
	db := testDB(t)
	meta := &ModelMetadata{
		DownloadURL: "https://example.com/model.gguf",
		Dest:        "/models/model.gguf",
		ModelName:   "Model",
		Tags:        []string{"llm", "gguf"},
		Favorite:    true,
	}
	if err := db.UpsertMetadata(meta); err != nil {
		t.Fatalf("upsert metadata: %v", err)
	}

	got, err := db.GetMetadataByDest(meta.Dest)
	if err != nil {
		t.Fatalf("get metadata by dest: %v", err)
	}
	if got == nil || got.DownloadURL != meta.DownloadURL || len(got.Tags) != 2 || !got.Favorite {
		t.Fatalf("unexpected metadata by dest: %+v", got)
	}
	missing, err := db.GetMetadataByDest("/models/missing.gguf")
	if err != nil {
		t.Fatalf("missing metadata by dest: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil missing metadata, got %+v", missing)
	}
	if _, err := db.GetMetadataByDest(""); err == nil {
		t.Fatal("expected empty dest error")
	}

	if err := db.UpdateMetadataUsage(meta.DownloadURL); err != nil {
		t.Fatalf("update metadata usage: %v", err)
	}
	got, err = db.GetMetadataByDest(meta.Dest)
	if err != nil {
		t.Fatalf("get metadata after usage: %v", err)
	}
	if got.TimesUsed != 1 || got.LastUsed == nil {
		t.Fatalf("expected usage update, got %+v", got)
	}
	if err := db.UpdateMetadataUsage("missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for missing usage update, got %v", err)
	}
}

func TestSubscribeReceivesCoalescedStateChanges(t *testing.T) {
	db := testDownloadDB(t)
	events, cancel := db.Subscribe()
	defer cancel()

	if err := db.UpsertDownload(DownloadRow{URL: "https://example.com/a", Dest: "/tmp/a", Status: "pending"}); err != nil {
		t.Fatalf("upsert download: %v", err)
	}
	expectEvent(t, events)

	if err := db.WithTx(func(tx *sql.Tx) error {
		return db.UpsertDownloadTx(tx, DownloadRow{URL: "https://example.com/b", Dest: "/tmp/b", Status: "pending"})
	}); err != nil {
		t.Fatalf("tx upsert: %v", err)
	}
	expectEvent(t, events)

	cancel()
	if _, ok := <-events; ok {
		t.Fatal("expected subscription channel to close after cancel")
	}
}

func expectEvent(t *testing.T, events <-chan struct{}) {
	t.Helper()
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for state event")
	}
}
