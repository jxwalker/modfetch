package downloader

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/resolver"
	"github.com/jxwalker/modfetch/internal/state"
	"github.com/jxwalker/modfetch/internal/testutil"
)

func TestCivitAIResolveAndDownload(t *testing.T) {
	// Integration test that downloads a small file from CivitAI
	// Requires CIVITAI_TOKEN environment variable to be set
	// Note: CivitAI API can be flaky, so this test uses retry logic
	if testing.Short() {
		t.Skip("-short set")
	}
	if os.Getenv("CIVITAI_TOKEN") == "" {
		t.Skip("Skipping CivitAI test: CIVITAI_TOKEN not set")
	}

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte("version: 1\n" +
		"general:\n" +
		"  data_root: \"" + tmp + "/data\"\n" +
		"  download_root: \"" + tmp + "/dl\"\n" +
		"sources:\n" +
		"  civitai:\n" +
		"    enabled: true\n" +
		"    token_env: \"CIVITAI_TOKEN\"\n")
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	defer func() { _ = st.SQL.Close() }()

	// Test with a small TextualInversion model (model ID 2114201)
	// This is a small file (~232 KB) suitable for testing without long download times
	uri := "civitai://model/2114201"

	// Resolve with retry helper (API can be flaky)
	var res *resolver.Resolved
	testutil.RetryOnAPIError(t, testutil.MaxAPIRetries, func() error {
		var err error
		res, err = resolver.Resolve(context.Background(), uri, cfg)
		return err
	}, "CivitAI resolve")

	// Verify URL and headers before download
	if res.URL == "" {
		t.Fatalf("empty download URL")
	}
	if res.Headers["Authorization"] == "" {
		t.Fatalf("missing authorization header")
	}

	// Download with retry helper (API can return 400, 503, or other transient errors)
	dl := NewChunked(cfg, log, st, nil)
	var dest, sha string
	testutil.RetryOnAPIError(t, testutil.MaxAPIRetries, func() error {
		var err error
		dest, sha, err = dl.Download(context.Background(), res.URL, "", "", res.Headers, false)
		return err
	}, "CivitAI download")

	// Verify file was downloaded
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("dest stat: %v", err)
	}

	// Verify SHA256 hash was calculated
	if len(sha) != 64 {
		t.Fatalf("sha length: %d", len(sha))
	}

	// File should be relatively small (TextualInversion embedding)
	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// TextualInversion files are typically < 1MB
	if fi.Size() > 10*1024*1024 {
		t.Fatalf("downloaded file unexpectedly large: %d bytes", fi.Size())
	}
	if fi.Size() == 0 {
		t.Fatalf("downloaded file is empty")
	}
}
