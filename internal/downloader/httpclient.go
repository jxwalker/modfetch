package downloader

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
)

type dnsCacheEntry struct {
	addrs   []net.IPAddr
	expires time.Time
}

type cachingDialer struct {
	base     *net.Dialer
	resolver *net.Resolver
	ttl      time.Duration
	mu       sync.Mutex
	cache    map[string]dnsCacheEntry
}

func newHTTPClient(cfg *config.Config) *http.Client {
	timeoutSeconds := 0
	perHost := 10
	if cfg != nil {
		timeoutSeconds = cfg.Network.TimeoutSeconds
		if cfg.Concurrency.PerHostRequests > 0 {
			perHost = cfg.Concurrency.PerHostRequests
		}
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	dialContext := baseDialer.DialContext
	if cfg != nil && cfg.Network.DNSCacheTTLSeconds > 0 {
		dialContext = (&cachingDialer{
			base:     baseDialer,
			resolver: net.DefaultResolver,
			ttl:      time.Duration(cfg.Network.DNSCacheTTLSeconds) * time.Second,
			cache:    map[string]dnsCacheEntry{},
		}).DialContext
	}

	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   perHost,
		MaxConnsPerHost:       perHost,
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

func (d *cachingDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || d == nil || d.ttl <= 0 || net.ParseIP(host) != nil {
		return d.base.DialContext(ctx, network, address)
	}
	addrs, err := d.lookup(ctx, network, host)
	if err != nil || len(addrs) == 0 {
		return d.base.DialContext(ctx, network, address)
	}
	var lastErr error
	for _, addr := range addrs {
		ip := addr.IP
		if network == "tcp4" && ip.To4() == nil {
			continue
		}
		if network == "tcp6" && ip.To4() != nil {
			continue
		}
		conn, err := d.base.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return d.base.DialContext(ctx, network, address)
}

func (d *cachingDialer) lookup(ctx context.Context, network, host string) ([]net.IPAddr, error) {
	key := network + "|" + strings.ToLower(host)
	now := time.Now()
	d.mu.Lock()
	if entry, ok := d.cache[key]; ok && now.Before(entry.expires) {
		addrs := append([]net.IPAddr(nil), entry.addrs...)
		d.mu.Unlock()
		return addrs, nil
	}
	d.mu.Unlock()

	addrs, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	d.cache[key] = dnsCacheEntry{addrs: append([]net.IPAddr(nil), addrs...), expires: now.Add(d.ttl)}
	d.mu.Unlock()
	return addrs, nil
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
func resolveRedirectURL(ctx context.Context, baseClient *http.Client, rawURL string, headers map[string]string, ua string) (string, bool) {
	// clone client with redirect disabled
	cl := *baseClient
	cl.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
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
