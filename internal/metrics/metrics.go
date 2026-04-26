package metrics

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/jxwalker/modfetch/internal/config"
)

type Manager struct {
	path   string
	ticker *time.Ticker
	stop   chan struct{}
	once   sync.Once

	// counters
	bytesTotal       atomic.Int64
	retriesTotal     atomic.Int64
	downloadsSuccess atomic.Int64
	lastDownloadSec  atomic.Float64
	downloadSecSum   atomic.Float64
	downloadSecCount atomic.Int64
	downloadSecLe1   atomic.Int64
	downloadSecLe5   atomic.Int64
	downloadSecLe30  atomic.Int64
	downloadSecLe120 atomic.Int64
	downloadSecLe600 atomic.Int64
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
	if sec < 0 || math.IsNaN(sec) || math.IsInf(sec, 0) {
		return
	}
	m.lastDownloadSec.Store(sec)
	m.downloadSecSum.Add(sec)
	m.downloadSecCount.Add(1)
	if sec <= 1 {
		m.downloadSecLe1.Add(1)
	}
	if sec <= 5 {
		m.downloadSecLe5.Add(1)
	}
	if sec <= 30 {
		m.downloadSecLe30.Add(1)
	}
	if sec <= 120 {
		m.downloadSecLe120.Add(1)
	}
	if sec <= 600 {
		m.downloadSecLe600.Add(1)
	}
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
	if m == nil || m.path == "" {
		return nil
	}
	f, err := os.CreateTemp(filepath.Dir(m.path), ".metrics.tmp.*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	// Prometheus textfile format
	// Use modfetch_ prefix
	if _, err := fmt.Fprintf(f, "# HELP modfetch_bytes_downloaded_total Total bytes downloaded.\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "# TYPE modfetch_bytes_downloaded_total counter\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "modfetch_bytes_downloaded_total %d\n", m.bytesTotal.Load()); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(f, "# HELP modfetch_retries_total Total chunk retries.\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "# TYPE modfetch_retries_total counter\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "modfetch_retries_total %d\n", m.retriesTotal.Load()); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(f, "# HELP modfetch_downloads_success_total Total successful downloads.\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "# TYPE modfetch_downloads_success_total counter\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "modfetch_downloads_success_total %d\n", m.downloadsSuccess.Load()); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(f, "# HELP modfetch_last_download_seconds Duration of the last completed download in seconds.\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "# TYPE modfetch_last_download_seconds gauge\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "modfetch_last_download_seconds %.6f\n", m.lastDownloadSec.Load()); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(f, "# HELP modfetch_download_seconds Duration distribution for completed downloads in seconds.\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "# TYPE modfetch_download_seconds histogram\n"); err != nil {
		return err
	}
	count := m.downloadSecCount.Load()
	buckets := []struct {
		le    string
		count int64
	}{
		{le: "1", count: m.downloadSecLe1.Load()},
		{le: "5", count: m.downloadSecLe5.Load()},
		{le: "30", count: m.downloadSecLe30.Load()},
		{le: "120", count: m.downloadSecLe120.Load()},
		{le: "600", count: m.downloadSecLe600.Load()},
		{le: "+Inf", count: count},
	}
	for _, bucket := range buckets {
		if _, err := fmt.Fprintf(f, "modfetch_download_seconds_bucket{le=%q} %d\n", bucket.le, bucket.count); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(f, "modfetch_download_seconds_sum %.6f\n", m.downloadSecSum.Load()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "modfetch_download_seconds_count %d\n", count); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(f, "# HELP modfetch_active_downloads Number of active downloads.\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "# TYPE modfetch_active_downloads gauge\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "modfetch_active_downloads %d\n", m.activeDownloads.Load()); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(f, "# HELP modfetch_metrics_timestamp_seconds UNIX timestamp when this file was written.\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "# TYPE modfetch_metrics_timestamp_seconds gauge\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "modfetch_metrics_timestamp_seconds %d\n", time.Now().Unix()); err != nil {
		return err
	}

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
	m.once.Do(func() {
		if m.ticker != nil {
			m.ticker.Stop()
		}
		if m.stop != nil {
			close(m.stop)
		}
		_ = m.Write()
	})
}
