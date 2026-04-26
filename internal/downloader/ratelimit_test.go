package downloader

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestConfiguredBandwidthLimitersShareGlobalOnly(t *testing.T) {
	cfg := &config.Config{
		Network: config.Network{
			GlobalBandwidthBytesPerSecond:      1024,
			PerDownloadBandwidthBytesPerSecond: 512,
		},
	}

	globalA, perA := configuredBandwidthLimiters(cfg)
	globalB, perB := configuredBandwidthLimiters(cfg)

	if globalA == nil || globalB == nil {
		t.Fatal("expected global bandwidth limiter")
	}
	if globalA != globalB {
		t.Fatal("expected global limiter to be shared for a config instance")
	}
	if perA == nil || perB == nil {
		t.Fatal("expected per-download limiters")
	}
	if perA == perB {
		t.Fatal("expected per-download limiter to be new for each download")
	}
}

func TestThrottledReaderReturnsOriginalReaderWhenUnlimited(t *testing.T) {
	reader := strings.NewReader("abc")
	got := newThrottledReader(context.Background(), reader, nil)
	if got != reader {
		t.Fatal("expected original reader when no limiter is active")
	}
}

func TestThrottledReaderPassesDataThrough(t *testing.T) {
	reader := newThrottledReader(context.Background(), strings.NewReader("abc"), newBandwidthLimiter(1<<30))

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "abc" {
		t.Fatalf("expected abc, got %q", string(got))
	}
}
