package metadata

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestOllamaFetcherCanHandle(t *testing.T) {
	f := NewOllamaFetcher(&http.Client{})
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "library model", url: "https://ollama.com/library/llama3.2", want: true},
		{name: "library tag", url: "https://ollama.com/library/llama3.2:1b", want: true},
		{name: "tags page ignored", url: "https://ollama.com/library/llama3.2/tags", want: true},
		{name: "other host", url: "https://example.com/library/llama3.2", want: false},
		{name: "ollama docs", url: "https://ollama.com/download", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f.CanHandle(tt.url); got != tt.want {
				t.Fatalf("CanHandle(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestNewOllamaFetcherDefaultsNilClient(t *testing.T) {
	f := NewOllamaFetcher(nil)
	if f.client == nil {
		t.Fatal("NewOllamaFetcher(nil) should default to a non-nil HTTP client")
	}
}

func TestOllamaFetcherFetchMetadataSuccess(t *testing.T) {
	client := routedHTTPClient(t, "ollama.com", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/library/llama3.2" {
			t.Errorf("unexpected Ollama page path: %s", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
<head>
  <title>ignored title</title>
  <meta property="og:title" content="llama3.2" />
  <meta name="description" content="Meta&#39;s Llama 3.2 goes small with 1B and 3B models." />
  <meta property="og:image" content="https://ollama.com/public/og.png" />
</head>
<body>
  <span data-pull-count>67.2M</span>
  <span data-size>1b</span>
  <span data-size>3b</span>
</body>
</html>`))
	}))

	f := NewOllamaFetcher(client)
	meta, err := f.FetchMetadata(context.Background(), "https://ollama.com/library/llama3.2:1b")
	if err != nil {
		t.Fatalf("FetchMetadata: %v", err)
	}
	if meta.Source != "ollama" || meta.ModelName != "llama3.2" || meta.ModelID != "ollama/llama3.2" {
		t.Fatalf("unexpected identity metadata: %+v", meta)
	}
	if meta.Version != "1b" {
		t.Fatalf("Version = %q, want 1b", meta.Version)
	}
	if meta.Description != "Meta's Llama 3.2 goes small with 1B and 3B models." {
		t.Fatalf("Description = %q", meta.Description)
	}
	if meta.DownloadCount != 67_200_000 {
		t.Fatalf("DownloadCount = %d, want 67200000", meta.DownloadCount)
	}
	for _, want := range []string{"ollama", "1b", "3b"} {
		if !containsStringFold(meta.Tags, want) {
			t.Fatalf("Tags = %v, missing %q", meta.Tags, want)
		}
	}
	if meta.ModelType != "LLM" || meta.FileFormat != "ollama" {
		t.Fatalf("unexpected type metadata: %+v", meta)
	}
}

func TestOllamaFetcherParsesLiveStyleAttributes(t *testing.T) {
	page := `<span x-test-pull-count>42K</span><span>Downloads</span><span x-test-size>7b</span>`
	if got := parseCompactCount(pullCountText(page)); got != 42_000 {
		t.Fatalf("live-style pull count = %d, want 42000", got)
	}
	if tags := sizeTags(page); len(tags) != 1 || tags[0] != "7b" {
		t.Fatalf("live-style size tags = %v, want [7b]", tags)
	}
}

func TestOllamaFetcherFallsBackOnHTTPFailure(t *testing.T) {
	client := routedHTTPClient(t, "ollama.com", http.NotFoundHandler())
	f := NewOllamaFetcher(client)
	meta, err := f.FetchMetadata(context.Background(), "https://ollama.com/library/qwen2.5")
	if err != nil {
		t.Fatalf("FetchMetadata should fall back on HTTP failure: %v", err)
	}
	if meta.Source != "ollama" || meta.ModelName != "qwen2.5" || meta.HomepageURL != "https://ollama.com/library/qwen2.5" {
		t.Fatalf("unexpected fallback metadata: %+v", meta)
	}
}

func TestParseOllamaLibraryURLRejectsTraversal(t *testing.T) {
	tests := []string{
		"https://ollama.com/library/..%2Fdownload",
		"https://ollama.com/library/%2E",
		"https://ollama.com/library/%2E%2E",
		"https://ollama.com/library/model..name",
		"https://ollama.com/library/model%252Fname",
	}
	for _, rawURL := range tests {
		if _, _, err := parseOllamaLibraryURL(rawURL); err == nil {
			t.Fatalf("parseOllamaLibraryURL(%q) should reject traversal model", rawURL)
		}
	}
}

func TestRegistryIncludesOllamaFetcher(t *testing.T) {
	registry := NewRegistry()
	if !registry.CanHandle("https://ollama.com/library/llama3.2") {
		t.Fatal("registry should handle Ollama library URLs")
	}
}

func TestParseCompactCount(t *testing.T) {
	tests := map[string]int{
		"67.2M": 67_200_000,
		"1.2K":  1_200,
		"120K":  120_000,
		"1,250": 1_250,
		"2B":    2_000_000_000,
		"bad":   0,
	}
	for input, want := range tests {
		if got := parseCompactCount(input); got != want {
			t.Fatalf("parseCompactCount(%q) = %d, want %d", input, got, want)
		}
	}
}

func containsStringFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}
