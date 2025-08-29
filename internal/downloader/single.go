package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

type Single struct {
	cfg    *config.Config
	log    *logging.Logger
	client *http.Client
	st     *state.DB
}

func NewSingle(cfg *config.Config, log *logging.Logger, st *state.DB) *Single {
	timeout := time.Duration(cfg.Network.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Single{
		cfg: cfg,
		log: log,
		st:  st,
		client: &http.Client{Timeout: timeout},
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

	part := destPath + ".part"

	// HEAD for metadata
	etag, lastMod, size, rangeOK := s.head(ctx, url, headers)
	s.log.Debugf("HEAD: etag=%s last-mod=%s size=%d range=%v", etag, lastMod, size, rangeOK)
	_ = s.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ETag: etag, LastModified: lastMod, Size: size, Status: "planning"})

	// Prepare hasher and file
	var hasher = sha256.New()
	var start int64 = 0
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
	if s.cfg.Network.UserAgent != "" { req.Header.Set("User-Agent", s.cfg.Network.UserAgent) }
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

	mw := io.MultiWriter(f, hasher)
	if _, err := io.Copy(mw, resp.Body); err != nil {
		_ = s.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ETag: etag, LastModified: lastMod, Size: size, Status: "error"})
		return "", "", err
	}

	actualSHA := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA != "" && !equalSHA(expectedSHA, actualSHA) {
		_ = s.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ETag: etag, LastModified: lastMod, Size: size, Status: "checksum_mismatch"})
		return "", actualSHA, fmt.Errorf("sha256 mismatch: expected=%s actual=%s", expectedSHA, actualSHA)
	}

	// Rename to final
	if err := os.Rename(part, destPath); err != nil { return "", "", err }
	// Write checksum file
	if err := os.WriteFile(destPath+".sha256", []byte(actualSHA+"  "+filepath.Base(destPath)+"\n"), 0o644); err != nil {
		return "", "", err
	}
	_ = s.st.UpsertDownload(state.DownloadRow{URL: url, Dest: destPath, ExpectedSHA256: expectedSHA, ETag: etag, LastModified: lastMod, Size: size, Status: "complete"})
	return destPath, actualSHA, nil
}

func (s *Single) head(ctx context.Context, url string, headers map[string]string) (etag, lastMod string, size int64, rangeOK bool) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if s.cfg.Network.UserAgent != "" { req.Header.Set("User-Agent", s.cfg.Network.UserAgent) }
	for k, v := range headers { req.Header.Set(k, v) }
	resp, err := s.client.Do(req)
	if err != nil { return "", "", 0, false }
	defer resp.Body.Close()
	etag = strings.Trim(resp.Header.Get("ETag"), "\"")
	lastMod = resp.Header.Get("Last-Modified")
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		var n int64
		_, _ = fmt.Sscan(cl, &n)
		size = n
	}
	rangeOK = strings.Contains(strings.ToLower(resp.Header.Get("Accept-Ranges")), "bytes")
	return
}

func lastURLSegment(u string) string {
	if i := strings.LastIndex(u, "/"); i >= 0 && i < len(u)-1 {
		return u[i+1:]
	}
	return ""
}

func equalSHA(exp, got string) bool {
	return strings.EqualFold(strings.TrimSpace(exp), strings.TrimSpace(got))
}

