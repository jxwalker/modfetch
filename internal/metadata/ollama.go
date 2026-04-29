package metadata

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/state"
)

type OllamaFetcher struct {
	client *http.Client
}

func NewOllamaFetcher(client *http.Client) *OllamaFetcher {
	return &OllamaFetcher{client: client}
}

func (f *OllamaFetcher) Source() string {
	return "ollama"
}

func (f *OllamaFetcher) CanHandle(rawURL string) bool {
	_, _, err := parseOllamaLibraryURL(rawURL)
	return err == nil
}

func (f *OllamaFetcher) FetchMetadata(ctx context.Context, rawURL string) (*state.ModelMetadata, error) {
	model, tag, err := parseOllamaLibraryURL(rawURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ollamaLibraryURL(model), nil)
	if err != nil {
		return f.basicMetadata(rawURL, model, tag), nil
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := f.client.Do(req)
	if err != nil {
		return f.basicMetadata(rawURL, model, tag), nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return f.basicMetadata(rawURL, model, tag), nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return f.basicMetadata(rawURL, model, tag), nil
	}

	page := string(body)
	meta := f.basicMetadata(rawURL, model, tag)
	if title := firstNonEmpty(metaContent(page, "og:title"), htmlTitle(page)); title != "" {
		meta.ModelName = title
	}
	meta.Description = truncateString(firstNonEmpty(metaContent(page, "description"), metaContent(page, "og:description")), 5000)
	meta.ThumbnailURL = metaContent(page, "og:image")
	meta.DownloadCount = parseCompactCount(firstNonEmpty(textForAttr(page, "x-test-pull-count"), textForAttr(page, "data-pull-count")))
	meta.Tags = appendUniqueStrings(meta.Tags, sizeTags(page)...)
	return meta, nil
}

func (f *OllamaFetcher) basicMetadata(rawURL, model, tag string) *state.ModelMetadata {
	tags := []string{"ollama"}
	if tag != "" {
		tags = append(tags, tag)
	}
	return &state.ModelMetadata{
		DownloadURL: rawURL,
		ModelName:   model,
		ModelID:     "ollama/" + model,
		Version:     tag,
		Source:      "ollama",
		Tags:        tags,
		RepoURL:     ollamaLibraryURL(model),
		HomepageURL: ollamaLibraryURL(model),
		ModelType:   "LLM",
		FileFormat:  "ollama",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func parseOllamaLibraryURL(rawURL string) (model, tag string, err error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", fmt.Errorf("parse Ollama URL: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	if host != "ollama.com" && !strings.HasSuffix(host, ".ollama.com") {
		return "", "", fmt.Errorf("invalid Ollama host")
	}
	parts := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	if len(parts) < 2 || parts[0] != "library" || parts[1] == "" || parts[1] == "tags" {
		return "", "", fmt.Errorf("invalid Ollama library URL format")
	}
	modelSpec, err := url.PathUnescape(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("decode Ollama model: %w", err)
	}
	if strings.Contains(modelSpec, ":") {
		model, tag, _ = strings.Cut(modelSpec, ":")
	} else {
		model = modelSpec
	}
	model = strings.TrimSpace(model)
	tag = strings.TrimSpace(tag)
	if model == "" {
		return "", "", fmt.Errorf("invalid Ollama library URL format")
	}
	return model, tag, nil
}

func ollamaLibraryURL(model string) string {
	return (&url.URL{Scheme: "https", Host: "ollama.com", Path: path.Join("/library", model)}).String()
}

var (
	metaTagRe    = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
	doubleAttrRe = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*"([^"]*)"`)
	singleAttrRe = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*'([^']*)'`)
	titleRe      = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	spanRe       = regexp.MustCompile(`(?is)<span([^>]*)>(.*?)</span>`)
	tagStripRe   = regexp.MustCompile(`(?is)<[^>]*>`)
)

func metaContent(page, name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, tag := range metaTagRe.FindAllString(page, -1) {
		attrs := htmlAttrs(tag)
		if strings.EqualFold(attrs["name"], name) || strings.EqualFold(attrs["property"], name) {
			return cleanHTMLText(attrs["content"])
		}
	}
	return ""
}

func htmlAttrs(tag string) map[string]string {
	attrs := map[string]string{}
	for _, re := range []*regexp.Regexp{doubleAttrRe, singleAttrRe} {
		for _, match := range re.FindAllStringSubmatch(tag, -1) {
			if len(match) == 3 {
				attrs[strings.ToLower(match[1])] = html.UnescapeString(match[2])
			}
		}
	}
	return attrs
}

func htmlTitle(page string) string {
	match := titleRe.FindStringSubmatch(page)
	if len(match) < 2 {
		return ""
	}
	return cleanHTMLText(match[1])
}

func textForAttr(page, attr string) string {
	for _, match := range spanRe.FindAllStringSubmatch(page, -1) {
		if len(match) < 3 || !hasHTMLAttr(match[1], attr) {
			continue
		}
		return cleanHTMLText(match[2])
	}
	return ""
}

func sizeTags(page string) []string {
	tags := []string{}
	for _, match := range spanRe.FindAllStringSubmatch(page, -1) {
		if len(match) < 3 || !hasHTMLAttr(match[1], "x-test-size") {
			continue
		}
		if tag := cleanHTMLText(match[2]); tag != "" {
			tags = append(tags, strings.ToLower(tag))
		}
	}
	return tags
}

func hasHTMLAttr(attrs, attr string) bool {
	re := regexp.MustCompile(fmt.Sprintf(`(?is)(?:^|\s)%s(?:\s|=|$)`, regexp.QuoteMeta(attr)))
	return re.MatchString(attrs)
}

func cleanHTMLText(value string) string {
	value = tagStripRe.ReplaceAllString(value, "")
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(value), " ")
}

func parseCompactCount(value string) int {
	value = strings.TrimSpace(strings.ToUpper(strings.ReplaceAll(value, ",", "")))
	if value == "" {
		return 0
	}
	multiplier := 1.0
	switch {
	case strings.HasSuffix(value, "K"):
		multiplier = 1_000
		value = strings.TrimSuffix(value, "K")
	case strings.HasSuffix(value, "M"):
		multiplier = 1_000_000
		value = strings.TrimSuffix(value, "M")
	case strings.HasSuffix(value, "B"):
		multiplier = 1_000_000_000
		value = strings.TrimSuffix(value, "B")
	}
	count, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return int(count * multiplier)
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	out := make([]string, 0, len(values)+len(additions))
	for _, value := range append(values, additions...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[strings.ToLower(value)] {
			continue
		}
		seen[strings.ToLower(value)] = true
		out = append(out, value)
	}
	return out
}
