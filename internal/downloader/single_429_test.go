package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

// Ensure Single.Download marks job as hold with a clear retry-after message on 429.
func TestSingle_429RateLimited_HoldWithRetryAfter(t *testing.T) {
	tmp := t.TempDir()
	mux := http.NewServeMux()
	mux.HandleFunc("/rl.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "1234")
			w.WriteHeader(200)
			return
		}
		// For GET: return 429 and include Retry-After
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	url := ts.URL + "/rl.bin"

	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte(strings.Join([]string{
		"version: 1",
		"general:",
		"  data_root: \"" + tmp + "/data\"",
		"  download_root: \"" + tmp + "/dl\"",
	}, "\n"))
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	cfg, err := config.Load(cfgPath)
	if err != nil { t.Fatalf("config: %v", err) }
	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil { t.Fatalf("state: %v", err) }
	defer st.SQL.Close()

	dl := NewSingle(cfg, log, st, nil)
	_, _, err = dl.Download(context.Background(), url, "", "", nil, false)
	if err == nil {
		t.Fatalf("expected error on 429 rate limited")
	}
	rows, err := st.ListDownloads()
	if err != nil { t.Fatalf("list: %v", err) }
	found := false
	for _, r := range rows {
		if r.URL == url {
			found = true
			if strings.ToLower(strings.TrimSpace(r.Status)) != "hold" {
				t.Fatalf("expected Status=hold, got %q", r.Status)
			}
			le := strings.ToLower(r.LastError)
			if !strings.Contains(le, "429") || !strings.Contains(le, "rate limited") {
				t.Fatalf("last_error missing 429/rate limited: %q", r.LastError)
			}
			if !strings.Contains(le, "retry-after=60") {
				t.Fatalf("last_error missing retry-after hint: %q", r.LastError)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected a downloads row for %s", url)
	}
}

