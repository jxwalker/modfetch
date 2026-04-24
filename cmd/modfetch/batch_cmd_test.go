package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jxwalker/modfetch/internal/batch"
)

func TestBatchImport_FromTextURLs_NoNetworkRequired(t *testing.T) {
	// Prepare temp dirs/files
	d := t.TempDir()
	cfgPath := filepath.Join(d, "config.yaml")
	downloadRoot := filepath.Join(d, "dlroot")
	if err := os.MkdirAll(downloadRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "version: 1\n" +
		"general:\n" +
		"  data_root: " + fmt.Sprintf("%q", d) + "\n" +
		"  download_root: " + fmt.Sprintf("%q", downloadRoot) + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	inPath := filepath.Join(d, "input.txt")
	// Use simple URLs; importer tolerates probe failures
	input := strings.Join([]string{
		"https://example.com/a.bin",
		"https://example.com/b.bin",
		"https://example.com/path/c.bin",
	}, "\n")
	if err := os.WriteFile(inPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(d, "batch.yaml")
	args := []string{
		"--config", cfgPath,
		"--input", inPath,
		"--output", outPath,
		"--dest-dir", downloadRoot,
		"--sha-mode", "none",
	}
	if err := handleBatchImport(context.Background(), args); err != nil {
		t.Fatalf("batch import failed: %v", err)
	}

	bf, err := batch.Load(outPath)
	if err != nil {
		t.Fatalf("load batch: %v", err)
	}
	if bf.Version != 1 {
		t.Fatalf("bad version: %d", bf.Version)
	}
	if len(bf.Jobs) != 3 {
		t.Fatalf("expected 3 jobs; got %d", len(bf.Jobs))
	}
	for i, j := range bf.Jobs {
		if !strings.HasPrefix(j.Dest, downloadRoot) {
			t.Fatalf("job %d dest not under dest-dir: %s", i, j.Dest)
		}
		if strings.TrimSpace(j.URI) == "" {
			t.Fatalf("job %d empty uri", i)
		}
	}
}

func TestDownloadBatch_UsesOrderedMirrorFallback(t *testing.T) {
	d := t.TempDir()
	cfgPath := filepath.Join(d, "config.yaml")
	downloadRoot := filepath.Join(d, "downloads")
	if err := os.MkdirAll(downloadRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "version: 1\n" +
		"general:\n" +
		"  data_root: " + fmt.Sprintf("%q", d) + "\n" +
		"  download_root: " + fmt.Sprintf("%q", downloadRoot) + "\n" +
		"concurrency:\n" +
		"  max_retries: 1\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer primary.Close()
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("mirror ok"))
	}))
	defer mirror.Close()

	dest := filepath.Join(downloadRoot, "model.bin")
	batchPath := filepath.Join(d, "jobs.yaml")
	batchYAML := "version: 1\njobs:\n" +
		"  - uri: \"" + primary.URL + "/model.bin\"\n" +
		"    mirrors:\n" +
		"      - \"" + mirror.URL + "/model.bin\"\n" +
		"    dest: \"" + dest + "\"\n"
	if err := os.WriteFile(batchPath, []byte(batchYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	args := []string{"--config", cfgPath, "--batch", batchPath, "--quiet"}
	if err := handleDownload(context.Background(), args); err != nil {
		t.Fatalf("download batch failed: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != "mirror ok" {
		t.Fatalf("expected mirror content, got %q", string(got))
	}
}

func TestDownloadBatch_EnqueuesHigherPriorityFirst(t *testing.T) {
	d := t.TempDir()
	cfgPath := filepath.Join(d, "config.yaml")
	downloadRoot := filepath.Join(d, "downloads")
	if err := os.MkdirAll(downloadRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "version: 1\n" +
		"general:\n" +
		"  data_root: " + fmt.Sprintf("%q", d) + "\n" +
		"  download_root: " + fmt.Sprintf("%q", downloadRoot) + "\n" +
		"concurrency:\n" +
		"  per_host_requests: 1\n" +
		"  max_retries: 1\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var order []string
	mkServer := func(name, body string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
				w.WriteHeader(http.StatusOK)
				return
			}
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			_, _ = w.Write([]byte(body))
		}))
	}
	low := mkServer("low", "low")
	defer low.Close()
	high := mkServer("high", "high")
	defer high.Close()

	batchPath := filepath.Join(d, "jobs.yaml")
	batchYAML := "version: 1\njobs:\n" +
		"  - uri: \"" + low.URL + "/model.bin\"\n" +
		"    priority: 1\n" +
		"    dest: \"" + filepath.Join(downloadRoot, "low.bin") + "\"\n" +
		"  - uri: \"" + high.URL + "/model.bin\"\n" +
		"    priority: 10\n" +
		"    dest: \"" + filepath.Join(downloadRoot, "high.bin") + "\"\n"
	if err := os.WriteFile(batchPath, []byte(batchYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	args := []string{"--config", cfgPath, "--batch", batchPath, "--batch-parallel", "1", "--quiet"}
	if err := handleDownload(context.Background(), args); err != nil {
		t.Fatalf("download batch failed: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	firstHigh, firstLow := -1, -1
	for i, name := range order {
		if name == "high" && firstHigh < 0 {
			firstHigh = i
		}
		if name == "low" && firstLow < 0 {
			firstLow = i
		}
	}
	if firstHigh < 0 || firstLow < 0 || firstHigh > firstLow {
		t.Fatalf("expected high priority first, got %v", order)
	}
}
