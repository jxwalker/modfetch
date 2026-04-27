package catalog

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/jxwalker/modfetch/internal/state"
)

func TestExportImportRoundTripPreservesLibraryCatalog(t *testing.T) {
	source := testDB(t)
	destPath := filepath.Join(t.TempDir(), "llama.gguf")
	meta := state.ModelMetadata{
		DownloadURL:   "https://example.com/llama.gguf",
		Dest:          destPath,
		ModelName:     "Llama Test",
		ModelID:       "org/llama-test",
		Source:        "huggingface",
		ModelType:     "LLM",
		Tags:          []string{"llama", "gguf"},
		Quantization:  "Q4_K_M",
		FileSize:      42,
		FileFormat:    ".gguf",
		UserNotes:     "portable favorite",
		UserRating:    5,
		Favorite:      true,
		HomepageURL:   "https://example.com/llama",
		ThumbnailURL:  "https://example.com/llama.png",
		DownloadCount: 7,
		TimesUsed:     3,
	}
	if err := source.UpsertMetadata(&meta); err != nil {
		t.Fatalf("upsert metadata: %v", err)
	}
	if err := source.UpsertDownload(state.DownloadRow{
		URL:            meta.DownloadURL,
		Dest:           meta.Dest,
		ExpectedSHA256: "expected",
		ActualSHA256:   "actual",
		Size:           meta.FileSize,
		Status:         "completed",
	}); err != nil {
		t.Fatalf("upsert download: %v", err)
	}

	var buf bytes.Buffer
	if err := Export(source, &buf); err != nil {
		t.Fatalf("export: %v", err)
	}
	var exported Catalog
	if err := json.Unmarshal(buf.Bytes(), &exported); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	if exported.CatalogVersion != Version || len(exported.Models) != 1 {
		t.Fatalf("unexpected exported catalog: %+v", exported)
	}
	if exported.Models[0].Download == nil || exported.Models[0].Download.ActualSHA256 != "actual" {
		t.Fatalf("expected checksum snapshot in export, got %+v", exported.Models[0].Download)
	}

	target := testDB(t)
	result, err := Import(target, bytes.NewReader(buf.Bytes()), ImportOptions{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Creates != 1 || result.Updates != 0 || result.Skips != 0 || result.Conflicts != 0 {
		t.Fatalf("unexpected import result: %+v", result)
	}

	imported, err := target.GetMetadata(meta.DownloadURL)
	if err != nil {
		t.Fatalf("get imported metadata: %v", err)
	}
	if imported.ModelName != meta.ModelName || !imported.Favorite || imported.UserNotes != meta.UserNotes {
		t.Fatalf("metadata was not preserved: %+v", imported)
	}
	downloads, err := target.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(downloads) != 1 || downloads[0].ActualSHA256 != "actual" || downloads[0].ExpectedSHA256 != "expected" {
		t.Fatalf("download checksum was not preserved: %+v", downloads)
	}

	second, err := Import(target, bytes.NewReader(buf.Bytes()), ImportOptions{})
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if second.Skips != 1 || second.Creates != 0 || second.Updates != 0 || second.Conflicts != 0 {
		t.Fatalf("expected idempotent skip, got %+v", second)
	}
}

func TestImportDryRunReportsCreatesWithoutWriting(t *testing.T) {
	db := testDB(t)
	cat := Catalog{
		App:            "modfetch",
		CatalogVersion: Version,
		Models: []CatalogEntry{{
			Metadata: state.ModelMetadata{
				DownloadURL: "https://example.com/model.safetensors",
				Dest:        "/models/model.safetensors",
				ModelName:   "Dry Run Model",
			},
		}},
	}
	payload := encodeCatalog(t, cat)
	result, err := Import(db, bytes.NewReader(payload), ImportOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run import: %v", err)
	}
	if !result.DryRun || result.Creates != 1 {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if _, err := db.GetMetadata("https://example.com/model.safetensors"); err == nil {
		t.Fatal("dry-run import wrote metadata")
	}
}

func TestImportDryRunSimulatesEarlierEntries(t *testing.T) {
	db := testDB(t)
	cat := Catalog{
		App:            "modfetch",
		CatalogVersion: Version,
		Models: []CatalogEntry{
			{
				Metadata: state.ModelMetadata{
					DownloadURL: "https://example.com/first.gguf",
					Dest:        "/models/shared.gguf",
					ModelName:   "First",
				},
			},
			{
				Metadata: state.ModelMetadata{
					DownloadURL: "https://example.com/second.gguf",
					Dest:        "/models/shared.gguf",
					ModelName:   "Second",
				},
			},
		},
	}
	result, err := Import(db, bytes.NewReader(encodeCatalog(t, cat)), ImportOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run import: %v", err)
	}
	if result.Creates != 1 || result.Conflicts != 1 {
		t.Fatalf("expected dry-run to simulate destination conflict, got %+v", result)
	}
	if _, err := db.GetMetadata("https://example.com/first.gguf"); err == nil {
		t.Fatal("dry-run import wrote metadata")
	}
}

func TestImportReportsDestinationConflict(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertMetadata(&state.ModelMetadata{
		DownloadURL: "https://example.com/existing.gguf",
		Dest:        "/models/shared.gguf",
		ModelName:   "Existing",
	}); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	cat := Catalog{
		App:            "modfetch",
		CatalogVersion: Version,
		Models: []CatalogEntry{{
			Metadata: state.ModelMetadata{
				DownloadURL: "https://example.com/incoming.gguf",
				Dest:        "/models/shared.gguf",
				ModelName:   "Incoming",
			},
		}},
	}
	result, err := Import(db, bytes.NewReader(encodeCatalog(t, cat)), ImportOptions{})
	if err != nil {
		t.Fatalf("import conflict catalog: %v", err)
	}
	if result.Conflicts != 1 || result.Entries[0].Action != "conflict" {
		t.Fatalf("expected destination conflict, got %+v", result)
	}
}

func TestImportRejectsMismatchedDownloadSnapshot(t *testing.T) {
	db := testDB(t)
	cat := Catalog{
		App:            "modfetch",
		CatalogVersion: Version,
		Models: []CatalogEntry{{
			Metadata: state.ModelMetadata{
				DownloadURL: "https://example.com/model.gguf",
				Dest:        "/models/model.gguf",
				ModelName:   "Model",
			},
			Download: &DownloadSnapshot{
				URL:    "https://example.com/other.gguf",
				Dest:   "/models/model.gguf",
				Status: "completed",
			},
		}},
	}
	result, err := Import(db, bytes.NewReader(encodeCatalog(t, cat)), ImportOptions{})
	if err != nil {
		t.Fatalf("import mismatch catalog: %v", err)
	}
	if result.Conflicts != 1 || result.Entries[0].Action != "conflict" {
		t.Fatalf("expected snapshot mismatch conflict, got %+v", result)
	}
}

func TestImportNormalizesSnapshotOnlyDestination(t *testing.T) {
	db := testDB(t)
	cat := Catalog{
		App:            "modfetch",
		CatalogVersion: Version,
		Models: []CatalogEntry{{
			Metadata: state.ModelMetadata{
				DownloadURL: "https://example.com/model.gguf",
				ModelName:   "Model",
			},
			Download: &DownloadSnapshot{
				URL:    "https://example.com/model.gguf",
				Dest:   "/models/model.gguf",
				Status: "completed",
			},
		}},
	}
	result, err := Import(db, bytes.NewReader(encodeCatalog(t, cat)), ImportOptions{})
	if err != nil {
		t.Fatalf("import snapshot-only dest catalog: %v", err)
	}
	if result.Creates != 1 || result.Conflicts != 0 {
		t.Fatalf("expected create without conflict, got %+v", result)
	}
	meta, err := db.GetMetadata("https://example.com/model.gguf")
	if err != nil {
		t.Fatalf("get metadata: %v", err)
	}
	if meta.Dest != "/models/model.gguf" {
		t.Fatalf("expected metadata destination to be normalized, got %q", meta.Dest)
	}
}

func TestImportDryRunMovedDestinationFreesOldPath(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertMetadata(&state.ModelMetadata{
		DownloadURL: "https://example.com/moved.gguf",
		Dest:        "/models/old.gguf",
		ModelName:   "Moved",
	}); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	cat := Catalog{
		App:            "modfetch",
		CatalogVersion: Version,
		Models: []CatalogEntry{
			{
				Metadata: state.ModelMetadata{
					DownloadURL: "https://example.com/moved.gguf",
					Dest:        "/models/new.gguf",
					ModelName:   "Moved",
				},
			},
			{
				Metadata: state.ModelMetadata{
					DownloadURL: "https://example.com/replacement.gguf",
					Dest:        "/models/old.gguf",
					ModelName:   "Replacement",
				},
			},
		},
	}
	result, err := Import(db, bytes.NewReader(encodeCatalog(t, cat)), ImportOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run import moved destination catalog: %v", err)
	}
	if result.Updates != 1 || result.Creates != 1 || result.Conflicts != 0 {
		t.Fatalf("expected move to free old destination in dry-run, got %+v", result)
	}
}

func TestImportEmptySnapshotStatusPreservesExistingDownloadStatus(t *testing.T) {
	db := testDB(t)
	meta := state.ModelMetadata{
		DownloadURL: "https://example.com/model.gguf",
		Dest:        "/models/model.gguf",
		ModelName:   "Model",
	}
	if err := db.UpsertMetadata(&meta); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	if err := db.UpsertDownload(state.DownloadRow{
		URL:    meta.DownloadURL,
		Dest:   meta.Dest,
		Size:   10,
		Status: "completed",
	}); err != nil {
		t.Fatalf("seed download: %v", err)
	}
	meta.ModelName = "Model Updated"
	cat := Catalog{
		App:            "modfetch",
		CatalogVersion: Version,
		Models: []CatalogEntry{{
			Metadata: meta,
			Download: &DownloadSnapshot{
				URL:  meta.DownloadURL,
				Dest: meta.Dest,
				Size: 10,
			},
		}},
	}
	result, err := Import(db, bytes.NewReader(encodeCatalog(t, cat)), ImportOptions{})
	if err != nil {
		t.Fatalf("import empty-status catalog: %v", err)
	}
	if result.Updates != 1 {
		t.Fatalf("expected update, got %+v", result)
	}
	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 1 || rows[0].Status != "completed" {
		t.Fatalf("expected existing status to be preserved, got %+v", rows)
	}
}

func encodeCatalog(t *testing.T, cat Catalog) []byte {
	t.Helper()
	payload, err := json.Marshal(cat)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func testDB(t *testing.T) *state.DB {
	t.Helper()
	db, err := state.NewDB(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
