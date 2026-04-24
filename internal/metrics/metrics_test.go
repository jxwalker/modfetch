package metrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestNewDisabledReturnsNil(t *testing.T) {
	cfg := &config.Config{}
	if got := New(cfg); got != nil {
		t.Fatalf("New() = %#v, want nil when metrics are disabled", got)
	}
}

func TestWriteNoopsWithEmptyPath(t *testing.T) {
	m := &Manager{}
	m.AddBytes(100)
	if err := m.Write(); err != nil {
		t.Fatalf("Write() with empty path: %v", err)
	}
}

func TestCloseIsIdempotentAndWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "modfetch.prom")
	cfg := &config.Config{}
	cfg.Metrics.PrometheusTextfile.Enabled = true
	cfg.Metrics.PrometheusTextfile.Path = path

	m := New(cfg)
	if m == nil {
		t.Fatal("New() returned nil")
	}
	m.AddBytes(42)
	m.IncRetries(2)
	m.IncDownloadsSuccess()
	m.ObserveDownloadSeconds(1.25)
	m.IncActive(1)

	m.Close()
	m.Close()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read metrics file: %v", err)
	}
	out := string(b)
	for _, want := range []string{
		"modfetch_bytes_downloaded_total 42",
		"modfetch_retries_total 2",
		"modfetch_downloads_success_total 1",
		"modfetch_last_download_seconds 1.250000",
		"modfetch_active_downloads 1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("metrics output missing %q:\n%s", want, out)
		}
	}
}
