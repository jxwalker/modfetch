package downloader

import (
	"net/http"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
)

func TestNewHTTPClientUsesConfiguredPerHostPoolLimit(t *testing.T) {
	cfg := &config.Config{
		Network:     config.Network{TimeoutSeconds: 15},
		Concurrency: config.Concurrency{PerHostRequests: 3},
	}

	client := newHTTPClient(cfg)
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if tr.MaxIdleConnsPerHost != 3 {
		t.Fatalf("expected MaxIdleConnsPerHost=3, got %d", tr.MaxIdleConnsPerHost)
	}
	if tr.MaxConnsPerHost != 3 {
		t.Fatalf("expected MaxConnsPerHost=3, got %d", tr.MaxConnsPerHost)
	}
}

func TestAutoAndFallbackShareHTTPClient(t *testing.T) {
	cfg := &config.Config{Concurrency: config.Concurrency{PerHostRequests: 2}}
	auto := NewAuto(cfg, logging.New("error", false), nil, nil)
	if auto.client == nil {
		t.Fatal("expected Auto to initialize shared client")
	}

	chunked := newChunkedWithClient(cfg, logging.New("error", false), nil, nil, auto.client)
	if chunked.client != auto.client {
		t.Fatal("expected chunked downloader to reuse Auto client")
	}

	single := newSingleWithClient(cfg, logging.New("error", false), nil, nil, chunked.client)
	if single.client != auto.client {
		t.Fatal("expected single fallback to reuse Auto client")
	}
}
