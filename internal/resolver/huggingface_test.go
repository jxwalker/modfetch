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
	// Integration test that verifies HuggingFace API access with authentication
	// Requires HF_TOKEN environment variable to be set
	if os.Getenv("HF_TOKEN") == "" {
		t.Skip("Skipping HuggingFace test: HF_TOKEN not set")
	}

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yml")
	// Configure HuggingFace source with token_env
	cfgYaml := []byte("version: 1\n" +
		"general:\n  data_root: \"" + tmp + "\"\n  download_root: \"" + tmp + "\"\n" +
		"sources:\n  huggingface:\n    enabled: true\n    token_env: \"HF_TOKEN\"\n")
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
		t.Fatalf("resolve failed: %v", err)
	}
	if res.URL == "" {
		t.Fatalf("empty url")
	}
	if res.Headers == nil {
		t.Fatalf("nil headers")
	}
	// Verify Authorization header is set
	if auth, ok := res.Headers["Authorization"]; !ok || auth == "" {
		t.Fatalf("Authorization header not set")
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
