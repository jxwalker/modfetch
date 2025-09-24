package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/downloader"
	"github.com/jxwalker/modfetch/internal/state"
)

type TUIModel struct {
	cfg     *config.Config
	st      *state.DB
	rows    []state.DownloadRow
	running map[string]context.CancelFunc
	ephems  map[string]ephemeral
	prev    map[string]obs
}

type ephemeral struct {
	url, dest string
	headers   map[string]string
	sha       string
}

type obs struct {
}

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
	return m.running
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
			return row.Size, row.Size, row.Status
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
	} else {
		d := strings.TrimSpace(dest)
		if d != "" {
			if err := os.MkdirAll(filepath.Dir(d), 0o755); err != nil {
				return fmt.Errorf("create dest dir: %w", err)
			}
			if err := tryWrite(filepath.Dir(d)); err != nil {
				return fmt.Errorf("dest dir not writable: %w", err)
			}
		}
	}
	return nil
}

func (m *TUIModel) StartDownload(ctx context.Context, url, dest, sha string, headers map[string]string) error {
	if err := m.PreflightForDownload(url, dest); err != nil {
		return err
	}

	key := url + "|" + dest
	if _, exists := m.running[key]; exists {
		return fmt.Errorf("already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	m.running[key] = cancel

	go func() {
		defer func() {
			delete(m.running, key)
		}()

		chunked := downloader.NewChunked(m.cfg, nil, m.st, nil)
		_, _, err := chunked.Download(ctx, url, dest, sha, headers, false)
		if err != nil {
			return
		}
	}()

	return nil
}
