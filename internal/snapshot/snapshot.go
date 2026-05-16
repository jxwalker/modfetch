package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/batch"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/util"
)

const maxTreeResponseBytes = 16 * 1024 * 1024

// HuggingFaceBaseURL is overridden by tests.
var HuggingFaceBaseURL = "https://huggingface.co"

var getenv = os.Getenv

type Options struct {
	Rev      string
	Includes []string
	Excludes []string
	DestDir  string
	MaxFiles int
}

type Manifest struct {
	Version int    `json:"version"`
	Source  string `json:"source"`
	Repo    string `json:"repo"`
	Rev     string `json:"rev"`
	Prefix  string `json:"prefix,omitempty"`
	Files   []File `json:"files"`
}

type File struct {
	Path string `json:"path"`
	Size int64  `json:"size,omitempty"`
	Type string `json:"type,omitempty"`
	URI  string `json:"uri"`
	Dest string `json:"dest,omitempty"`
}

type hfTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

func BuildHuggingFace(ctx context.Context, cfg *config.Config, rawURI string, opts Options) (*Manifest, error) {
	repoID, prefix, rev, err := parseHuggingFaceURI(rawURI, opts.Rev)
	if err != nil {
		return nil, err
	}
	includes := cleanPatterns(opts.Includes)
	excludes := cleanPatterns(opts.Excludes)
	if err := validatePatterns("--include", includes); err != nil {
		return nil, err
	}
	if err := validatePatterns("--exclude", excludes); err != nil {
		return nil, err
	}
	tree, err := fetchHuggingFaceTree(ctx, cfg, repoID, rev)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(tree))
	for _, item := range tree {
		if item.Type != "file" {
			continue
		}
		if !pathHasPrefix(item.Path, prefix) {
			continue
		}
		if !includedByPatterns(item.Path, includes) || excludedByPatterns(item.Path, excludes) {
			continue
		}
		files = append(files, File{
			Path: item.Path,
			Size: item.Size,
			Type: artifactType(item.Path),
			URI:  "hf://" + repoID + "/" + item.Path + "?rev=" + url.QueryEscape(rev),
			Dest: snapshotDest(opts.DestDir, repoID, item.Path),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if len(files) == 0 {
		return nil, fmt.Errorf("snapshot matched no files for %s", rawURI)
	}
	if opts.MaxFiles > 0 && len(files) > opts.MaxFiles {
		return nil, fmt.Errorf("snapshot matched %d files, above --max-files %d; narrow with --include/--exclude or raise the limit", len(files), opts.MaxFiles)
	}
	return &Manifest{
		Version: 1,
		Source:  "huggingface",
		Repo:    repoID,
		Rev:     rev,
		Prefix:  prefix,
		Files:   files,
	}, nil
}

func (m *Manifest) Batch() *batch.File {
	out := &batch.File{Version: 1}
	if m == nil {
		return out
	}
	for _, file := range m.Files {
		out.Jobs = append(out.Jobs, batch.BatchJob{
			URI:  file.URI,
			Dest: file.Dest,
			Type: file.Type,
		})
	}
	return out
}

func (m *Manifest) JSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func parseHuggingFaceURI(rawURI, revOverride string) (repoID, prefix, rev string, err error) {
	rawURI = strings.TrimSpace(rawURI)
	if !strings.HasPrefix(rawURI, "hf://") {
		return "", "", "", fmt.Errorf("snapshot currently supports Hugging Face URIs like hf://owner/repo[/path], got %q", rawURI)
	}
	body := strings.TrimPrefix(rawURI, "hf://")
	rawPath := body
	rawQuery := ""
	if i := strings.IndexByte(body, '?'); i >= 0 {
		rawPath = body[:i]
		rawQuery = body[i+1:]
	}
	parts := splitPath(rawPath)
	if len(parts) == 0 {
		return "", "", "", errors.New("hf snapshot URI must include a repo")
	}
	if len(parts) == 1 {
		repoID = parts[0]
	} else if len(parts) == 2 && looksLikeSnapshotRootFile(parts[1]) {
		repoID = parts[0]
		prefix = parts[1]
	} else {
		repoID = parts[0] + "/" + parts[1]
		if len(parts) > 2 {
			prefix = path.Join(parts[2:]...)
		}
	}
	rev = "main"
	if rawQuery != "" {
		values, _ := url.ParseQuery(rawQuery)
		if v := strings.TrimSpace(values.Get("rev")); v != "" {
			rev = v
		}
	}
	if v := strings.TrimSpace(revOverride); v != "" {
		rev = v
	}
	return repoID, prefix, rev, nil
}

func fetchHuggingFaceTree(ctx context.Context, cfg *config.Config, repoID, rev string) ([]hfTreeEntry, error) {
	apiURL := strings.TrimRight(HuggingFaceBaseURL, "/") + "/api/models/" + repoID + "/tree/" + url.PathEscape(rev) + "?recursive=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	if cfg != nil && cfg.Sources.HuggingFace.Enabled {
		if env := strings.TrimSpace(cfg.Sources.HuggingFace.TokenEnv); env != "" {
			if token := strings.TrimSpace(getenv(env)); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("huggingface tree request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("huggingface tree returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	limited := io.LimitReader(resp.Body, maxTreeResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(body) > maxTreeResponseBytes {
		return nil, fmt.Errorf("huggingface tree response exceeded %d bytes", maxTreeResponseBytes)
	}
	var tree []hfTreeEntry
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, fmt.Errorf("decode huggingface tree: %w", err)
	}
	return tree, nil
}

func splitPath(value string) []string {
	raw := strings.Split(value, "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func pathHasPrefix(candidate, prefix string) bool {
	candidate = strings.Trim(candidate, "/")
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return true
	}
	return candidate == prefix || strings.HasPrefix(candidate, prefix+"/")
}

func includedByPatterns(candidate string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if matchPattern(pattern, candidate) {
			return true
		}
	}
	return false
}

func excludedByPatterns(candidate string, patterns []string) bool {
	for _, pattern := range cleanPatterns(patterns) {
		if matchPattern(pattern, candidate) {
			return true
		}
	}
	return false
}

func cleanPatterns(patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			out = append(out, pattern)
		}
	}
	return out
}

func validatePatterns(flagName string, patterns []string) error {
	for _, pattern := range patterns {
		if _, err := path.Match(pattern, ""); err != nil {
			return fmt.Errorf("%s pattern %q is invalid: %w", flagName, pattern, err)
		}
	}
	return nil
}

func looksLikeSnapshotRootFile(segment string) bool {
	base := strings.ToLower(path.Base(strings.TrimSpace(segment)))
	switch base {
	case ".gitattributes", ".gitignore", "dockerfile", "license", "notice", "readme":
		return true
	}
	return path.Ext(base) != ""
}

func matchPattern(pattern, candidate string) bool {
	if ok, _ := path.Match(pattern, candidate); ok {
		return true
	}
	if !strings.Contains(pattern, "/") {
		ok, _ := path.Match(pattern, path.Base(candidate))
		return ok
	}
	if strings.HasPrefix(pattern, "**/") {
		return matchPattern(strings.TrimPrefix(pattern, "**/"), candidate)
	}
	return false
}

func snapshotDest(destDir, repoID, repoPath string) string {
	destDir = strings.TrimSpace(destDir)
	if destDir == "" {
		return ""
	}
	repoDir := util.SafeFileName(strings.ReplaceAll(repoID, "/", "__"))
	return filepath.Join(destDir, repoDir, safeRelativePath(repoPath))
}

func safeRelativePath(repoPath string) string {
	parts := splitPath(repoPath)
	safe := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "." || part == ".." {
			continue
		}
		if clean := util.SafeFileName(part); clean != "" {
			safe = append(safe, clean)
		}
	}
	if len(safe) == 0 {
		return util.SafeFileName(path.Base(repoPath))
	}
	return filepath.Join(safe...)
}

func artifactType(repoPath string) string {
	switch strings.ToLower(path.Ext(repoPath)) {
	case ".gguf":
		return "gguf"
	case ".safetensors":
		return "safetensors"
	case ".onnx":
		return "onnx"
	case ".bin", ".pt", ".pth":
		return "checkpoint"
	case ".json", ".txt", ".model", ".vocab", ".merges":
		return "metadata"
	default:
		return ""
	}
}
