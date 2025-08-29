package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

type Chunked struct {
	cfg     *config.Config
	log     *logging.Logger
	client  *http.Client
	st      *state.DB
	metrics interface {
		AddBytes(int64)
		IncRetries(int64)
		IncDownloadsSuccess()
		ObserveDownloadSeconds(float64)
		Write() error
	}
}

func NewChunked(cfg *config.Config, log *logging.Logger, st *state.DB, m interface{ AddBytes(int64); IncRetries(int64); IncDownloadsSuccess(); ObserveDownloadSeconds(float64); Write() error }) *Chunked {
	timeout := time.Duration(cfg.Network.TimeoutSeconds) * time.Second
	if timeout <= 0 { timeout = 60 * time.Second }
	return &Chunked{cfg: cfg, log: log, st: st, client: newHTTPClient(cfg), metrics: m}
}

type headInfo struct {
	etag        string
	lastMod     string
	size        int64
	acceptRange bool
	filename    string
	finalURL    string
}

func (e *Chunked) head(ctx context.Context, url string, headers map[string]string) (headInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	req.Header.Set("User-Agent", userAgent(e.cfg))
	for k, v := range headers { req.Header.Set(k, v) }
	resp, err := e.client.Do(req)
	if err != nil { return headInfo{}, err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return headInfo{}, fmt.Errorf("HEAD status: %s", resp.Status)
	}
	var h headInfo
	h.etag = resp.Header.Get("ETag")
	h.lastMod = resp.Header.Get("Last-Modified")
	if cl := resp.Header.Get("Content-Length"); cl != "" { _, _ = fmt.Sscan(cl, &h.size) }
	h.acceptRange = strings.Contains(strings.ToLower(resp.Header.Get("Accept-Ranges")), "bytes")
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if fn := parseDispositionFilename(cd); fn != "" { h.filename = fn }
	}
	return h, nil
}

// Download orchestrates a chunked download if possible; otherwise falls back to single-stream.
func (e *Chunked) Download(ctx context.Context, url, destPath, expectedSHA string, headers map[string]string) (string, string, error) {
	if url == "" { return "", "", errors.New("url required") }
	// Ensure download root exists; we will finalize destPath after probing headers
	if err := os.MkdirAll(e.cfg.General.DownloadRoot, 0o755); err != nil { return "", "", err }
	startTime := time.Now()
	if a, ok := e.metrics.(interface{ IncActive(int64); Write() error }); ok { a.IncActive(1); _ = a.Write(); defer func(){ a.IncActive(-1); _ = a.Write() }() }

	// Host capability cache is advisory only; we always probe to avoid stale data.
	h, err := e.head(ctx, url, headers)
	// If HEAD failed or Range not advertised, try a Range GET probe (bytes=0-0)
	if (err != nil || !h.acceptRange || h.size <= 0) && ctx != nil {
		if hp, perr := e.probeRangeGET(ctx, url, headers); perr == nil && hp.acceptRange && hp.size > 0 {
			e.log.Debugf("range GET probe succeeded (size=%d)", hp.size)
			h = hp
			err = nil
		} else {
			// As a last resort, resolve signed redirect then retry probe on the final URL
			if ru, ok := resolveRedirectURL(e.client, url, headers, userAgent(e.cfg)); ok {
				e.log.Debugf("resolved redirect -> %s", ru)
				url = ru
				// Do not forward Authorization to different host
				if u1, _ := neturl.Parse(ru); u1 != nil {
					if u0, _ := neturl.Parse(url); u0 == nil || !strings.EqualFold(u0.Host, u1.Host) {
						delete(headers, "Authorization")
					}
				}
				if hp2, perr2 := e.probeRangeGET(ctx, url, headers); perr2 == nil && hp2.acceptRange && hp2.size > 0 {
					e.log.Debugf("range GET probe after redirect succeeded (size=%d)", hp2.size)
					h = hp2
					err = nil
				}
			}
		}
	}
	// Persist host capabilities based on what we know
	if u, perr := neturl.Parse(url); perr == nil {
		headOK := err == nil
		acc := h.acceptRange
		_ = e.st.UpsertHostCaps(strings.ToLower(u.Hostname()), headOK, acc)
	}
	// If caller did not supply a destination, derive a good filename now
	if destPath == "" {
		var name string
		if h.filename != "" { name = h.filename }
		if name == "" && h.finalURL != "" { name = baseNameFromURL(h.finalURL) }
		if name == "" { name = baseNameFromURL(url) }
		name = safeFileName(name)
		destPath = filepath.Join(e.cfg.General.DownloadRoot, name)
	}
	// Attempt to migrate any previous wrong-named partial from raw URL base
	if destPath != "" {
		oldBase := filepath.Base(url) // may include query; prior versions used this
		oldPart := filepath.Join(e.cfg.General.DownloadRoot, oldBase) + ".part"
		newPart := destPath + ".part"
		if oldPart != newPart {
			if _, errOld := os.Stat(oldPart); errOld == nil {
				if _, errNew := os.Stat(newPart); errors.Is(errNew, os.ErrNotExist) {
					e.log.Warnf("moving existing partial from %s to %s", oldPart, newPart)
					_ = os.Rename(oldPart, newPart)
				}
			}
		}
	}
	if err != nil || h.size <= 0 || !h.acceptRange {
		e.log.WarnfThrottled(fmt.Sprintf("fallback:%s|%s", url, destPath), 2*time.Second, "chunked: falling back to single: %v", err)
		return e.singleWithRetry(ctx, url, destPath, expectedSHA, headers)
	}
	_ = e.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ActualSHA256: "", ETag: h.etag, LastModified: h.lastMod, Size: h.size, Status: "planning"})

	part := destPath + ".part"
	// If final exists and no part exists, rename to part to verify & resume
	if _, err := os.Stat(part); errors.Is(err, os.ErrNotExist) {
		if _, err2 := os.Stat(destPath); err2 == nil {
			e.log.Warnf("dest exists; moving to .part for verification")
			if err := os.Rename(destPath, part); err != nil { return "", "", err }
		}
	}

	f, err := os.OpenFile(part, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil { return "", "", err }
	defer f.Close()
	if fi, _ := f.Stat(); fi.Size() != h.size {
		if err := f.Truncate(h.size); err != nil { return "", "", err }
	}

	// Plan chunks and persist state
	chunks, err := e.st.ListChunks(url, destPath)
	if err != nil { return "", "", err }
	if len(chunks) == 0 {
		chunkSize := int64(e.cfg.Concurrency.ChunkSizeMB) * 1024 * 1024
		if chunkSize <= 0 { chunkSize = 8 * 1024 * 1024 }
		var idx int
		for start := int64(0); start < h.size; start += chunkSize {
			end := start + chunkSize - 1
			if end >= h.size { end = h.size - 1 }
			sz := end - start + 1
			cr := state.ChunkRow{URL: url, Dest: destPath, Index: idx, Start: start, End: end, Size: sz, Status: "pending"}
			if err := e.st.UpsertChunk(cr); err != nil { return "", "", err }
			idx++
		}
		chunks, err = e.st.ListChunks(url, destPath)
		if err != nil { return "", "", err }
	}

	// Preflight: verify any previously complete chunks before writing further
	if len(chunks) > 0 {
		markedDirty := 0
		for _, c := range chunks {
			if strings.EqualFold(c.Status, "complete") {
				sha2, err := hashRange(f, c.Start, c.Size)
				if err != nil { return "", "", err }
				if !stringsEqualHex(sha2, c.SHA256) {
					e.log.Debugf("chunk %d sha mismatch on resume; marking dirty", c.Index)
					_ = e.st.UpdateChunkStatus(url, destPath, c.Index, "dirty")
					markedDirty++
				}
			}
		}
		if markedDirty > 0 {
			key := fmt.Sprintf("dirty:%s|%s", url, destPath)
			e.log.WarnfThrottled(key, 2*time.Second, "resume verification: %d chunk(s) marked dirty; will refetch", markedDirty)
		}
		// Refresh chunk list to pick up dirty statuses
		chunks, _ = e.st.ListChunks(url, destPath)
	}

	// Download chunks concurrently
	perFile := e.cfg.Concurrency.PerFileChunks
	if perFile <= 0 { perFile = 4 }
	if ph := e.cfg.Concurrency.PerHostRequests; ph > 0 && perFile > ph { perFile = ph }
	sem := make(chan struct{}, perFile)
	var wg sync.WaitGroup
	var dErr error
	var dMu sync.Mutex

	downloadOne := func(c state.ChunkRow) {
		defer wg.Done()
		sem <- struct{}{}
		defer func(){ <-sem }()

		if c.Status == "complete" { return }
if err := e.st.UpdateChunkStatus(url, destPath, c.Index, "running"); err != nil { setErr(&dErr, &dMu, err); return }
sha, err := e.fetchChunk(ctx, url, destPath, h, f, c, headers)
		if err != nil { setErr(&dErr, &dMu, err); return }
		_ = e.st.UpdateChunkSHA(url, destPath, c.Index, sha)
		_ = e.st.UpdateChunkStatus(url, destPath, c.Index, "complete")
	}

	for _, c := range chunks {
		wg.Add(1)
		go downloadOne(c)
	}
	wg.Wait()
	if dErr != nil {
		// Fallback to single-stream with retry if chunked failed (e.g., server ignored Range)
		_ = f.Close()
		return e.singleWithRetry(ctx, url, destPath, expectedSHA, headers)
	}

	// Verify final SHA by streaming the part file
	hasher := sha256.New()
	if _, err := f.Seek(0, io.SeekStart); err != nil { return "", "", err }
	if _, err := io.Copy(hasher, f); err != nil { return "", "", err }
	finalSHA := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA != "" && !stringsEqualHex(expectedSHA, finalSHA) {
		e.log.Warnf("final sha mismatch; scanning chunks for corruption")
		// Re-hash each chunk and compare with recorded chunk sha
		repaired := false
		chunks, _ = e.st.ListChunks(url, destPath)
		for _, c := range chunks {
			sha2, err := hashRange(f, c.Start, c.Size)
			if err != nil { return "", "", err }
			if !stringsEqualHex(sha2, c.SHA256) {
				// re-download this chunk
				e.log.Debugf("chunk %d sha mismatch; re-fetching", c.Index)
				_ = e.st.UpdateChunkStatus(url, destPath, c.Index, "dirty")
sha3, err := e.fetchChunk(ctx, url, destPath, h, f, c, headers)
				if err != nil { return "", "", err }
				_ = e.st.UpdateChunkSHA(url, destPath, c.Index, sha3)
				_ = e.st.UpdateChunkStatus(url, destPath, c.Index, "complete")
				repaired = true
			}
		}
		if repaired {
			// re-hash full file
			if _, err := f.Seek(0, io.SeekStart); err != nil { return "", "", err }
			hasher.Reset()
			if _, err := io.Copy(hasher, f); err != nil { return "", "", err }
			finalSHA = hex.EncodeToString(hasher.Sum(nil))
		}
		if expectedSHA != "" && !stringsEqualHex(expectedSHA, finalSHA) {
		_ = e.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ActualSHA256: finalSHA, ETag: h.etag, LastModified: h.lastMod, Size: h.size, Status: "checksum_mismatch"})
			return "", finalSHA, fmt.Errorf("sha256 mismatch after repair: expected=%s got=%s", expectedSHA, finalSHA)
		}
	}

	// Ensure part file is flushed before finalize
	_ = f.Sync()
	// Finalize
	if err := os.Rename(part, destPath); err != nil { return "", "", err }
	// If safetensors, trim any trailing bytes beyond header-declared size (and fail if incomplete)
	if _, err := adjustSafetensors(destPath, e.log); err != nil { return "", "", err }
	// Optional deep verify for safetensors
	if e.cfg.Validation.SafetensorsDeepVerifyAfterDownload && (strings.HasSuffix(strings.ToLower(destPath), ".safetensors") || strings.HasSuffix(strings.ToLower(destPath), ".sft")) {
		ok, _, verr := deepVerifySafetensors(destPath)
		if !ok || verr != nil {
			_ = e.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ActualSHA256: "", ETag: h.etag, LastModified: h.lastMod, Size: h.size, Status: "verify_failed"})
			return "", "", fmt.Errorf("deep verify failed: %v", verr)
		}
	}
	// Recompute SHA after any adjustment
	{
		ff, err := os.Open(destPath)
		if err != nil { return "", "", err }
		h := sha256.New()
		if _, err := io.Copy(h, ff); err != nil { _ = ff.Close(); return "", "", err }
		_ = ff.Close()
		finalSHA = hex.EncodeToString(h.Sum(nil))
	}
	if err := writeAndSync(destPath+".sha256", []byte(finalSHA+"  "+filepath.Base(destPath)+"\n")); err != nil { return "", "", err }
	_ = fsyncDir(filepath.Dir(destPath))
	_ = e.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ActualSHA256: finalSHA, ETag: h.etag, LastModified: h.lastMod, Size: h.size, Status: "complete"})
	if e.metrics != nil {
		e.metrics.IncDownloadsSuccess()
		e.metrics.ObserveDownloadSeconds(time.Since(startTime).Seconds())
		_ = e.metrics.Write()
	}
	return destPath, finalSHA, nil
}

func (e *Chunked) fetchChunk(ctx context.Context, url string, destPath string, h headInfo, f *os.File, c state.ChunkRow, headers map[string]string) (string, error) {
	// retry loop
	max := e.cfg.Concurrency.MaxRetries
	if max <= 0 { max = 8 }
	var lastErr error
	for attempt := 0; attempt < max; attempt++ {
sha, err := e.tryFetchChunk(ctx, url, h, f, c, headers)
		if err == nil { return sha, nil }
		lastErr = err
		if e.metrics != nil { e.metrics.IncRetries(1) }
		_ = e.st.IncDownloadRetries(url, destPath, 1)
		// backoff
		b := e.cfg.Concurrency.Backoff
		min := b.MinMS; if min <= 0 { min = 200 }
		maxb := b.MaxMS; if maxb <= 0 { maxb = 30000 }
		dur := time.Duration(min)*time.Millisecond + time.Duration(rand.Intn(maxb-min+1))*time.Millisecond
		time.Sleep(dur)
	}
	return "", lastErr
}

func (e *Chunked) tryFetchChunk(ctx context.Context, url string, h headInfo, f *os.File, c state.ChunkRow, headers map[string]string) (string, error) {
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", userAgent(e.cfg))
	for k, v := range headers { req.Header.Set(k, v) }
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", c.Start, c.End))
	if h.etag != "" { req.Header.Set("If-Range", h.etag) } else if h.lastMod != "" { req.Header.Set("If-Range", h.lastMod) }
	resp, err := e.client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	// Require 206 for partial ranges; allow 200 only if the requested range is the entire file
	if resp.StatusCode == http.StatusOK {
		if !(c.Start == 0 && c.End == h.size-1) {
			return "", fmt.Errorf("chunk %d: server ignored Range; got 200 for partial request", c.Index)
		}
	} else if resp.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("chunk %d: bad status %s", c.Index, resp.Status)
	}
	// Write this chunk using a WriteAt-backed offset writer to avoid concurrent Seek/Write races
	hasher := sha256.New()
	ow := &offsetWriterAt{w: f, off: c.Start}
	mw := io.MultiWriter(ow, hasher)
	written, err := io.CopyN(mw, resp.Body, c.Size)
	if e.metrics != nil && written > 0 { e.metrics.AddBytes(written) }
	if err != nil && !errors.Is(err, io.EOF) { return "", err }
	if written != c.Size { return "", fmt.Errorf("chunk %d: short write %d!=%d", c.Index, written, c.Size) }
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// probeRangeGET attempts to fetch a single byte (0-0) via GET with Range header
// to infer total size and whether Range requests are honored when HEAD is blocked.
func (e *Chunked) probeRangeGET(ctx context.Context, url string, headers map[string]string) (headInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", userAgent(e.cfg))
	for k, v := range headers { req.Header.Set(k, v) }
	req.Header.Set("Range", "bytes=0-0")
	resp, err := e.client.Do(req)
	if err != nil { return headInfo{}, err }
	defer resp.Body.Close()
	// Expect 206 Partial Content
	if resp.StatusCode != http.StatusPartialContent {
		return headInfo{}, fmt.Errorf("probe: unexpected status %s", resp.Status)
	}
	cr := resp.Header.Get("Content-Range")
	// Format: bytes 0-0/TOTAL
	var start, end, total int64
	if _, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &start, &end, &total); err != nil || total <= 0 {
		return headInfo{}, fmt.Errorf("probe: invalid Content-Range: %q", cr)
	}
	var h headInfo
	h.size = total
	h.acceptRange = true
	h.etag = resp.Header.Get("ETag")
	h.lastMod = resp.Header.Get("Last-Modified")
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if fn := parseDispositionFilename(cd); fn != "" { h.filename = fn }
	}
	if resp.Request != nil && resp.Request.URL != nil { h.finalURL = resp.Request.URL.String() }
	return h, nil
}

func (e *Chunked) singleWithRetry(ctx context.Context, url, destPath, expectedSHA string, headers map[string]string) (string, string, error) {
	max := e.cfg.Concurrency.MaxRetries
	if max <= 0 { max = 8 }
	var lastErr error
	for attempt := 0; attempt < max; attempt++ {
		final, sha, err := NewSingle(e.cfg, e.log, e.st, e.metrics).Download(ctx, url, destPath, expectedSHA, headers)
		if err == nil { return final, sha, nil }
		lastErr = err
		_ = e.st.IncDownloadRetries(url, destPath, 1)
		// backoff between attempts
		b := e.cfg.Concurrency.Backoff
		min := b.MinMS; if min <= 0 { min = 200 }
		maxb := b.MaxMS; if maxb <= 0 { maxb = 30000 }
		dur := time.Duration(min)*time.Millisecond + time.Duration(rand.Intn(maxb-min+1))*time.Millisecond
		time.Sleep(dur)
	}
	return "", "", lastErr
}

// parseDispositionFilename extracts a filename from a Content-Disposition header.
func parseDispositionFilename(cd string) string {
	// prefer filename*
	parts := strings.Split(cd, ";")
	for _, p := range parts {
		pt := strings.TrimSpace(p)
		plt := strings.ToLower(pt)
		if strings.HasPrefix(plt, "filename*=") {
			v := strings.TrimSpace(pt[len("filename*="):])
			v = strings.Trim(v, "\"")
			// format: UTF-8''percent-encoded
			if i := strings.Index(v, "''"); i >= 0 && i+2 < len(v) {
				enc := v[i+2:]
				if dec, err := neturl.QueryUnescape(enc); err == nil { return safeFileName(dec) }
			}
			return safeFileName(v)
		}
	}
	for _, p := range parts {
		pt := strings.TrimSpace(p)
		plt := strings.ToLower(pt)
		if strings.HasPrefix(plt, "filename=") {
			v := strings.TrimSpace(pt[len("filename="):])
			v = strings.Trim(v, "\"'")
			return safeFileName(v)
		}
	}
	return ""
}

// baseNameFromURL returns the last path segment from a URL, ignoring query/fragments.
func baseNameFromURL(uStr string) string {
	u, err := neturl.Parse(uStr)
	if err != nil || u.Path == "" { return "download" }
	b := path.Base(u.Path)
	if b == "/" || b == "." || b == "" { return "download" }
	return b
}

// safeFileName strips directory components and disallowed characters.
func safeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	if name == "" { return "download" }
	return name
}

func hashRange(f *os.File, start, size int64) (string, error) {
	if _, err := f.Seek(start, io.SeekStart); err != nil { return "", err }
	h := sha256.New()
	if _, err := io.CopyN(h, f, size); err != nil && !errors.Is(err, io.EOF) { return "", err }
	return hex.EncodeToString(h.Sum(nil)), nil
}

func setErr(dst *error, mu *sync.Mutex, err error) {
	mu.Lock(); defer mu.Unlock()
	if *dst == nil { *dst = err }
}

func stringsEqualHex(a, b string) bool {
	if len(a) != len(b) { return false }
	return equalFoldHex(a, b)
}

func equalFoldHex(a, b string) bool {
	// case-insensitive compare for hex
	if len(a) != len(b) { return false }
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'F' { ca += 32 }
		if cb >= 'A' && cb <= 'F' { cb += 32 }
		if ca != cb { return false }
	}
	return true
}

// offsetWriterAt wraps an io.WriterAt and maintains a moving offset implementing io.Writer.
type offsetWriterAt struct {
	w   io.WriterAt
	off int64
}

func (o *offsetWriterAt) Write(p []byte) (int, error) {
	n, err := o.w.WriteAt(p, o.off)
	o.off += int64(n)
	return n, err
}

