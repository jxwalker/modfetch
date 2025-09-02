package resolver

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path"
	"strings"

	"modfetch/internal/config"
	"modfetch/internal/util"
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
	var owner, repo string
	var filePath string
	if len(parts) == 2 {
		owner = ""
		repo = parts[0]
		filePath = parts[1]
	} else {
		owner = parts[0]
		repo = parts[1]
		filePath = path.Join(parts[2:]...)
	}
	rev := "main"
	if rawQuery != "" {
		q, _ := url.ParseQuery(rawQuery)
		if v := q.Get("rev"); v != "" { rev = v }
	}
	// Construct resolve URL
	repoID := repo
	if strings.TrimSpace(owner) != "" { repoID = owner + "/" + repo }
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
	// Compute SuggestedFilename via naming pattern if provided
	fileName := path.Base(filePath)
	var suggested string
	if cfg != nil && strings.TrimSpace(cfg.Sources.HuggingFace.Naming.Pattern) != "" {
		pat := strings.TrimSpace(cfg.Sources.HuggingFace.Naming.Pattern)
		tokens := map[string]string{
			"owner":     owner,
			"repo":      repo,
			"path":      filePath,
			"rev":       rev,
			"file_name": fileName,
		}
		suggested = util.SafeFileName(util.ExpandPattern(pat, tokens))
		if strings.TrimSpace(suggested) == "" { suggested = "" }
	}
	if suggested == "" {
		suggested = util.SafeFileName(fileName)
	}
	return &Resolved{URL: resURL, Headers: headers, FileName: fileName, SuggestedFilename: suggested, RepoOwner: owner, RepoName: repo, RepoPath: filePath, Rev: rev}, nil
}

// getenv split to enable testing
var getenv = os.Getenv

