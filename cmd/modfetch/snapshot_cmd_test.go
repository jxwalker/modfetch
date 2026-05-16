package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/snapshot"
)

func TestSnapshotWritesBatchManifestFromHuggingFaceTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[
			{"path":"config.json","type":"file","size":100},
			{"path":"model.safetensors","type":"file","size":200},
			{"path":"optimizer.pt","type":"file","size":300}
		]`)
	}))
	defer server.Close()
	oldBase := snapshot.HuggingFaceBaseURL
	snapshot.HuggingFaceBaseURL = server.URL
	defer func() { snapshot.HuggingFaceBaseURL = oldBase }()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	writeStarterTestConfig(t, cfgPath, tmp)
	outPath := filepath.Join(tmp, "snapshot.yml")
	if err := handleSnapshot(context.Background(), []string{
		"--config", cfgPath,
		"--include", "*.json",
		"--include", "*.safetensors",
		"--exclude", "optimizer*",
		"--output", outPath,
		"hf://owner/repo",
	}); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	text := string(body)
	for _, want := range []string{"version: 1", "hf://owner/repo/config.json?rev=main", "model.safetensors"} {
		if !strings.Contains(text, want) {
			t.Fatalf("snapshot manifest missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "optimizer.pt") {
		t.Fatalf("snapshot should exclude optimizer.pt:\n%s", text)
	}
}

func TestSnapshotDryRunJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[{"path":"weights/a.gguf","type":"file","size":100}]`)
	}))
	defer server.Close()
	oldBase := snapshot.HuggingFaceBaseURL
	snapshot.HuggingFaceBaseURL = server.URL
	defer func() { snapshot.HuggingFaceBaseURL = oldBase }()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	writeStarterTestConfig(t, cfgPath, tmp)
	out := captureStdout(t, func() {
		err := handleSnapshot(context.Background(), []string{
			"--config", cfgPath,
			"--json",
			"--dry-run",
			"hf://owner/repo/weights",
		})
		if err != nil {
			t.Fatalf("snapshot dry-run: %v", err)
		}
	})
	for _, want := range []string{`"source": "huggingface"`, `"prefix": "weights"`, `"weights/a.gguf"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run JSON missing %q in:\n%s", want, out)
		}
	}
}
