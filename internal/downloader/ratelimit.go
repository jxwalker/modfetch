package downloader

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
)

type bandwidthLimiter struct {
	bytesPerSecond int64
	mu             sync.Mutex
	next           time.Time
}

var globalBandwidthLimiters sync.Map

func configuredBandwidthLimiters(cfg *config.Config) (global, perDownload *bandwidthLimiter) {
	if cfg == nil {
		return nil, nil
	}
	if bps := cfg.Network.GlobalBandwidthBytesPerSecond; bps > 0 {
		if existing, ok := globalBandwidthLimiters.Load(bps); ok {
			global = existing.(*bandwidthLimiter)
		} else {
			limiter := newBandwidthLimiter(bps)
			existing, _ := globalBandwidthLimiters.LoadOrStore(bps, limiter)
			global = existing.(*bandwidthLimiter)
		}
	}
	if bps := cfg.Network.PerDownloadBandwidthBytesPerSecond; bps > 0 {
		perDownload = newBandwidthLimiter(bps)
	}
	return global, perDownload
}

func newPerDownloadBandwidthLimiter(cfg *config.Config) *bandwidthLimiter {
	if cfg == nil {
		return nil
	}
	return newBandwidthLimiter(cfg.Network.PerDownloadBandwidthBytesPerSecond)
}

func newBandwidthLimiter(bytesPerSecond int64) *bandwidthLimiter {
	if bytesPerSecond <= 0 {
		return nil
	}
	return &bandwidthLimiter{bytesPerSecond: bytesPerSecond}
}

func (l *bandwidthLimiter) wait(ctx context.Context, n int) error {
	if l == nil || l.bytesPerSecond <= 0 || n <= 0 {
		return nil
	}
	delay := time.Duration(int64(time.Second) * int64(n) / l.bytesPerSecond)
	if delay <= 0 {
		return nil
	}

	l.mu.Lock()
	now := time.Now()
	if l.next.Before(now) {
		l.next = now
	}
	ready := l.next.Add(delay)
	l.next = ready
	l.mu.Unlock()

	wait := time.Until(ready)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type throttledReader struct {
	ctx      context.Context
	reader   io.Reader
	limiters []*bandwidthLimiter
}

func newThrottledReader(ctx context.Context, reader io.Reader, limiters ...*bandwidthLimiter) io.Reader {
	active := make([]*bandwidthLimiter, 0, len(limiters))
	for _, limiter := range limiters {
		if limiter != nil && limiter.bytesPerSecond > 0 {
			active = append(active, limiter)
		}
	}
	if len(active) == 0 {
		return reader
	}
	return &throttledReader{ctx: ctx, reader: reader, limiters: active}
}

func (r *throttledReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		for _, limiter := range r.limiters {
			if waitErr := limiter.wait(r.ctx, n); waitErr != nil {
				return n, waitErr
			}
		}
	}
	return n, err
}
