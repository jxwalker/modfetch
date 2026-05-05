package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

// maxDiscoveryBodyBytes caps the size of any response body we will buffer
// before JSON decoding. A misbehaving server can stream arbitrarily large
// payloads, so we enforce a hard ceiling regardless of the API's `limit`
// parameter to keep memory predictable.
const maxDiscoveryBodyBytes = 8 * 1024 * 1024

// errDiscoveryResponseTooLarge is returned when a provider response exceeds
// maxDiscoveryBodyBytes. Callers wrap it with provider-specific context.
var errDiscoveryResponseTooLarge = errors.New("response body exceeded maximum size")

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

// modelScopeFilesResponse is the envelope returned by the ModelScope file
// listing endpoint (`/api/v1/models/{owner}/{name}/repo/files`).
type modelScopeFilesResponse struct {
	Code    int    `json:"Code"`
	Success bool   `json:"Success"`
	Message string `json:"Message"`
	Data    struct {
		Files []modelScopeFile `json:"Files"`
	} `json:"Data"`
}

type modelScopeFile struct {
	Type     string `json:"Type"`
	Path     string `json:"Path"`
	Name     string `json:"Name"`
	Size     int64  `json:"Size"`
	Revision string `json:"Revision"`
}

const modelScopeDefaultRevision = "master"

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
	if err := checkModelScopeEnvelope(payload.Success, payload.Code, payload.Message); err != nil {
		return nil, fmt.Errorf("modelscope search: %w", err)
	}
	out := make([]Result, 0, len(payload.Data.Models))
	var fileErrs []error
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

		// Resolve a concrete downloadable file so callers that pass Result.URI
		// to `download --url` get a real artifact. If the file listing fails or
		// returns nothing usable (private repo, transient error, repo without
		// downloadable weights, etc.) skip the result entirely: the model page
		// URL is HTML and would silently produce a downloaded HTML page rather
		// than a model file. Oversized responses short-circuit the whole
		// search; other per-model fetch errors are collected so that if every
		// model fails, the user sees a real provider error instead of an
		// empty-looking success.
		fileURI, filePath, fileSize, fileErr := bestModelScopeFile(ctx, client, owner, name, modelScopeDefaultRevision)
		if errors.Is(fileErr, errDiscoveryResponseTooLarge) {
			return nil, fmt.Errorf("modelscope file listing for %s: %w", modelID, fileErr)
		}
		if fileErr != nil {
			fileErrs = append(fileErrs, fmt.Errorf("modelscope file listing for %s: %w", modelID, fileErr))
			continue
		}
		if fileURI == "" || filePath == "" {
			continue
		}

		result := Result{
			Provider:  ProviderModelScope,
			ModelID:   modelID,
			Name:      displayName,
			Tags:      model.Tags,
			Downloads: model.Downloads,
			Likes:     model.Likes,
			URI:       fileURI,
			FilePath:  filePath,
			FileName:  path.Base(filePath),
			FileType:  strings.TrimPrefix(strings.ToLower(path.Ext(filePath)), "."),
			Size:      fileSize,
		}
		out = append(out, result)
	}
	if len(out) == 0 && len(fileErrs) > 0 {
		return nil, errors.Join(fileErrs...)
	}
	return out, nil
}

// checkModelScopeEnvelope validates the common Success/Code envelope returned
// by ModelScope APIs, formatting a useful error when Message is blank.
func checkModelScopeEnvelope(success bool, code int, message string) error {
	if success && (code == 0 || code == http.StatusOK) {
		return nil
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "unknown error"
	}
	if code != 0 {
		return fmt.Errorf("code=%d %s", code, msg)
	}
	return fmt.Errorf("%s", msg)
}

// bestModelScopeFile fetches the repository file list and selects a single
// downloadable file using the same scoring heuristic as HuggingFace. It
// returns a fully-formed `/models/<owner>/<name>/resolve/<rev>/<path>` URL on
// success. The error is non-nil only for fetch-level failures (network,
// oversized response, malformed envelope); a successful fetch with no usable
// file returns empty strings and a nil error so the caller can decide whether
// to skip the model.
func bestModelScopeFile(ctx context.Context, client *http.Client, owner, name, revision string) (uri, filePath string, size int64, err error) {
	if revision == "" {
		revision = modelScopeDefaultRevision
	}
	u, parseErr := url.Parse(strings.TrimRight(modelScopeBaseURL, "/") + "/api/v1/models/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/repo/files")
	if parseErr != nil {
		return "", "", 0, parseErr
	}
	q := u.Query()
	q.Set("Revision", revision)
	q.Set("Recursive", "true")
	q.Set("PageSize", "200")
	u.RawQuery = q.Encode()
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if reqErr != nil {
		return "", "", 0, reqErr
	}
	var payload modelScopeFilesResponse
	if jerr := doJSON(client, req, &payload); jerr != nil {
		return "", "", 0, jerr
	}
	if envErr := checkModelScopeEnvelope(payload.Success, payload.Code, payload.Message); envErr != nil {
		return "", "", 0, envErr
	}

	bestScore := -1
	var best modelScopeFile
	for _, file := range payload.Data.Files {
		if !isModelScopeBlob(file.Type) {
			continue
		}
		p := strings.TrimSpace(file.Path)
		if p == "" {
			p = strings.TrimSpace(file.Name)
		}
		if p == "" {
			continue
		}
		score := hfFileScore(p)
		if score < 0 {
			continue
		}
		if score > bestScore || (score == bestScore && preferSmallerNonZero(file.Size, best.Size)) {
			best = file
			best.Path = p
			bestScore = score
		}
	}
	if best.Path == "" {
		return "", "", 0, nil
	}
	uri = strings.TrimRight(modelScopeBaseURL, "/") + "/models/" +
		url.PathEscape(owner) + "/" + url.PathEscape(name) +
		"/resolve/" + url.PathEscape(revision) + "/" + escapePathSegments(best.Path)
	return uri, best.Path, best.Size, nil
}

// isModelScopeBlob accepts the variants ModelScope's API has used to label a
// file (vs a directory): "blob", "file", or empty (some responses omit Type).
func isModelScopeBlob(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "tree", "directory", "dir":
		return false
	default:
		return true
	}
}

// escapePathSegments escapes each '/'-separated segment of a path while
// preserving the slashes themselves, so the result is safe to embed as a
// URL path without breaking the directory structure.
func escapePathSegments(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
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
	// Read at most maxDiscoveryBodyBytes+1 so we can detect oversized responses
	// explicitly rather than silently truncating them and confusing JSON decoding.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDiscoveryBodyBytes+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > maxDiscoveryBodyBytes {
		return errDiscoveryResponseTooLarge
	}
	return json.Unmarshal(body, dst)
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
