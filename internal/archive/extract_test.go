package archive

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "model.zip")
	zf, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, err := zw.Create("nested/model.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("model")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "out")
	files, err := Extract(context.Background(), src, outDir)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one extracted file, got %d", len(files))
	}
	got, err := os.ReadFile(filepath.Join(outDir, "nested", "model.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "model" {
		t.Fatalf("unexpected extracted content: %q", string(got))
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.zip")
	zf, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	if _, err := zw.Create("../escape.bin"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Extract(context.Background(), src, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected traversal error")
	}
}
