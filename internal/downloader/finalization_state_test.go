package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/state"
)

func testConfig(t *testing.T, root string) *config.Config {
	t.Helper()
	cfgYaml := []byte(fmt.Sprintf(`version: 1
general:
  data_root: "%s/data"
  download_root: "%s/dl"
  placement_mode: "symlink"
  quarantine: false
  allow_overwrite: true
  stage_partials: false
network:
  timeout_seconds: 10
concurrency:
  per_file_chunks: 2
  chunk_size_mb: 1
`, root, root))
	cfgPath := filepath.Join(root, "config.yml")
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	return cfg
}

func TestSingleDownload_PersistsCompleteWhenSidecarWriteFails(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig(t, tmp)
	st, err := state.Open(cfg)
	if err != nil {
		t.Fatalf("state open: %v", err)
	}
	defer func() { _ = st.Close() }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "11")
			return
		}
		_, _ = w.Write([]byte("hello world"))
	}))
	defer srv.Close()

	oldWrite := writeAndSyncFile
	writeAndSyncFile = func(path string, b []byte) error {
		if filepath.Ext(path) == ".sha256" {
			return fmt.Errorf("simulated sidecar failure")
		}
		return oldWrite(path, b)
	}
	defer func() { writeAndSyncFile = oldWrite }()

	dl := NewSingle(cfg, logging.New("error", false), st, nil)
	dest, sha, err := dl.Download(context.Background(), srv.URL+"/model.bin", "", "", nil, false)
	if err != nil {
		t.Fatalf("download returned error: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("final file missing: %v", err)
	}
	if len(sha) != 64 {
		t.Fatalf("unexpected sha length: %d", len(sha))
	}
	rows, err := st.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Status != "complete" {
		t.Fatalf("status = %q, want complete", rows[0].Status)
	}
	if rows[0].LastError == "" {
		t.Fatal("expected warning in last_error for sidecar failure")
	}
}

