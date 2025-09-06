package downloader

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"runtime"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
)

func newHTTPClient(cfg *config.Config) *http.Client {
	timeout := time.Duration(cfg.Network.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
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
		if len(via) == 0 {
			return nil
		}
		prev := via[len(via)-1]
		if ua := prev.Header.Get("User-Agent"); ua != "" {
			req.Header.Set("User-Agent", ua)
		}
		if rng := prev.Header.Get("Range"); rng != "" {
			req.Header.Set("Range", rng)
		}
		if ir := prev.Header.Get("If-Range"); ir != "" {
			req.Header.Set("If-Range", ir)
		}
		// Only forward Authorization when host is the same
		if prev.URL != nil && req.URL != nil && strings.EqualFold(prev.URL.Host, req.URL.Host) {
			if auth := prev.Header.Get("Authorization"); auth != "" {
				req.Header.Set("Authorization", auth)
			}
		}
		return nil
	}
	return client
}

// userAgent returns the configured User-Agent, or a sensible default
// like "github.com/jxwalker/modfetch/<version> (<goos>/<goarch>)" when not set.
func userAgent(cfg *config.Config) string {
	if cfg != nil && cfg.Network.UserAgent != "" {
		return cfg.Network.UserAgent
	}
	return fmt.Sprintf("github.com/jxwalker/modfetch/%s (%s/%s)", versionString(), runtime.GOOS, runtime.GOARCH)
}

// versionString fetches main.version via linker -X if available.
func versionString() string {
	// main.version is defined in cmd/modfetch/main.go; expose via a weak link.
	// We duplicate minimal logic here to avoid import cycles.
	return defaultVersion
}

var defaultVersion = "dev"

// resolveRedirectURL performs a single GET without following redirects to capture the Location.
// Returns the absolute redirected URL if present.
func resolveRedirectURL(baseClient *http.Client, rawURL string, headers map[string]string, ua string) (string, bool) {
	// clone client with redirect disabled
	cl := *baseClient
	cl.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	req, _ := http.NewRequest(http.MethodGet, rawURL, nil)
	req.Header.Set("User-Agent", ua)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := cl.Do(req)
	if err != nil {
		return "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return "", false
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", false
	}
	u, err := neturl.Parse(loc)
	if err != nil {
		return "", false
	}
	if !u.IsAbs() {
		base, err := neturl.Parse(rawURL)
		if err != nil {
			return "", false
		}
		u = base.ResolveReference(u)
	}
	return u.String(), true
}
