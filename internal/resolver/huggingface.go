package resolver

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path"
	"strings"

	"modfetch/internal/config"
)

type HuggingFace struct{}

// Accepts URIs of the form:
//   hf://{owner}/{repo}/{path}?rev=main
// If rev is omitted, defaults to "main".
func (h *HuggingFace) CanHandle(u string) bool { return strings.HasPrefix(u, "hf://") }

func (h *HuggingFace) Resolve(ctx context.Context, uri string, cfg *config.Config) (*Resolved, error) {
	if !h.CanHandle(uri) { return nil, errors.New("unsupported scheme") }
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
		return nil, errors.New("hf uri must be hf://repo/path or hf://owner/repo/path")
	}
	var repoID string
	var filePath string
	if len(parts) == 2 {
		repoID = parts[0]
		filePath = parts[1]
	} else {
		repoID = parts[0] + "/" + parts[1]
		filePath = path.Join(parts[2:]...)
	}
	rev := "main"
	if rawQuery != "" {
		q, _ := url.ParseQuery(rawQuery)
		if v := q.Get("rev"); v != "" { rev = v }
	}
	// Construct resolve URL
	base := "https://huggingface.co/" + repoID
	resURL := base + "/resolve/" + url.PathEscape(rev) + "/" + strings.ReplaceAll(filePath, "+", "%2B")

	headers := map[string]string{}
	// Token support (if provided in config and env)
	if cfg != nil && cfg.Sources.HuggingFace.Enabled {
		if env := strings.TrimSpace(cfg.Sources.HuggingFace.TokenEnv); env != "" {
			if tok := strings.TrimSpace(getenv(env)); tok != "" {
				headers["Authorization"] = "Bearer " + tok
			}
		}
	}
	return &Resolved{URL: resURL, Headers: headers}, nil
}

// getenv split to enable testing
var getenv = os.Getenv

