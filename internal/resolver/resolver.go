package resolver

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jxwalker/modfetch/internal/config"
)

// Quantization represents a specific quantization variant of a model file.
type Quantization struct {
	Name     string // Detected quantization (e.g., "Q4_K_M", "fp16", "Q5_0")
	FilePath string // Path in repository
	Size     int64  // File size in bytes
	FileType string // File extension type (e.g., "gguf", "safetensors", "bin")
}

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
	// Quantization support (HuggingFace)
	AvailableQuantizations []Quantization // List of detected quantization variants
	SelectedQuantization   string         // Which quantization was selected (if any)
}

type Resolver interface {
	CanHandle(uri string) bool
	Resolve(ctx context.Context, uri string, cfg *config.Config) (*Resolved, error)
}

var (
	registryMu      sync.RWMutex
	nextResolverID  uint64
	resolverEntries []resolverEntry
)

type resolverEntry struct {
	id       uint64
	resolver Resolver
}

// Register adds a resolver ahead of the built-in resolvers and returns an
// unregister function for tests or plugin lifecycle cleanup.
func Register(r Resolver) func() {
	if r == nil {
		return func() {}
	}
	registryMu.Lock()
	nextResolverID++
	id := nextResolverID
	resolverEntries = append(resolverEntries, resolverEntry{id: id, resolver: r})
	registryMu.Unlock()
	return func() {
		registryMu.Lock()
		defer registryMu.Unlock()
		for i, entry := range resolverEntries {
			if entry.id == id {
				resolverEntries = append(resolverEntries[:i], resolverEntries[i+1:]...)
				return
			}
		}
	}
}

func resolverSnapshot() []Resolver {
	registryMu.RLock()
	out := make([]Resolver, 0, len(resolverEntries)+3)
	for _, entry := range resolverEntries {
		out = append(out, entry.resolver)
	}
	registryMu.RUnlock()
	out = append(out, &Starter{}, &HuggingFace{}, &CivitAI{})
	return out
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
	for _, resolver := range resolverSnapshot() {
		if resolver.CanHandle(uri) {
			res, err = resolver.Resolve(ctx, uri, cfg)
			break
		}
	}
	if res == nil && err == nil {
		err = fmt.Errorf("no resolver for uri scheme: %s", uri)
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
