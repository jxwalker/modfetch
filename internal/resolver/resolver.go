package resolver

import (
	"context"
	"errors"
	"strings"

	"modfetch/internal/config"
)

type Resolved struct {
	URL               string
	Headers           map[string]string
	// Optional metadata (primarily for CivitAI) — may be empty for other resolvers
	ModelName         string
	VersionName       string
	VersionID         string
	FileName          string
	FileType          string // Source-specific type (e.g., CivitAI file.type: Model|VAE|TextualInversion)
	SuggestedFilename string
}

type Resolver interface {
	CanHandle(uri string) bool
	Resolve(ctx context.Context, uri string, cfg *config.Config) (*Resolved, error)
}

func Resolve(ctx context.Context, uri string, cfg *config.Config) (*Resolved, error) {
	uri = strings.TrimSpace(uri)
	switch {
	case strings.HasPrefix(uri, "hf://"):
		return (&HuggingFace{}).Resolve(ctx, uri, cfg)
	case strings.HasPrefix(uri, "civitai://"):
		return (&CivitAI{}).Resolve(ctx, uri, cfg)
	default:
		return nil, errors.New("no resolver for uri scheme")
	}
}

