package resolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"modfetch/internal/config"
)

type CivitAI struct{}

// civitaiBaseURL allows tests to override the API host.
var civitaiBaseURL = "https://civitai.com"

// CivitaiBaseForTest returns the current base for tests.
func CivitaiBaseForTest() string { return civitaiBaseURL }

// SetCivitaiBaseForTest overrides the base URL for CivitAI API (tests only).
func SetCivitaiBaseForTest(u string) { civitaiBaseURL = strings.TrimRight(u, "/") }

func (c *CivitAI) CanHandle(u string) bool { return strings.HasPrefix(u, "civitai://") }

// civitai://model/{id}?version={versionId}&file={substring}
func (c *CivitAI) Resolve(ctx context.Context, uri string, cfg *config.Config) (*Resolved, error) {
	if !c.CanHandle(uri) { return nil, errors.New("unsupported scheme") }
	s := strings.TrimPrefix(uri, "civitai://")
	var rawPath, rawQuery string
	if i := strings.IndexByte(s, '?'); i >= 0 {
		rawPath = s[:i]
		rawQuery = s[i+1:]
	} else {
		rawPath = s
	}
	parts := strings.Split(rawPath, "/")
	if len(parts) < 2 || parts[0] != "model" {
		return nil, errors.New("civitai uri must be civitai://model/{id}")
	}
	modelID := parts[1]
	q, _ := url.ParseQuery(rawQuery)
	versionID := q.Get("version")
	fileSub := strings.ToLower(q.Get("file"))

	client := &http.Client{Timeout: 30 * time.Second}
	headers := map[string]string{}
	if cfg != nil && cfg.Sources.CivitAI.Enabled {
		if env := strings.TrimSpace(cfg.Sources.CivitAI.TokenEnv); env != "" {
			if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
				headers["Authorization"] = "Bearer " + tok
			}
		}
	}

	var files []civitFile
	if versionID != "" {
		vf, err := civitaiFetchVersionFiles(ctx, client, headers, versionID)
		if err != nil { return nil, err }
		files = vf
	} else {
		mf, err := civitaiFetchModelFiles(ctx, client, headers, modelID)
		if err != nil { return nil, err }
		files = mf
	}
	if len(files) == 0 { return nil, errors.New("no files found for civitai model/version") }

	// Select file
	pick := -1
	for i, f := range files {
		if fileSub != "" && strings.Contains(strings.ToLower(f.Name), fileSub) {
			pick = i; break
		}
	}
	if pick == -1 {
		for i, f := range files { if f.Primary { pick = i; break } }
	}
	if pick == -1 {
		for i, f := range files { if strings.EqualFold(f.Type, "Model") { pick = i; break } }
	}
	if pick == -1 { pick = 0 }

	download := files[pick].DownloadURL
	if download == "" { return nil, errors.New("selected civitai file has empty downloadUrl") }

	return &Resolved{URL: download, Headers: headers}, nil
}

type civitModel struct {
	ModelVersions []civitVersion `json:"modelVersions"`
}

type civitVersion struct {
	ID    int         `json:"id"`
	Files []civitFile `json:"files"`
}

type civitFile struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Primary     bool   `json:"primary"`
	DownloadURL string `json:"downloadUrl"`
}

func civitaiFetchModelFiles(ctx context.Context, client *http.Client, headers map[string]string, modelID string) ([]civitFile, error) {
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/models/%s", civitaiBaseURL, url.PathEscape(modelID)), nil)
	for k, v := range headers { req.Header.Set(k, v) }
	resp, err := client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 { return nil, fmt.Errorf("civitai models: %s", resp.Status) }
	var m civitModel
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil { return nil, err }
	if len(m.ModelVersions) == 0 { return nil, errors.New("civitai: no modelVersions") }
	// choose latest by ID
	v := m.ModelVersions[0]
	for _, vv := range m.ModelVersions { if vv.ID > v.ID { v = vv } }
	return v.Files, nil
}

func civitaiFetchVersionFiles(ctx context.Context, client *http.Client, headers map[string]string, versionID string) ([]civitFile, error) {
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/model-versions/%s", civitaiBaseURL, url.PathEscape(versionID)), nil)
	for k, v := range headers { req.Header.Set(k, v) }
	resp, err := client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 { return nil, fmt.Errorf("civitai version: %s", resp.Status) }
	var v civitVersion
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil { return nil, err }
	return v.Files, nil
}

