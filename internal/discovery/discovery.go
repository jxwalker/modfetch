package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	ProviderHuggingFace = "huggingface"
	ProviderCivitAI     = "civitai"
	ProviderModelScope  = "modelscope"
	ProviderAll         = "all"
)

var (
	huggingFaceBaseURL = "https://huggingface.co"
	civitaiBaseURL     = "https://civitai.com"
	modelScopeBaseURL  = "https://modelscope.cn"
)

type Options struct {
	Provider string
	Query    string
	Limit    int
}

type Result struct {
	Index       int      `json:"index"`
	Provider    string   `json:"provider"`
	ModelID     string   `json:"model_id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Pipeline    string   `json:"pipeline,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Downloads   int64    `json:"downloads,omitempty"`
	Likes       int64    `json:"likes,omitempty"`
	VersionID   string   `json:"version_id,omitempty"`
	FilePath    string   `json:"file_path,omitempty"`
	FileName    string   `json:"file_name,omitempty"`
	FileType    string   `json:"file_type,omitempty"`
	Size        int64    `json:"size,omitempty"`
	URI         string   `json:"uri"`
}

type hfModel struct {
	ID          string   `json:"id"`
	ModelID     string   `json:"modelId"`
	Downloads   int64    `json:"downloads"`
	Likes       int64    `json:"likes"`
	Tags        []string `json:"tags"`
	PipelineTag string   `json:"pipeline_tag"`
}

type hfRepoFile struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type civitaiSearchResponse struct {
	Items []civitaiModel `json:"items"`
}

type civitaiModel struct {
	ID            int              `json:"id"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	Type          string           `json:"type"`
	Tags          []string         `json:"tags"`
	ModelVersions []civitaiVersion `json:"modelVersions"`
	Stats         struct {
		DownloadCount int64 `json:"downloadCount"`
		ThumbsUpCount int64 `json:"thumbsUpCount"`
	} `json:"stats"`
}

type civitaiVersion struct {
	ID    int           `json:"id"`
	Name  string        `json:"name"`
	Files []civitaiFile `json:"files"`
}

type civitaiFile struct {
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	SizeKB float64 `json:"sizeKB"`
}

func Search(ctx context.Context, opts Options) ([]Result, error) {
	opts.Provider = normalizeProvider(opts.Provider)
	opts.Query = strings.TrimSpace(opts.Query)
	if opts.Query == "" {
		return nil, errors.New("discover query is required")
	}
	if opts.Limit <= 0 {
		opts.Limit = 5
	}
	if opts.Limit > 25 {
		opts.Limit = 25
	}
	if opts.Provider != ProviderHuggingFace && opts.Provider != ProviderCivitAI && opts.Provider != ProviderModelScope && opts.Provider != ProviderAll {
		return nil, fmt.Errorf("unknown discovery provider %q", opts.Provider)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var (
		results []Result
		errs    []error
	)
	if opts.Provider == ProviderHuggingFace || opts.Provider == ProviderAll {
		hf, err := searchHuggingFace(ctx, client, opts.Query, opts.Limit)
		if err != nil {
			errs = append(errs, err)
		}
		results = append(results, hf...)
	}
	if opts.Provider == ProviderCivitAI || opts.Provider == ProviderAll {
		civ, err := searchCivitAI(ctx, client, opts.Query, opts.Limit)
		if err != nil {
			errs = append(errs, err)
		}
		results = append(results, civ...)
	}
	if opts.Provider == ProviderModelScope || opts.Provider == ProviderAll {
		ms, err := searchModelScope(ctx, client, opts.Query, opts.Limit)
		if err != nil {
			errs = append(errs, err)
		}
		results = append(results, ms...)
	}
	if len(results) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Provider != results[j].Provider {
			return results[i].Provider < results[j].Provider
		}
		if results[i].Downloads != results[j].Downloads {
			return results[i].Downloads > results[j].Downloads
		}
		return results[i].Likes > results[j].Likes
	})
	if len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	for i := range results {
		results[i].Index = i + 1
	}
	return results, nil
}

func normalizeProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "hf", "huggingface", "hugging-face":
		return ProviderHuggingFace
	case "civ", "civitai", "civit-ai":
		return ProviderCivitAI
	case "ms", "modelscope", "model-scope":
		return ProviderModelScope
	case "all":
		return ProviderAll
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

func searchHuggingFace(ctx context.Context, client *http.Client, query string, limit int) ([]Result, error) {
	u, err := url.Parse(huggingFaceBaseURL + "/api/models")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("search", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("sort", "downloads")
	q.Set("direction", "-1")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	var models []hfModel
	if err := doJSON(client, req, &models); err != nil {
		return nil, fmt.Errorf("huggingface search: %w", err)
	}
	out := make([]Result, 0, len(models))
	for _, model := range models {
		modelID := strings.TrimSpace(model.ModelID)
		if modelID == "" {
			modelID = strings.TrimSpace(model.ID)
		}
		if modelID == "" {
			continue
		}
		candidate, _ := bestHFFile(ctx, client, modelID)
		result := Result{
			Provider:  ProviderHuggingFace,
			ModelID:   modelID,
			Name:      modelID,
			Pipeline:  model.PipelineTag,
			Tags:      model.Tags,
			Downloads: model.Downloads,
			Likes:     model.Likes,
			URI:       "hf://" + modelID + "?rev=main",
		}
		if candidate.Path != "" {
			result.FilePath = candidate.Path
			result.FileName = path.Base(candidate.Path)
			result.FileType = strings.TrimPrefix(strings.ToLower(path.Ext(candidate.Path)), ".")
			result.Size = candidate.Size
			result.URI = "hf://" + modelID + "/" + candidate.Path + "?rev=main"
		}
		out = append(out, result)
	}
	return out, nil
}

func bestHFFile(ctx context.Context, client *http.Client, modelID string) (hfRepoFile, error) {
	u := strings.TrimRight(huggingFaceBaseURL, "/") + "/api/models/" + modelID + "/tree/main?recursive=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return hfRepoFile{}, err
	}
	var files []hfRepoFile
	if err := doJSON(client, req, &files); err != nil {
		return hfRepoFile{}, err
	}
	var best hfRepoFile
	bestScore := -1
	for _, file := range files {
		if file.Type != "file" {
			continue
		}
		score := hfFileScore(file.Path)
		if score < 0 {
			continue
		}
		if score > bestScore || (score == bestScore && preferSmallerNonZero(file.Size, best.Size)) {
			best = file
			bestScore = score
		}
	}
	if best.Path == "" {
		return hfRepoFile{}, errors.New("no downloadable model file found")
	}
	return best, nil
}

func hfFileScore(filePath string) int {
	name := strings.ToLower(path.Base(filePath))
	ext := strings.ToLower(path.Ext(name))
	var score int
	switch ext {
	case ".gguf":
		score = 100
	case ".safetensors":
		score = 90
	case ".bin":
		score = 80
	case ".pt", ".pth", ".ckpt":
		score = 70
	case ".onnx":
		score = 50
	default:
		return -1
	}
	switch {
	case strings.Contains(name, "q4_k_m"):
		score += 12
	case strings.Contains(name, "q5_k_m"):
		score += 10
	case strings.Contains(name, "q4"):
		score += 8
	case strings.Contains(name, "fp16") || strings.Contains(name, "f16"):
		score += 5
	}
	if strings.Contains(name, "optimizer") || strings.Contains(name, "training_args") {
		score -= 100
	}
	return score
}

func searchCivitAI(ctx context.Context, client *http.Client, query string, limit int) ([]Result, error) {
	u, err := url.Parse(civitaiBaseURL + "/api/v1/models")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("query", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("sort", "Most Downloaded")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	var payload civitaiSearchResponse
	if err := doJSON(client, req, &payload); err != nil {
		return nil, fmt.Errorf("civitai search: %w", err)
	}
	out := make([]Result, 0, len(payload.Items))
	for _, model := range payload.Items {
		if model.ID == 0 {
			continue
		}
		version, file := bestCivitAIFile(model.ModelVersions)
		uri := fmt.Sprintf("civitai://model/%d", model.ID)
		if version.ID != 0 {
			uri += "?version=" + url.QueryEscape(fmt.Sprintf("%d", version.ID))
			if strings.TrimSpace(file.Name) != "" {
				uri += "&file=" + url.QueryEscape(file.Name)
			}
		}
		out = append(out, Result{
			Provider:    ProviderCivitAI,
			ModelID:     fmt.Sprintf("%d", model.ID),
			Name:        strings.TrimSpace(model.Name),
			Description: strings.TrimSpace(model.Type),
			Tags:        model.Tags,
			Downloads:   model.Stats.DownloadCount,
			Likes:       model.Stats.ThumbsUpCount,
			VersionID:   zeroIntString(version.ID),
			FileName:    file.Name,
			FileType:    file.Type,
			Size:        int64(file.SizeKB * 1024),
			URI:         uri,
		})
	}
	return out, nil
}

func bestCivitAIFile(versions []civitaiVersion) (civitaiVersion, civitaiFile) {
	var version civitaiVersion
	for _, candidate := range versions {
		if candidate.ID > version.ID {
			version = candidate
		}
	}
	var best civitaiFile
	for _, file := range version.Files {
		if strings.EqualFold(file.Type, "Model") {
			return version, file
		}
		if best.Name == "" {
			best = file
		}
	}
	return version, best
}

// modelScopeSearchResponse is the envelope returned by the ModelScope model list API.
type modelScopeSearchResponse struct {
	Code    int    `json:"Code"`
	Success bool   `json:"Success"`
	Message string `json:"Message"`
	Data    struct {
		Models []modelScopeSearchModel `json:"Models"`
	} `json:"Data"`
}

type modelScopeSearchModel struct {
	ID        string   `json:"Id"`
	Name      string   `json:"Name"`
	ChName    string   `json:"ChineseName"`
	Owner     string   `json:"Owner"`
	Downloads int64    `json:"Downloads"`
	Likes     int64    `json:"Likes"`
	Tags      []string `json:"Tags"`
	Type      string   `json:"ModelType"`
}

func searchModelScope(ctx context.Context, client *http.Client, query string, limit int) ([]Result, error) {
	u, err := url.Parse(modelScopeBaseURL + "/api/v1/models")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("Name", query)
	q.Set("page_size", fmt.Sprintf("%d", limit))
	q.Set("Status", "1")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	var payload modelScopeSearchResponse
	if err := doJSON(client, req, &payload); err != nil {
		return nil, fmt.Errorf("modelscope search: %w", err)
	}
	if !payload.Success {
		return nil, fmt.Errorf("modelscope search: %s", payload.Message)
	}
	out := make([]Result, 0, len(payload.Data.Models))
	for _, model := range payload.Data.Models {
		owner := strings.TrimSpace(model.Owner)
		name := strings.TrimSpace(model.Name)
		if owner == "" || name == "" {
			continue
		}
		modelID := owner + "/" + name
		displayName := name
		if ch := strings.TrimSpace(model.ChName); ch != "" {
			displayName = ch
		}
		uri := modelScopeBaseURL + "/models/" + owner + "/" + name
		out = append(out, Result{
			Provider:  ProviderModelScope,
			ModelID:   modelID,
			Name:      displayName,
			Tags:      model.Tags,
			Downloads: model.Downloads,
			Likes:     model.Likes,
			URI:       uri,
		})
	}
	return out, nil
}

func doJSON(client *http.Client, req *http.Request, dst any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func preferSmallerNonZero(a, b int64) bool {
	if a <= 0 {
		return false
	}
	if b <= 0 {
		return true
	}
	return a < b
}

func zeroIntString(v int) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%d", v)
}
