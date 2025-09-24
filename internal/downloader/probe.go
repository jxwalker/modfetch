package downloader

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/util"
)

type ProbeMeta struct {
	FinalURL     string
	Filename     string
	Size         int64
	ETag         string
	LastModified string
	AcceptRange  bool
}

// ProbeURL queries metadata for a URL using a HEAD request, with fallbacks to a small Range GET and
// one-step redirect resolution. It returns filename (from Content-Disposition when present),
// final URL after redirects, size (if known), and other useful headers.
func ProbeURL(ctx context.Context, cfg *config.Config, rawURL string, headers map[string]string) (ProbeMeta, error) {
	cl := newHTTPClient(cfg)
	ua := userAgent(cfg)
	// First try HEAD (follows redirects)
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	req.Header.Set("User-Agent", ua)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := cl.Do(req)
	var meta ProbeMeta
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.Request != nil && resp.Request.URL != nil {
			meta.FinalURL = resp.Request.URL.String()
		}
		meta.ETag = resp.Header.Get("ETag")
		meta.LastModified = resp.Header.Get("Last-Modified")
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			if fn := parseDispositionFilename(cd); fn != "" {
				meta.Filename = fn
			}
		}
		if clh := resp.Header.Get("Content-Length"); clh != "" {
			if n, err := strconv.ParseInt(strings.TrimSpace(clh), 10, 64); err == nil && n >= 0 {
				meta.Size = n
			}
		}
		meta.AcceptRange = false
		if ar := resp.Header.Get("Accept-Ranges"); strings.EqualFold(strings.TrimSpace(ar), "bytes") {
			meta.AcceptRange = true
		}
	}
	// If we still lack size or accept-range, try Range GET probe
	if meta.Size <= 0 || !meta.AcceptRange {
		// Request 0-0
		req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		req2.Header.Set("User-Agent", ua)
		for k, v := range headers {
			req2.Header.Set(k, v)
		}
		req2.Header.Set("Range", "bytes=0-0")
		resp2, err2 := cl.Do(req2)
		if err2 == nil {
			defer func() { _ = resp2.Body.Close() }()
			if resp2.StatusCode == http.StatusPartialContent {
				var start, end, total int64
				if _, err := fmt.Sscanf(resp2.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total); err == nil && total > 0 {
					meta.Size = total
					meta.AcceptRange = true
				}
				if meta.FinalURL == "" && resp2.Request != nil && resp2.Request.URL != nil {
					meta.FinalURL = resp2.Request.URL.String()
				}
				if meta.Filename == "" {
					if cd := resp2.Header.Get("Content-Disposition"); cd != "" {
						if fn := parseDispositionFilename(cd); fn != "" {
							meta.Filename = fn
						}
					}
				}
			}
		}
	}
	// Final fallback: try to resolve a redirect without following, then re-probe
	if (meta.Size <= 0 || !meta.AcceptRange) && meta.FinalURL == "" {
		if ru, ok := resolveRedirectURL(cl, rawURL, headers, ua); ok {
			meta.FinalURL = ru
			// try a HEAD on resolved
			req3, _ := http.NewRequestWithContext(ctx, http.MethodHead, ru, nil)
			req3.Header.Set("User-Agent", ua)
			for k, v := range headers {
				req3.Header.Set(k, v)
			}
			if resp3, err3 := cl.Do(req3); err3 == nil {
				defer func() { _ = resp3.Body.Close() }()
				if clh := resp3.Header.Get("Content-Length"); clh != "" && meta.Size <= 0 {
					if n, err := strconv.ParseInt(strings.TrimSpace(clh), 10, 64); err == nil && n >= 0 {
						meta.Size = n
					}
				}
				if cd := resp3.Header.Get("Content-Disposition"); cd != "" && meta.Filename == "" {
					if fn := parseDispositionFilename(cd); fn != "" {
						meta.Filename = fn
					}
				}
			}
		}
	}
	if meta.FinalURL == "" {
		meta.FinalURL = rawURL
	}
	return meta, nil
}

// ComputeRemoteSHA256 streams the content at URL and computes its SHA256 without saving to disk.
// This fully downloads the resource, so it can be slow and bandwidth-heavy.
func ComputeRemoteSHA256(ctx context.Context, cfg *config.Config, rawURL string, headers map[string]string) (string, error) {
	cl := newHTTPClient(cfg)
	ua := userAgent(cfg)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	req.Header.Set("User-Agent", ua)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := cl.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("GET status: %s", resp.Status)
	}
	return util.HashReaderSHA256(resp.Body)
}

// CheckReachable performs a quick HEAD to determine network reachability to the resource.
// It returns reachable=true when an HTTP response is received regardless of status code.
// Only network errors (DNS failure, connect timeout, etc.) cause reachable=false.
func CheckReachable(ctx context.Context, cfg *config.Config, rawURL string, headers map[string]string) (bool, string) {
	cl := newHTTPClient(cfg)
	ua := userAgent(cfg)
	// Ensure a short timeout for UI responsiveness
	if _, ok := ctx.Deadline(); !ok {
		c2, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ctx = c2
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	req.Header.Set("User-Agent", ua)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := cl.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	return true, resp.Status
}
