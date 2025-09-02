package resolver

import (
	"context"
	"errors"
	"strings"

	"modfetch/internal/config"
)

type Resolved struct {
	URL     string
	Headers map[string]string
	// Optional metadata (primarily for CivitAI) — may be empty for other resolvers
	ModelName         string
	VersionName       string
	VersionID         string
	FileName          string
	FileType          string // Source-specific type (e.g., CivitAI file.type: Model|VAE|TextualInversion)
	SuggestedFilename string
	// Optional metadata for Hugging Face
	RepoOwner string
	RepoName  string
	RepoPath  string
	Rev       string
}

type Resolver interface {
	CanHandle(uri string) bool
	Resolve(ctx context.Context, uri string, cfg *config.Config) (*Resolved, error)
}

func Resolve(ctx context.Context, uri string, cfg *config.Config) (*Resolved, error) {
	uri = strings.TrimSpace(uri)
	if cfg != nil {
		if res, ok, err := cacheGet(cfg, uri); err == nil && ok {
			return res, nil
		}
	}
	var (
		res *Resolved
		err error
	)
	switch {
	case strings.HasPrefix(uri, "hf://"):
		res, err = (&HuggingFace{}).Resolve(ctx, uri, cfg)
	case strings.HasPrefix(uri, "civitai://"):
		res, err = (&CivitAI{}).Resolve(ctx, uri, cfg)
	default:
		err = errors.New("no resolver for uri scheme")
	}
	if err == nil && cfg != nil {
		_ = cacheSet(cfg, uri, res)
	} else if err != nil && cfg != nil && isNotFound(err) {
		_ = cacheDelete(cfg, uri)
	}
	return res, err
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "404")
}
