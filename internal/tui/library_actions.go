package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	if m.st == nil {
		return nil
	}
	rows, err := m.st.ListMetadata(state.MetadataFilters{OrderBy: "name", Limit: 1000})
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
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
	for _, row := range rows {
		if strings.TrimSpace(row.DownloadURL) == "" || strings.TrimSpace(row.Dest) == "" {
			continue
		}
		_ = m.st.UpsertDownload(state.DownloadRow{URL: row.DownloadURL, Dest: row.Dest, Status: "pending"})
		cmds = append(cmds, m.startDownloadCmd(row.DownloadURL, row.Dest, false, row.ModelType))
	}
	if len(cmds) > 0 {
		m.addToast(fmt.Sprintf("retrying %d library item(s)", len(cmds)))
	}
	return tea.Batch(cmds...)
}

func (m *Model) toggleFavoriteLibraryRows(rows []state.ModelMetadata) tea.Cmd {
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
	count := 0
	for _, row := range rows {
		row.Favorite = makeFavorite
		if err := m.st.UpsertMetadata(&row); err == nil {
			count++
		}
	}
	if count > 0 {
		if makeFavorite {
			m.addToast(fmt.Sprintf("favorited %d", count))
		} else {
			m.addToast(fmt.Sprintf("unfavorited %d", count))
		}
	}
	m.libraryNeedsRefresh = true
	m.refreshLibraryData()
	return nil
}

func (m *Model) placeLibraryRowsCmd(rows []state.ModelMetadata) tea.Cmd {
	return func() tea.Msg {
		if m.cfg == nil {
			return libraryBulkMsg{action: "place", err: fmt.Errorf("missing config")}
		}
		count := 0
		for _, row := range rows {
			if strings.TrimSpace(row.Dest) == "" {
				continue
			}
			if _, err := os.Stat(row.Dest); err != nil {
				return libraryBulkMsg{action: "place", count: count, err: err}
			}
			if _, err := placer.Place(m.cfg, row.Dest, "", ""); err != nil {
				return libraryBulkMsg{action: "place", count: count, err: err}
			}
			count++
		}
		return libraryBulkMsg{action: "place", count: count}
	}
}

func (m *Model) verifyLibraryRowsCmd(rows []state.ModelMetadata) tea.Cmd {
	return func() tea.Msg {
		if m.st == nil {
			return libraryBulkMsg{action: "verify", err: fmt.Errorf("missing database")}
		}
		downloadRows, _ := m.st.ListDownloads()
		downloads := map[string]state.DownloadRow{}
		for _, row := range downloadRows {
			downloads[keyFor(row)] = row
		}
		count := 0
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
				_ = m.st.UpsertDownload(row)
				return libraryBulkMsg{action: "verify", count: count, err: err}
			}
			want := strings.TrimSpace(row.ExpectedSHA256)
			if want == "" {
				want = strings.TrimSpace(row.ActualSHA256)
			}
			if want != "" && !strings.EqualFold(want, sum) {
				row.ActualSHA256 = sum
				row.Status = "verify_failed"
				row.LastError = "checksum mismatch"
				_ = m.st.UpsertDownload(row)
				return libraryBulkMsg{action: "verify", count: count, err: fmt.Errorf("checksum mismatch for %s", meta.Dest)}
			}
			row.ActualSHA256 = sum
			row.Status = "completed"
			row.LastError = ""
			_ = m.st.UpsertDownload(row)
			count++
		}
		return libraryBulkMsg{action: "verify", count: count}
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
		for _, row := range rows {
			if strings.TrimSpace(row.DownloadURL) != "" && strings.TrimSpace(row.Dest) != "" {
				if err := m.st.DeleteDownloadAndChunks(row.DownloadURL, row.Dest); err == nil {
					count++
				}
			}
			if m.isStagedLibraryPath(row.Dest) {
				_ = os.Remove(row.Dest)
			}
			delete(m.librarySelectedKeys, libraryKey(row))
		}
		return libraryBulkMsg{action: "delete staged", count: count}
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
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")
}
