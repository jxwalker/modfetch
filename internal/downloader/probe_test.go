package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestCheckReachable_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()
	cfg := &config.Config{Network: config.Network{TimeoutSeconds: 2}}
	ok, status := CheckReachable(context.Background(), cfg, ts.URL, nil)
	if !ok {
		t.Fatalf("expected reachable, got false status=%q", status)
	}
	if status == "" || status[:3] != "200" {
		t.Fatalf("expected 200 status, got %q", status)
	}
}

func TestCheckReachable_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()
	cfg := &config.Config{Network: config.Network{TimeoutSeconds: 2}}
	ok, status := CheckReachable(context.Background(), cfg, ts.URL, nil)
	if !ok {
		t.Fatalf("expected reachable=true, got false")
	}
	if status == "" || status[:3] != "401" {
		t.Fatalf("expected 401 status, got %q", status)
	}
}

func TestCheckReachable_NetworkError(t *testing.T) {
	// Non-listening localhost port should fail quickly
	cfg := &config.Config{Network: config.Network{TimeoutSeconds: 1}}
	ok, status := CheckReachable(context.Background(), cfg, "http://127.0.0.1:1/", nil)
	if ok {
		t.Fatalf("expected reachable=false for network error, got true (%q)", status)
	}
	if status == "" {
		t.Fatalf("expected error string, got empty")
	}
}
