package metadata

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestHuggingFaceFetcher_CanHandle(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "HuggingFace URL",
			url:  "https://huggingface.co/TheBloke/Llama-2-7B-GGUF/resolve/main/llama-2-7b.Q4_K_M.gguf",
			want: true,
		},
		{
			name: "HuggingFace repo URL",
			url:  "https://huggingface.co/TheBloke/Llama-2-7B-GGUF",
			want: true,
		},
		{
			name: "CivitAI URL",
			url:  "https://civitai.com/api/download/models/12345",
			want: false,
		},
		{
			name: "Direct URL",
			url:  "https://example.com/model.gguf",
			want: false,
		},
	}

	f := NewHuggingFaceFetcher(&http.Client{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f.CanHandle(tt.url); got != tt.want {
				t.Errorf("CanHandle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHuggingFaceFetcher_FetchMetadata_Success(t *testing.T) {
	response := `{
		"id": "TheBloke/Llama-2-7B-GGUF",
		"modelId": "TheBloke/Llama-2-7B-GGUF",
		"author": "TheBloke",
		"tags": ["llama", "llama-2", "gguf", "text-generation"],
		"downloads": 125000,
		"likes": 450,
		"license": "llama2",
		"description": "Llama 2 7B GGUF format model",
		"cardData": {
			"thumbnail": "https://huggingface.co/thumbnail.png"
		}
	}`

	client := routedHTTPClient(t, "huggingface.co", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/TheBloke/Llama-2-7B-GGUF" {
			t.Errorf("unexpected Hugging Face API path: %s", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))

	f := NewHuggingFaceFetcher(client)
	ctx := context.Background()
	url := "https://huggingface.co/TheBloke/Llama-2-7B-GGUF/resolve/main/llama-2-7b.Q4_K_M.gguf"

	meta, err := f.FetchMetadata(ctx, url)
	if err != nil {
		t.Fatalf("FetchMetadata() error = %v", err)
	}

	// Verify basic fields
	if meta.Source != "huggingface" {
		t.Errorf("Source = %q, want %q", meta.Source, "huggingface")
	}

	if meta.ModelID != "TheBloke/Llama-2-7B-GGUF" {
		t.Errorf("ModelID = %q, want %q", meta.ModelID, "TheBloke/Llama-2-7B-GGUF")
	}

	if meta.Author != "TheBloke" {
		t.Errorf("Author = %q, want %q", meta.Author, "TheBloke")
	}

	if meta.ModelType != "LLM" {
		t.Errorf("ModelType = %q, want %q", meta.ModelType, "LLM")
	}

	if meta.Quantization != "Q4_K_M" {
		t.Errorf("Quantization = %q, want %q", meta.Quantization, "Q4_K_M")
	}

	if meta.License != "llama2" {
		t.Errorf("License = %q, want %q", meta.License, "llama2")
	}

	if len(meta.Tags) == 0 {
		t.Error("Tags should not be empty")
	}
}

func TestHuggingFaceFetcher_FetchMetadata_APIFailure(t *testing.T) {
	client := routedHTTPClient(t, "huggingface.co", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/TheBloke/Llama-2-7B-GGUF" {
			t.Errorf("unexpected Hugging Face API path: %s", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))

	f := NewHuggingFaceFetcher(client)
	ctx := context.Background()
	url := "https://huggingface.co/TheBloke/Llama-2-7B-GGUF/resolve/main/llama-2-7b.Q4_K_M.gguf"

	meta, err := f.FetchMetadata(ctx, url)
	if err != nil {
		t.Fatalf("FetchMetadata() should not error on API failure, got error = %v", err)
	}

	// Should return basic metadata
	if meta.Source != "huggingface" {
		t.Errorf("Source = %q, want %q", meta.Source, "huggingface")
	}

	if meta.ModelID != "TheBloke/Llama-2-7B-GGUF" {
		t.Errorf("ModelID = %q, want %q", meta.ModelID, "TheBloke/Llama-2-7B-GGUF")
	}
}

func TestInferModelType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		tags     []string
		want     string
	}{
		{
			name:     "GGUF file",
			filename: "model.gguf",
			tags:     []string{},
			want:     "LLM",
		},
		{
			name:     "LoRA in filename",
			filename: "character_lora.safetensors",
			tags:     []string{},
			want:     "LoRA",
		},
		{
			name:     "LoRA in tags",
			filename: "model.safetensors",
			tags:     []string{"lora", "character"},
			want:     "LoRA",
		},
		{
			name:     "VAE file",
			filename: "vae-ft-mse.safetensors",
			tags:     []string{},
			want:     "VAE",
		},
		{
			name:     "Embedding",
			filename: "embeddings/textual_inversion.pt",
			tags:     []string{"textual-inversion"},
			want:     "Embedding",
		},
		{
			name:     "Checkpoint",
			filename: "model.ckpt",
			tags:     []string{},
			want:     "Checkpoint",
		},
		{
			name:     "Text generation from tags",
			filename: "model.bin",
			tags:     []string{"text-generation", "transformers"},
			want:     "LLM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferModelType(tt.filename, tt.tags)
			if got != tt.want {
				t.Errorf("inferModelType(%q, %v) = %q, want %q", tt.filename, tt.tags, got, tt.want)
			}
		})
	}
}

func TestRegistry_FetchMetadata(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name       string
		url        string
		wantSource string
	}{
		{
			name:       "HuggingFace URL",
			url:        "https://huggingface.co/TheBloke/Model/resolve/main/file.gguf",
			wantSource: "huggingface",
		},
		{
			name:       "CivitAI URL",
			url:        "https://civitai.com/api/download/models/12345",
			wantSource: "civitai",
		},
		{
			name:       "Direct URL",
			url:        "https://example.com/model.gguf",
			wantSource: "direct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			meta, err := registry.FetchMetadata(ctx, tt.url)
			if err != nil && tt.wantSource != "direct" {
				// Network errors are expected when this reaches a live registry.
				t.Skipf("Skipping due to network requirement: %v", err)
			}

			if meta.Source != tt.wantSource {
				t.Errorf("Source = %q, want %q", meta.Source, tt.wantSource)
			}
		})
	}
}
