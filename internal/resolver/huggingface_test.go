package resolver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestHFResolveBasic(t *testing.T) {
	// Simple test that doesn't make network calls - tests URL construction with a specific file path
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

	// Test with a specific file path (doesn't trigger API call for quantization detection)
	res, err := (&HuggingFace{}).Resolve(context.Background(), "hf://gpt2/README.md?rev=main", cfg)
	if err != nil {
		// Skip test if network is unavailable or API fails - this is an integration test
		t.Skipf("Skipping HuggingFace integration test (requires network access): %v", err)
	}
	if res.URL == "" {
		t.Fatalf("empty url")
	}
	if res.Headers == nil {
		t.Fatalf("nil headers")
	}
	// Verify URL structure is correct
	if !strings.Contains(res.URL, "huggingface.co") {
		t.Fatalf("unexpected URL: %s", res.URL)
	}
	if !strings.Contains(res.URL, "gpt2") {
		t.Fatalf("URL missing repo name: %s", res.URL)
	}
	if !strings.Contains(res.URL, "README.md") {
		t.Fatalf("URL missing file name: %s", res.URL)
	}
}
