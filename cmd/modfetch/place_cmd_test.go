package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlePlacePresetDryRunWithoutConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("MODFETCH_CONFIG", "")

	src := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(src, []byte("GGUF"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	out := captureStdout(t, func() {
		if err := handlePlace(context.Background(), []string{"--path", src, "--preset", "ollama", "--dry-run"}); err != nil {
			t.Fatalf("handle place: %v", err)
		}
	})

	if !strings.Contains(out, "Detected type: llm.gguf") {
		t.Fatalf("expected detected type in output, got:\n%s", out)
	}
	if !strings.Contains(out, "confidence=high") {
		t.Fatalf("expected confidence in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Placement mode: symlink") {
		t.Fatalf("expected placement mode in output, got:\n%s", out)
	}
	want := filepath.Join(home, ".ollama", "models", "model.gguf")
	if !strings.Contains(out, want) {
		t.Fatalf("expected planned ollama target %q in output:\n%s", want, out)
	}
}

func TestHandlePlaceListPresets(t *testing.T) {
	out := captureStdout(t, func() {
		if err := handlePlace(context.Background(), []string{"--list-presets"}); err != nil {
			t.Fatalf("list presets: %v", err)
		}
	})
	for _, name := range []string{"automatic1111", "comfyui", "forge", "hf-cache", "ollama"} {
		if !strings.Contains(out, name) {
			t.Fatalf("preset list missing %q:\n%s", name, out)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() { _ = r.Close() }()
	closedWriter := false
	closeWriter := func() {
		if closedWriter {
			return
		}
		closedWriter = true
		if err := w.Close(); err != nil {
			t.Fatalf("close pipe writer: %v", err)
		}
	}
	defer closeWriter()
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	fn()
	closeWriter()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String()
}
