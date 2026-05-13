package downloader

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/state"
)

const (
	adaptiveMinLimit       = 1
	adaptiveProgressWindow = 2 * time.Second
	adaptiveStallWindow    = 3 * time.Second
)

type adaptiveTransferController struct {
	cfg  *config.Config
	log  *logging.Logger
	st   *state.DB
	host string

	mu             sync.Mutex
	limit          int
	max            int
	active         int
	totalBytes     int64
	windowBytes    int64
	lastProgress   time.Time
	lastAdjust     time.Time
	windowStarted  time.Time
	emaBPS         float64
	rateLimited    bool
	stopped        bool
	stopCh         chan struct{}
	monitorStopped chan struct{}
}

func newAdaptiveTransferController(cfg *config.Config, log *logging.Logger, st *state.DB, rawURL string, max int) *adaptiveTransferController {
	if max <= 0 {
		max = 1
	}
	host := hostFromURL(rawURL)
	now := time.Now()
	c := &adaptiveTransferController{
		cfg:            cfg,
		log:            log,
		st:             st,
		host:           strings.ToLower(host),
		limit:          initialAdaptiveLimit(st, host, max),
		max:            max,
		lastProgress:   now,
		lastAdjust:     now,
		windowStarted:  now,
		stopCh:         make(chan struct{}),
		monitorStopped: make(chan struct{}),
	}
	go c.monitor()
	return c
}

func initialAdaptiveLimit(st *state.DB, host string, max int) int {
	if max <= adaptiveMinLimit {
		return max
	}
	if st != nil {
		if row, ok, err := st.BestTransferHistory(strings.ToLower(host), "modfetch"); err == nil && ok && row.Connections > 0 {
			connections := row.Connections
			if row.RateLimited {
				connections = (connections + 1) / 2
			}
			return clampInt(connections, adaptiveMinLimit, max)
		}
	}
	if max <= 4 {
		return max
	}
	return 4
}

func (c *adaptiveTransferController) acquire(ctx context.Context) error {
	for {
		c.mu.Lock()
		if c.stopped {
			c.mu.Unlock()
			return context.Canceled
		}
		if c.active < c.limit {
			c.active++
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func (c *adaptiveTransferController) release() {
	c.mu.Lock()
	if c.active > 0 {
		c.active--
	}
	c.mu.Unlock()
}

func (c *adaptiveTransferController) observeBytes(n int64) {
	if n <= 0 {
		return
	}
	c.mu.Lock()
	now := time.Now()
	c.totalBytes += n
	c.windowBytes += n
	c.lastProgress = now
	c.mu.Unlock()
}

func (c *adaptiveTransferController) observeRateLimit() {
	c.mu.Lock()
	c.rateLimited = true
	old := c.limit
	c.limit = clampInt((c.limit+1)/2, adaptiveMinLimit, c.max)
	c.lastAdjust = time.Now()
	c.windowStarted = c.lastAdjust
	c.windowBytes = 0
	c.mu.Unlock()
	if old != c.limit && c.log != nil {
		c.log.Warnf("adaptive transfer: 429 backoff for %s, connections %d -> %d", c.host, old, c.limit)
	}
}

func (c *adaptiveTransferController) monitor() {
	ticker := time.NewTicker(adaptiveProgressWindow)
	defer func() {
		ticker.Stop()
		close(c.monitorStopped)
	}()
	for {
		select {
		case <-ticker.C:
			c.adjust()
		case <-c.stopCh:
			return
		}
	}
}

func (c *adaptiveTransferController) adjust() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	now := time.Now()
	elapsed := now.Sub(c.windowStarted)
	if elapsed <= 0 {
		return
	}
	old := c.limit
	if c.active > 0 && now.Sub(c.lastProgress) >= adaptiveStallWindow {
		c.limit = clampInt((c.limit+1)/2, adaptiveMinLimit, c.max)
		c.lastAdjust = now
		c.windowStarted = now
		c.windowBytes = 0
		c.logLimitChangeLocked("stall backoff", old, c.limit, 0)
		return
	}
	bps := float64(c.windowBytes) / elapsed.Seconds()
	if bps <= 0 {
		return
	}
	if c.emaBPS == 0 {
		c.emaBPS = bps
	} else {
		c.emaBPS = 0.70*c.emaBPS + 0.30*bps
	}
	if now.Sub(c.lastAdjust) < adaptiveProgressWindow {
		c.windowStarted = now
		c.windowBytes = 0
		return
	}
	switch {
	case c.limit > adaptiveMinLimit && c.emaBPS > 0 && bps < c.emaBPS*0.55:
		c.limit--
		c.lastAdjust = now
	case c.active >= c.limit && c.limit < c.max && bps >= c.emaBPS*0.90:
		c.limit++
		c.lastAdjust = now
	}
	c.windowStarted = now
	c.windowBytes = 0
	if c.limit != old {
		c.logLimitChangeLocked("throughput", old, c.limit, bps)
	}
}

func (c *adaptiveTransferController) stop() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.stopped = true
	close(c.stopCh)
	c.mu.Unlock()
	<-c.monitorStopped
}

func (c *adaptiveTransferController) wasRateLimited() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rateLimited
}

func (c *adaptiveTransferController) finalLimit() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.limit
}

func (c *adaptiveTransferController) logLimitChangeLocked(reason string, old, next int, bps float64) {
	if old == next || c.log == nil {
		return
	}
	if bps > 0 {
		c.log.Infof("adaptive transfer: %s for %s, connections %d -> %d at %s/s", reason, c.host, old, next, humanize.Bytes(uint64(bps)))
		return
	}
	c.log.Infof("adaptive transfer: %s for %s, connections %d -> %d", reason, c.host, old, next)
}

type adaptiveProgressReader struct {
	r       io.Reader
	onBytes func(int64)
}

func (r *adaptiveProgressReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 && r.onBytes != nil {
		r.onBytes(int64(n))
	}
	return n, err
}

func recordModfetchTransferHistory(st *state.DB, rawURL string, connections, chunkSizeMB int, bytes int64, elapsed time.Duration, status string, rateLimited bool, lastErr string) {
	if st == nil || bytes <= 0 || elapsed <= 0 {
		return
	}
	host := strings.ToLower(hostFromURL(rawURL))
	if host == "" {
		return
	}
	_ = st.UpsertTransferHistory(state.TransferHistoryRow{
		Host:        host,
		Tool:        "modfetch",
		Connections: connections,
		ChunkSizeMB: chunkSizeMB,
		AvgBPS:      float64(bytes) / elapsed.Seconds(),
		RateLimited: rateLimited,
		LastStatus:  status,
		LastError:   lastErr,
	})
}

func chunkSizeMBForHistory(cfg *config.Config) int {
	if cfg == nil || cfg.Concurrency.ChunkSizeMB <= 0 {
		return 0
	}
	return cfg.Concurrency.ChunkSizeMB
}

func HostFromURLForHistory(rawURL string) string {
	return strings.ToLower(hostFromURL(rawURL))
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
