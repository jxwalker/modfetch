package downloader

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"

	"modfetch/internal/config"
)

func newHTTPClient(cfg *config.Config) *http.Client {
	timeout := time.Duration(cfg.Network.TimeoutSeconds) * time.Second
	if timeout <= 0 { timeout = 60 * time.Second }
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	client := &http.Client{Transport: tr, Timeout: timeout}
	// Preserve important headers across redirects (Range, If-Range, UA). Avoid leaking Authorization across hosts.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) == 0 { return nil }
		prev := via[len(via)-1]
		if ua := prev.Header.Get("User-Agent"); ua != "" { req.Header.Set("User-Agent", ua) }
		if rng := prev.Header.Get("Range"); rng != "" { req.Header.Set("Range", rng) }
		if ir := prev.Header.Get("If-Range"); ir != "" { req.Header.Set("If-Range", ir) }
		// Only forward Authorization when host is the same
		if prev.URL != nil && req.URL != nil && strings.EqualFold(prev.URL.Host, req.URL.Host) {
			if auth := prev.Header.Get("Authorization"); auth != "" { req.Header.Set("Authorization", auth) }
		}
		return nil
	}
	return client
}

// userAgent returns the configured User-Agent, or a sensible default
// like "modfetch/<version> (<goos>/<goarch>)" when not set.
func userAgent(cfg *config.Config) string {
	if cfg != nil && cfg.Network.UserAgent != "" {
		return cfg.Network.UserAgent
	}
	return fmt.Sprintf("modfetch/%s (%s/%s)", versionString(), runtime.GOOS, runtime.GOARCH)
}

// versionString fetches main.version via linker -X if available.
func versionString() string {
	// main.version is defined in cmd/modfetch/main.go; expose via a weak link.
	// We duplicate minimal logic here to avoid import cycles.
	return defaultVersion
}

var defaultVersion = "dev"

