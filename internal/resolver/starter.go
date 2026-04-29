package resolver

import (
	"context"
	"strings"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/starter"
)

// Starter maps beginner-friendly starter:// IDs onto the normal resolver stack.
type Starter struct{}

func (Starter) CanHandle(uri string) bool {
	return strings.HasPrefix(strings.TrimSpace(uri), "starter://")
}

func (Starter) Resolve(ctx context.Context, uri string, cfg *config.Config) (*Resolved, error) {
	target, err := starter.MustURI(uri)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return &Resolved{URL: target, Headers: map[string]string{}}, nil
	}
	return Resolve(ctx, target, cfg)
}
