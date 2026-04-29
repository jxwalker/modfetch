package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchHuggingFaceReturnsDownloadableURI(t *testing.T) {
	old := huggingFaceBaseURL
	defer func() { huggingFaceBaseURL = old }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models":
			if got := r.URL.Query().Get("search"); got != "tiny" {
				t.Fatalf("search query = %q", got)
			}
			_, _ = w.Write([]byte(`[{"id":"owner/tiny","modelId":"owner/tiny","downloads":42,"likes":7,"pipeline_tag":"text-generation","tags":["gguf"]}]`))
		case "/api/models/owner/tiny/tree/main":
			_, _ = w.Write([]byte(`[
				{"type":"file","path":"README.md","size":123},
				{"type":"file","path":"tiny.Q4_K_M.gguf","size":2048},
				{"type":"file","path":"model.safetensors","size":4096}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	huggingFaceBaseURL = server.URL

	results, err := Search(context.Background(), Options{Provider: ProviderHuggingFace, Query: "tiny", Limit: 3})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d", len(results))
	}
	got := results[0]
	if got.URI != "hf://owner/tiny/tiny.Q4_K_M.gguf?rev=main" {
		t.Fatalf("uri = %q", got.URI)
	}
	if got.Size != 2048 || got.FileType != "gguf" || got.Index != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestSearchCivitAIReturnsResolverURI(t *testing.T) {
	old := civitaiBaseURL
	defer func() { civitaiBaseURL = old }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"id":123,"name":"Tiny Checkpoint","type":"Checkpoint","tags":["sd"],"stats":{"downloadCount":9,"thumbsUpCount":2},"modelVersions":[{"id":456,"name":"v1","files":[{"name":"tiny.safetensors","type":"Model","sizeKB":10.5}]}]}]}`))
	}))
	defer server.Close()
	civitaiBaseURL = server.URL

	results, err := Search(context.Background(), Options{Provider: ProviderCivitAI, Query: "tiny", Limit: 3})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d", len(results))
	}
	got := results[0]
	if got.URI != "civitai://model/123?version=456&file=tiny.safetensors" {
		t.Fatalf("uri = %q", got.URI)
	}
	if got.Size != 10752 || got.FileType != "Model" || got.VersionID != "456" {
		t.Fatalf("unexpected result: %+v", got)
	}
}
