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
	m.ObserveDownloadSeconds(45)
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
		"modfetch_last_download_seconds 45.000000",
		`modfetch_download_seconds_bucket{le="1"} 0`,
		`modfetch_download_seconds_bucket{le="5"} 1`,
		`modfetch_download_seconds_bucket{le="30"} 1`,
		`modfetch_download_seconds_bucket{le="120"} 2`,
		`modfetch_download_seconds_bucket{le="600"} 2`,
		`modfetch_download_seconds_bucket{le="+Inf"} 2`,
		"modfetch_download_seconds_sum 46.250000",
		"modfetch_download_seconds_count 2",
		"modfetch_active_downloads 1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("metrics output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveDownloadSecondsIgnoresInvalidValues(t *testing.T) {
	m := &Manager{}
	m.ObserveDownloadSeconds(-1)
	m.ObserveDownloadSeconds(0.5)

	if got := m.downloadSecCount.Load(); got != 1 {
		t.Fatalf("download count = %d, want 1", got)
	}
	if got := m.downloadSecLe1.Load(); got != 1 {
		t.Fatalf("<=1s bucket = %d, want 1", got)
	}
}
