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
	"testing"

	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

// Test that single-stream downloader returns a SHA mismatch error, leaves .part, and records DB status.
func TestSingle_SHA256Mismatch_ErrorAndStatus(t *testing.T) {
	tmp := t.TempDir()
	payload := []byte("abcdef0123456789")
	mux := http.NewServeMux()
	mux.HandleFunc("/s.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(200)
			return
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(200)
			w.Write(payload)
			return
		}
		w.WriteHeader(405)
	})
	ts := httptest.NewServer(mux); defer ts.Close()
	url := ts.URL + "/s.bin"

	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte(strings.Join([]string{
		"version: 1",
		"general:",
		"  data_root: \""+tmp+"/data\"",
		"  download_root: \""+tmp+"/dl\"",
		"  stage_partials: false",
		"concurrency:",
		"  max_retries: 1",
	}, "\n"))
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	cfg, err := config.Load(cfgPath); if err != nil { t.Fatalf("config: %v", err) }
	log := logging.New("info", false)
	st, err := state.Open(cfg); if err != nil { t.Fatalf("state: %v", err) }
	defer st.SQL.Close()

	dl := NewSingle(cfg, log, st, nil)
	dest := filepath.Join(cfg.General.DownloadRoot, "s.bin")
	wrong := strings.Repeat("0", 64)
	final, got, err := dl.Download(context.Background(), url, dest, wrong, nil, false)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch error, got %v", err)
	}
	if final != "" {
		t.Fatalf("expected empty final path on mismatch, got %s", final)
	}
	// final file should not exist
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("expected no final file, statErr=%v", statErr)
	}
	// .part should exist next to dest
	part := StagePartPath(cfg, url, dest)
	if _, err := os.Stat(part); err != nil {
		t.Fatalf("expected part file to remain: %v", err)
	}
	// DB should have status checksum_mismatch with actual SHA set
	rows, err := st.ListDownloads(); if err != nil { t.Fatalf("list: %v", err) }
	found := false
	for _, r := range rows {
		if r.URL == url && r.Dest == dest {
			found = true
			if strings.ToLower(r.Status) != "checksum_mismatch" {
				t.Fatalf("status=%s want checksum_mismatch", r.Status)
			}
			if len(r.ActualSHA256) != 64 || !strings.EqualFold(r.ActualSHA256, got) {
				t.Fatalf("db actual sha mismatch: row=%s got=%s", r.ActualSHA256, got)
			}
		}
	}
	if !found { t.Fatalf("no downloads row found for url+dest") }
}

// Test that chunked downloader returns a SHA mismatch error, leaves .part, and records DB status.
func TestChunked_SHA256Mismatch_ErrorAndStatus(t *testing.T) {
	tmp := t.TempDir()
	// payload 600 KiB so multiple chunks with chunk_size_mb=1
	size := 600 * 1024
	payload := make([]byte, size)
	for i := 0; i < size; i++ { payload[i] = byte((i*13 + 5) % 251) }
	mux := http.NewServeMux()
	mux.HandleFunc("/c.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(200); return
		}
		if r.Method != http.MethodGet { w.WriteHeader(405); return }
		rng := r.Header.Get("Range")
		if rng == "" { w.WriteHeader(200); w.Write(payload); return }
		var start, end int
		if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err != nil { w.WriteHeader(416); return }
		if start < 0 || end >= len(payload) || end < start { w.WriteHeader(416); return }
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.WriteHeader(206)
		w.Write(payload[start : end+1])
	})
	ts := httptest.NewServer(mux); defer ts.Close()
	url := ts.URL + "/c.bin"

	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte(strings.Join([]string{
		"version: 1",
		"general:",
		"  data_root: \""+tmp+"/data\"",
		"  download_root: \""+tmp+"/dl\"",
		"  stage_partials: false",
		"concurrency:",
		"  per_file_chunks: 4",
		"  chunk_size_mb: 1",
		"  max_retries: 1",
		"network:",
		"  retry_on_rate_limit: false",
	}, "\n"))
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	cfg, err := config.Load(cfgPath); if err != nil { t.Fatalf("config: %v", err) }
	log := logging.New("info", false)
	st, err := state.Open(cfg); if err != nil { t.Fatalf("state: %v", err) }
	defer st.SQL.Close()

	dl := NewAuto(cfg, log, st, nil)
	dest := filepath.Join(cfg.General.DownloadRoot, "c.bin")
	wrong := strings.Repeat("f", 64)
	final, got, err := dl.Download(context.Background(), url, dest, wrong, nil, false)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch error, got %v", err)
	}
	if final != "" {
		t.Fatalf("expected empty final path on mismatch, got %s", final)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("expected no final file, statErr=%v", statErr)
	}
	part := StagePartPath(cfg, url, dest)
	if _, err := os.Stat(part); err != nil {
		t.Fatalf("expected part file to remain: %v", err)
	}
	rows, err := st.ListDownloads(); if err != nil { t.Fatalf("list: %v", err) }
	found := false
	for _, r := range rows {
		if r.URL == url && r.Dest == dest {
			found = true
			if strings.ToLower(r.Status) != "checksum_mismatch" {
				t.Fatalf("status=%s want checksum_mismatch", r.Status)
			}
			if len(r.ActualSHA256) != 64 || !strings.EqualFold(r.ActualSHA256, got) {
				t.Fatalf("db actual sha mismatch: row=%s got=%s", r.ActualSHA256, got)
			}
		}
	}
	if !found { t.Fatalf("no downloads row found for url+dest") }
}

// Test ComputeRemoteSHA256 streaming helper.
func TestComputeRemoteSHA256_Basic(t *testing.T) {
	tmp := t.TempDir()
	payload := make([]byte, 128*1024)
	for i := range payload { payload[i] = byte((i*7 + 11) % 251) }
	sum := sha256.Sum256(payload)
	expected := hex.EncodeToString(sum[:])
	mux := http.NewServeMux()
	mux.HandleFunc("/h.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead { w.Header().Set("Content-Length", strconv.Itoa(len(payload))); w.WriteHeader(200); return }
		if r.Method == http.MethodGet { w.WriteHeader(200); w.Write(payload); return }
		w.WriteHeader(405)
	})
	ts := httptest.NewServer(mux); defer ts.Close()
	url := ts.URL + "/h.bin"

	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte(strings.Join([]string{
		"version: 1",
		"general:",
		"  data_root: \""+tmp+"/data\"",
		"  download_root: \""+tmp+"/dl\"",
	}, "\n"))
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	cfg, err := config.Load(cfgPath); if err != nil { t.Fatalf("config: %v", err) }

	got, err := ComputeRemoteSHA256(context.Background(), cfg, url, nil)
	if err != nil { t.Fatalf("compute sha: %v", err) }
	if !strings.EqualFold(got, expected) {
		t.Fatalf("sha mismatch: got=%s want=%s", got, expected)
	}
}
