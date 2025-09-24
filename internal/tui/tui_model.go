package tui

import (
	"context"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/downloader"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/resolver"
	"github.com/jxwalker/modfetch/internal/state"
)

type TUIModel struct {
	cfg       *config.Config
	st        *state.DB
	rows      []state.DownloadRow
	running   map[string]context.CancelFunc
	runningMu sync.RWMutex
	ephems    map[string]ephemeral
	prev      map[string]obs
}

type ephemeral struct {
	url, dest string
	headers   map[string]string
	sha       string
}

type obs struct {
}

type noopMetrics struct{}

func (n *noopMetrics) AddBytes(int64)                 {}
func (n *noopMetrics) IncRetries(int64)               {}
func (n *noopMetrics) IncDownloadsSuccess()           {}
func (n *noopMetrics) ObserveDownloadSeconds(float64) {}
func (n *noopMetrics) Write() error                   { return nil }

func NewTUIModel(cfg *config.Config, st *state.DB) *TUIModel {
	return &TUIModel{
		cfg:     cfg,
		st:      st,
		running: make(map[string]context.CancelFunc),
		ephems:  make(map[string]ephemeral),
		prev:    make(map[string]obs),
	}
}

func (m *TUIModel) LoadRows() error {
	rows, err := m.st.ListDownloads()
	if err != nil {
		return err
	}
	m.rows = rows
	return nil
}

func (m *TUIModel) GetRows() []state.DownloadRow {
	return m.rows
}

func (m *TUIModel) GetRunning() map[string]context.CancelFunc {
	m.runningMu.RLock()
	defer m.runningMu.RUnlock()

	snapshot := make(map[string]context.CancelFunc)
	for k, v := range m.running {
		snapshot[k] = v
	}
	return snapshot
}

func (m *TUIModel) GetEphems() map[string]ephemeral {
	return m.ephems
}

func (m *TUIModel) GetPrev() map[string]obs {
	return m.prev
}

func (m *TUIModel) AddEphemeral(url, dest string, headers map[string]string, sha string) {
	m.ephems[url+"|"+dest] = ephemeral{url: url, dest: dest, headers: headers, sha: sha}
}

func (m *TUIModel) ProgressFor(url, dest string) (int64, int64, string) {
	for _, row := range m.rows {
		if row.URL == url && row.Dest == dest {
			return 0, row.Size, row.Status
		}
	}
	return 0, 0, "unknown"
}

func (m *TUIModel) FilteredRows(statuses []string) []state.DownloadRow {
	if len(statuses) == 0 {
		return m.rows
	}
	var filtered []state.DownloadRow
	for _, row := range m.rows {
		for _, status := range statuses {
			if row.Status == status {
				filtered = append(filtered, row)
				break
			}
		}
	}
	return filtered
}

func (m *TUIModel) DestGuess(url string) string {
	if url == "" {
		return ""
	}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			filename := parts[len(parts)-1]
			if filename != "" && !strings.Contains(filename, "?") {
				return filepath.Join(m.cfg.General.DownloadRoot, filename)
			}
		}
	}
	return filepath.Join(m.cfg.General.DownloadRoot, "download")
}

func (m *TUIModel) PreflightForDownload(url, dest string) error {
	if url == "" {
		return fmt.Errorf("url required")
	}
	if dest == "" {
		return fmt.Errorf("dest required")
	}
	if err := os.MkdirAll(m.cfg.General.DownloadRoot, 0o755); err != nil {
		return fmt.Errorf("download_root not writable: %w", err)
	}
	if m.cfg.General.StagePartials {
		parts := m.cfg.General.PartialsRoot
		if strings.TrimSpace(parts) == "" {
			parts = filepath.Join(m.cfg.General.DownloadRoot, ".parts")
		}
		if err := os.MkdirAll(parts, 0o755); err != nil {
			return fmt.Errorf("create parts dir: %w", err)
		}
		if err := tryWrite(parts); err != nil {
			return fmt.Errorf("parts dir not writable: %w", err)
		}
	}

	d := strings.TrimSpace(dest)
	if d != "" {
		if err := os.MkdirAll(filepath.Dir(d), 0o755); err != nil {
			return fmt.Errorf("create dest dir: %w", err)
		}
		if err := tryWrite(filepath.Dir(d)); err != nil {
			return fmt.Errorf("dest dir not writable: %w", err)
		}
	}

	return nil
}

func (m *TUIModel) StartDownload(ctx context.Context, urlStr, dest, sha string, headers map[string]string) error {
	if err := m.PreflightForDownload(urlStr, dest); err != nil {
		return err
	}

	resolved := urlStr
	if headers == nil {
		headers = map[string]string{}
	}

	if strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://") {
		if u, err := neturl.Parse(resolved); err == nil {
			h := strings.ToLower(u.Hostname())
			if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				if len(parts) >= 2 {
					modelID := parts[1]
					q := u.Query()
					ver := q.Get("modelVersionId")
					if ver == "" {
						ver = q.Get("version")
					}
					civ := "civitai://model/" + modelID
					if strings.TrimSpace(ver) != "" {
						civ += "?version=" + neturl.QueryEscape(ver)
					}
					if res, err := resolver.Resolve(ctx, civ, m.cfg); err == nil {
						resolved = res.URL
						headers = res.Headers
					}
				}
			}
			if hostIs(h, "huggingface.co") {
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				if len(parts) >= 5 && parts[2] == "blob" {
					owner := parts[0]
					repo := parts[1]
					rev := parts[3]
					filePath := strings.Join(parts[4:], "/")
					hf := "hf://" + owner + "/" + repo + "/" + filePath + "?rev=" + rev
					if res, err := resolver.Resolve(ctx, hf, m.cfg); err == nil {
						resolved = res.URL
						headers = res.Headers
					}
				}
			}
		}
	}

	if strings.HasPrefix(resolved, "hf://") || strings.HasPrefix(resolved, "civitai://") {
		res, err := resolver.Resolve(ctx, resolved, m.cfg)
		if err != nil {
			return err
		}
		resolved = res.URL
		headers = res.Headers
	} else {
		if u, err := neturl.Parse(resolved); err == nil {
			h := strings.ToLower(u.Hostname())
			if hostIs(h, "huggingface.co") && m.cfg.Sources.HuggingFace.Enabled {
				env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv)
				if env == "" {
					env = "HF_TOKEN"
				}
				if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
					headers["Authorization"] = "Bearer " + tok
				}
			}
			if hostIs(h, "civitai.com") && m.cfg.Sources.CivitAI.Enabled {
				env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
				if env == "" {
					env = "CIVITAI_TOKEN"
				}
				if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
					headers["Authorization"] = "Bearer " + tok
				}
			}
		}
	}

	key := resolved + "|" + dest

	m.runningMu.Lock()
	if _, exists := m.running[key]; exists {
		m.runningMu.Unlock()
		return fmt.Errorf("already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	m.running[key] = cancel
	m.runningMu.Unlock()

	go func() {
		defer func() {
			m.runningMu.Lock()
			delete(m.running, key)
			m.runningMu.Unlock()
		}()

		if !m.cfg.Network.DisableAuthPreflight {
			reach, info := downloader.CheckReachable(ctx, m.cfg, resolved, headers)
			if !reach {
				_ = m.st.UpsertDownload(state.DownloadRow{URL: resolved, Dest: dest, Status: "hold", LastError: info})
				return
			}
		}

		log := logging.New("error", false)
		chunked := downloader.NewChunked(m.cfg, log, m.st, &noopMetrics{})
		_, _, err := chunked.Download(ctx, resolved, dest, sha, headers, false)
		if err != nil {
			return
		}
	}()

	return nil
}

func hostIs(hostname, target string) bool {
	return strings.EqualFold(hostname, target) || strings.HasSuffix(strings.ToLower(hostname), "."+strings.ToLower(target))
}
