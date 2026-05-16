package taskpack

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jxwalker/modfetch/internal/snapshot"
	"github.com/jxwalker/modfetch/internal/util"
)

type Pack struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Task        string `json:"task"`
	Description string `json:"description"`
	BestFor     string `json:"best_for"`
	Files       []File `json:"files"`
}

type File struct {
	Path       string `json:"path"`
	URI        string `json:"uri"`
	Type       string `json:"type,omitempty"`
	ApproxSize string `json:"approx_size,omitempty"`
}

var packs = []Pack{
	{
		ID:          "llm-smoke",
		Name:        "Tiny LLM tokenizer smoke pack",
		Task:        "chat",
		Description: "A small GPT-2 config/tokenizer bundle for proving multi-file Hugging Face downloads without pulling a full model.",
		BestFor:     "first multi-file download and resolver smoke test",
		Files: []File{
			{Path: "config.json", URI: "hf://gpt2/config.json?rev=607a30d783dfa663caf39e06633721c8d4cfcd7e", Type: "metadata", ApproxSize: "<1 KB"},
			{Path: "tokenizer.json", URI: "hf://gpt2/tokenizer.json?rev=607a30d783dfa663caf39e06633721c8d4cfcd7e", Type: "metadata", ApproxSize: "~1.3 MB"},
			{Path: "vocab.json", URI: "hf://gpt2/vocab.json?rev=607a30d783dfa663caf39e06633721c8d4cfcd7e", Type: "metadata", ApproxSize: "~1 MB"},
			{Path: "merges.txt", URI: "hf://gpt2/merges.txt?rev=607a30d783dfa663caf39e06633721c8d4cfcd7e", Type: "metadata", ApproxSize: "~450 KB"},
		},
	},
	{
		ID:          "embedding-smoke",
		Name:        "Tiny BERT embedding smoke pack",
		Task:        "embedding",
		Description: "A tiny public BERT-style repository subset with config, tokenizer, vocabulary, and safetensors weights.",
		BestFor:     "validating a complete small text-embedding style repo layout",
		Files: []File{
			{Path: "config.json", URI: "hf://hf-internal-testing/tiny-random-bert/config.json?rev=main", Type: "metadata", ApproxSize: "<5 KB"},
			{Path: "tokenizer.json", URI: "hf://hf-internal-testing/tiny-random-bert/tokenizer.json?rev=main", Type: "metadata", ApproxSize: "~500 KB"},
			{Path: "vocab.txt", URI: "hf://hf-internal-testing/tiny-random-bert/vocab.txt?rev=main", Type: "metadata", ApproxSize: "~5 KB"},
			{Path: "model.safetensors", URI: "hf://hf-internal-testing/tiny-random-bert/model.safetensors?rev=main", Type: "safetensors", ApproxSize: "~520 KB"},
		},
	},
}

func List() []Pack {
	out := append([]Pack(nil), packs...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func Get(id string) (Pack, bool) {
	id = strings.TrimSpace(id)
	for _, pack := range packs {
		if pack.ID == id {
			return pack, true
		}
	}
	return Pack{}, false
}

func MustManifest(id, destDir string) (*snapshot.Manifest, error) {
	pack, ok := Get(id)
	if !ok {
		return nil, fmt.Errorf("unknown pack %q", strings.TrimSpace(id))
	}
	return Manifest(pack, destDir), nil
}

func Manifest(pack Pack, destDir string) *snapshot.Manifest {
	files := make([]snapshot.File, 0, len(pack.Files))
	for _, file := range pack.Files {
		files = append(files, snapshot.File{
			Path: file.Path,
			Type: file.Type,
			URI:  file.URI,
			Dest: packDest(destDir, pack.ID, file.Path),
		})
	}
	return &snapshot.Manifest{
		Version: 1,
		Source:  "taskpack",
		Repo:    pack.ID,
		Rev:     "curated",
		Files:   files,
	}
}

func packDest(destDir, packID, filePath string) string {
	destDir = strings.TrimSpace(destDir)
	if destDir == "" {
		return ""
	}
	packDir := util.SafeFileName(packID)
	return filepath.Join(destDir, packDir, filepath.FromSlash(filePath))
}
