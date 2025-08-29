package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"modfetch/internal/config"
)

type Manager struct {
	path string
	mu   sync.Mutex
	// counters
	bytesTotal       int64
	retriesTotal     int64
	downloadsSuccess int64
	lastDownloadSec  float64
}

func New(cfg *config.Config) *Manager {
	if cfg == nil || !cfg.Metrics.PrometheusTextfile.Enabled || cfg.Metrics.PrometheusTextfile.Path == "" {
		return nil
	}
	p := cfg.Metrics.PrometheusTextfile.Path
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	return &Manager{path: p}
}

func (m *Manager) AddBytes(n int64) {
	if m == nil { return }
	m.mu.Lock(); m.bytesTotal += n; m.mu.Unlock()
}

func (m *Manager) IncRetries(n int64) {
	if m == nil { return }
	m.mu.Lock(); m.retriesTotal += n; m.mu.Unlock()
}

func (m *Manager) IncDownloadsSuccess() {
	if m == nil { return }
	m.mu.Lock(); m.downloadsSuccess++; m.mu.Unlock()
}

func (m *Manager) ObserveDownloadSeconds(sec float64) {
	if m == nil { return }
	m.mu.Lock(); m.lastDownloadSec = sec; m.mu.Unlock()
}

func (m *Manager) Write() error {
	if m == nil { return nil }
	m.mu.Lock(); defer m.mu.Unlock()
	f, err := os.CreateTemp(filepath.Dir(m.path), ".metrics.tmp.*")
	if err != nil { return err }
	defer os.Remove(f.Name())
	// Prometheus textfile format
	// Use modfetch_ prefix
	fmt.Fprintf(f, "# HELP modfetch_bytes_downloaded_total Total bytes downloaded.\n")
	fmt.Fprintf(f, "# TYPE modfetch_bytes_downloaded_total counter\n")
	fmt.Fprintf(f, "modfetch_bytes_downloaded_total %d\n", m.bytesTotal)

	fmt.Fprintf(f, "# HELP modfetch_retries_total Total chunk retries.\n")
	fmt.Fprintf(f, "# TYPE modfetch_retries_total counter\n")
	fmt.Fprintf(f, "modfetch_retries_total %d\n", m.retriesTotal)

	fmt.Fprintf(f, "# HELP modfetch_downloads_success_total Total successful downloads.\n")
	fmt.Fprintf(f, "# TYPE modfetch_downloads_success_total counter\n")
	fmt.Fprintf(f, "modfetch_downloads_success_total %d\n", m.downloadsSuccess)

	fmt.Fprintf(f, "# HELP modfetch_last_download_seconds Duration of the last completed download in seconds.\n")
	fmt.Fprintf(f, "# TYPE modfetch_last_download_seconds gauge\n")
	fmt.Fprintf(f, "modfetch_last_download_seconds %.6f\n", m.lastDownloadSec)

	fmt.Fprintf(f, "# HELP modfetch_metrics_timestamp_seconds UNIX timestamp when this file was written.\n")
	fmt.Fprintf(f, "# TYPE modfetch_metrics_timestamp_seconds gauge\n")
	fmt.Fprintf(f, "modfetch_metrics_timestamp_seconds %d\n", time.Now().Unix())

	if err := f.Close(); err != nil { return err }
	return os.Rename(f.Name(), m.path)
}

