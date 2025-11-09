package tui

import (
	"context"
	"net/url"
	"strings"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/resolver"
)

// NormalizeHuggingFaceURL converts huggingface.co blob URLs to hf:// URIs.
// Example: https://huggingface.co/owner/repo/blob/main/path/to/file.bin
//
//	-> hf://owner/repo/path/to/file.bin?rev=main
//
// Returns the normalized URI if the input is a HF blob URL,
// otherwise returns the original input unchanged.
func NormalizeHuggingFaceURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}

	// Only process http(s) URLs
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return s
	}

	u, err := url.Parse(s)
	if err != nil {
		return s
	}

	// Check if it's a huggingface.co domain
	hostname := strings.ToLower(u.Hostname())
	if !hostIs(hostname, "huggingface.co") {
		return s
	}

	// Parse HF blob URL: /owner/repo/blob/rev/path/to/file
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 5 || parts[2] != "blob" {
		return s
	}

	owner := parts[0]
	repo := parts[1]
	rev := parts[3]
	filePath := strings.Join(parts[4:], "/")

	// Build hf:// URI
	normalized := "hf://" + owner + "/" + repo + "/" + filePath + "?rev=" + rev
	return normalized
}

// NormalizeCivitAIURL converts civitai.com model page URLs to civitai:// URIs.
// Example: https://civitai.com/models/12345?modelVersionId=67890
//
//	-> civitai://model/12345?version=67890
//
// Returns the normalized URI if the input is a CivitAI model page URL,
// otherwise returns the original input unchanged.
func NormalizeCivitAIURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}

	// Only process http(s) URLs
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return s
	}

	u, err := url.Parse(s)
	if err != nil {
		return s
	}

	// Check if it's a civitai.com domain
	hostname := strings.ToLower(u.Hostname())
	if !hostIs(hostname, "civitai.com") {
		return s
	}

	// Check if it's a model page URL (/models/...)
	if !strings.HasPrefix(u.Path, "/models/") {
		return s
	}

	// Extract model ID from path
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return s
	}
	modelID := parts[1]

	// Extract version from query parameters
	q := u.Query()
	ver := q.Get("modelVersionId")
	if ver == "" {
		ver = q.Get("version")
	}

	// Build civitai:// URI
	normalized := "civitai://model/" + modelID
	if strings.TrimSpace(ver) != "" {
		normalized += "?version=" + url.QueryEscape(ver)
	}

	return normalized
}

// NormalizeURL normalizes various URL formats to their URI equivalents.
// Handles both CivitAI model pages and Hugging Face blob URLs.
func NormalizeURL(raw string) string {
	// Try HF normalization first
	normalized := NormalizeHuggingFaceURL(raw)
	if normalized != raw {
		return normalized
	}

	// Try CivitAI normalization
	normalized = NormalizeCivitAIURL(raw)
	if normalized != raw {
		return normalized
	}

	return raw
}

// ResolveWithMeta resolves a URL/URI using the resolver and returns metadata.
// Handles hf://, civitai://, and direct HTTP(S) URLs.
// For CivitAI model page URLs and HF blob URLs, normalizes them first.
func ResolveWithMeta(ctx context.Context, raw string, cfg *config.Config) (resolvedURL, fileName, suggested, civType string, err error) {
	s := NormalizeURL(raw)

	// If it's a resolvable URI (hf:// or civitai://), use the resolver
	if strings.HasPrefix(s, "hf://") || strings.HasPrefix(s, "civitai://") {
		res, resolveErr := resolver.Resolve(ctx, s, cfg)
		if resolveErr != nil {
			return "", "", "", "", resolveErr
		}
		return res.URL, res.FileName, res.SuggestedFilename, res.FileType, nil
	}

	// For direct URLs, return as-is
	return s, "", "", "", nil
}

// ResolveAndAttachAuth resolves a URL/URI and attaches authentication headers.
// This is the main function for preparing downloads - it handles:
// - URL normalization (CivitAI, Hugging Face)
// - URI resolution (hf://, civitai://)
// - Authentication header attachment
//
// Returns the resolved URL and headers ready for download.
func ResolveAndAttachAuth(ctx context.Context, raw string, cfg *config.Config, existingHeaders map[string]string) (resolvedURL string, headers map[string]string, err error) {
	if existingHeaders == nil {
		headers = make(map[string]string)
	} else {
		// Copy existing headers
		headers = make(map[string]string, len(existingHeaders))
		for k, v := range existingHeaders {
			headers[k] = v
		}
	}

	// Normalize URL (CivitAI model pages, HF blob URLs -> URIs)
	normalized := NormalizeURL(raw)

	// If it's a resolvable URI (hf:// or civitai://), use the resolver
	if strings.HasPrefix(normalized, "hf://") || strings.HasPrefix(normalized, "civitai://") {
		res, resolveErr := resolver.Resolve(ctx, normalized, cfg)
		if resolveErr != nil {
			return "", nil, resolveErr
		}
		// Use headers from resolver (includes auth)
		return res.URL, res.Headers, nil
	}

	// For direct URLs, attach auth headers if applicable
	headers = AttachAuthHeaders(normalized, cfg, headers)
	return normalized, headers, nil
}

// hostIs checks if hostname matches target domain (including subdomains).
// Example: hostIs("api.civitai.com", "civitai.com") -> true
//
//	hostIs("civitai.com", "civitai.com") -> true
//	hostIs("example.com", "civitai.com") -> false
func hostIs(hostname, target string) bool {
	return hostname == target || strings.HasSuffix(hostname, "."+target)
}

// AttachAuthHeaders adds authentication headers for known domains.
// Currently supports Hugging Face and CivitAI tokens from environment.
func AttachAuthHeaders(urlStr string, cfg *config.Config, headers map[string]string) map[string]string {
	if headers == nil {
		headers = map[string]string{}
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return headers
	}

	hostname := strings.ToLower(u.Hostname())

	// Attach Hugging Face token if configured and targeting HF domain
	if hostIs(hostname, "huggingface.co") && cfg.Sources.HuggingFace.Enabled {
		if token := cfg.Sources.HuggingFace.Token(); token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	}

	// Attach CivitAI token if configured and targeting CivitAI domain
	if hostIs(hostname, "civitai.com") && cfg.Sources.CivitAI.Enabled {
		if token := cfg.Sources.CivitAI.Token(); token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	}

	return headers
}
