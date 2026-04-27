package tui

import (
	"context"
	"encoding/json"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/catalog"
	"github.com/jxwalker/modfetch/internal/placer"
	"github.com/jxwalker/modfetch/internal/state"
	"github.com/jxwalker/modfetch/internal/util"
)

func libraryKey(meta state.ModelMetadata) string {
	if strings.TrimSpace(meta.DownloadURL) != "" {
		return meta.DownloadURL
	}
	return meta.Dest
}

func (m *Model) currentLibraryKey() string {
	if m.librarySelected >= 0 && m.librarySelected < len(m.libraryRows) {
		return libraryKey(m.libraryRows[m.librarySelected])
	}
	return ""
}

func (m *Model) restoreLibrarySelection(key string) {
	if len(m.libraryRows) == 0 {
		m.librarySelected = 0
		return
	}
	if key != "" {
		for i, row := range m.libraryRows {
			if libraryKey(row) == key {
				m.librarySelected = i
				return
			}
		}
	}
	if key == "" && m.librarySelected >= len(m.libraryRows) {
		m.librarySelected = 0
		return
	}
	if m.librarySelected < 0 {
		m.librarySelected = 0
	}
	if m.librarySelected >= len(m.libraryRows) {
		m.librarySelected = len(m.libraryRows) - 1
	}
}

func (m *Model) selectedLibraryRows() []state.ModelMetadata {
	if len(m.librarySelectedKeys) == 0 {
		if m.librarySelected >= 0 && m.librarySelected < len(m.libraryRows) {
			return []state.ModelMetadata{m.libraryRows[m.librarySelected]}
		}
		return nil
	}
	rows := make([]state.ModelMetadata, 0, len(m.librarySelectedKeys))
	for _, row := range m.libraryRows {
		if m.librarySelectedKeys[libraryKey(row)] {
			rows = append(rows, row)
		}
	}
	return rows
}

func (m *Model) applyLibraryFilters(rows []state.ModelMetadata) []state.ModelMetadata {
	filtered := rows[:0]
	for _, row := range rows {
		if m.libraryFilterType != "" && row.ModelType != m.libraryFilterType {
			continue
		}
		if m.libraryFilterSource != "" && row.Source != m.libraryFilterSource {
			continue
		}
		if m.libraryShowFavorites && !row.Favorite {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

const libraryRowLimit = 1000

func normalizeLibraryRows(rows []state.ModelMetadata) []state.ModelMetadata {
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
	})
	if len(rows) > libraryRowLimit {
		return rows[:libraryRowLimit]
	}
	return rows
}

func (m *Model) updateLibraryFilterMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	selectedKey := m.currentLibraryKey()
	if m.libraryFilterEditing {
		switch s {
		case "enter", "ctrl+j":
			m.libraryFilterEditing = false
			m.librarySearch = strings.TrimSpace(m.librarySearchInput.Value())
			m.refreshLibraryData()
			m.restoreLibrarySelection(selectedKey)
			return m, nil
		case "esc":
			m.libraryFilterEditing = false
			m.librarySearchInput.SetValue(m.librarySearch)
			return m, nil
		}
		var cmd tea.Cmd
		m.librarySearchInput, cmd = m.librarySearchInput.Update(msg)
		return m, cmd
	}

	switch s {
	case "esc", "F":
		m.libraryFilterMenu = false
		return m, nil
	case "j", "down":
		if m.libraryFilterIndex < 4 {
			m.libraryFilterIndex++
		}
		return m, nil
	case "k", "up":
		if m.libraryFilterIndex > 0 {
			m.libraryFilterIndex--
		}
		return m, nil
	case "enter", "ctrl+j", " ":
		switch m.libraryFilterIndex {
		case 0:
			m.libraryFilterEditing = true
			m.librarySearchInput.SetValue(m.librarySearch)
			m.librarySearchInput.Focus()
		case 1:
			m.libraryFilterType = nextValue(m.libraryFilterType, m.libraryFilterValues("type"))
			m.refreshLibraryData()
			m.restoreLibrarySelection(selectedKey)
		case 2:
			m.libraryFilterSource = nextValue(m.libraryFilterSource, m.libraryFilterValues("source"))
			m.refreshLibraryData()
			m.restoreLibrarySelection(selectedKey)
		case 3:
			m.libraryShowFavorites = !m.libraryShowFavorites
			m.refreshLibraryData()
			m.restoreLibrarySelection(selectedKey)
		case 4:
			m.librarySearch = ""
			m.librarySearchInput.SetValue("")
			m.libraryFilterType = ""
			m.libraryFilterSource = ""
			m.libraryShowFavorites = false
			m.refreshLibraryData()
			m.restoreLibrarySelection(selectedKey)
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) updateLibraryConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "N":
		m.libraryConfirm = nil
		return m, nil
	case "y", "Y", "enter", "ctrl+j":
		action := m.libraryConfirm
		m.libraryConfirm = nil
		if action != nil && action.action == "delete-staged" {
			return m, m.deleteLibraryStagedCmd(action.rows)
		}
	}
	return m, nil
}

func (m *Model) libraryFilterValues(field string) []string {
	seen := map[string]bool{}
	rows := m.libraryFilterRows
	if len(rows) == 0 {
		rows = m.libraryRows
	}
	for _, row := range rows {
		var v string
		switch field {
		case "type":
			v = row.ModelType
		case "source":
			v = row.Source
		}
		v = strings.TrimSpace(v)
		if v != "" {
			seen[v] = true
		}
	}
	values := make([]string, 0, len(seen)+1)
	values = append(values, "")
	for v := range seen {
		values = append(values, v)
	}
	sort.Strings(values[1:])
	return values
}

func nextValue(current string, values []string) string {
	if len(values) == 0 {
		return ""
	}
	for i, value := range values {
		if value == current {
			return values[(i+1)%len(values)]
		}
	}
	return values[0]
}

func (m *Model) retryLibraryRows(rows []state.ModelMetadata) tea.Cmd {
	if m.st == nil {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(rows))
	skipped := 0
	now := time.Now()
	for _, row := range rows {
		if strings.TrimSpace(row.DownloadURL) == "" || strings.TrimSpace(row.Dest) == "" {
			continue
		}
		if !retryableLibraryURL(row.DownloadURL) {
			skipped++
			continue
		}
		key := row.DownloadURL + "|" + row.Dest
		if _, ok := m.running[key]; ok {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.running[key] = cancel
		m.retrying[key] = now
		cmds = append(cmds, m.retryLibraryRowCmd(ctx, row))
	}
	if len(cmds) > 0 {
		m.addToast(fmt.Sprintf("retrying %d library item(s)", len(cmds)))
	}
	if skipped > 0 {
		m.addToast(fmt.Sprintf("skipped %d local item(s); only remote downloads can retry", skipped))
	}
	return tea.Batch(cmds...)
}

func (m *Model) retryLibraryRowCmd(ctx context.Context, row state.ModelMetadata) tea.Cmd {
	return func() tea.Msg {
		if err := m.st.UpsertDownload(state.DownloadRow{URL: row.DownloadURL, Dest: row.Dest, Status: "pending"}); err != nil {
			return libraryBulkMsg{action: "retry", err: err}
		}
		return m.startDownloadCmdCtx(ctx, row.DownloadURL, row.Dest, false, row.ModelType)()
	}
}

func retryableLibraryURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "hf://") || strings.HasPrefix(raw, "civitai://") {
		return true
	}
	u, err := neturl.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func (m *Model) toggleFavoriteLibraryRowsCmd(rows []state.ModelMetadata) tea.Cmd {
	if len(rows) == 0 || m.st == nil {
		return nil
	}
	makeFavorite := false
	for _, row := range rows {
		if !row.Favorite {
			makeFavorite = true
			break
		}
	}
	clearKeys := !makeFavorite && m.libraryShowFavorites
	return func() tea.Msg {
		count := 0
		keys := make([]string, 0, len(rows))
		for _, row := range rows {
			row.Favorite = makeFavorite
			if err := m.st.UpsertMetadata(&row); err != nil {
				return libraryBulkMsg{action: favoriteActionName(makeFavorite), count: count, keys: keys, err: err}
			}
			if clearKeys {
				keys = append(keys, libraryKey(row))
			}
			count++
		}
		return libraryBulkMsg{action: favoriteActionName(makeFavorite), count: count, keys: keys}
	}
}

func favoriteActionName(makeFavorite bool) string {
	if makeFavorite {
		return "favorite"
	}
	return "unfavorite"
}

func (m *Model) placeLibraryRowsCmd(rows []state.ModelMetadata) tea.Cmd {
	return func() tea.Msg {
		if m.cfg == nil {
			return libraryBulkMsg{action: "place", err: fmt.Errorf("missing config")}
		}
		count := 0
		var firstErr error
		for _, row := range rows {
			if strings.TrimSpace(row.Dest) == "" {
				continue
			}
			if _, err := os.Stat(row.Dest); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if _, err := placer.Place(m.cfg, row.Dest, "", ""); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			count++
		}
		return libraryBulkMsg{action: "place", count: count, err: firstErr}
	}
}

func (m *Model) verifyLibraryRowsCmd(rows []state.ModelMetadata) tea.Cmd {
	return func() tea.Msg {
		if m.st == nil {
			return libraryBulkMsg{action: "verify", err: fmt.Errorf("missing database")}
		}
		downloadRows, err := m.st.ListDownloads()
		if err != nil {
			return libraryBulkMsg{action: "verify", err: err}
		}
		downloads := map[string]state.DownloadRow{}
		for _, row := range downloadRows {
			downloads[keyFor(row)] = row
		}
		count := 0
		var firstErr error
		for _, meta := range rows {
			if strings.TrimSpace(meta.Dest) == "" {
				continue
			}
			sum, err := util.HashFileSHA256(meta.Dest)
			key := meta.DownloadURL + "|" + meta.Dest
			row := downloads[key]
			row.URL = meta.DownloadURL
			row.Dest = meta.Dest
			if err != nil {
				row.Status = "verify_failed"
				row.LastError = err.Error()
				if upsertErr := m.st.UpsertDownload(row); upsertErr != nil && firstErr == nil {
					firstErr = upsertErr
				}
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			want := strings.TrimSpace(row.ExpectedSHA256)
			if want == "" {
				want = strings.TrimSpace(row.ActualSHA256)
			}
			if want != "" && !strings.EqualFold(want, sum) {
				row.ActualSHA256 = sum
				row.Status = "verify_failed"
				row.LastError = "checksum mismatch"
				if upsertErr := m.st.UpsertDownload(row); upsertErr != nil && firstErr == nil {
					firstErr = upsertErr
				}
				if firstErr == nil {
					firstErr = fmt.Errorf("checksum mismatch for %s", meta.Dest)
				}
				continue
			}
			row.ActualSHA256 = sum
			row.Status = "completed"
			row.LastError = ""
			if err := m.st.UpsertDownload(row); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			count++
		}
		return libraryBulkMsg{action: "verify", count: count, err: firstErr}
	}
}

func (m *Model) exportLibraryRowsCmd(rows []state.ModelMetadata) tea.Cmd {
	return func() tea.Msg {
		if m.st == nil {
			return libraryBulkMsg{action: "export", err: fmt.Errorf("missing database")}
		}
		cat, err := catalog.Build(m.st)
		if err != nil {
			return libraryBulkMsg{action: "export", err: err}
		}
		selected := map[string]bool{}
		for _, row := range rows {
			selected[libraryKey(row)] = true
		}
		entries := cat.Models[:0]
		for _, entry := range cat.Models {
			if selected[libraryKey(entry.Metadata)] {
				entries = append(entries, entry)
			}
		}
		cat.Models = entries
		path := m.libraryExportPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return libraryBulkMsg{action: "export", err: err}
		}
		f, err := os.Create(path)
		if err != nil {
			return libraryBulkMsg{action: "export", err: err}
		}
		defer func() { _ = f.Close() }()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cat); err != nil {
			return libraryBulkMsg{action: "export", err: err}
		}
		return libraryBulkMsg{action: "export", count: len(entries), path: path}
	}
}

func (m *Model) deleteLibraryStagedCmd(rows []state.ModelMetadata) tea.Cmd {
	return func() tea.Msg {
		if m.st == nil {
			return libraryBulkMsg{action: "delete staged", err: fmt.Errorf("missing database")}
		}
		count := 0
		keys := make([]string, 0, len(rows))
		var firstErr error
		for _, row := range rows {
			if !m.isStagedLibraryPath(row.Dest) {
				continue
			}
			dbDeleted := strings.TrimSpace(row.DownloadURL) == "" || strings.TrimSpace(row.Dest) == ""
			if strings.TrimSpace(row.DownloadURL) != "" && strings.TrimSpace(row.Dest) != "" {
				if err := m.st.DeleteDownloadAndChunks(row.DownloadURL, row.Dest); err != nil {
					if firstErr == nil {
						firstErr = err
					}
				} else {
					dbDeleted = true
				}
			}
			fileDeleted := true
			if err := os.Remove(row.Dest); err != nil && !os.IsNotExist(err) {
				fileDeleted = false
				if firstErr == nil {
					firstErr = err
				}
			}
			if dbDeleted && fileDeleted {
				count++
				keys = append(keys, libraryKey(row))
			}
		}
		return libraryBulkMsg{action: "delete staged", count: count, keys: keys, err: firstErr}
	}
}

func (m *Model) libraryExportPath() string {
	if m.cfg == nil {
		return filepath.Join(os.TempDir(), "modfetch-library-selected-catalog.json")
	}
	root := strings.TrimSpace(m.cfg.General.DataRoot)
	if root == "" {
		root = filepath.Dir(m.uiStatePath())
	}
	return filepath.Join(root, "library-selected-catalog.json")
}

func (m *Model) isStagedLibraryPath(path string) bool {
	if m.cfg == nil {
		return false
	}
	root := strings.TrimSpace(m.cfg.General.DownloadRoot)
	if root == "" || strings.TrimSpace(path) == "" {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return false
	}
	info, err := os.Stat(absPath)
	return err == nil && !info.IsDir()
}
