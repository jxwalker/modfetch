package metadata

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCivitAIFetcher_CanHandle(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "CivitAI API download URL",
			url:  "https://civitai.com/api/download/models/12345",
			want: true,
		},
		{
			name: "CivitAI model page URL",
			url:  "https://civitai.com/models/12345",
			want: true,
		},
		{
			name: "HuggingFace URL",
			url:  "https://huggingface.co/TheBloke/Model",
			want: false,
		},
		{
			name: "Direct URL",
			url:  "https://example.com/model.safetensors",
			want: false,
		},
	}

	f := NewCivitAIFetcher(&http.Client{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f.CanHandle(tt.url); got != tt.want {
				t.Errorf("CanHandle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCivitAIFetcher_FetchMetadata_Success(t *testing.T) {
	mockResponse := `{
		"id": 12345,
		"name": "Realistic Vision",
		"description": "A photorealistic checkpoint model",
		"type": "Checkpoint",
		"tags": ["realistic", "photorealistic"],
		"creator": {
			"username": "TestCreator"
		},
		"stats": {
			"downloadCount": 50000,
			"rating": 4.5
		},
		"modelVersions": [{
			"id": 67890,
			"name": "v5.0",
			"baseModel": "SD 1.5",
			"files": [{
				"name": "model.safetensors",
				"sizeKB": 2000000,
				"type": "Model",
				"metadata": {
					"format": "SafeTensor",
					"fp": "fp16"
				}
			}],
			"images": [{
				"url": "https://civitai.com/image.jpg"
			}]
		}]
	}`

	client := &http.Client{
		Transport: &mockTransport{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(mockResponse)),
				Header:     make(http.Header),
			},
		},
	}

	f := NewCivitAIFetcher(client)
	ctx := context.Background()
	url := "https://civitai.com/api/download/models/12345"

	meta, err := f.FetchMetadata(ctx, url)
	if err != nil {
		t.Fatalf("FetchMetadata() error = %v", err)
	}

	if meta.Source != "civitai" {
		t.Errorf("Source = %q, want %q", meta.Source, "civitai")
	}

	if meta.ModelName != "Realistic Vision" {
		t.Errorf("ModelName = %q, want %q", meta.ModelName, "Realistic Vision")
	}

	if meta.ModelID != "civitai-12345" {
		t.Errorf("ModelID = %q, want %q", meta.ModelID, "civitai-12345")
	}

	if meta.ModelType != "Checkpoint" {
		t.Errorf("ModelType = %q, want %q", meta.ModelType, "Checkpoint")
	}

	if meta.Author != "TestCreator" {
		t.Errorf("Author = %q, want %q", meta.Author, "TestCreator")
	}

	if meta.BaseModel != "SD 1.5" {
		t.Errorf("BaseModel = %q, want %q", meta.BaseModel, "SD 1.5")
	}

	if meta.FileSize != 2000000*1024 {
		t.Errorf("FileSize = %d, want %d", meta.FileSize, 2000000*1024)
	}

	if meta.Quantization != "fp16" {
		t.Errorf("Quantization = %q, want %q", meta.Quantization, "fp16")
	}

	if meta.DownloadCount != 50000 {
		t.Errorf("DownloadCount = %d, want %d", meta.DownloadCount, 50000)
	}
}

func TestCivitAIFetcher_FetchMetadata_Unauthorized(t *testing.T) {
	client := &http.Client{
		Transport: &mockTransport{
			response: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader("unauthorized")),
				Header:     make(http.Header),
			},
		},
	}

	f := NewCivitAIFetcher(client)
	ctx := context.Background()
	url := "https://civitai.com/api/download/models/12345"

	_, err := f.FetchMetadata(ctx, url)
	if err == nil {
		t.Error("FetchMetadata() expected error for unauthorized, got nil")
	}

	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("Error should mention 'access denied', got: %v", err)
	}
}

func TestMapCivitAIType(t *testing.T) {
	tests := []struct {
		name        string
		civitaiType string
		want        string
	}{
		{
			name:        "Checkpoint",
			civitaiType: "Checkpoint",
			want:        "Checkpoint",
		},
		{
			name:        "LoRA",
			civitaiType: "LORA",
			want:        "LoRA",
		},
		{
			name:        "LyCORIS",
			civitaiType: "LyCORIS",
			want:        "LoRA",
		},
		{
			name:        "Textual Inversion",
			civitaiType: "TextualInversion",
			want:        "Embedding",
		},
		{
			name:        "VAE",
			civitaiType: "VAE",
			want:        "VAE",
		},
		{
			name:        "ControlNet",
			civitaiType: "controlnet",
			want:        "ControlNet",
		},
		{
			name:        "Unknown type",
			civitaiType: "SomethingNew",
			want:        "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapCivitAIType(tt.civitaiType)
			if got != tt.want {
				t.Errorf("mapCivitAIType(%q) = %q, want %q", tt.civitaiType, got, tt.want)
			}
		})
	}
}

func TestParseCivitAIModelID(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name:    "API download URL",
			url:     "https://civitai.com/api/download/models/12345",
			want:    "12345",
			wantErr: false,
		},
		{
			name:    "Model page URL",
			url:     "https://civitai.com/models/67890",
			want:    "67890",
			wantErr: false,
		},
		{
			name:    "Invalid URL",
			url:     "https://example.com/model.safetensors",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCivitAIModelID(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCivitAIModelID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseCivitAIModelID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseCivitAIVersionID(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    int64
		wantErr bool
	}{
		{
			name:    "Valid version ID",
			url:     "https://civitai.com/api/download/models/12345",
			want:    12345,
			wantErr: false,
		},
		{
			name:    "Invalid URL",
			url:     "https://civitai.com/models/invalid",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCivitAIVersionID(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCivitAIVersionID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseCivitAIVersionID() = %d, want %d", got, tt.want)
			}
		})
	}
}
