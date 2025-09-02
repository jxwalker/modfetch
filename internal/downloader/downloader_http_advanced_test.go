package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

// 5xx backoff: first attempt(s) for each chunk return 503, then success
func TestChunked_RangeTransient5xxThenSuccess(t *testing.T) {
	tmp := t.TempDir()
	// 2 MiB payload to ensure multiple chunks
	size := 2 << 20
	payload := make([]byte, size)
	for i := 0; i < size; i++ { payload[i] = byte((i*13 + 5) % 251) }

	var mu sync.Mutex
	tries := map[int]int{}
	mux := http.NewServeMux()
	mux.HandleFunc("/fivexx.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(200); return
		}
		if r.Method != http.MethodGet { w.WriteHeader(405); return }
		rng := r.Header.Get("Range")
		if rng == "" {
	w.WriteHeader(200); _, _ = w.Write(payload); return
		}
		var start, end int
		if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err != nil { w.WriteHeader(416); return }
		mu.Lock()
		tries[start]++
		count := tries[start]
		mu.Unlock()
		// First attempt per start offset returns 503
		if count == 1 { w.WriteHeader(503); return }
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.WriteHeader(206)
	_, _ = w.Write(payload[start:end+1])
	})
	ts := httptest.NewServer(mux); defer ts.Close()
	url := ts.URL + "/fivexx.bin"

	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte(strings.Join([]string{
		"version: 1",
		"general:",
		"  data_root: \""+tmp+"/data\"",
		"  download_root: \""+tmp+"/dl\"",
		"network:",
		"  timeout_seconds: 10",
		"concurrency:",
		"  per_file_chunks: 4",
		"  chunk_size_mb: 1",
		"  max_retries: 4",
		"  backoff:",
		"    min_ms: 1",
		"    max_ms: 3",
	}, "\n"))
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	cfg, err := config.Load(cfgPath)
	if err != nil { t.Fatalf("config: %v", err) }
	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil { t.Fatalf("state: %v", err) }
	defer func() { _ = st.SQL.Close() }()

	dl := NewAuto(cfg, log, st, nil)
	dest, sha, err := dl.Download(context.Background(), url, "", "", nil, false)
	if err != nil { t.Fatalf("download: %v", err) }
	if _, err := os.Stat(dest); err != nil { t.Fatalf("dest: %v", err) }
	if len(sha) != 64 { t.Fatalf("sha len: %d", len(sha)) }
}

// Slow body: server writes slowly but within timeout; download should succeed
func TestChunked_SlowBodyCompletes(t *testing.T) {
	tmp := t.TempDir()
	// 1 MiB payload
	size := 1 << 20
	payload := make([]byte, size)
	for i := 0; i < size; i++ { payload[i] = byte((i*29 + 11) % 251) }

	mux := http.NewServeMux()
	mux.HandleFunc("/slow.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(200); return
		}
		rng := r.Header.Get("Range")
		var start, end int
		if rng == "" {
			start = 0; end = len(payload)-1
		} else {
			if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err != nil { w.WriteHeader(416); return }
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
			w.WriteHeader(206)
		}
		if rng == "" { w.WriteHeader(200) }
		// write in small chunks with small delays; total << client timeout
		chunk := 64 * 1024
		for off := start; off <= end; off += chunk {
			to := off + chunk
			if to > end+1 { to = end+1 }
	_, _ = w.Write(payload[off:to])
			time.Sleep(5 * time.Millisecond)
		}
	})
	ts := httptest.NewServer(mux); defer ts.Close()
	url := ts.URL + "/slow.bin"

	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte(strings.Join([]string{
		"version: 1",
		"general:",
		"  data_root: \""+tmp+"/data\"",
		"  download_root: \""+tmp+"/dl\"",
		"network:",
		"  timeout_seconds: 5",
		"concurrency:",
		"  per_file_chunks: 2",
		"  chunk_size_mb: 1",
	}, "\n"))
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	cfg, err := config.Load(cfgPath)
	if err != nil { t.Fatalf("config: %v", err) }
	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil { t.Fatalf("state: %v", err) }
	defer func() { _ = st.SQL.Close() }()

	dl := NewAuto(cfg, log, st, nil)
	dest, sha, err := dl.Download(context.Background(), url, "", "", nil, false)
	if err != nil { t.Fatalf("download: %v", err) }
	if _, err := os.Stat(dest); err != nil { t.Fatalf("dest: %v", err) }
	if len(sha) != 64 { t.Fatalf("sha len: %d", len(sha)) }
}

// Truncated body mid-chunk on first attempt; retry then complete
func TestChunked_TruncatedBodyThenRetrySuccess(t *testing.T) {
	tmp := t.TempDir()
	// 1 MiB payload
	size := 1 << 20
	payload := make([]byte, size)
	for i := 0; i < size; i++ { payload[i] = byte((i*7 + 19) % 251) }
	h := sha256.Sum256(payload)
	expected := hex.EncodeToString(h[:])

	var once int32
	mux := http.NewServeMux()
	mux.HandleFunc("/trunc.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(200); return
		}
		rng := r.Header.Get("Range")
		var start, end int
		if rng == "" {
			start = 0; end = len(payload)-1; w.WriteHeader(200)
		} else {
			if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err != nil { w.WriteHeader(416); return }
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
			w.WriteHeader(206)
		}
		// First attempt for the first requested range: write only half
		if atomic.CompareAndSwapInt32(&once, 0, 1) {
			half := start + (end-start+1)/2
			_, _ = w.Write(payload[start:half])
			return
		}
		_, _ = w.Write(payload[start : end+1])
	})
	ts := httptest.NewServer(mux); defer ts.Close()
	url := ts.URL + "/trunc.bin"

	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte(strings.Join([]string{
		"version: 1",
		"general:",
		"  data_root: \""+tmp+"/data\"",
		"  download_root: \""+tmp+"/dl\"",
		"network:",
		"  timeout_seconds: 10",
		"concurrency:",
		"  per_file_chunks: 4",
		"  chunk_size_mb: 1",
		"  max_retries: 3",
		"  backoff:",
		"    min_ms: 1",
		"    max_ms: 3",
	}, "\n"))
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	cfg, err := config.Load(cfgPath)
	if err != nil { t.Fatalf("config: %v", err) }
	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil { t.Fatalf("state: %v", err) }
	defer func() { _ = st.SQL.Close() }()

	dl := NewAuto(cfg, log, st, nil)
	dest, sha, err := dl.Download(context.Background(), url, "", expected, nil, false)
	if err != nil { t.Fatalf("download: %v", err) }
	if sha != expected { t.Fatalf("expected sha %s got %s", expected, sha) }
	if _, err := os.Stat(dest); err != nil { t.Fatalf("dest: %v", err) }
}
