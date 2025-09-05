package downloader

import (
	"strings"
	"testing"

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
