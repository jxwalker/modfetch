package starter

import (
	"fmt"
	"sort"
	"strings"
)

// Entry is a beginner-safe download target that exercises the normal resolver
// and downloader paths without requiring tokens or multi-gigabyte files.
type Entry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Kind        string `json:"kind"`
	URI         string `json:"uri"`
	Description string `json:"description"`
	ApproxSize  string `json:"approx_size"`
	BestFor     string `json:"best_for"`
}

var entries = []Entry{
	{
		ID:          "gpt2-config",
		Name:        "GPT-2 config",
		Provider:    "huggingface",
		Kind:        "llm-metadata",
		URI:         "hf://gpt2/config.json?rev=main",
		Description: "Tiny public Hugging Face artifact for checking resolver, download, checksum, and library state without a large model.",
		ApproxSize:  "<1 KB",
		BestFor:     "first download smoke test",
	},
	{
		ID:          "gpt2-tokenizer",
		Name:        "GPT-2 tokenizer",
		Provider:    "huggingface",
		Kind:        "llm-tokenizer",
		URI:         "hf://gpt2/tokenizer.json?rev=main",
		Description: "Small public tokenizer artifact that behaves like a normal Hugging Face model file but remains quick to download.",
		ApproxSize:  "~1.3 MB",
		BestFor:     "beginner Hugging Face test",
	},
	{
		ID:          "public-1mb",
		Name:        "Public 1 MB file",
		Provider:    "direct-http",
		Kind:        "network-smoke",
		URI:         "https://proof.ovh.net/files/1Mb.dat",
		Description: "Known public HTTP file for validating the raw downloader path independent of model providers.",
		ApproxSize:  "1 MB",
		BestFor:     "network and resume smoke test",
	},
}

// List returns the stable starter catalog sorted by ID.
func List() []Entry {
	out := append([]Entry(nil), entries...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// Get returns one starter entry by ID. The starter:// prefix is accepted so
// callers can pass user input directly.
func Get(id string) (Entry, bool) {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "starter://")
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return Entry{}, false
}

// MustURI converts a starter ID into the underlying URI used by the regular
// download pipeline.
func MustURI(id string) (string, error) {
	entry, ok := Get(id)
	if !ok {
		return "", fmt.Errorf("unknown starter %q", strings.TrimPrefix(strings.TrimSpace(id), "starter://"))
	}
	return entry.URI, nil
}
