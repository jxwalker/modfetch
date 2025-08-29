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
	"sync/atomic"
	"testing"

	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

// Test a Range-capable server that returns 429 once per chunk then succeeds.
func TestChunked_RangeWithTransient429(t *testing.T) {
	tmp := t.TempDir()
	// Prepare payload 512 KiB
	size := 512 * 1024
	payload := make([]byte, size)
	for i := 0; i < size; i++ { payload[i] = byte((i*31 + 7) % 251) }
	// Server
	var tries int64
	mux := http.NewServeMux()
	mux.HandleFunc("/file.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Header().Set("ETag", "\"t\"")
			w.Header().Set("Last-Modified", r.Header.Get("Date"))
			w.WriteHeader(200)
			return
		}
		if r.Method != http.MethodGet { w.WriteHeader(405); return }
		rng := r.Header.Get("Range")
		if rng == "" {
			w.WriteHeader(200)
			w.Write(payload)
			return
		}
		var start, end int
		if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err != nil { w.WriteHeader(416); return }
		if start < 0 || end >= len(payload) || end < start { w.WriteHeader(416); return }
		// Return 429 on the first attempt overall to exercise retry
		if atomic.AddInt64(&tries, 1) == 1 {
			w.WriteHeader(429); return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.WriteHeader(206)
		w.Write(payload[start : end+1])
	})
	ts := httptest.NewServer(mux); defer ts.Close()
	url := ts.URL + "/file.bin"

	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte(strings.Join([]string{
		"version: 1",
		"general:",
		"  data_root: \""+tmp+"/data\"",
		"  download_root: \""+tmp+"/dl\"",
		"concurrency:",
		"  per_file_chunks: 4",
		"  chunk_size_mb: 1",
		"  max_retries: 3",
		"  backoff:",
		"    min_ms: 1",
		"    max_ms: 2",
	}, "\n"))
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	cfg, err := config.Load(cfgPath)
	if err != nil { t.Fatalf("config: %v", err) }
	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil { t.Fatalf("state: %v", err) }
	defer st.SQL.Close()

	dl := NewAuto(cfg, log, st, nil)
	dest, sha, err := dl.Download(context.Background(), url, "", "", nil)
	if err != nil { t.Fatalf("download: %v", err) }
	if _, err := os.Stat(dest); err != nil { t.Fatalf("dest: %v", err) }
	if len(sha) != 64 { t.Fatalf("sha len: %d", len(sha)) }
}

// Test fallback when HEAD/Range are not supported (no Accept-Ranges)
func TestFallback_NoRangeSupport(t *testing.T) {
	tmp := t.TempDir()
	payload := []byte("hello world")
	mux := http.NewServeMux()
	mux.HandleFunc("/norange.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(200); return
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(200); w.Write(payload); return
		}
		w.WriteHeader(405)
	})
	ts := httptest.NewServer(mux); defer ts.Close()
	url := ts.URL + "/norange.bin"

	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte(strings.Join([]string{
		"version: 1",
		"general:",
		"  data_root: \""+tmp+"/data\"",
		"  download_root: \""+tmp+"/dl\"",
		"concurrency:",
		"  per_file_chunks: 4",
		"  chunk_size_mb: 1",
	}, "\n"))
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	cfg, err := config.Load(cfgPath)
	if err != nil { t.Fatalf("config: %v", err) }
	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil { t.Fatalf("state: %v", err) }
	defer st.SQL.Close()

	dl := NewAuto(cfg, log, st, nil)
	dest, _, err := dl.Download(context.Background(), url, "", "", nil)
	if err != nil { t.Fatalf("download: %v", err) }
	b, err := os.ReadFile(dest)
	if err != nil { t.Fatalf("read: %v", err) }
	if string(b) != string(payload) { t.Fatalf("payload mismatch") }
}

// Test corruption of one chunk on first attempt and repair on retry using expected SHA.
func TestChunked_CorruptOneChunkThenRepair(t *testing.T) {
	tmp := t.TempDir()
	// Build payload 1MiB deterministic
	size := 1 << 20
	payload := make([]byte, size)
	for i := 0; i < size; i++ { payload[i] = byte((i*17 + 3) % 251) }
	// expected SHA
	h := sha256.New(); h.Write(payload); expected := hex.EncodeToString(h.Sum(nil))
	// Corrupt the first GET for the chunk starting at offset 256KiB
	var corruptOnce int32 = 0
	chunkStart := 256 * 1024
	mux := http.NewServeMux()
	mux.HandleFunc("/c.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Header().Set("ETag", "\"t\"")
			w.WriteHeader(200); return
		}
		if r.Method != http.MethodGet { w.WriteHeader(405); return }
		rng := r.Header.Get("Range")
		if rng == "" {
			w.WriteHeader(200); w.Write(payload); return
		}
		var start, end int
		if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err != nil { w.WriteHeader(416); return }
		if start < 0 || end >= len(payload) || end < start { w.WriteHeader(416); return }
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.WriteHeader(206)
		chunk := make([]byte, end-start+1)
		copy(chunk, payload[start:end+1])
		if start == chunkStart && atomic.CompareAndSwapInt32(&corruptOnce, 0, 1) {
			// corrupt bytes
			for i := range chunk { chunk[i] ^= 0xFF }
		}
		w.Write(chunk)
	})
	ts := httptest.NewServer(mux); defer ts.Close()
	url := ts.URL + "/c.bin"

	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte(strings.Join([]string{
		"version: 1",
		"general:",
		"  data_root: \""+tmp+"/data\"",
		"  download_root: \""+tmp+"/dl\"",
		"concurrency:",
		"  per_file_chunks: 4",
		"  chunk_size_mb: 1",
		"  max_retries: 3",
		"  backoff:",
		"    min_ms: 1",
		"    max_ms: 2",
	}, "\n"))
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	cfg, err := config.Load(cfgPath)
	if err != nil { t.Fatalf("config: %v", err) }
	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil { t.Fatalf("state: %v", err) }
	defer st.SQL.Close()

	dl := NewAuto(cfg, log, st, nil)
	dest, sha, err := dl.Download(context.Background(), url, "", expected, nil)
	if err != nil { t.Fatalf("download: %v", err) }
	if sha != expected {
		b, _ := os.ReadFile(dest)
		h2 := sha256.Sum256(b)
		if hex.EncodeToString(h2[:]) != expected {
			t.Fatalf("sha not repaired: got=%s expected=%s", sha, expected)
		}
	}
}

