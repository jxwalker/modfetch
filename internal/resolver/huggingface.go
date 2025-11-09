package resolver

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
	"regexp"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/util"
)

type HuggingFace struct{}

// hfRepoFile represents a file in a HuggingFace repository.
type hfRepoFile struct {
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "directory"
	Size int64  `json:"size"`
	Oid  string `json:"oid"` // Git object ID
}

// Quantization patterns for common model quantizations
var quantizationPatterns = map[string]*regexp.Regexp{
	// GGUF/GGML quantizations
	"Q2_K":   regexp.MustCompile(`(?i)[._-]q2[._-]k[._-]?`),
	"Q3_K_S": regexp.MustCompile(`(?i)[._-]q3[._-]k[._-]s[._-]?`),
	"Q3_K_M": regexp.MustCompile(`(?i)[._-]q3[._-]k[._-]m[._-]?`),
	"Q3_K_L": regexp.MustCompile(`(?i)[._-]q3[._-]k[._-]l[._-]?`),
	"Q4_0":   regexp.MustCompile(`(?i)[._-]q4[._-]0[._-]?`),
	"Q4_1":   regexp.MustCompile(`(?i)[._-]q4[._-]1[._-]?`),
	"Q4_K_S": regexp.MustCompile(`(?i)[._-]q4[._-]k[._-]s[._-]?`),
	"Q4_K_M": regexp.MustCompile(`(?i)[._-]q4[._-]k[._-]m[._-]?`),
	"Q5_0":   regexp.MustCompile(`(?i)[._-]q5[._-]0[._-]?`),
	"Q5_1":   regexp.MustCompile(`(?i)[._-]q5[._-]1[._-]?`),
	"Q5_K_S": regexp.MustCompile(`(?i)[._-]q5[._-]k[._-]s[._-]?`),
	"Q5_K_M": regexp.MustCompile(`(?i)[._-]q5[._-]k[._-]m[._-]?`),
	"Q6_K":   regexp.MustCompile(`(?i)[._-]q6[._-]k[._-]?`),
	"Q8_0":   regexp.MustCompile(`(?i)[._-]q8[._-]0[._-]?`),
	"F16":    regexp.MustCompile(`(?i)[._-]f16[._-]?`),
	"F32":    regexp.MustCompile(`(?i)[._-]f32[._-]?`),
	// Safetensors/PyTorch quantizations
	"fp16": regexp.MustCompile(`(?i)[._-]fp16[._-]?`),
	"fp32": regexp.MustCompile(`(?i)[._-]fp32[._-]?`),
	"bf16": regexp.MustCompile(`(?i)[._-]bf16[._-]?`),
	"4bit": regexp.MustCompile(`(?i)[._-]4bit[._-]?`),
	"8bit": regexp.MustCompile(`(?i)[._-]8bit[._-]?`),
	"AWQ":  regexp.MustCompile(`(?i)[._-]awq[._-]?`),
	"GPTQ": regexp.MustCompile(`(?i)[._-]gptq[._-]?`),
}

// Preferred quantization order (best quality/size tradeoff first)
var preferredQuantizations = []string{
	"Q4_K_M", "Q5_K_M", "Q4_0", "Q5_0", "fp16", "Q6_K", "Q8_0", "F16",
	"Q4_K_S", "Q5_K_S", "Q3_K_M", "4bit", "8bit", "GPTQ", "AWQ",
	"Q3_K_L", "Q3_K_S", "Q2_K", "bf16", "fp32", "F32",
}

// Accepts URIs of the form:
//
//	hf://{owner}/{repo}/{path}?rev=main&quant={quantization}
//
// If rev is omitted, defaults to "main".
// If quant is specified, selects that specific quantization variant.
// If path points to a directory or is omitted, will list and detect quantizations.
func (h *HuggingFace) CanHandle(u string) bool { return strings.HasPrefix(u, "hf://") }

// listRepoFiles fetches the file tree from HuggingFace API.
func (h *HuggingFace) listRepoFiles(ctx context.Context, repoID, rev string, headers map[string]string) ([]hfRepoFile, error) {
	apiURL := fmt.Sprintf("https://huggingface.co/api/models/%s/tree/%s?recursive=true", repoID, rev)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add authorization header if provided
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hf api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Limit error body read to 1KB to prevent memory issues
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("hf api returned %d: %s", resp.StatusCode, string(body))
	}

	var files []hfRepoFile
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("decode hf api response: %w", err)
	}

	return files, nil
}

// detectQuantization attempts to detect quantization type from filename.
// Returns the quantization name and true if detected, or empty string and false.
func detectQuantization(filename string) (string, bool) {
	// Try each pattern in preference order
	for _, quant := range preferredQuantizations {
		if pattern, ok := quantizationPatterns[quant]; ok {
			if pattern.MatchString(filename) {
				return quant, true
			}
		}
	}
	return "", false
}

// groupQuantizations groups files by their detected quantization.
// Returns a map of quantization name -> list of Quantization structs.
func groupQuantizations(files []hfRepoFile) map[string][]Quantization {
	groups := make(map[string][]Quantization)

	for _, f := range files {
		// Skip directories
		if f.Type != "file" {
			continue
		}

		// Only consider model files (common extensions)
		ext := strings.ToLower(filepath.Ext(f.Path))
		if ext != ".gguf" && ext != ".safetensors" && ext != ".bin" && ext != ".pt" && ext != ".pth" {
			continue
		}

		quant, detected := detectQuantization(f.Path)
		if !detected {
			// No quantization detected, use "full" or "default"
			quant = "default"
		}

		groups[quant] = append(groups[quant], Quantization{
			Name:     quant,
			FilePath: f.Path,
			Size:     f.Size,
			FileType: strings.TrimPrefix(ext, "."),
		})
	}

	return groups
}

// selectBestQuantization selects the best quantization from available options.
// Prefers based on preferredQuantizations order.
func selectBestQuantization(groups map[string][]Quantization, requestedQuant string) (Quantization, bool) {
	// If specific quantization requested, try to find it
	if requestedQuant != "" {
		if quants, ok := groups[requestedQuant]; ok && len(quants) > 0 {
			return quants[0], true
		}
		// Try case-insensitive match
		for quant, files := range groups {
			if strings.EqualFold(quant, requestedQuant) && len(files) > 0 {
				return files[0], true
			}
		}
		return Quantization{}, false
	}

	// Auto-select based on preference order
	for _, pref := range preferredQuantizations {
		if quants, ok := groups[pref]; ok && len(quants) > 0 {
			return quants[0], true
		}
	}

	// Fallback: pick first available (largest file if no quant detected)
	if quants, ok := groups["default"]; ok && len(quants) > 0 {
		// Pick largest file from default group
		best := quants[0]
		for _, q := range quants {
			if q.Size > best.Size {
				best = q
			}
		}
		return best, true
	}

	return Quantization{}, false
}

// flattenQuantizations converts grouped quantizations to a flat sorted list.
func flattenQuantizations(groups map[string][]Quantization) []Quantization {
	var all []Quantization

	// Add in preference order
	for _, pref := range preferredQuantizations {
		if quants, ok := groups[pref]; ok {
			all = append(all, quants...)
		}
	}

	// Add any remaining (not in preferred list)
	for quant, files := range groups {
		isPreferred := false
		for _, pref := range preferredQuantizations {
			if quant == pref {
				isPreferred = true
				break
			}
		}
		if !isPreferred {
			all = append(all, files...)
		}
	}

	return all
}

func (h *HuggingFace) Resolve(ctx context.Context, uri string, cfg *config.Config) (*Resolved, error) {
	if !h.CanHandle(uri) {
		return nil, errors.New("unsupported scheme")
	}
	s := strings.TrimPrefix(uri, "hf://")
	// Split off query
	var rawPath, rawQuery string
	if i := strings.IndexByte(s, '?'); i >= 0 {
		rawPath = s[:i]
		rawQuery = s[i+1:]
	} else {
		rawPath = s
	}
	parts := strings.Split(rawPath, "/")
	if len(parts) < 2 {
		return nil, errors.New("hf uri must be hf://owner/repo or hf://owner/repo/path")
	}
	var owner, repo string
	var filePath string
	if len(parts) == 2 {
		// hf://owner/repo - need to list files
		owner = parts[0]
		repo = parts[1]
		filePath = ""
	} else {
		owner = parts[0]
		repo = parts[1]
		filePath = path.Join(parts[2:]...)
	}

	// Parse query parameters
	rev := "main"
	var requestedQuant string
	if rawQuery != "" {
		q, _ := url.ParseQuery(rawQuery)
		if v := q.Get("rev"); v != "" {
			rev = v
		}
		requestedQuant = q.Get("quant")
	}

	// Construct repo ID
	repoID := owner + "/" + repo

	// Prepare headers (auth)
	headers := map[string]string{}
	if cfg != nil && cfg.Sources.HuggingFace.Enabled {
		if env := strings.TrimSpace(cfg.Sources.HuggingFace.TokenEnv); env != "" {
			if tok := strings.TrimSpace(getenv(env)); tok != "" {
				headers["Authorization"] = "Bearer " + tok
			}
		}
	}

	// Determine if we need quantization detection
	needsQuantDetection := (filePath == "" || requestedQuant != "")

	// If specific file path given and no quant requested, use direct resolution (backward compatible)
	if filePath != "" && !needsQuantDetection {
		base := "https://huggingface.co/" + repoID
		resURL := base + "/resolve/" + url.PathEscape(rev) + "/" + strings.ReplaceAll(filePath, "+", "%2B")

		fileName := path.Base(filePath)
		suggested := h.computeSuggestedFilename(cfg, owner, repo, filePath, rev, fileName, "")

		return &Resolved{
			URL:               resURL,
			Headers:           headers,
			FileName:          fileName,
			SuggestedFilename: suggested,
			RepoOwner:         owner,
			RepoName:          repo,
			RepoPath:          filePath,
			Rev:               rev,
		}, nil
	}

	// List files and detect quantizations
	files, err := h.listRepoFiles(ctx, repoID, rev, headers)
	if err != nil {
		return nil, fmt.Errorf("list repo files: %w", err)
	}

	// Group files by quantization
	groups := groupQuantizations(files)
	if len(groups) == 0 {
		return nil, errors.New("no model files found in repository")
	}

	// Select quantization
	selected, ok := selectBestQuantization(groups, requestedQuant)
	if !ok {
		if requestedQuant != "" {
			return nil, fmt.Errorf("requested quantization %q not found", requestedQuant)
		}
		return nil, errors.New("no suitable quantization found")
	}

	// Build resolved URL with selected file
	base := "https://huggingface.co/" + repoID
	resURL := base + "/resolve/" + url.PathEscape(rev) + "/" + strings.ReplaceAll(selected.FilePath, "+", "%2B")

	fileName := path.Base(selected.FilePath)
	suggested := h.computeSuggestedFilename(cfg, owner, repo, selected.FilePath, rev, fileName, selected.Name)

	// Collect all available quantizations for metadata
	allQuants := flattenQuantizations(groups)

	return &Resolved{
		URL:                    resURL,
		Headers:                headers,
		FileName:               fileName,
		SuggestedFilename:      suggested,
		RepoOwner:              owner,
		RepoName:               repo,
		RepoPath:               selected.FilePath,
		Rev:                    rev,
		AvailableQuantizations: allQuants,
		SelectedQuantization:   selected.Name,
	}, nil
}

// computeSuggestedFilename computes the suggested filename using naming pattern.
func (h *HuggingFace) computeSuggestedFilename(cfg *config.Config, owner, repo, filePath, rev, fileName, quant string) string {
	var suggested string
	if cfg != nil && strings.TrimSpace(cfg.Sources.HuggingFace.Naming.Pattern) != "" {
		pat := strings.TrimSpace(cfg.Sources.HuggingFace.Naming.Pattern)
		tokens := map[string]string{
			"owner":        owner,
			"repo":         repo,
			"path":         filePath,
			"rev":          rev,
			"file_name":    fileName,
			"quantization": quant,
		}
		suggested = util.SafeFileName(util.ExpandPattern(pat, tokens))
		if strings.TrimSpace(suggested) == "" {
			suggested = ""
		}
	}
	if suggested == "" {
		suggested = util.SafeFileName(fileName)
	}
	return suggested
}

// getenv split to enable testing
var getenv = os.Getenv
