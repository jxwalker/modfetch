package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/state"
)

// CivitAIFetcher fetches metadata from CivitAI
type CivitAIFetcher struct {
	client *http.Client
	apiKey string // optional API key for authenticated requests

	// Regex patterns for CivitAI URLs
	modelPattern *regexp.Regexp // matches civitai.com/models/{id}
	apiPattern   *regexp.Regexp // matches civitai.com/api/download/models/{id}
}

// NewCivitAIFetcher creates a new CivitAI metadata fetcher
func NewCivitAIFetcher(client *http.Client) *CivitAIFetcher {
	return &CivitAIFetcher{
		client:       client,
		modelPattern: regexp.MustCompile(`civitai\.com/models/(\d+)`),
		apiPattern:   regexp.MustCompile(`civitai\.com/api/download/models/(\d+)`),
	}
}

// SetAPIKey sets the CivitAI API key for authenticated requests
func (f *CivitAIFetcher) SetAPIKey(key string) {
	f.apiKey = key
}

func (f *CivitAIFetcher) Source() string {
	return "civitai"
}

func (f *CivitAIFetcher) CanHandle(url string) bool {
	return strings.Contains(url, "civitai.com")
}

func (f *CivitAIFetcher) FetchMetadata(ctx context.Context, url string) (*state.ModelMetadata, error) {
	// Extract model ID from URL
	var modelID string

	// Try API pattern first
	matches := f.apiPattern.FindStringSubmatch(url)
	if len(matches) >= 2 {
		modelID = matches[1]
	} else {
		// Try model page pattern
		matches = f.modelPattern.FindStringSubmatch(url)
		if len(matches) >= 2 {
			modelID = matches[1]
		}
	}

	if modelID == "" {
		return nil, fmt.Errorf("could not extract CivitAI model ID from URL")
	}

	// Fetch model metadata from CivitAI API
	apiURL := fmt.Sprintf("https://civitai.com/api/v1/models/%s", modelID)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Add API key if available
	if f.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+f.apiKey)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		// Check if this might be a connectivity issue (e.g., needs VPN)
		if strings.Contains(err.Error(), "no such host") ||
			strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "timeout") {
			return nil, fmt.Errorf("CivitAI connection failed (VPN may be required): %w", err)
		}
		return nil, fmt.Errorf("fetching metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("CivitAI access denied - API key may be required or VPN needed")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CivitAI API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var apiResp civitAIModelResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	// Build metadata from API response
	meta := &state.ModelMetadata{
		DownloadURL:   url,
		ModelName:     apiResp.Name,
		ModelID:       fmt.Sprintf("civitai-%s", modelID),
		Source:        "civitai",
		Description:   truncateString(apiResp.Description, 5000),
		Tags:          apiResp.Tags,
		ModelType:     mapCivitAIType(apiResp.Type),
		HomepageURL:   fmt.Sprintf("https://civitai.com/models/%s", modelID),
		DownloadCount: int(apiResp.Stats.DownloadCount),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Set author info only if username is non-empty
	if apiResp.Creator.Username != "" {
		meta.Author = apiResp.Creator.Username
		meta.AuthorURL = fmt.Sprintf("https://civitai.com/user/%s", apiResp.Creator.Username)
	}

	// Get the first model version (usually the latest)
	if len(apiResp.ModelVersions) > 0 {
		version := apiResp.ModelVersions[0]
		meta.Version = version.Name

		// Look for base model info
		if version.BaseModel != "" {
			meta.BaseModel = version.BaseModel
		}

		// Get thumbnail from first image
		if len(version.Images) > 0 {
			meta.ThumbnailURL = version.Images[0].URL
		}

		// Get file info from first file
		if len(version.Files) > 0 {
			file := version.Files[0]
			meta.FileSize = file.SizeKB * 1024 // Convert KB to bytes
			meta.FileFormat = strings.ToLower(file.Type)

			// Extract metadata from file
			if file.Metadata.Format != "" {
				meta.Architecture = file.Metadata.Format
			}
			if file.Metadata.FP != "" {
				meta.Quantization = file.Metadata.FP
			}
		}
	}

	// Convert rating (civitai uses 1-5 scale, same as ours)
	if apiResp.Stats.Rating > 0 {
		meta.UserRating = int(apiResp.Stats.Rating)
	}

	return meta, nil
}

// civitAIModelResponse represents the CivitAI API model response
type civitAIModelResponse struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	POI         bool     `json:"poi"`
	NSFW        bool     `json:"nsfw"`
	Tags        []string `json:"tags"`
	Creator     struct {
		Username string `json:"username"`
		Image    string `json:"image"`
	} `json:"creator"`
	Stats struct {
		DownloadCount int64   `json:"downloadCount"`
		FavoriteCount int64   `json:"favoriteCount"`
		CommentCount  int64   `json:"commentCount"`
		Rating        float64 `json:"rating"`
		RatingCount   int64   `json:"ratingCount"`
	} `json:"stats"`
	ModelVersions []civitAIModelVersion `json:"modelVersions"`
}

type civitAIModelVersion struct {
	ID            int64          `json:"id"`
	ModelID       int64          `json:"modelId"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	CreatedAt     string         `json:"createdAt"`
	DownloadURL   string         `json:"downloadUrl"`
	TrainedWords  []string       `json:"trainedWords"`
	BaseModel     string         `json:"baseModel"`
	BaseModelType string         `json:"baseModelType"`
	Files         []civitAIFile  `json:"files"`
	Images        []civitAIImage `json:"images"`
}

type civitAIFile struct {
	Name     string              `json:"name"`
	ID       int64               `json:"id"`
	SizeKB   int64               `json:"sizeKB"`
	Type     string              `json:"type"`
	Format   string              `json:"format"`
	Metadata civitAIFileMetadata `json:"metadata"`
}

type civitAIFileMetadata struct {
	Format string `json:"format"` // e.g., "SafeTensor"
	FP     string `json:"fp"`     // e.g., "fp16", "fp32"
	Size   string `json:"size"`   // e.g., "full", "pruned"
}

type civitAIImage struct {
	URL    string `json:"url"`
	NSFW   string `json:"nsfw"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Hash   string `json:"hash"`
}

// mapCivitAIType maps CivitAI model types to our internal types
func mapCivitAIType(civitaiType string) string {
	civitaiType = strings.ToLower(civitaiType)

	switch civitaiType {
	case "checkpoint":
		return "Checkpoint"
	case "lora":
		return "LoRA"
	case "lycoris":
		return "LoRA" // LyCORIS is similar to LoRA
	case "textualinversion", "textual inversion":
		return "Embedding"
	case "hypernetwork":
		return "Hypernetwork"
	case "aestheticgradient":
		return "AestheticGradient"
	case "vae":
		return "VAE"
	case "controlnet":
		return "ControlNet"
	case "poses":
		return "Pose"
	case "wildcards":
		return "Wildcard"
	default:
		return "Unknown"
	}
}

// CheckConnectivity tests if CivitAI is accessible (useful for VPN detection)
func (f *CivitAIFetcher) CheckConnectivity(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://civitai.com", nil)
	if err != nil {
		return fmt.Errorf("creating connectivity check request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "no such host") ||
			strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "timeout") {
			return fmt.Errorf("CivitAI unreachable - VPN may be required (UK restriction)")
		}
		return fmt.Errorf("connectivity check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("CivitAI returned status %d - access may be restricted", resp.StatusCode)
	}

	return nil
}

// ValidateAPIKey checks if the provided API key is valid
func (f *CivitAIFetcher) ValidateAPIKey(ctx context.Context, apiKey string) error {
	// Try fetching a known model with the API key
	req, err := http.NewRequestWithContext(ctx, "GET", "https://civitai.com/api/v1/models", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("invalid API key")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// ParseModelIDFromURL extracts the model ID from a CivitAI URL
func ParseCivitAIModelID(url string) (string, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`civitai\.com/models/(\d+)`),
		regexp.MustCompile(`civitai\.com/api/download/models/(\d+)`),
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(url); len(matches) >= 2 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("could not extract model ID from URL")
}

// GetModelVersionID extracts version ID from download URLs like /api/download/models/{versionId}
func ParseCivitAIVersionID(url string) (int64, error) {
	pattern := regexp.MustCompile(`civitai\.com/api/download/models/(\d+)`)
	matches := pattern.FindStringSubmatch(url)
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not extract version ID from URL")
	}

	id, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid version ID: %w", err)
	}

	return id, nil
}
