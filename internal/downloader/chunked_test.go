package downloader

import (
	"context"
	"os"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/state"
	"github.com/jxwalker/modfetch/internal/util"
)

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
