package downloader

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestNewHTTPClientHonorsTLSVerify(t *testing.T) {
	cfg := &config.Config{
		Network:     config.Network{TLSVerify: false},
		Concurrency: config.Concurrency{PerHostRequests: 2},
	}

	client := newHTTPClient(cfg)
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected TLS verification to be disabled")
	}
}

func TestNewHTTPClientHonorsMaxRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/one", http.StatusFound)
		case "/one":
			http.Redirect(w, r, "/two", http.StatusFound)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := newHTTPClient(&config.Config{Network: config.Network{MaxRedirects: 1}})
	resp, err := client.Get(server.URL + "/start")
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "stopped after 1 redirects") {
		t.Fatalf("expected redirect limit error, got %v", err)
	}
}

func TestCachingDialerNilReceiverDoesNotPanic(t *testing.T) {
	var dialer *cachingDialer
	if _, err := dialer.DialContext(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", "1")); err == nil {
		t.Fatal("expected connection error from fallback dialer")
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
