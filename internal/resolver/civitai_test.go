package resolver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestCivitAIResolveBasic(t *testing.T) {
	// Integration test that verifies CivitAI API access with authentication
	// Requires CIVITAI_TOKEN environment variable to be set
	if os.Getenv("CIVITAI_TOKEN") == "" {
		t.Skip("Skipping CivitAI test: CIVITAI_TOKEN not set")
	}

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yml")
	// Configure CivitAI source with token_env
	cfgYaml := []byte("version: 1\n" +
		"general:\n  data_root: \"" + tmp + "\"\n  download_root: \"" + tmp + "\"\n" +
		"sources:\n  civitai:\n    enabled: true\n    token_env: \"CIVITAI_TOKEN\"\n")
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	// Test with a known public model (Pony Diffusion V6 XL - model ID 257749)
	// This is one of the most popular models on CivitAI, very unlikely to be removed
	res, err := (&CivitAI{}).Resolve(context.Background(), "civitai://model/257749", cfg)
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
	// Verify Bearer token format
	if auth := res.Headers["Authorization"]; !strings.HasPrefix(auth, "Bearer ") {
		t.Fatalf("Authorization header should use Bearer token, got: %s", auth)
	}
	// Verify URL structure is correct
	if !strings.Contains(res.URL, "civitai.com") {
		t.Fatalf("unexpected URL: %s", res.URL)
	}
	// Should have a suggested filename
	if res.SuggestedFilename == "" {
		t.Fatalf("empty suggested filename")
	}
}

func TestCivitAIResolve_WithVersion(t *testing.T) {
	// Test version-specific resolution
	if os.Getenv("CIVITAI_TOKEN") == "" {
		t.Skip("Skipping CivitAI test: CIVITAI_TOKEN not set")
	}

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte("version: 1\n" +
		"general:\n  data_root: \"" + tmp + "\"\n  download_root: \"" + tmp + "\"\n" +
		"sources:\n  civitai:\n    enabled: true\n    token_env: \"CIVITAI_TOKEN\"\n")
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	// Test with specific version
	res, err := (&CivitAI{}).Resolve(context.Background(), "civitai://model/257749?version=290640", cfg)
	if err != nil {
		t.Fatalf("resolve with version failed: %v", err)
	}
	if res.URL == "" {
		t.Fatalf("empty url")
	}
	if !strings.Contains(res.URL, "civitai.com") {
		t.Fatalf("unexpected URL: %s", res.URL)
	}
}
