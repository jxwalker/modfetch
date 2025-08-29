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
	"os"
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
	return &Chunked{cfg: cfg, log: log, st: st, client: &http.Client{Timeout: timeout}, metrics: m}
}

type headInfo struct {
	etag       string
	lastMod    string
	size       int64
	acceptRange bool
}

func (e *Chunked) head(ctx context.Context, url string, headers map[string]string) (headInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if ua := e.cfg.Network.UserAgent; ua != "" { req.Header.Set("User-Agent", ua) }
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
	return h, nil
}

// Download orchestrates a chunked download if possible; otherwise falls back to single-stream.
func (e *Chunked) Download(ctx context.Context, url, destPath, expectedSHA string, headers map[string]string) (string, string, error) {
	if url == "" { return "", "", errors.New("url required") }
	if destPath == "" {
		name := filepath.Base(url)
		destPath = filepath.Join(e.cfg.General.DownloadRoot, name)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil { return "", "", err }
	startTime := time.Now()

	h, err := e.head(ctx, url, headers)
	if err != nil || h.size <= 0 || !h.acceptRange {
		e.log.Warnf("chunked: falling back to single: %v", err)
		return NewSingle(e.cfg, e.log, e.st, e.metrics).Download(ctx, url, destPath, expectedSHA, headers)
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

	// Download chunks concurrently
	perFile := e.cfg.Concurrency.PerFileChunks
	if perFile <= 0 { perFile = 4 }
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
sha, err := e.fetchChunk(ctx, url, h, f, c, headers)
		if err != nil { setErr(&dErr, &dMu, err); return }
		_ = e.st.UpdateChunkSHA(url, destPath, c.Index, sha)
		_ = e.st.UpdateChunkStatus(url, destPath, c.Index, "complete")
	}

	for _, c := range chunks {
		wg.Add(1)
		go downloadOne(c)
	}
	wg.Wait()
	if dErr != nil { return "", "", dErr }

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
				e.log.Warnf("chunk %d sha mismatch; re-fetching", c.Index)
				_ = e.st.UpdateChunkStatus(url, destPath, c.Index, "dirty")
sha3, err := e.fetchChunk(ctx, url, h, f, c, headers)
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

	// Finalize
	if err := os.Rename(part, destPath); err != nil { return "", "", err }
	if err := os.WriteFile(destPath+".sha256", []byte(finalSHA+"  "+filepath.Base(destPath)+"\n"), 0o644); err != nil { return "", "", err }
	_ = e.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ActualSHA256: finalSHA, ETag: h.etag, LastModified: h.lastMod, Size: h.size, Status: "complete"})
	if e.metrics != nil {
		e.metrics.IncDownloadsSuccess()
		e.metrics.ObserveDownloadSeconds(time.Since(startTime).Seconds())
		_ = e.metrics.Write()
	}
	return destPath, finalSHA, nil
}

func (e *Chunked) fetchChunk(ctx context.Context, url string, h headInfo, f *os.File, c state.ChunkRow, headers map[string]string) (string, error) {
	// retry loop
	max := e.cfg.Concurrency.MaxRetries
	if max <= 0 { max = 8 }
	var lastErr error
	for attempt := 0; attempt < max; attempt++ {
sha, err := e.tryFetchChunk(ctx, url, h, f, c, headers)
		if err == nil { return sha, nil }
		lastErr = err
		if e.metrics != nil { e.metrics.IncRetries(1) }
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
	if ua := e.cfg.Network.UserAgent; ua != "" { req.Header.Set("User-Agent", ua) }
	for k, v := range headers { req.Header.Set(k, v) }
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", c.Start, c.End))
	if h.etag != "" { req.Header.Set("If-Range", h.etag) } else if h.lastMod != "" { req.Header.Set("If-Range", h.lastMod) }
	resp, err := e.client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chunk %d: bad status %s", c.Index, resp.Status)
	}
	// Write at offset
	if _, err := f.Seek(c.Start, io.SeekStart); err != nil { return "", err }
	hasher := sha256.New()
	mw := io.MultiWriter(f, hasher)
	written, err := io.CopyN(mw, resp.Body, c.Size)
	if e.metrics != nil && written > 0 { e.metrics.AddBytes(written) }
	if err != nil && !errors.Is(err, io.EOF) { return "", err }
	if written != c.Size { return "", fmt.Errorf("chunk %d: short write %d!=%d", c.Index, written, c.Size) }
	return hex.EncodeToString(hasher.Sum(nil)), nil
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

