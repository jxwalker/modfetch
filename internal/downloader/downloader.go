package downloader

import (
	"context"

	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

// Interface is the common downloader interface used across implementations.
type Interface interface {
	Download(ctx context.Context, url, destPath, expectedSHA string, headers map[string]string, noResume bool) (finalPath string, sha256 string, err error)
}

// Auto implements Interface by delegating to the chunked downloader which
// already contains robust fallback to the single-stream downloader.
// This centralizes the selection logic behind Interface.
type Auto struct{
	c *config.Config
	l *logging.Logger
	st *state.DB
	m interface{ AddBytes(int64); IncRetries(int64); IncDownloadsSuccess(); ObserveDownloadSeconds(float64); Write() error }
}

func NewAuto(c *config.Config, l *logging.Logger, st *state.DB, m interface{ AddBytes(int64); IncRetries(int64); IncDownloadsSuccess(); ObserveDownloadSeconds(float64); Write() error }) *Auto {
	return &Auto{c: c, l: l, st: st, m: m}
}

func (a *Auto) Download(ctx context.Context, url, destPath, expectedSHA string, headers map[string]string, noResume bool) (string, string, error) {
	// Delegate to chunked downloader which handles head probe and fallback.
	return NewChunked(a.c, a.l, a.st, a.m).Download(ctx, url, destPath, expectedSHA, headers, noResume)
}
