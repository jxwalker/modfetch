package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBenchModfetchRunsRealDownloadSample(t *testing.T) {
	ts := newBenchRangeServer(t, 512*1024)
	defer ts.Close()
	cfgPath := writeBenchConfig(t)

	var runErr error
	out := captureStdout(t, func() {
		runErr = handleBench(context.Background(), []string{
			"--config", cfgPath,
			"--url", ts.URL + "/model.bin",
			"--tools", "modfetch",
			"--duration", "2s",
			"--json",
			"--profile", "default",
		})
	})
	if runErr != nil {
		t.Fatalf("bench: %v", runErr)
	}
	var got benchSummary
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode bench JSON: %v\n%s", err, out)
	}
	if len(got.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(got.Results))
	}
	result := got.Results[0]
	if result.Tool != "modfetch" {
		t.Fatalf("tool = %q, want modfetch", result.Tool)
	}
	if result.Status == "error" {
		t.Fatalf("modfetch bench errored: %s", result.Error)
	}
	if result.Bytes <= 0 || result.AvgBPS <= 0 {
		t.Fatalf("expected positive bytes/rate, got %+v", result)
	}
}

func TestBenchAria2RunsWhenInstalled(t *testing.T) {
	if _, err := exec.LookPath("aria2c"); err != nil {
		t.Skip("aria2c not installed")
	}
	ts := newBenchRangeServer(t, 512*1024)
	defer ts.Close()
	cfgPath := writeBenchConfig(t)

	var runErr error
	out := captureStdout(t, func() {
		runErr = handleBench(context.Background(), []string{
			"--config", cfgPath,
			"--url", ts.URL + "/model.bin",
			"--tools", "aria2",
			"--duration", "2s",
			"--json",
			"--connections", "4",
			"--chunk-size-mb", "1",
		})
	})
	if runErr != nil {
		t.Fatalf("bench aria2: %v", runErr)
	}
	var got benchSummary
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode bench JSON: %v\n%s", err, out)
	}
	if len(got.Results) != 1 || got.Results[0].Tool != "aria2" {
		t.Fatalf("unexpected results: %+v", got.Results)
	}
	if got.Results[0].Status == "error" {
		t.Fatalf("aria2 bench errored: %s", got.Results[0].Error)
	}
	if got.Results[0].Bytes <= 0 {
		t.Fatalf("aria2 bytes = %d, want > 0", got.Results[0].Bytes)
	}
}

func TestParseBenchToolsDedupesAndDefaults(t *testing.T) {
	got := parseBenchTools(" modfetch,aria2,modfetch ,,")
	want := []string{"modfetch", "aria2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("parseBenchTools = %v, want %v", got, want)
	}
	got = parseBenchTools(" , ")
	if len(got) != 1 || got[0] != "modfetch" {
		t.Fatalf("empty parseBenchTools = %v, want modfetch", got)
	}
}

func writeBenchConfig(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	cfgBody := "version: 1\n" +
		"general:\n" +
		"  data_root: " + strconv.Quote(filepath.Join(tmp, "data")) + "\n" +
		"  download_root: " + strconv.Quote(filepath.Join(tmp, "downloads")) + "\n" +
		"concurrency:\n" +
		"  per_file_chunks: 4\n" +
		"  per_host_requests: 4\n" +
		"  chunk_size_mb: 1\n"
	if err := os.WriteFile(cfgPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return cfgPath
}

func newBenchRangeServer(t *testing.T, size int) *httptest.Server {
	t.Helper()
	body := bytes.Repeat([]byte("a"), size)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		if r.Method == http.MethodHead {
			return
		}
		if rng := r.Header.Get("Range"); rng != "" {
			var start, end int
			if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err == nil {
				if end >= len(body) || end < 0 {
					end = len(body) - 1
				}
				if start < 0 {
					start = 0
				}
				if start <= end && start < len(body) {
					w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(body)))
					w.WriteHeader(http.StatusPartialContent)
					_, _ = w.Write(body[start : end+1])
					return
				}
			}
		}
		_, _ = w.Write(body)
	}))
}
