package resolver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"modfetch/internal/config"
)

func TestHFResolveBasic(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte("version: 1\ngeneral:\n  data_root: \"" + tmp + "\"\n  download_root: \"" + tmp + "\"\n")
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	res, err := (&HuggingFace{}).Resolve(context.Background(), "hf://gpt2/README.md?rev=main", cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.URL == "" {
		t.Fatalf("empty url")
	}
	if res.Headers == nil {
		t.Fatalf("nil headers")
	}
}
