package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/atomic"

	"modfetch/internal/config"
)

type Manager struct {
	path   string
	ticker *time.Ticker
	stop   chan struct{}

	// counters
	bytesTotal       atomic.Int64
	retriesTotal     atomic.Int64
	downloadsSuccess atomic.Int64
	lastDownloadSec  atomic.Float64
	activeDownloads  atomic.Int64
}

func New(cfg *config.Config) *Manager {
	if cfg == nil || !cfg.Metrics.PrometheusTextfile.Enabled || cfg.Metrics.PrometheusTextfile.Path == "" {
		return nil
	}
	p := cfg.Metrics.PrometheusTextfile.Path
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	m := &Manager{path: p}
	m.Start()
	return m
}

// Start begins periodically writing metrics to the configured file.
func (m *Manager) Start() {
	if m == nil || m.ticker != nil {
		return
	}
	m.ticker = time.NewTicker(15 * time.Second)
	m.stop = make(chan struct{})
	go func() {
		for {
			select {
			case <-m.ticker.C:
				_ = m.Write()
			case <-m.stop:
				return
			}
		}
	}()
}

func (m *Manager) AddBytes(n int64) {
	if m == nil {
		return
	}
	m.bytesTotal.Add(n)
}

func (m *Manager) IncRetries(n int64) {
	if m == nil {
		return
	}
	m.retriesTotal.Add(n)
}

func (m *Manager) IncDownloadsSuccess() {
	if m == nil {
		return
	}
	m.downloadsSuccess.Add(1)
}

func (m *Manager) ObserveDownloadSeconds(sec float64) {
	if m == nil {
		return
	}
	m.lastDownloadSec.Store(sec)
}

func (m *Manager) IncActive(n int64) {
	if m == nil {
		return
	}
	for {
		cur := m.activeDownloads.Load()
		next := cur + n
		if next < 0 {
			next = 0
		}
		if m.activeDownloads.CompareAndSwap(cur, next) {
			return
		}
	}
}

func (m *Manager) Write() error {
	if m == nil {
		return nil
	}
	f, err := os.CreateTemp(filepath.Dir(m.path), ".metrics.tmp.*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	// Prometheus textfile format
	// Use modfetch_ prefix
	_, _ = fmt.Fprintf(f, "# HELP modfetch_bytes_downloaded_total Total bytes downloaded.\n")
	_, _ = fmt.Fprintf(f, "# TYPE modfetch_bytes_downloaded_total counter\n")
	_, _ = fmt.Fprintf(f, "modfetch_bytes_downloaded_total %d\n", m.bytesTotal.Load())

	_, _ = fmt.Fprintf(f, "# HELP modfetch_retries_total Total chunk retries.\n")
	_, _ = fmt.Fprintf(f, "# TYPE modfetch_retries_total counter\n")
	_, _ = fmt.Fprintf(f, "modfetch_retries_total %d\n", m.retriesTotal.Load())

	_, _ = fmt.Fprintf(f, "# HELP modfetch_downloads_success_total Total successful downloads.\n")
	_, _ = fmt.Fprintf(f, "# TYPE modfetch_downloads_success_total counter\n")
	_, _ = fmt.Fprintf(f, "modfetch_downloads_success_total %d\n", m.downloadsSuccess.Load())

	_, _ = fmt.Fprintf(f, "# HELP modfetch_last_download_seconds Duration of the last completed download in seconds.\n")
	_, _ = fmt.Fprintf(f, "# TYPE modfetch_last_download_seconds gauge\n")
	_, _ = fmt.Fprintf(f, "modfetch_last_download_seconds %.6f\n", m.lastDownloadSec.Load())

	_, _ = fmt.Fprintf(f, "# HELP modfetch_active_downloads Number of active downloads.\n")
	_, _ = fmt.Fprintf(f, "# TYPE modfetch_active_downloads gauge\n")
	_, _ = fmt.Fprintf(f, "modfetch_active_downloads %d\n", m.activeDownloads.Load())

	_, _ = fmt.Fprintf(f, "# HELP modfetch_metrics_timestamp_seconds UNIX timestamp when this file was written.\n")
	_, _ = fmt.Fprintf(f, "# TYPE modfetch_metrics_timestamp_seconds gauge\n")
	_, _ = fmt.Fprintf(f, "modfetch_metrics_timestamp_seconds %d\n", time.Now().Unix())

	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), m.path)
}

// Close stops background metric collection and flushes the latest values to disk.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	if m.ticker != nil {
		m.ticker.Stop()
	}
	if m.stop != nil {
		close(m.stop)
	}
	_ = m.Write()
}
