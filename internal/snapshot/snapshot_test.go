package snapshot

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestBuildHuggingFaceFiltersAndBuildsBatchManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/models/owner/repo/tree/main" {
			t.Fatalf("unexpected path: %s", got)
		}
		if got := r.URL.Query().Get("recursive"); got != "true" {
			t.Fatalf("recursive query = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		_, _ = fmt.Fprint(w, `[
			{"path":"config.json","type":"file","size":100},
			{"path":"weights/model.Q4_K_M.gguf","type":"file","size":2048},
			{"path":"weights/model.Q8_0.gguf","type":"file","size":4096},
			{"path":"notes/readme.md","type":"file","size":10},
			{"path":"weights","type":"directory"}
		]`)
	}))
	defer server.Close()
	oldBase := HuggingFaceBaseURL
	HuggingFaceBaseURL = server.URL
	defer func() { HuggingFaceBaseURL = oldBase }()
	t.Setenv("HF_TOKEN", "test-token")

	manifest, err := BuildHuggingFace(context.Background(), &config.Config{
		Sources: config.Sources{HuggingFace: config.SourceWithToken{Enabled: true, TokenEnv: "HF_TOKEN"}},
	}, "hf://owner/repo/weights", Options{
		Includes: []string{"*.gguf"},
		Excludes: []string{"*Q8*"},
		DestDir:  filepath.Join(t.TempDir(), "models"),
		MaxFiles: 5,
	})
	if err != nil {
		t.Fatalf("BuildHuggingFace: %v", err)
	}
	if manifest.Repo != "owner/repo" || manifest.Prefix != "weights" || manifest.Rev != "main" {
		t.Fatalf("unexpected manifest metadata: %#v", manifest)
	}
	if len(manifest.Files) != 1 {
		t.Fatalf("files len = %d, want 1: %#v", len(manifest.Files), manifest.Files)
	}
	file := manifest.Files[0]
	if file.URI != "hf://owner/repo/weights/model.Q4_K_M.gguf?rev=main" {
		t.Fatalf("uri = %q", file.URI)
	}
	if !strings.HasSuffix(file.Dest, filepath.Join("owner__repo", "weights", "model.Q4_K_M.gguf")) {
		t.Fatalf("dest = %q", file.Dest)
	}
	bf := manifest.Batch()
	if len(bf.Jobs) != 1 || bf.Jobs[0].URI != file.URI || bf.Jobs[0].Dest != file.Dest || bf.Jobs[0].Type != "gguf" {
		t.Fatalf("batch = %#v", bf)
	}
}

func TestBuildHuggingFaceRejectsTooManyFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[
			{"path":"a.json","type":"file"},
			{"path":"b.json","type":"file"}
		]`)
	}))
	defer server.Close()
	oldBase := HuggingFaceBaseURL
	HuggingFaceBaseURL = server.URL
	defer func() { HuggingFaceBaseURL = oldBase }()

	_, err := BuildHuggingFace(context.Background(), nil, "hf://repo", Options{Includes: []string{"*.json"}, MaxFiles: 1})
	if err == nil || !strings.Contains(err.Error(), "above --max-files") {
		t.Fatalf("expected max-files error, got %v", err)
	}
}

func TestBuildHuggingFaceSupportsRootFileShorthand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/models/gpt2/tree/main" {
			t.Fatalf("unexpected path: %s", got)
		}
		_, _ = fmt.Fprint(w, `[
			{"path":"config.json","type":"file","size":100},
			{"path":"tokenizer.json","type":"file","size":200}
		]`)
	}))
	defer server.Close()
	oldBase := HuggingFaceBaseURL
	HuggingFaceBaseURL = server.URL
	defer func() { HuggingFaceBaseURL = oldBase }()

	manifest, err := BuildHuggingFace(context.Background(), nil, "hf://gpt2/config.json", Options{MaxFiles: 5})
	if err != nil {
		t.Fatalf("BuildHuggingFace: %v", err)
	}
	if manifest.Repo != "gpt2" || manifest.Prefix != "config.json" {
		t.Fatalf("unexpected manifest repo/prefix: %#v", manifest)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Path != "config.json" {
		t.Fatalf("files = %#v", manifest.Files)
	}
}

func TestBuildHuggingFaceRejectsInvalidGlob(t *testing.T) {
	_, err := BuildHuggingFace(context.Background(), nil, "hf://repo", Options{Includes: []string{"["}})
	if err == nil || !strings.Contains(err.Error(), "--include pattern") {
		t.Fatalf("expected include pattern error, got %v", err)
	}
}
