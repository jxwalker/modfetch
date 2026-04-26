package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/jxwalker/modfetch/internal/state"
)

func TestDedupeDuplicateGroupsHardlinksVerifiedDuplicates(t *testing.T) {
	dir := t.TempDir()
	content := []byte("same model")
	sum := sha256.Sum256(content)
	sha := hex.EncodeToString(sum[:])
	canonical := filepath.Join(dir, "a.gguf")
	duplicate := filepath.Join(dir, "b.gguf")
	if err := os.WriteFile(canonical, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(duplicate, content, 0o644); err != nil {
		t.Fatal(err)
	}

	results := dedupeDuplicateGroups([]duplicateGroup{{
		SHA256: sha,
		Rows: []state.DownloadRow{
			{Dest: canonical, ActualSHA256: sha, Status: "complete"},
			{Dest: duplicate, ActualSHA256: sha, Status: "complete"},
		},
	}}, "hardlink", false)
	if len(results) != 2 {
		t.Fatalf("expected two results, got %+v", results)
	}
	if results[0].Action != "canonical" || results[1].Action != "hardlink" || results[1].Error != "" {
		t.Fatalf("unexpected dedupe results: %+v", results)
	}
	aInfo, err := os.Stat(canonical)
	if err != nil {
		t.Fatal(err)
	}
	bInfo, err := os.Stat(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(aInfo, bInfo) {
		t.Fatal("expected duplicate to be replaced with hardlink to canonical")
	}
}

func TestDedupeDuplicateGroupsDryRunDoesNotModify(t *testing.T) {
	dir := t.TempDir()
	content := []byte("same model")
	sum := sha256.Sum256(content)
	sha := hex.EncodeToString(sum[:])
	canonical := filepath.Join(dir, "a.gguf")
	duplicate := filepath.Join(dir, "b.gguf")
	if err := os.WriteFile(canonical, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(duplicate, content, 0o644); err != nil {
		t.Fatal(err)
	}

	results := dedupeDuplicateGroups([]duplicateGroup{{
		SHA256: sha,
		Rows: []state.DownloadRow{
			{Dest: canonical, ActualSHA256: sha, Status: "complete"},
			{Dest: duplicate, ActualSHA256: sha, Status: "complete"},
		},
	}}, "symlink", true)
	if len(results) != 2 || results[1].Action != "would_symlink" {
		t.Fatalf("unexpected dry-run results: %+v", results)
	}
	info, err := os.Lstat(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("dry run should not replace duplicate")
	}
}

func TestDedupeDuplicateGroupsSkipsMismatchedContent(t *testing.T) {
	dir := t.TempDir()
	content := []byte("same model")
	sum := sha256.Sum256(content)
	sha := hex.EncodeToString(sum[:])
	canonical := filepath.Join(dir, "a.gguf")
	duplicate := filepath.Join(dir, "b.gguf")
	if err := os.WriteFile(canonical, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(duplicate, []byte("different"), 0o644); err != nil {
		t.Fatal(err)
	}

	results := dedupeDuplicateGroups([]duplicateGroup{{
		SHA256: sha,
		Rows: []state.DownloadRow{
			{Dest: canonical, ActualSHA256: sha, Status: "complete"},
			{Dest: duplicate, ActualSHA256: sha, Status: "complete"},
		},
	}}, "hardlink", false)
	if len(results) != 2 || results[1].Action != "skipped" || results[1].Error == "" {
		t.Fatalf("expected mismatch skip, got %+v", results)
	}
}
