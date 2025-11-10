package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/state"
)

// Fetcher is the interface for fetching model metadata from various sources
type Fetcher interface {
	// CanHandle returns true if this fetcher can handle the given URL
	CanHandle(url string) bool

	// FetchMetadata retrieves metadata for the given URL
	FetchMetadata(ctx context.Context, url string) (*state.ModelMetadata, error)

	// Source returns the name of this metadata source
	Source() string
}

// Registry holds all available metadata fetchers
type Registry struct {
	fetchers []Fetcher
	client   *http.Client
}

// NewRegistry creates a new fetcher registry with default fetchers
func NewRegistry() *Registry {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	r := &Registry{
		client: client,
	}

	// Register default fetchers
	r.Register(NewHuggingFaceFetcher(client))
	r.Register(NewCivitAIFetcher(client))

	return r
}

// Register adds a fetcher to the registry
func (r *Registry) Register(f Fetcher) {
	r.fetchers = append(r.fetchers, f)
}

// FetchMetadata attempts to fetch metadata using the appropriate fetcher
func (r *Registry) FetchMetadata(ctx context.Context, url string) (*state.ModelMetadata, error) {
	for _, f := range r.fetchers {
		if f.CanHandle(url) {
			return f.FetchMetadata(ctx, url)
		}
	}

	// Return minimal metadata for unknown sources
	return &state.ModelMetadata{
		DownloadURL: url,
		Source:      "direct",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// HuggingFaceFetcher fetches metadata from HuggingFace
type HuggingFaceFetcher struct {
	client *http.Client

	// Regex patterns for HuggingFace URLs
	repoPattern *regexp.Regexp // matches huggingface.co/{user}/{repo}
	filePattern *regexp.Regexp // matches huggingface.co/{user}/{repo}/resolve/{branch}/{file}
}

// NewHuggingFaceFetcher creates a new HuggingFace metadata fetcher
func NewHuggingFaceFetcher(client *http.Client) *HuggingFaceFetcher {
	return &HuggingFaceFetcher{
		client:      client,
		repoPattern: regexp.MustCompile(`huggingface\.co/([^/]+)/([^/]+)(?:/|$)`),
		filePattern: regexp.MustCompile(`huggingface\.co/([^/]+)/([^/]+)/resolve/([^/]+)/(.+)$`),
	}
}

func (f *HuggingFaceFetcher) Source() string {
	return "huggingface"
}

func (f *HuggingFaceFetcher) CanHandle(url string) bool {
	return strings.Contains(url, "huggingface.co")
}

func (f *HuggingFaceFetcher) FetchMetadata(ctx context.Context, url string) (*state.ModelMetadata, error) {
	// Extract repo and file info from URL
	matches := f.filePattern.FindStringSubmatch(url)
	if len(matches) < 5 {
		// Try just repo pattern
		matches = f.repoPattern.FindStringSubmatch(url)
		if len(matches) < 3 {
			return nil, fmt.Errorf("invalid HuggingFace URL format")
		}
	}

	user := matches[1]
	repo := matches[2]
	modelID := fmt.Sprintf("%s/%s", user, repo)

	var filename string
	var version string
	if len(matches) >= 5 {
		version = matches[3] // branch name
		filename = matches[4]
	}

	// Fetch repository metadata from HuggingFace API
	apiURL := fmt.Sprintf("https://huggingface.co/api/models/%s", modelID)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		// If API fails, return basic metadata
		return f.basicMetadata(url, modelID, filename, version), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// API failed, return basic metadata
		return f.basicMetadata(url, modelID, filename, version), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return f.basicMetadata(url, modelID, filename, version), nil
	}

	var apiResp huggingFaceModelResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return f.basicMetadata(url, modelID, filename, version), nil
	}

	// Build metadata from API response
	meta := &state.ModelMetadata{
		DownloadURL:  url,
		ModelName:    repo,
		ModelID:      modelID,
		Version:      version,
		Source:       "huggingface",
		Description:  truncateString(apiResp.Description, 5000),
		Author:       user,
		AuthorURL:    fmt.Sprintf("https://huggingface.co/%s", user),
		License:      apiResp.License,
		Tags:         apiResp.Tags,
		RepoURL:      fmt.Sprintf("https://huggingface.co/%s", modelID),
		HomepageURL:  fmt.Sprintf("https://huggingface.co/%s", modelID),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Infer model type from tags or filename
	meta.ModelType = inferModelType(filename, apiResp.Tags)

	// Extract quantization from filename if present
	if quant := ExtractQuantization(filename); quant != "" {
		meta.Quantization = quant
	}

	// Try to extract file format
	if filename != "" {
		meta.FileFormat = strings.ToLower(filepath.Ext(filename))
	}

	// Look for thumbnail
	if apiResp.CardData != nil && apiResp.CardData.ThumbnailURL != "" {
		meta.ThumbnailURL = apiResp.CardData.ThumbnailURL
	}

	return meta, nil
}

func (f *HuggingFaceFetcher) basicMetadata(url, modelID, filename, version string) *state.ModelMetadata {
	parts := strings.Split(modelID, "/")
	user := ""
	repo := modelID
	if len(parts) >= 2 {
		user = parts[0]
		repo = parts[1]
	}

	return &state.ModelMetadata{
		DownloadURL: url,
		ModelName:   repo,
		ModelID:     modelID,
		Version:     version,
		Source:      "huggingface",
		Author:      user,
		AuthorURL:   fmt.Sprintf("https://huggingface.co/%s", user),
		RepoURL:     fmt.Sprintf("https://huggingface.co/%s", modelID),
		ModelType:   inferModelType(filename, nil),
		Quantization: ExtractQuantization(filename),
		FileFormat:  strings.ToLower(filepath.Ext(filename)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// huggingFaceModelResponse represents the HuggingFace API response
type huggingFaceModelResponse struct {
	ID          string   `json:"id"`
	ModelID     string   `json:"modelId"`
	Author      string   `json:"author"`
	SHA         string   `json:"sha"`
	LastModified string  `json:"lastModified"`
	Private     bool     `json:"private"`
	Disabled    bool     `json:"disabled"`
	Gated       bool     `json:"gated"`
	Tags        []string `json:"tags"`
	Downloads   int64    `json:"downloads"`
	Likes       int64    `json:"likes"`
	License     string   `json:"license"`
	Description string   `json:"description"`
	CardData    *struct {
		ThumbnailURL string `json:"thumbnail"`
	} `json:"cardData"`
}

// Helper functions

func inferModelType(filename string, tags []string) string {
	filename = strings.ToLower(filename)

	// Check tags first
	for _, tag := range tags {
		tag = strings.ToLower(tag)
		if strings.Contains(tag, "lora") {
			return "LoRA"
		}
		if strings.Contains(tag, "embedding") || strings.Contains(tag, "textual-inversion") {
			return "Embedding"
		}
		if strings.Contains(tag, "vae") {
			return "VAE"
		}
		if strings.Contains(tag, "text-generation") || strings.Contains(tag, "llm") {
			return "LLM"
		}
	}

	// Check filename patterns
	if strings.Contains(filename, "lora") {
		return "LoRA"
	}
	if strings.Contains(filename, "vae") {
		return "VAE"
	}
	if strings.Contains(filename, "embedding") || strings.Contains(filename, "textual") {
		return "Embedding"
	}
	if strings.Contains(filename, ".gguf") || strings.Contains(filename, ".ggml") {
		return "LLM"
	}
	if strings.Contains(filename, ".safetensors") || strings.Contains(filename, ".ckpt") {
		return "Checkpoint"
	}

	return "Unknown"
}

// ExtractQuantization extracts quantization info from filename (exported for scanner)
func ExtractQuantization(filename string) string {
	filename = strings.ToUpper(filename)

	// Common GGUF quantization patterns
	quantPatterns := []string{
		"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L", "Q4_0", "Q4_1",
		"Q4_K_S", "Q4_K_M", "Q5_0", "Q5_1", "Q5_K_S", "Q5_K_M",
		"Q6_K", "Q8_0", "F16", "F32", "FP16", "FP32",
	}

	for _, pattern := range quantPatterns {
		if strings.Contains(filename, pattern) {
			return pattern
		}
	}

	return ""
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
