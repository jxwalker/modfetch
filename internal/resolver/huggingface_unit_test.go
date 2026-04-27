package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestHuggingFaceQuantizationHelpers(t *testing.T) {
	if got, ok := detectQuantization("models/TinyLlama.Q4_K_M.gguf"); !ok || got != "Q4_K_M" {
		t.Fatalf("expected Q4_K_M, got %q ok=%v", got, ok)
	}
	if _, ok := detectQuantization("models/TinyLlama.gguf"); ok {
		t.Fatal("did not expect quantization for untagged filename")
	}

	groups := groupQuantizations([]hfRepoFile{
		{Path: "models", Type: "directory"},
		{Path: "README.md", Type: "file", Size: 10},
		{Path: "model.Q5_K_M.gguf", Type: "file", Size: 100},
		{Path: "model.Q4_K_M.gguf", Type: "file", Size: 80},
		{Path: "full/model.safetensors", Type: "file", Size: 200},
		{Path: "full/model-small.safetensors", Type: "file", Size: 50},
	})
	if len(groups["Q4_K_M"]) != 1 || len(groups["Q5_K_M"]) != 1 || len(groups["default"]) != 2 {
		t.Fatalf("unexpected groups: %+v", groups)
	}

	if q, ok := selectBestQuantization(groups, "q5_k_m"); !ok || q.Name != "Q5_K_M" {
		t.Fatalf("expected requested Q5_K_M, got %+v ok=%v", q, ok)
	}
	if q, ok := selectBestQuantization(groups, "missing"); ok || q.Name != "" {
		t.Fatalf("expected missing requested quant to fail, got %+v ok=%v", q, ok)
	}
	if q, ok := selectBestQuantization(groups, ""); !ok || q.Name != "Q4_K_M" {
		t.Fatalf("expected preferred Q4_K_M, got %+v ok=%v", q, ok)
	}
	if q, ok := selectBestQuantization(map[string][]Quantization{"default": groups["default"]}, ""); !ok || q.Size != 200 {
		t.Fatalf("expected largest default file, got %+v ok=%v", q, ok)
	}

	flat := flattenQuantizations(groups)
	if len(flat) != 4 || flat[0].Name != "Q4_K_M" || flat[1].Name != "Q5_K_M" {
		t.Fatalf("unexpected flattened quantization order: %+v", flat)
	}
}

func TestHuggingFaceResolveWithLocalTreeAndNamingPattern(t *testing.T) {
	oldBase := hfBaseURL
	oldGetenv := getenv
	defer func() {
		hfBaseURL = oldBase
		getenv = oldGetenv
	}()
	getenv = func(key string) string {
		if key == "HF_TEST_TOKEN" {
			return "secret"
		}
		return ""
	}

	var sawAuth atomic.Bool
	var handlerErr atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/acme/tiny/tree/main" {
			handlerErr.Store(fmt.Sprintf("unexpected path: %s", r.URL.Path))
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		sawAuth.Store(r.Header.Get("Authorization") == "Bearer secret")
		files := []hfRepoFile{
			{Path: "tiny.Q4_K_M.gguf", Type: "file", Size: 42},
			{Path: "tiny.Q8_0.gguf", Type: "file", Size: 84},
		}
		if err := json.NewEncoder(w).Encode(files); err != nil {
			handlerErr.Store(fmt.Sprintf("encode files: %v", err))
			return
		}
	}))
	defer server.Close()
	hfBaseURL = server.URL

	cfg := &config.Config{
		Sources: config.Sources{
			HuggingFace: config.SourceWithToken{
				Enabled:  true,
				TokenEnv: "HF_TEST_TOKEN",
				Naming: config.SourceNaming{
					Pattern: "{owner}-{repo}-{quantization}-{file_name}",
				},
			},
		},
	}
	res, err := (&HuggingFace{}).Resolve(context.Background(), "hf://acme/tiny?quant=q8_0", cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v := handlerErr.Load(); v != nil {
		t.Fatal(v)
	}
	if !sawAuth.Load() {
		t.Fatal("expected Authorization header on tree request")
	}
	if res.SelectedQuantization != "Q8_0" || res.RepoPath != "tiny.Q8_0.gguf" {
		t.Fatalf("unexpected selected quantization: %+v", res)
	}
	if !strings.Contains(res.URL, "/acme/tiny/resolve/main/tiny.Q8_0.gguf") {
		t.Fatalf("unexpected resolve URL: %s", res.URL)
	}
	if res.Headers["Authorization"] != "Bearer secret" {
		t.Fatalf("expected auth header in resolved result, got %+v", res.Headers)
	}
	if res.SuggestedFilename != "acme-tiny-Q8_0-tiny.Q8_0.gguf" {
		t.Fatalf("unexpected suggested filename: %q", res.SuggestedFilename)
	}
	if len(res.AvailableQuantizations) != 2 {
		t.Fatalf("expected available quantizations, got %+v", res.AvailableQuantizations)
	}
}

func TestHuggingFaceResolveSingleSegmentRepoAliasWithLocalTree(t *testing.T) {
	oldBase := hfBaseURL
	defer func() { hfBaseURL = oldBase }()

	var handlerErr atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/gpt2/tree/main" {
			handlerErr.Store(fmt.Sprintf("unexpected path: %s", r.URL.Path))
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		files := []hfRepoFile{
			{Path: "pytorch_model.bin", Type: "file", Size: 42},
		}
		if err := json.NewEncoder(w).Encode(files); err != nil {
			handlerErr.Store(fmt.Sprintf("encode files: %v", err))
			return
		}
	}))
	defer server.Close()
	hfBaseURL = server.URL

	res, err := (&HuggingFace{}).Resolve(context.Background(), "hf://gpt2", nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v := handlerErr.Load(); v != nil {
		t.Fatal(v)
	}
	if res.URL != server.URL+"/gpt2/resolve/main/pytorch_model.bin" {
		t.Fatalf("unexpected resolve URL: %s", res.URL)
	}
	if res.RepoOwner != "" || res.RepoName != "gpt2" || res.RepoPath != "pytorch_model.bin" {
		t.Fatalf("unexpected repo metadata: %+v", res)
	}
}

func TestHuggingFacePathParsingPrefersNamespacedRepos(t *testing.T) {
	tests := []struct {
		name         string
		rawPath      string
		wantOwner    string
		wantRepo     string
		wantRepoID   string
		wantFilePath string
	}{
		{
			name:       "namespaced repo with dotted name",
			rawPath:    "owner/model.v1",
			wantOwner:  "owner",
			wantRepo:   "model.v1",
			wantRepoID: "owner/model.v1",
		},
		{
			name:         "namespaced repo file path",
			rawPath:      "owner/model.v1/README.md",
			wantOwner:    "owner",
			wantRepo:     "model.v1",
			wantRepoID:   "owner/model.v1",
			wantFilePath: "README.md",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			owner, repo, repoID, filePath, err := parseHFPath(tc.rawPath)
			if err != nil {
				t.Fatalf("parseHFPath: %v", err)
			}
			if owner != tc.wantOwner || repo != tc.wantRepo || repoID != tc.wantRepoID || filePath != tc.wantFilePath {
				t.Fatalf("unexpected parse: owner=%q repo=%q repoID=%q filePath=%q", owner, repo, repoID, filePath)
			}
		})
	}
}

func TestHuggingFaceResolveDottedNamespacedRepo(t *testing.T) {
	oldBase := hfBaseURL
	defer func() { hfBaseURL = oldBase }()

	var handlerErr atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/acme/model.v1/tree/main" {
			handlerErr.Store(fmt.Sprintf("unexpected path: %s", r.URL.Path))
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		files := []hfRepoFile{
			{Path: "model.Q4_K_M.gguf", Type: "file", Size: 42},
		}
		if err := json.NewEncoder(w).Encode(files); err != nil {
			handlerErr.Store(fmt.Sprintf("encode files: %v", err))
			return
		}
	}))
	defer server.Close()
	hfBaseURL = server.URL

	res, err := (&HuggingFace{}).Resolve(context.Background(), "hf://acme/model.v1?rev=main", nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v := handlerErr.Load(); v != nil {
		t.Fatal(v)
	}
	if res.URL != server.URL+"/acme/model.v1/resolve/main/model.Q4_K_M.gguf" {
		t.Fatalf("unexpected resolve URL: %s", res.URL)
	}
	if res.RepoOwner != "acme" || res.RepoName != "model.v1" || res.RepoPath != "model.Q4_K_M.gguf" {
		t.Fatalf("unexpected repo metadata: %+v", res)
	}
}

func TestHuggingFaceAliasFileFallback(t *testing.T) {
	oldBase := hfBaseURL
	defer func() { hfBaseURL = oldBase }()

	var sawCanonicalAttempt atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/models/gpt2/README.md/tree/main" {
			sawCanonicalAttempt.Store(true)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "unexpected path", http.StatusNotFound)
	}))
	defer server.Close()
	hfBaseURL = server.URL

	res, err := (&HuggingFace{}).Resolve(context.Background(), "hf://gpt2/README.md?rev=main", nil)
	if err != nil {
		t.Fatalf("Resolve direct file alias: %v", err)
	}
	if !sawCanonicalAttempt.Load() {
		t.Fatal("expected canonical owner/repo tree lookup before alias fallback")
	}
	if res.URL != server.URL+"/gpt2/resolve/main/README.md" {
		t.Fatalf("unexpected resolve URL: %s", res.URL)
	}
	if res.RepoOwner != "" || res.RepoName != "gpt2" || res.RepoPath != "README.md" {
		t.Fatalf("unexpected repo metadata: %+v", res)
	}
}

func TestHuggingFaceAliasFileFallbackSupportsExtensionlessFiles(t *testing.T) {
	oldBase := hfBaseURL
	defer func() { hfBaseURL = oldBase }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/gpt2/LICENSE/tree/main" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()
	hfBaseURL = server.URL

	res, err := (&HuggingFace{}).Resolve(context.Background(), "hf://gpt2/LICENSE?rev=main", nil)
	if err != nil {
		t.Fatalf("Resolve direct file alias: %v", err)
	}
	if res.URL != server.URL+"/gpt2/resolve/main/LICENSE" {
		t.Fatalf("unexpected resolve URL: %s", res.URL)
	}
	if res.RepoOwner != "" || res.RepoName != "gpt2" || res.RepoPath != "LICENSE" {
		t.Fatalf("unexpected repo metadata: %+v", res)
	}
}

func TestHuggingFaceResolveLocalTreeErrors(t *testing.T) {
	oldBase := hfBaseURL
	defer func() { hfBaseURL = oldBase }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()
	hfBaseURL = server.URL

	_, err := (&HuggingFace{}).Resolve(context.Background(), "hf://acme/tiny", nil)
	if err == nil || !strings.Contains(err.Error(), "hf api returned 404") {
		t.Fatalf("expected 404 tree error, got %v", err)
	}
}

func TestHuggingFaceDirectFileResolutionAndSuggestedFilename(t *testing.T) {
	cfg := &config.Config{
		Sources: config.Sources{
			HuggingFace: config.SourceWithToken{
				Naming: config.SourceNaming{Pattern: "{owner}_{repo}_{rev}_{file_name}"},
			},
		},
	}
	res, err := (&HuggingFace{}).Resolve(context.Background(), "hf://acme/tiny/path/to/model+v1.gguf?rev=dev", cfg)
	if err != nil {
		t.Fatalf("Resolve direct file: %v", err)
	}
	if !strings.Contains(res.URL, "model%2Bv1.gguf") {
		t.Fatalf("expected plus to be escaped in URL, got %s", res.URL)
	}
	if res.SuggestedFilename != "acme_tiny_dev_model-v1.gguf" {
		t.Fatalf("unexpected suggested filename: %q", res.SuggestedFilename)
	}
}

func TestHuggingFaceDirectFileResolutionForSingleSegmentRepoAlias(t *testing.T) {
	oldBase := hfBaseURL
	defer func() { hfBaseURL = oldBase }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()
	hfBaseURL = server.URL

	res, err := (&HuggingFace{}).Resolve(context.Background(), "hf://gpt2/README.md?rev=main", nil)
	if err != nil {
		t.Fatalf("Resolve direct file: %v", err)
	}
	if res.URL != server.URL+"/gpt2/resolve/main/README.md" {
		t.Fatalf("unexpected resolve URL: %s", res.URL)
	}
	if res.RepoOwner != "" || res.RepoName != "gpt2" || res.RepoPath != "README.md" {
		t.Fatalf("unexpected repo metadata: %+v", res)
	}
	if res.FileName != "README.md" || res.SuggestedFilename != "README.md" {
		t.Fatalf("unexpected file naming: %+v", res)
	}
}
