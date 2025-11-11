package downloader

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/resolver"
	"github.com/jxwalker/modfetch/internal/state"
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

	// Resolve with retry logic (API can be flaky)
	var res *resolver.Resolved
	var resolveErr error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		res, resolveErr = resolver.Resolve(context.Background(), uri, cfg)

		if resolveErr == nil {
			break
		}

		// Check if error is API-related
		errMsg := resolveErr.Error()
		if attempt < maxRetries && (strings.Contains(errMsg, "503") || strings.Contains(errMsg, "Service Unavailable")) {
			t.Logf("Resolve attempt %d/%d failed with API error: %v (retrying...)", attempt, maxRetries, resolveErr)
			continue
		}

		break
	}

	// If resolver fails with API errors after retries, skip test
	if resolveErr != nil {
		errMsg := resolveErr.Error()
		if strings.Contains(errMsg, "503") || strings.Contains(errMsg, "Service Unavailable") {
			t.Skipf("CivitAI API unavailable (resolver) after %d attempts: %v", maxRetries, resolveErr)
		}
		t.Fatalf("resolve: %v", resolveErr)
	}

	// Verify URL and headers before download
	if res.URL == "" {
		t.Fatalf("empty download URL")
	}
	if res.Headers["Authorization"] == "" {
		t.Fatalf("missing authorization header")
	}

	// Download with retry logic to handle CivitAI API flakiness
	// CivitAI can return 400, 503, or other transient errors
	dl := NewChunked(cfg, log, st, nil)
	var dest, sha string
	var downloadErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		dest, sha, downloadErr = dl.Download(context.Background(), res.URL, "", "", res.Headers, false)

		if downloadErr == nil {
			// Success!
			break
		}

		// Check if error indicates API unavailability
		errMsg := downloadErr.Error()
		isAPIError := (attempt < maxRetries) &&
			(strings.Contains(errMsg, "400") || strings.Contains(errMsg, "503") ||
			 strings.Contains(errMsg, "Bad Request") || strings.Contains(errMsg, "Service Unavailable"))

		if isAPIError {
			t.Logf("Attempt %d/%d failed with API error: %v (retrying...)", attempt, maxRetries, downloadErr)
			continue
		}

		// Non-retryable error or last attempt
		break
	}

	// If all retries failed with API errors, skip the test
	if downloadErr != nil {
		errMsg := downloadErr.Error()
		if strings.Contains(errMsg, "400") || strings.Contains(errMsg, "503") ||
		   strings.Contains(errMsg, "Bad Request") || strings.Contains(errMsg, "Service Unavailable") {
			t.Skipf("CivitAI API unavailable after %d attempts: %v", maxRetries, downloadErr)
		}
		t.Fatalf("download failed: %v", downloadErr)
	}

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
