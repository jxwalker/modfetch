package downloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/state"
	"github.com/jxwalker/modfetch/internal/util"
)

func TestChunkedCancelPreservesPartialAndChunksForResume(t *testing.T) {
	const size = 3 * 1024 * 1024
	data := bytes.Repeat([]byte("0123456789abcdef"), size/16)
	sum := sha256.Sum256(data)
	wantSHA := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", `"range-test"`)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		if r.Method == http.MethodHead {
			return
		}
		start, end := int64(0), int64(len(data)-1)
		if rg := strings.TrimSpace(r.Header.Get("Range")); rg != "" {
			if _, err := fmt.Sscanf(rg, "bytes=%d-%d", &start, &end); err != nil {
				http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			if start < 0 || end < start || end >= int64(len(data)) {
				http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
			w.WriteHeader(http.StatusPartialContent)
		}
		chunk := int64(32 * 1024)
		for off := start; off <= end; off += chunk {
			next := off + chunk
			if next > end+1 {
				next = end + 1
			}
			if _, err := w.Write(data[off:next]); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(2 * time.Millisecond)
		}
	}))
	defer srv.Close()

	tmp := t.TempDir()
	cfg := writeChunkedTestConfig(t, tmp, 1)
	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	defer func() { _ = st.SQL.Close() }()

	dl := NewChunked(cfg, log, st, nil)
	url := srv.URL + "/model.gguf"
	dest := filepath.Join(cfg.General.DownloadRoot, "model.gguf")
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, _, err := dl.Download(ctx, url, dest, "", nil, false)
		errCh <- err
	}()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			cancel()
			t.Fatal("timed out waiting for first completed chunk")
		default:
		}
		chunks, err := st.ListChunks(url, dest)
		if err != nil {
			t.Fatalf("list chunks: %v", err)
		}
		if countChunkStatus(chunks, "complete") >= 1 {
			cancel()
			goto canceled
		}
		time.Sleep(10 * time.Millisecond)
	}

canceled:
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled download error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for canceled download to return")
	}

	part := stagePartPath(cfg, url, dest)
	if fi, err := os.Stat(part); err != nil {
		t.Fatalf("expected staged partial to be preserved at %s: %v", part, err)
	} else if fi.Size() != size {
		t.Fatalf("staged partial logical size = %d, want %d", fi.Size(), size)
	}
	chunks, err := st.ListChunks(url, dest)
	if err != nil {
		t.Fatalf("list chunks after cancel: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("chunk plan length after cancel = %d, want 3", len(chunks))
	}
	if complete := countChunkStatus(chunks, "complete"); complete == 0 {
		t.Fatalf("expected at least one completed chunk to be preserved, got %+v", chunks)
	}
	if running := countChunkStatus(chunks, "running"); running != 0 {
		t.Fatalf("running chunks should be reset for resume, got %+v", chunks)
	}

	final, gotSHA, err := dl.Download(context.Background(), url, dest, wantSHA, nil, false)
	if err != nil {
		t.Fatalf("resume download: %v", err)
	}
	if final != dest {
		t.Fatalf("final path = %s, want %s", final, dest)
	}
	if gotSHA != wantSHA {
		t.Fatalf("sha = %s, want %s", gotSHA, wantSHA)
	}
	if _, err := os.Stat(part); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged partial should be removed after completion, stat err=%v", err)
	}
}

func writeChunkedTestConfig(t *testing.T, tmp string, perFileChunks int) *config.Config {
	t.Helper()
	cfgYaml := []byte(fmt.Sprintf(`version: 1
general:
  data_root: %q
  download_root: %q
  placement_mode: "symlink"
  quarantine: false
  allow_overwrite: true
concurrency:
  per_file_chunks: %d
  chunk_size_mb: 1
`, filepath.Join(tmp, "data"), filepath.Join(tmp, "dl"), perFileChunks))
	cfgPath := filepath.Join(tmp, "config.yml")
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	return cfg
}

func countChunkStatus(chunks []state.ChunkRow, status string) int {
	var n int
	for _, c := range chunks {
		if strings.EqualFold(c.Status, status) {
			n++
		}
	}
	return n
}

// Integration: chunked download of a 1MB test file with multiple chunks.
func TestChunkedDownload1MB(t *testing.T) {
	if testing.Short() {
		t.Skip("-short set")
	}
	tmp := t.TempDir()
	cfgYaml := []byte("" +
		"version: 1\n" +
		"general:\n" +
		"  data_root: \"" + tmp + "/data\"\n" +
		"  download_root: \"" + tmp + "/dl\"\n" +
		"  placement_mode: \"symlink\"\n" +
		"  quarantine: false\n" +
		"  allow_overwrite: true\n" +
		"concurrency:\n" +
		"  per_file_chunks: 4\n" +
		"  chunk_size_mb: 1\n") // 1 MB chunks
	cfgPath := tmp + "/config.yml"
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	defer func() { _ = st.SQL.Close() }()

	dl := NewChunked(cfg, log, st, nil)
	url := "https://proof.ovh.net/files/1Mb.dat"
	dest, sha, err := dl.Download(context.Background(), url, "", "", nil, false)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Size() < 1000000 {
		t.Fatalf("expected ~1MB, got %d", fi.Size())
	}
	if len(sha) != 64 {
		t.Fatalf("sha length %d", len(sha))
	}
}

// Integration: corrupt a chunk and ensure repair when expected SHA is provided.
func TestChunkedCorruptAndRepair(t *testing.T) {
	if testing.Short() {
		t.Skip("-short set")
	}
	tmp := t.TempDir()
	cfgYaml := []byte("" +
		"version: 1\n" +
		"general:\n" +
		"  data_root: \"" + tmp + "/data\"\n" +
		"  download_root: \"" + tmp + "/dl\"\n" +
		"  placement_mode: \"symlink\"\n" +
		"  quarantine: false\n" +
		"  allow_overwrite: true\n" +
		"concurrency:\n" +
		"  per_file_chunks: 8\n" +
		"  chunk_size_mb: 1\n")
	cfgPath := tmp + "/config.yml"
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	defer func() { _ = st.SQL.Close() }()

	dl := NewChunked(cfg, log, st, nil)
	url := "https://proof.ovh.net/files/1Mb.dat"
	// First download to get good SHA
	dest, goodSHA, err := dl.Download(context.Background(), url, "", "", nil, false)
	if err != nil {
		t.Fatalf("download1: %v", err)
	}
	// Corrupt middle 64 bytes
	f, err := os.OpenFile(dest, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open dest: %v", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(512*1024, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	bad := make([]byte, 64)
	for i := range bad {
		bad[i] = 0xFF
	}
	if _, err := f.Write(bad); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	// Run downloader again with expected SHA; it should repair the corrupted chunk
	_, fixedSHA, err := dl.Download(context.Background(), url, dest, goodSHA, nil, false)
	if err != nil {
		t.Fatalf("download2(repair): %v", err)
	}
	if fixedSHA != goodSHA {
		// confirm by local recompute
		s, err := util.HashFileSHA256(dest)
		if err != nil {
			t.Fatalf("hash file: %v", err)
		}
		if s != goodSHA {
			t.Fatalf("sha not repaired: got=%s want=%s", s, goodSHA)
		}
	}
}
