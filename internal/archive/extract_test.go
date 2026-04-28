package archive

import (
	"archive/zip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestExtract7zUsesExternalBackend(t *testing.T) {
	bin, err := find7z()
	if err != nil {
		t.Skipf("7z backend unavailable: %v", err)
	}

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "nested", "model.bin"), []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "model.7z")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "a", "-t7z", src, ".")
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create 7z archive: %v\n%s", err, out)
	}

	outDir := filepath.Join(dir, "out")
	files, err := Extract(context.Background(), src, outDir)
	if err != nil {
		t.Fatalf("extract 7z: %v", err)
	}
	if len(files) != 1 || files[0] != filepath.Join(outDir, "nested", "model.bin") {
		t.Fatalf("unexpected extracted files: %+v", files)
	}
	got, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "model" {
		t.Fatalf("unexpected extracted content: %q", string(got))
	}
}

func TestExtract7zReportsMissingBackend(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", filepath.Join(dir, "empty-bin"))
	src := filepath.Join(dir, "model.7z")
	if err := os.WriteFile(src, []byte("sample archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Extract(context.Background(), src, filepath.Join(dir, "out"))
	if err == nil || !strings.Contains(err.Error(), "requires 7zz, 7z, or 7za") {
		t.Fatalf("expected missing 7z backend error, got %v", err)
	}
}

func TestMoveExtractedTreeRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup is platform-specific")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(src, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := moveExtractedTree(context.Background(), src, filepath.Join(dir, "out"))
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestMoveExtractedTreeHonorsContextCancellation(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "model.bin"), []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := moveExtractedTree(ctx, src, filepath.Join(dir, "out"))
	if err == nil || err != context.Canceled {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
