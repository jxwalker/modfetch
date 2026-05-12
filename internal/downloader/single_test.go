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
)

func TestSingleCancelPreservesPartialForResume(t *testing.T) {
	const size = 1024 * 1024
	data := bytes.Repeat([]byte("abcdef0123456789"), size/16)
	sum := sha256.Sum256(data)
	wantSHA := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		if r.Method == http.MethodHead {
			return
		}
		start, end := int64(0), int64(len(data)-1)
		if rg := strings.TrimSpace(r.Header.Get("Range")); rg != "" {
			if _, err := fmt.Sscanf(rg, "bytes=%d-", &start); err != nil {
				http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			if start < 0 || start >= int64(len(data)) {
				http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
			w.WriteHeader(http.StatusPartialContent)
		}
		chunk := int64(16 * 1024)
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
		t.Fatalf("state open: %v", err)
	}
	defer func() { _ = st.SQL.Close() }()

	dl := NewSingle(cfg, log, st, nil)
	url := srv.URL + "/single.bin"
	dest := filepath.Join(cfg.General.DownloadRoot, "single.bin")
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, _, err := dl.Download(ctx, url, dest, "", nil, false)
		errCh <- err
	}()

	part := stagePartPath(cfg, url, dest)
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			cancel()
			t.Fatal("timed out waiting for partial bytes")
		default:
		}
		if fi, err := os.Stat(part); err == nil && fi.Size() >= 64*1024 {
			cancel()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled download error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for canceled download to return")
	}
	if fi, err := os.Stat(part); err != nil {
		t.Fatalf("expected partial to be preserved: %v", err)
	} else if fi.Size() == 0 || fi.Size() >= size {
		t.Fatalf("partial size = %d, want between 1 and %d", fi.Size(), size-1)
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
}

// Integration test that downloads a tiny public text file from Hugging Face.
func TestSingleDownloadHF(t *testing.T) {
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
		"  allow_overwrite: true\n")
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
		t.Fatalf("state open: %v", err)
	}
	defer func() { _ = st.SQL.Close() }()

	dl := NewSingle(cfg, log, st, nil)
	final, sum, err := dl.Download(context.Background(), "https://raw.githubusercontent.com/github/gitignore/main/Go.gitignore", "", "", nil, false)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if _, err := os.Stat(final); err != nil {
		t.Fatalf("final file missing: %v", err)
	}
	if len(sum) != 64 {
		t.Fatalf("unexpected sha256 length: %d", len(sum))
	}
}
