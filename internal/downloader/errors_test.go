package downloader

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestDownloadErrorMessagesAndUnwrap(t *testing.T) {
	if got := (rateLimitedError{after: time.Second, msg: "slow down"}).Error(); got != "slow down" {
		t.Fatalf("rateLimitedError Error() = %q", got)
	}
	if got := (checksumMismatchError{msg: "bad sha"}).Error(); got != "bad sha" {
		t.Fatalf("checksumMismatchError Error() = %q", got)
	}

	status := httpStatusError{statusCode: http.StatusForbidden, msg: "403 Forbidden", remediation: "add token"}
	if got := status.Error(); !strings.Contains(got, "403 Forbidden") || !strings.Contains(got, "remediation: add token") {
		t.Fatalf("unexpected status error message: %q", got)
	}
	status.remediation = ""
	if got := status.Error(); got != "403 Forbidden" {
		t.Fatalf("expected bare status message, got %q", got)
	}

	base := errors.New("wrapped")
	nonRetryable := nonRetryableError{err: base}
	if got := nonRetryable.Error(); got != "wrapped" {
		t.Fatalf("nonRetryableError Error() = %q", got)
	}
	if !errors.Is(nonRetryable, base) {
		t.Fatal("expected nonRetryableError to unwrap base error")
	}
}

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

func TestRetryAfterAndHostHelpers(t *testing.T) {
	if got := parseRetryAfter("3"); got != 3*time.Second {
		t.Fatalf("delta retry-after = %v", got)
	}
	if got := parseRetryAfter("-1"); got != 0 {
		t.Fatalf("negative retry-after = %v", got)
	}
	if got := parseRetryAfter("not a date"); got != 0 {
		t.Fatalf("invalid retry-after = %v", got)
	}
	future := time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(future); got <= 0 {
		t.Fatalf("future HTTP-date retry-after = %v", got)
	}

	if !hostIs("cdn.huggingface.co.", "huggingface.co") {
		t.Fatal("expected subdomain host match")
	}
	if hostIs("not-huggingface.co", "huggingface.co") {
		t.Fatal("did not expect suffix without dot to match")
	}
	if got := hostFromURL("https://cdn.example.com/path"); got != "cdn.example.com" {
		t.Fatalf("hostFromURL = %q", got)
	}
	if got := hostFromURL("://bad"); got != "" {
		t.Fatalf("bad hostFromURL = %q", got)
	}
}

func TestFriendlyHTTPStatusProblemUsesConfiguredTokenEnv(t *testing.T) {
	cfg := &config.Config{}
	cfg.Sources.HuggingFace.TokenEnv = "CUSTOM_HF"
	cfg.Sources.CivitAI.TokenEnv = "CUSTOM_CIVITAI"

	msg, remediation := friendlyHTTPStatusProblem(cfg, "huggingface.co", http.StatusUnauthorized, "401 Unauthorized", false)
	if !strings.Contains(msg, "token required") || !strings.Contains(remediation, "CUSTOM_HF") {
		t.Fatalf("unexpected hf 401 guidance msg=%q remediation=%q", msg, remediation)
	}
	msg, remediation = friendlyHTTPStatusProblem(cfg, "civitai.com", http.StatusForbidden, "403 Forbidden", false)
	if !strings.Contains(msg, "access denied") || !strings.Contains(remediation, "CUSTOM_CIVITAI") {
		t.Fatalf("unexpected civitai 403 guidance msg=%q remediation=%q", msg, remediation)
	}
	msg, remediation = friendlyHTTPStatusProblem(cfg, "example.com", http.StatusTeapot, "418 I'm a teapot", false)
	if msg != "418 I'm a teapot" || remediation != "" {
		t.Fatalf("unexpected default status guidance msg=%q remediation=%q", msg, remediation)
	}
}
