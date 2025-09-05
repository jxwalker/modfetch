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
)

func TestHFResolveAndDownload(t *testing.T) {
	if testing.Short() {
		t.Skip("-short set")
	}
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte("version: 1\n" +
		"general:\n" +
		"  data_root: \"" + tmp + "/data\"\n" +
		"  download_root: \"" + tmp + "/dl\"\n" +
		"sources:\n" +
		"  huggingface:\n" +
		"    enabled: true\n" +
		"    token_env: \"HF_TOKEN\"\n")
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

	uri := "hf://gpt2/README.md?rev=main"
	res, err := resolver.Resolve(context.Background(), uri, cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	dl := NewChunked(cfg, log, st, nil)
	dest, sha, err := dl.Download(context.Background(), res.URL, "", "", res.Headers, false)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("dest stat: %v", err)
	}
	if len(sha) != 64 {
		t.Fatalf("sha length: %d", len(sha))
	}
}
