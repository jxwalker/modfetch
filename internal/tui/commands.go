package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

// Tab rendering

func (m *Model) renderTabs() string {
	labels := []struct {
		name string
		tab  int
	}{
		{"All", -1},
		{"Pending", 0},
		{"Active", 1},
		{"Completed", 2},
		{"Failed", 3},
		{"Library", 4},
		{"Settings", 5},
	}
	var sb strings.Builder
	for i, it := range labels {
		style := m.th.tabInactive
		if it.tab == m.activeTab {
			style = m.th.tabActive
		}
		var tabLabel string
		if it.tab == 5 {
			// Settings tab doesn't show a count
			tabLabel = it.name
		} else {
			count := 0
			if it.tab == -1 {
				count = len(m.rows)
			} else if it.tab == 4 {
				// Library tab shows count of metadata entries
				count = len(m.libraryRows)
			} else {
				count = len(m.filterRows(it.tab))
			}
			tabLabel = fmt.Sprintf("%s (%d)", it.name, count)
		}
		sb.WriteString(style.Render(tabLabel))
		if i < len(labels)-1 {
			sb.WriteString("  •  ")
		}
	}
	return sb.String()
}

// Toast notifications

func (m *Model) addToast(s string) {
	m.toasts = append(m.toasts, toast{msg: s, when: time.Now(), ttl: 5 * time.Second})
	m.gcToasts()
}

func (m *Model) gcToasts() {
	now := time.Now()
	fresh := m.toasts[:0]
	for _, t := range m.toasts {
		if now.Sub(t.when) < t.ttl {
			fresh = append(fresh, t)
		}
	}
	m.toasts = fresh
}

func (m *Model) renderToasts() string {
	m.gcToasts()
	if len(m.toasts) == 0 {
		return ""
	}
	return m.th.label.Render(m.toasts[len(m.toasts)-1].msg)
}

// Command bars

func (m *Model) renderCommandsBar() string {
	// concise single-line commands reference
	return m.th.footer.Render("n new • b batch • y/r start • p cancel • D delete • O open • / filter • s/e/R sort • o clear • g group host • t col • v compact • i inspector • H toasts • ? help • q quit")
}

func (m *Model) renderLibraryCommandsBar() string {
	// Library tab commands reference
	return m.th.footer.Render("j/k navigate • Enter details • Esc back • / search • S scan directories • f favorite • H toasts • ? help • q quit")
}

func (m *Model) renderSettingsCommandsBar() string {
	// Settings tab commands reference
	return m.th.footer.Render("View current configuration • H toasts • ? help • q quit")
}

// Help screen

func (m *Model) renderHelp() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("Help (TUI)") + "\n")
	sb.WriteString("Tabs: 1 Pending • 2 Active • 3 Completed • 4 Failed • 5/L Library • 6/M Settings\n")
	sb.WriteString("\n")
	sb.WriteString(m.th.head.Render("Download Tabs (1-4)") + "\n")
	sb.WriteString("Nav: j/k up/down\n")
	sb.WriteString("Filter: / to enter; Enter to apply; Esc to clear\n")
	sb.WriteString("Sort: s speed • e ETA • R remaining • o clear\n")
	sb.WriteString("Group: g group by host\n")
	sb.WriteString("Theme: T cycle presets\n")
	sb.WriteString("Toasts: H toggle drawer\n")
	sb.WriteString("Select: Space toggle • A select all • X clear selection\n")
	sb.WriteString("Probe: P re-probe reachability of selected/current (does not start download)\n")
	sb.WriteString("Columns: t cycle URL/DEST/HOST • v compact view\n")
	sb.WriteString("Actions: y/r start • p cancel • D delete • O open • C copy path • U copy URL\n")
	sb.WriteString("\n")
	sb.WriteString(m.th.head.Render("Library Tab (5/L)") + "\n")
	sb.WriteString("Nav: j/k up/down • Enter view details • Esc back to list\n")
	sb.WriteString("Search: / to enter; Enter to apply; Esc to clear\n")
	sb.WriteString("Scan: S scan configured directories for existing models\n")
	sb.WriteString("Favorite: f toggle favorite on selected model\n")
	sb.WriteString("\n")
	sb.WriteString(m.th.head.Render("Settings Tab (6/M)") + "\n")
	sb.WriteString("View current configuration including paths, tokens, and preferences\n")
	sb.WriteString("To edit settings, modify the YAML config file directly and restart\n")
	sb.WriteString("\n")
	sb.WriteString("Quit: q\n")
	return sb.String()
}

// Toast drawer

func (m *Model) renderToastDrawer() string {
	if len(m.toasts) == 0 {
		return m.th.label.Render("(no recent notifications)")
	}
	now := time.Now()
	var sb strings.Builder
	for i := len(m.toasts) - 1; i >= 0; i-- { // newest first
		t := m.toasts[i]
		dur := now.Sub(t.when).Round(time.Second)
		sb.WriteString(fmt.Sprintf("%s  %s\n", t.msg, m.th.label.Render(humanize.Time(t.when))))
	}
	return sb.String()
}
