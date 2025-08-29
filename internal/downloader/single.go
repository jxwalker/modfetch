package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

// local helper to probe size and range via GET bytes=0-0
func probeRangeGET(client *http.Client, url string, headers map[string]string, ua string) (size int64, ok bool) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", ua)
	for k, v := range headers { req.Header.Set(k, v) }
	req.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(req)
	if err != nil { return 0, false }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent { return 0, false }
	cr := resp.Header.Get("Content-Range")
	var s, e, total int64
	if _, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &s, &e, &total); err != nil || total <= 0 { return 0, false }
	return total, true
}

type Single struct {
	cfg    *config.Config
	log    *logging.Logger
	client *http.Client
	st     *state.DB
	metrics interface {
		AddBytes(int64)
		IncDownloadsSuccess()
		ObserveDownloadSeconds(float64)
		Write() error
	}
}

func NewSingle(cfg *config.Config, log *logging.Logger, st *state.DB, m interface{ AddBytes(int64); IncDownloadsSuccess(); ObserveDownloadSeconds(float64); Write() error }) *Single {
	timeout := time.Duration(cfg.Network.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Single{
		cfg: cfg,
		log: log,
		st:  st,
		client: newHTTPClient(cfg),
		metrics: m,
	}
}

// Download downloads a single file from url to destPath. If destPath is empty, it uses cfg.General.DownloadRoot + last URL segment.
// It resumes if a .part file exists and the server supports Range requests.
func (s *Single) Download(ctx context.Context, url, destPath, expectedSHA string, headers map[string]string) (string, string, error) {
	if url == "" { return "", "", errors.New("url required") }
	if destPath == "" {
		seg := lastURLSegment(url)
		if seg == "" { return "", "", errors.New("cannot infer destination filename") }
		destPath = filepath.Join(s.cfg.General.DownloadRoot, seg)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil { return "", "", err }
	startTime := time.Now()
	// metrics: mark active
	if a, ok := s.metrics.(interface{ IncActive(int64); Write() error }); ok { a.IncActive(1); _ = a.Write(); defer func(){ a.IncActive(-1); _ = a.Write() }() }

	part := destPath + ".part"

	// HEAD for metadata
	etag, lastMod, size, rangeOK := s.head(ctx, url, headers)
	s.log.Debugf("HEAD: etag=%s last-mod=%s size=%d range=%v", etag, lastMod, size, rangeOK)
	_ = s.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ETag: etag, LastModified: lastMod, Size: size, Status: "planning"})

	// Prepare hasher and file
	var hasher = sha256.New()
	var start int64 = 0
	// Clear any stale chunk state since we're using single-stream
	_ = s.st.DeleteChunks(url, destPath)
	if fi, err := os.Stat(part); err == nil {
		start = fi.Size()
		s.log.Infof("resuming: %s (have %d bytes)", part, start)
		// Prime hasher with existing bytes
		f, err := os.Open(part)
		if err != nil { return "", "", err }
		if _, err := io.Copy(hasher, f); err != nil { _ = f.Close(); return "", "", err }
		_ = f.Close()
	}

	// Open file for append/create
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil { return "", "", err }
	defer f.Close()
	if _, err := f.Seek(start, io.SeekStart); err != nil { return "", "", err }

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil { return "", "", err }
	req.Header.Set("User-Agent", userAgent(s.cfg))
	for k, v := range headers { req.Header.Set(k, v) }
	if start > 0 && rangeOK {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
	}
	resp, err := s.client.Do(req)
	if err != nil { return "", "", err }
	defer resp.Body.Close()

	if start > 0 && resp.StatusCode == http.StatusOK {
		// Server ignored Range; restart from 0
		s.log.Warnf("server ignored Range; restarting from beginning")
		if _, err := f.Seek(0, io.SeekStart); err != nil { return "", "", err }
		if err := f.Truncate(0); err != nil { return "", "", err }
		hasher = sha256.New()
		start = 0
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return "", "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	// Preallocate to expected size when known
	if size > 0 {
		_ = f.Truncate(size)
	}
	mw := io.MultiWriter(f, hasher)
	nWritten, err := io.Copy(mw, resp.Body)
	if s.metrics != nil && nWritten > 0 { s.metrics.AddBytes(nWritten) }
	if err != nil {
		_ = s.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ActualSHA256: "", ETag: etag, LastModified: lastMod, Size: size, Status: "error"})
		return "", "", friendlyIOError(err)
	}
	// Ensure file data is durable before rename
	_ = f.Sync()

	actualSHA := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA != "" && !equalSHA(expectedSHA, actualSHA) {
		_ = s.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ActualSHA256: actualSHA, ETag: etag, LastModified: lastMod, Size: size, Status: "checksum_mismatch"})
		return "", actualSHA, fmt.Errorf("sha256 mismatch: expected=%s actual=%s", expectedSHA, actualSHA)
	}

	// Rename to final
	if err := os.Rename(part, destPath); err != nil { return "", "", err }
	// If safetensors, trim any trailing bytes beyond header-declared size (and fail if incomplete)
	if _, err := adjustSafetensors(destPath, s.log); err != nil { return "", "", err }
	// Optional deep verify for safetensors
	if s.cfg.Validation.SafetensorsDeepVerifyAfterDownload && (strings.HasSuffix(strings.ToLower(destPath), ".safetensors") || strings.HasSuffix(strings.ToLower(destPath), ".sft")) {
		ok, _, verr := deepVerifySafetensors(destPath)
		if !ok || verr != nil {
			_ = s.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ActualSHA256: "", ETag: etag, LastModified: lastMod, Size: size, Status: "verify_failed"})
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
		actualSHA = hex.EncodeToString(h.Sum(nil))
	}
	// Write checksum file (durable)
	if err := writeAndSync(destPath+".sha256", []byte(actualSHA+"  "+filepath.Base(destPath)+"\n")); err != nil {
		return "", "", err
	}
	// Fsync parent directory to persist rename and sidecar
	_ = fsyncDir(filepath.Dir(destPath))
	_ = s.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ActualSHA256: actualSHA, ETag: etag, LastModified: lastMod, Size: size, Status: "complete"})
	if s.metrics != nil {
		s.metrics.IncDownloadsSuccess()
		s.metrics.ObserveDownloadSeconds(time.Since(startTime).Seconds())
		_ = s.metrics.Write()
	}
	return destPath, actualSHA, nil
}

func (s *Single) head(ctx context.Context, url string, headers map[string]string) (etag, lastMod string, size int64, rangeOK bool) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	req.Header.Set("User-Agent", userAgent(s.cfg))
	for k, v := range headers { req.Header.Set(k, v) }
	resp, err := s.client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		etag = strings.Trim(resp.Header.Get("ETag"), "\"")
		lastMod = resp.Header.Get("Last-Modified")
		if cl := resp.Header.Get("Content-Length"); cl != "" {
			var n int64
			_, _ = fmt.Sscan(cl, &n)
			size = n
		}
		rangeOK = strings.Contains(strings.ToLower(resp.Header.Get("Accept-Ranges")), "bytes")
	}
	// Fallback: if HEAD failed or size/range not known, try a 0-0 Range GET to infer
	if size <= 0 || !rangeOK {
		if sz, ok := probeRangeGET(s.client, url, headers, userAgent(s.cfg)); ok && sz > 0 {
			size = sz
			rangeOK = true
		}
	}
	return
}

func lastURLSegment(uStr string) string {
	if u, err := neturl.Parse(uStr); err == nil {
		b := path.Base(u.Path)
		if b != "/" && b != "." && b != "" {
			return b
		}
	}
	return ""
}

func equalSHA(exp, got string) bool {
	return strings.EqualFold(strings.TrimSpace(exp), strings.TrimSpace(got))
}


func friendlyIOError(err error) error {
	// Map ENOSPC to a friendlier message
	if errors.Is(err, os.ErrClosed) {
		return err
	}
	if pe, ok := err.(*os.PathError); ok {
		if pe.Err != nil && strings.Contains(strings.ToLower(pe.Err.Error()), "no space left") {
			return fmt.Errorf("write failed: no space left on device")
		}
	}
	return err
}

