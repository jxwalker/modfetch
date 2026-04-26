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

func TestCivitAIResolveWithLocalModelEndpoint(t *testing.T) {
	oldBase := CivitaiBaseForTest()
	defer SetCivitaiBaseForTest(oldBase)
	t.Setenv("CIVITAI_TEST_TOKEN", "secret")

	var sawAuth atomic.Bool
	var handlerErr atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models/123" {
			handlerErr.Store(fmt.Sprintf("unexpected path: %s", r.URL.Path))
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		sawAuth.Store(r.Header.Get("Authorization") == "Bearer secret")
		model := civitModel{
			Name: "Example Model",
			ModelVersions: []civitVersion{
				{
					ID:   1,
					Name: "old",
					Files: []civitFile{
						{ID: 10, Name: "old.safetensors", Type: "Model", Primary: true, DownloadURL: "https://cdn.example/old"},
					},
				},
				{
					ID:   2,
					Name: "new",
					Files: []civitFile{
						{ID: 20, Name: "preview.txt", Type: "Other", DownloadURL: "https://cdn.example/preview"},
						{ID: 21, Name: "vae file.safetensors", Type: "VAE", Primary: true, DownloadURL: "https://cdn.example/vae"},
					},
				},
			},
		}
		if err := json.NewEncoder(w).Encode(model); err != nil {
			handlerErr.Store(fmt.Sprintf("encode model: %v", err))
			return
		}
	}))
	defer server.Close()
	SetCivitaiBaseForTest(server.URL)

	cfg := &config.Config{
		Sources: config.Sources{
			CivitAI: config.SourceWithToken{
				Enabled:  true,
				TokenEnv: "CIVITAI_TEST_TOKEN",
				Naming: config.SourceNaming{
					Pattern: "{model_name}-{version_id}-{file_type}-{file_name}",
				},
			},
		},
	}
	res, err := (&CivitAI{}).Resolve(context.Background(), "civitai://model/123?file=vae", cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v := handlerErr.Load(); v != nil {
		t.Fatal(v)
	}
	if !sawAuth.Load() {
		t.Fatal("expected Authorization header on model request")
	}
	if res.URL != "https://cdn.example/vae" || res.FileName != "vae file.safetensors" || res.FileType != "VAE" {
		t.Fatalf("unexpected resolved file: %+v", res)
	}
	if res.ModelName != "Example Model" || res.VersionName != "new" || res.VersionID != "2" {
		t.Fatalf("unexpected model metadata: %+v", res)
	}
	if res.Headers["Authorization"] != "Bearer secret" {
		t.Fatalf("expected auth header in result, got %+v", res.Headers)
	}
	if res.SuggestedFilename != "Example-Model-2-VAE-vae-file.safetensors" {
		t.Fatalf("unexpected suggested filename: %q", res.SuggestedFilename)
	}
}

func TestCivitAIResolveWithLocalVersionEndpoint(t *testing.T) {
	oldBase := CivitaiBaseForTest()
	defer SetCivitaiBaseForTest(oldBase)

	var handlerErr atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/model-versions/44":
			version := civitVersion{
				ID:      44,
				Name:    "v44",
				ModelID: 123,
				Files: []civitFile{
					{ID: 1, Name: "notes.txt", Type: "Other", DownloadURL: "https://cdn.example/notes"},
					{ID: 2, Name: "model.safetensors", Type: "Model", DownloadURL: "https://cdn.example/model"},
				},
			}
			if err := json.NewEncoder(w).Encode(version); err != nil {
				handlerErr.Store(fmt.Sprintf("encode version: %v", err))
				return
			}
		case "/api/v1/models/123":
			if err := json.NewEncoder(w).Encode(civitModel{Name: "Versioned Model"}); err != nil {
				handlerErr.Store(fmt.Sprintf("encode model: %v", err))
				return
			}
		default:
			handlerErr.Store(fmt.Sprintf("unexpected path: %s", r.URL.Path))
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
	}))
	defer server.Close()
	SetCivitaiBaseForTest(server.URL)

	res, err := (&CivitAI{}).Resolve(context.Background(), "civitai://model/123?version=44", nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v := handlerErr.Load(); v != nil {
		t.Fatal(v)
	}
	if res.URL != "https://cdn.example/model" {
		t.Fatalf("expected Model file fallback, got %+v", res)
	}
	if res.SuggestedFilename != "Versioned-Model-model.safetensors" {
		t.Fatalf("unexpected suggested filename: %q", res.SuggestedFilename)
	}
}

func TestCivitAIResolveLocalErrorsAndFilenameHelpers(t *testing.T) {
	if !hasPrefixName("Example_Model-v1", "example model") {
		t.Fatal("expected normalized prefix match")
	}
	if normalizeAlphaNum("A b_C-1!") != "abc1" {
		t.Fatalf("unexpected normalization")
	}
	if slugFilename("  weird ++ name!!.safetensors") != "weird-name.safetensors" {
		t.Fatalf("unexpected slug filename")
	}
	if slugFilename("!!.bin") != "download.bin" {
		t.Fatalf("expected download fallback")
	}

	oldBase := CivitaiBaseForTest()
	defer SetCivitaiBaseForTest(oldBase)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer server.Close()
	SetCivitaiBaseForTest(server.URL)

	_, err := (&CivitAI{}).Resolve(context.Background(), "civitai://model/404", nil)
	if err == nil || !strings.Contains(err.Error(), "civitai models: 404") {
		t.Fatalf("expected model 404 error, got %v", err)
	}
	_, err = (&CivitAI{}).Resolve(context.Background(), "civitai://model/123?version=404", nil)
	if err == nil || !strings.Contains(err.Error(), "civitai version: 404") {
		t.Fatalf("expected version 404 error, got %v", err)
	}
}
