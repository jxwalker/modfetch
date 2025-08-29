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
}

type Resolver interface {
	CanHandle(uri string) bool
	Resolve(ctx context.Context, uri string, cfg *config.Config) (*Resolved, error)
}

func Resolve(ctx context.Context, uri string, cfg *config.Config) (*Resolved, error) {
	uri = strings.TrimSpace(uri)
	if strings.HasPrefix(uri, "hf://") {
		return (&HuggingFace{}).Resolve(ctx, uri, cfg)
	}
	return nil, errors.New("no resolver for uri scheme")
}

