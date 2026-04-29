package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/state"
)

type ModelScopeFetcher struct {
	client *http.Client
}

func NewModelScopeFetcher(client *http.Client) *ModelScopeFetcher {
	return &ModelScopeFetcher{client: client}
}

func (f *ModelScopeFetcher) Source() string {
	return "modelscope"
}

func (f *ModelScopeFetcher) CanHandle(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "modelscope.cn" || strings.HasSuffix(host, ".modelscope.cn")
}

func (f *ModelScopeFetcher) FetchMetadata(ctx context.Context, rawURL string) (*state.ModelMetadata, error) {
	modelID, owner, repo, revision, filename, err := parseModelScopeURL(rawURL)
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("https://modelscope.cn/api/v1/models/%s", modelID)
	if revision != "" {
		apiURL += "?Revision=" + url.QueryEscape(revision)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return f.basicMetadata(rawURL, modelID, owner, repo, revision, filename), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return f.basicMetadata(rawURL, modelID, owner, repo, revision, filename), nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return f.basicMetadata(rawURL, modelID, owner, repo, revision, filename), nil
	}

	var apiResp modelScopeModelResponse
	if err := json.Unmarshal(body, &apiResp); err != nil || !apiResp.Success || apiResp.Code != http.StatusOK {
		return f.basicMetadata(rawURL, modelID, owner, repo, revision, filename), nil
	}

	data := apiResp.Data
	meta := &state.ModelMetadata{
		DownloadURL:   rawURL,
		ModelName:     firstNonEmpty(mapString(data, "Name"), mapString(data, "ModelName"), repo),
		ModelID:       firstNonEmpty(mapString(data, "Id"), mapString(data, "ModelId"), modelID),
		Version:       revision,
		Source:        "modelscope",
		Description:   truncateString(mapString(data, "Description"), 5000),
		Author:        firstNonEmpty(mapString(data, "Owner"), mapString(data, "Author"), owner),
		AuthorURL:     fmt.Sprintf("https://modelscope.cn/profile/%s", owner),
		License:       mapString(data, "License"),
		Tags:          mapStringSlice(data, "Tags"),
		RepoURL:       fmt.Sprintf("https://modelscope.cn/models/%s", modelID),
		HomepageURL:   fmt.Sprintf("https://modelscope.cn/models/%s", modelID),
		ThumbnailURL:  firstNonEmpty(mapString(data, "ModelCover"), mapString(data, "Cover")),
		DownloadCount: mapInt(data, "Downloads"),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if meta.ModelName == "" {
		meta.ModelName = repo
	}
	meta.ModelType = inferModelType(filename, meta.Tags)
	if meta.ModelType == "Unknown" {
		meta.ModelType = modelScopeTaskType(data)
	}
	meta.Quantization = ExtractQuantization(filename)
	if filename != "" {
		meta.FileFormat = strings.ToLower(filepath.Ext(filename))
	}
	return meta, nil
}

func (f *ModelScopeFetcher) basicMetadata(rawURL, modelID, owner, repo, revision, filename string) *state.ModelMetadata {
	return &state.ModelMetadata{
		DownloadURL:  rawURL,
		ModelName:    repo,
		ModelID:      modelID,
		Version:      revision,
		Source:       "modelscope",
		Author:       owner,
		AuthorURL:    fmt.Sprintf("https://modelscope.cn/profile/%s", owner),
		RepoURL:      fmt.Sprintf("https://modelscope.cn/models/%s", modelID),
		HomepageURL:  fmt.Sprintf("https://modelscope.cn/models/%s", modelID),
		ModelType:    inferModelType(filename, nil),
		Quantization: ExtractQuantization(filename),
		FileFormat:   strings.ToLower(filepath.Ext(filename)),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

type modelScopeModelResponse struct {
	Code    int            `json:"Code"`
	Success bool           `json:"Success"`
	Message string         `json:"Message"`
	Data    map[string]any `json:"Data"`
}

func parseModelScopeURL(rawURL string) (modelID, owner, repo, revision, filename string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("parse ModelScope URL: %w", err)
	}
	parts := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "models" {
			continue
		}
		owner, err = url.PathUnescape(parts[i+1])
		if err != nil {
			return "", "", "", "", "", fmt.Errorf("decode ModelScope owner: %w", err)
		}
		repo, err = url.PathUnescape(parts[i+2])
		if err != nil {
			return "", "", "", "", "", fmt.Errorf("decode ModelScope repo: %w", err)
		}
		if i+5 < len(parts) && parts[i+3] == "resolve" {
			revision, err = url.PathUnescape(parts[i+4])
			if err != nil {
				return "", "", "", "", "", fmt.Errorf("decode ModelScope revision: %w", err)
			}
			filename, err = url.PathUnescape(strings.Join(parts[i+5:], "/"))
			if err != nil {
				return "", "", "", "", "", fmt.Errorf("decode ModelScope filename: %w", err)
			}
		}
		return owner + "/" + repo, owner, repo, revision, filename, nil
	}
	return "", "", "", "", "", fmt.Errorf("invalid ModelScope URL format")
}

func mapString(data map[string]any, key string) string {
	if value, ok := data[key]; ok {
		if s, ok := value.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func mapStringSlice(data map[string]any, key string) []string {
	value, ok := data[key]
	if !ok {
		return nil
	}
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
		return out
	default:
		return nil
	}
}

func mapInt(data map[string]any, key string) int {
	switch v := data[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func modelScopeTaskType(data map[string]any) string {
	tasks := mapStringSlice(data, "Tasks")
	for _, task := range tasks {
		task = strings.ToLower(task)
		if strings.Contains(task, "text-generation") || strings.Contains(task, "chat") || strings.Contains(task, "llm") {
			return "LLM"
		}
		if strings.Contains(task, "text-to-image") || strings.Contains(task, "image-generation") {
			return "Checkpoint"
		}
		if strings.Contains(task, "embedding") {
			return "Embedding"
		}
	}
	return "Unknown"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
