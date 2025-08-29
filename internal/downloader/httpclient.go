package downloader

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"runtime"
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
	return &http.Client{Transport: tr, Timeout: timeout}
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

