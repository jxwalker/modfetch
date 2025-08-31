package downloader

import (
    "fmt"
    neturl "net/url"
    "strings"

    "modfetch/internal/config"
)

// friendlyHTTPStatusMessage creates a host-aware error message for common auth-related statuses.
// hadAuth indicates whether an Authorization header was sent.
func friendlyHTTPStatusMessage(cfg *config.Config, host string, statusCode int, status string, hadAuth bool) string {
    h := strings.ToLower(strings.TrimSpace(host))
    hfEnv := strings.TrimSpace(cfg.Sources.HuggingFace.TokenEnv)
    if hfEnv == "" { hfEnv = "HF_TOKEN" }
    civEnv := strings.TrimSpace(cfg.Sources.CivitAI.TokenEnv)
    if civEnv == "" { civEnv = "CIVITAI_TOKEN" }

    mk := func(base string) string {
        if hostIs(h, "huggingface.co") {
            if hadAuth {
                return fmt.Sprintf("%s (Hugging Face: token present but not authorized; ensure you have access and have accepted the repository license)", base)
            }
            return fmt.Sprintf("%s (Hugging Face: token required; export %s)", base, hfEnv)
        }
        if hostIs(h, "civitai.com") {
            if hadAuth {
                return fmt.Sprintf("%s (CivitAI: token present but not authorized; ensure your account has access and age-gating or permissions allow download)", base)
            }
            return fmt.Sprintf("%s (CivitAI: token may be required; export %s)", base, civEnv)
        }
        return base
    }

    switch statusCode {
    case 429:
        return mk("429 Too Many Requests: rate limited")
    case 401:
        if hadAuth {
            return mk("401 Unauthorized: token present but not authorized")
        }
        return mk("401 Unauthorized: token required")
    case 403:
        if hadAuth {
            return mk("403 Forbidden: token lacks permission")
        }
        return mk("403 Forbidden: access denied (may require token)")
    case 404:
        if hadAuth {
            return mk("404 Not Found (or private): with token; check path/revision or permissions")
        }
        return mk("404 Not Found: check path/revision; if private, provide token")
    default:
        // Preserve original status text
        return fmt.Sprintf("unexpected status: %s", status)
    }
}

// hostIs returns true if h equals root or is a subdomain of root.
func hostIs(h, root string) bool {
    h = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
    root = strings.ToLower(strings.TrimSpace(root))
    return h == root || strings.HasSuffix(h, "."+root)
}

// hostFromURL extracts hostname from a URL string.
func hostFromURL(raw string) string {
    if u, err := neturl.Parse(raw); err == nil && u != nil { return u.Hostname() }
    return ""
}

