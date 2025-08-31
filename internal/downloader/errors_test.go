package downloader

import (
    "strings"
    "testing"

    "modfetch/internal/config"
)

func TestFriendlyStatus_429_RateLimited(t *testing.T) {
    cfg := &config.Config{}

    // Hugging Face host context
    hf := friendlyHTTPStatusMessage(cfg, "huggingface.co", 429, "429 Too Many Requests", false)
    if !strings.Contains(strings.ToLower(hf), "429") || !strings.Contains(strings.ToLower(hf), "rate limited") {
        t.Fatalf("expected 429 rate limited message for HF, got: %q", hf)
    }

    // CivitAI host context
    civ := friendlyHTTPStatusMessage(cfg, "civitai.com", 429, "429 Too Many Requests", true)
    if !strings.Contains(strings.ToLower(civ), "429") || !strings.Contains(strings.ToLower(civ), "rate limited") {
        t.Fatalf("expected 429 rate limited message for CivitAI, got: %q", civ)
    }
}

