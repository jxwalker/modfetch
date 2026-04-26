package downloader

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/state"
	"github.com/jxwalker/modfetch/internal/storage"
)

// Interface is the common downloader interface used across implementations.
type Interface interface {
	Download(ctx context.Context, url, destPath, expectedSHA string, headers map[string]string, noResume bool) (finalPath string, sha256 string, err error)
}

// Auto implements Interface by delegating to the chunked downloader which
// already contains robust fallback to the single-stream downloader.
// This centralizes the selection logic behind Interface.
type Auto struct {
	c             *config.Config
	l             *logging.Logger
	st            *state.DB
	client        *http.Client
	globalLimiter *bandwidthLimiter
	m             interface {
		AddBytes(int64)
		IncRetries(int64)
		IncDownloadsSuccess()
		ObserveDownloadSeconds(float64)
		Write() error
	}
}

func NewAuto(c *config.Config, l *logging.Logger, st *state.DB, m interface {
	AddBytes(int64)
	IncRetries(int64)
	IncDownloadsSuccess()
	ObserveDownloadSeconds(float64)
	Write() error
}) *Auto {
	global, _ := configuredBandwidthLimiters(c)
	return &Auto{c: c, l: l, st: st, client: newHTTPClient(c), globalLimiter: global, m: m}
}

func (a *Auto) Download(ctx context.Context, url, destPath, expectedSHA string, headers map[string]string, noResume bool) (string, string, error) {
	// Delegate to chunked downloader which handles head probe and fallback.
	dl := newChunkedWithClientAndLimiters(a.c, a.l, a.st, a.m, a.client, a.globalLimiter, nil)
	if !storage.IsS3URI(destPath) {
		return dl.Download(ctx, url, destPath, expectedSHA, headers, noResume)
	}
	localDest, err := storage.StagingPath(a.c, destPath, url)
	if err != nil {
		return "", "", err
	}
	finalLocal, sum, err := dl.Download(ctx, url, localDest, expectedSHA, headers, noResume)
	if err != nil {
		return "", "", err
	}
	client, err := storage.NewS3ClientFromConfig(a.c)
	if err != nil {
		a.markS3UploadError(url, finalLocal, err)
		return "", "", err
	}
	if err := client.PutFile(ctx, destPath, finalLocal, "application/octet-stream"); err != nil {
		a.markS3UploadError(url, finalLocal, err)
		return "", "", err
	}
	if _, err := os.Stat(finalLocal + ".sha256"); err == nil && a.c.Storage.S3.UploadSHA256File {
		if err := client.PutFile(ctx, destPath+".sha256", finalLocal+".sha256", "text/plain; charset=utf-8"); err != nil {
			if delErr := client.DeleteObject(ctx, destPath); delErr != nil && a.l != nil {
				a.l.Warnf("s3 cleanup failed for %s after checksum upload failure: %v; remove the object manually before retrying", destPath, delErr)
			}
			a.markS3UploadError(url, finalLocal, fmt.Errorf("checksum: %w", err))
			return "", "", err
		}
	}
	if a.st != nil {
		if err := a.st.ReplaceDownloadDest(url, finalLocal, destPath); err != nil {
			if a.l != nil {
				a.l.Warnf("s3 upload complete but state update failed url=%s local=%s remote=%s: %v; attempting remote cleanup", logging.SanitizeURL(url), finalLocal, destPath, err)
			}
			if delErr := client.DeleteObject(ctx, destPath); delErr != nil && a.l != nil {
				a.l.Warnf("s3 cleanup failed for %s after state update failure: %v; remove the object manually before retrying", destPath, delErr)
			}
			if _, statErr := os.Stat(finalLocal + ".sha256"); statErr == nil && a.c.Storage.S3.UploadSHA256File {
				if delErr := client.DeleteObject(ctx, destPath+".sha256"); delErr != nil && a.l != nil {
					a.l.Warnf("s3 checksum cleanup failed for %s after state update failure: %v; remove the object manually before retrying", destPath+".sha256", delErr)
				}
			}
			return "", "", err
		}
	}
	if a.l != nil {
		a.l.Infof("uploaded to s3: %s", destPath)
	}
	return destPath, sum, nil
}

func (a *Auto) markS3UploadError(url, dest string, err error) {
	if a.st == nil || err == nil {
		return
	}
	_ = a.st.UpdateDownloadStatus(url, dest, "error", fmt.Sprintf("s3 upload: %v", err))
}
