package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/resolver"
	"github.com/jxwalker/modfetch/internal/state"
)

func TestCivitAIResolveAndDownload_WithAuthAndChunks(t *testing.T) {
	if testing.Short() {
		t.Skip("-short set")
	}
	// Prepare payload
	size := 1024 * 1024 // 1MiB
	payload := make([]byte, size)
	for i := 0; i < size; i++ {
		payload[i] = byte(i % 251)
	}
	token := "sekrit"

	// Server that requires Authorization and supports Range
	var base string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/models/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = fmt.Fprintf(w, `{"modelVersions":[{"id":1,"files":[{"id":1,"name":"file.bin","type":"Model","primary":true,"downloadUrl":"%s/secure.bin"}]}]}`, base)
	})
	mux.HandleFunc("/secure.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			if got := r.Header.Get("Authorization"); got != "Bearer "+token {
				w.WriteHeader(401)
				return
			}
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Header().Set("ETag", "\"test\"")
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			w.WriteHeader(200)
			return
		}
		if r.Method == http.MethodGet {
			if got := r.Header.Get("Authorization"); got != "Bearer "+token {
				w.WriteHeader(401)
				return
			}
			rng := r.Header.Get("Range")
			if rng == "" {
				w.WriteHeader(200)
				_, _ = w.Write(payload)
				return
			}
			var start, end int
			if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err != nil {
				w.WriteHeader(416)
				return
			}
			if start < 0 || end >= len(payload) || end < start {
				w.WriteHeader(416)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
			w.WriteHeader(206)
			_, _ = w.Write(payload[start : end+1])
			return
		}
		w.WriteHeader(405)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	base = ts.URL

	// Override CivitAI base URL for resolver
	old := resolverTestBaseSwap(ts.URL)
	defer func() { resolverTestBaseSwap(old) }()

	// Config
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte("version: 1\n" +
		"general:\n  data_root: \"" + tmp + "/data\"\n  download_root: \"" + tmp + "/dl\"\n" +
		"network:\n  timeout_seconds: 10\n  user_agent: \"modfetch-test\"\n" +
		"concurrency:\n  chunk_size_mb: 1\n  per_file_chunks: 4\n" +
		"sources:\n  civitai:\n    enabled: true\n    token_env: \"CIVITAI_TEST_TOKEN\"\n")
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("CIVITAI_TEST_TOKEN", token)
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

	// Resolve and download
	uri := "civitai://model/xyz"
	res, err := resolver.Resolve(context.Background(), uri, cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	dl := NewChunked(cfg, log, st, nil)
	dest, sha, err := dl.Download(context.Background(), res.URL, "", "", res.Headers, false)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if int(fi.Size()) != size {
		t.Fatalf("size mismatch: %d", fi.Size())
	}
	if len(sha) != 64 {
		t.Fatalf("sha length: %d", len(sha))
	}
}

// helper to override resolver base url; returns previous value
func resolverTestBaseSwap(new string) string {
	old := resolver.CivitaiBaseForTest()
	resolver.SetCivitaiBaseForTest(new)
	return old
}
