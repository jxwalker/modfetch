package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type uiState struct {
	ThemeIndex int    `json:"theme_index"`
	ShowURL    bool   `json:"show_url"`
	ColumnMode string `json:"column_mode"`
	Compact    bool   `json:"compact"`
	GroupBy    string `json:"group_by"`
	SortMode   string `json:"sort_mode"`
}

func (m *Model) compactToggle()  { m.cfg.UI.Compact = !m.cfg.UI.Compact }
func (m *Model) isCompact() bool { return m.cfg.UI.Compact }

func (m *Model) uiStatePath() string {
	root := strings.TrimSpace(m.cfg.General.DataRoot)
	if root == "" {
		if h, err := os.UserHomeDir(); err == nil {
			root = filepath.Join(h, ".config", "modfetch")
		}
	}
	return filepath.Join(root, "ui_state_v2.json")
}

func (m *Model) loadUIState() {
	p := m.uiStatePath()
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var st uiState
	if err := json.Unmarshal(b, &st); err != nil {
		return
	}
	m.themeIndex = st.ThemeIndex
	presets := themePresets()
	if m.themeIndex >= 0 && m.themeIndex < len(presets) {
		m.th = presets[m.themeIndex]
	}
	if st.ColumnMode != "" {
		m.columnMode = st.ColumnMode
	} else if st.ShowURL {
		m.columnMode = "url"
	}
	if st.Compact {
		m.cfg.UI.Compact = true
	}
	if st.GroupBy != "" {
		m.groupBy = st.GroupBy
	}
	if st.SortMode != "" {
		m.sortMode = st.SortMode
	}
}

func (m *Model) saveUIState() {
	if m.cfg == nil {
		return
	}
	p := m.uiStatePath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	st := uiState{ThemeIndex: m.themeIndex, ShowURL: m.columnMode == "url", ColumnMode: m.columnMode, Compact: m.cfg.UI.Compact, GroupBy: m.groupBy, SortMode: m.sortMode}
	b, _ := json.MarshalIndent(st, "", "  ")
	_ = os.WriteFile(p, b, 0o644)
}
