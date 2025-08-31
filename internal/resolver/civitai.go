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
	"modfetch/internal/util"
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
	var modelName string
	var verName string
	var verID string
	if versionID != "" {
		v, err := civitaiFetchVersion(ctx, client, headers, versionID)
		if err != nil { return nil, err }
		files = v.Files
		verName = v.Name
		verID = fmt.Sprintf("%d", v.ID)
		// Fetch model to get model name if needed
		if v.ModelID != 0 {
			m, err := civitaiFetchModel(ctx, client, headers, fmt.Sprintf("%d", v.ModelID))
			if err == nil { modelName = m.Name }
		}
	} else {
		m, err := civitaiFetchModel(ctx, client, headers, modelID)
		if err != nil { return nil, err }
		modelName = m.Name
		// choose latest by version ID
		if len(m.ModelVersions) == 0 { return nil, errors.New("civitai: no modelVersions") }
		v := m.ModelVersions[0]
		for _, vv := range m.ModelVersions { if vv.ID > v.ID { v = vv } }
		files = v.Files
		verName = v.Name
		verID = fmt.Sprintf("%d", v.ID)
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
	fileName := files[pick].Name
	// Suggested filename: "<ModelName> - <OriginalFileName>"
	suggested := fileName
	if strings.TrimSpace(modelName) != "" {
		suggested = modelName + " - " + fileName
	}
	suggested = util.SafeFileName(suggested)

return &Resolved{URL: download, Headers: headers, ModelName: modelName, VersionName: verName, VersionID: verID, FileName: fileName, FileType: files[pick].Type, SuggestedFilename: suggested}, nil
}

type civitModel struct {
	Name          string        `json:"name"`
	ModelVersions []civitVersion `json:"modelVersions"`
}

type civitVersion struct {
	ID      int         `json:"id"`
	Name    string      `json:"name"`
	ModelID int         `json:"modelId"`
	Files   []civitFile `json:"files"`
}

type civitFile struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Primary     bool   `json:"primary"`
	DownloadURL string `json:"downloadUrl"`
}

func civitaiFetchModel(ctx context.Context, client *http.Client, headers map[string]string, modelID string) (civitModel, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/models/%s", civitaiBaseURL, url.PathEscape(modelID)), nil)
	for k, v := range headers { req.Header.Set(k, v) }
	resp, err := client.Do(req)
	if err != nil { return civitModel{}, err }
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 { return civitModel{}, fmt.Errorf("civitai models: %s", resp.Status) }
	var m civitModel
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil { return civitModel{}, err }
	return m, nil
}

func civitaiFetchModelFiles(ctx context.Context, client *http.Client, headers map[string]string, modelID string) ([]civitFile, error) {
	m, err := civitaiFetchModel(ctx, client, headers, modelID)
	if err != nil { return nil, err }
	if len(m.ModelVersions) == 0 { return nil, errors.New("civitai: no modelVersions") }
	// choose latest by ID
	v := m.ModelVersions[0]
	for _, vv := range m.ModelVersions { if vv.ID > v.ID { v = vv } }
	return v.Files, nil
}

func civitaiFetchVersion(ctx context.Context, client *http.Client, headers map[string]string, versionID string) (civitVersion, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/model-versions/%s", civitaiBaseURL, url.PathEscape(versionID)), nil)
	for k, v := range headers { req.Header.Set(k, v) }
	resp, err := client.Do(req)
	if err != nil { return civitVersion{}, err }
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 { return civitVersion{}, fmt.Errorf("civitai version: %s", resp.Status) }
	var v civitVersion
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil { return civitVersion{}, err }
	return v, nil
}

