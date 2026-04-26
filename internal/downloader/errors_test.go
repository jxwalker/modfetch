package downloader

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestFriendlyStatus_429_RateLimited(t *testing.T) {
	cfg := &config.Config{}
	cases := []struct {
		name    string
		host    string
		hadAuth bool
	}{
		{"HF anon", "huggingface.co", false},
		{"Civitai authed", "civitai.com", true},
		{"Generic host", "example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := friendlyHTTPStatusMessage(cfg, tc.host, 429, "429 Too Many Requests", tc.hadAuth)
			lo := strings.ToLower(out)
			if !strings.Contains(lo, "429") || !strings.Contains(lo, "rate limited") {
				t.Fatalf("expected 429 rate limited message for %s, got: %q", tc.host, out)
			}
		})
	}
}

func TestIsRetryableDownloadErrorByHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{name: "request timeout retries", statusCode: http.StatusRequestTimeout, want: true},
		{name: "too early retries", statusCode: http.StatusTooEarly, want: true},
		{name: "rate limit retries", statusCode: http.StatusTooManyRequests, want: true},
		{name: "server error retries", statusCode: http.StatusServiceUnavailable, want: true},
		{name: "unauthorized does not retry", statusCode: http.StatusUnauthorized, want: false},
		{name: "forbidden does not retry", statusCode: http.StatusForbidden, want: false},
		{name: "not found does not retry", statusCode: http.StatusNotFound, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := httpStatusError{statusCode: tt.statusCode, msg: http.StatusText(tt.statusCode)}
			if got := isRetryableDownloadError(err); got != tt.want {
				t.Fatalf("isRetryableDownloadError(%d) = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestIsRetryableDownloadErrorSpecialCases(t *testing.T) {
	if isRetryableDownloadError(checksumMismatchError{msg: "sha256 mismatch"}) {
		t.Fatal("checksum mismatch should not retry")
	}
	if isRetryableDownloadError(nonRetryableError{err: errors.New("server ignored range")}) {
		t.Fatal("non-retryable wrapper should not retry")
	}
	if !isRetryableDownloadError(rateLimitedError{after: time.Second, msg: "rate limited"}) {
		t.Fatal("rate limit error should be retryable")
	}
	if !isRetryableDownloadError(errors.New("temporary network failure")) {
		t.Fatal("plain transport-style errors should retry")
	}
}
