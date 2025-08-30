package tui2

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

type Theme struct {
	border     lipgloss.Style
	title      lipgloss.Style
	label      lipgloss.Style
	tabActive  lipgloss.Style
	tabInactive lipgloss.Style
	row        lipgloss.Style
	rowSelected lipgloss.Style
	head       lipgloss.Style
	footer     lipgloss.Style
}

func defaultTheme() Theme {
	b := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	return Theme{
		border:      b.BorderForeground(lipgloss.Color("63")),
		title:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
		label:       lipgloss.NewStyle().Faint(true),
		tabActive:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("219")),
		tabInactive: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		row:         lipgloss.NewStyle(),
		rowSelected: lipgloss.NewStyle().Bold(true),
		head:        lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true),
		footer:      lipgloss.NewStyle().Faint(true),
	}
}

type tickMsg time.Time

type Model struct {
	cfg   *config.Config
	st    *state.DB
	th    Theme
	w,h   int
	activeTab int // 0: Pending, 1: Active, 2: Completed, 3: Failed
	rows  []state.DownloadRow
	selected int
	filter string
	lastRefresh time.Time
}

func New(cfg *config.Config, st *state.DB) tea.Model {
	return &Model{cfg: cfg, st: st, th: defaultTheme(), activeTab: 1}
}

func (m *Model) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		s := msg.String()
		switch s {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1": m.activeTab = 0; m.selected = 0; return m, nil
		case "2": m.activeTab = 1; m.selected = 0; return m, nil
		case "3": m.activeTab = 2; m.selected = 0; return m, nil
		case "4": m.activeTab = 3; m.selected = 0; return m, nil
		case "j", "down": if m.selected < len(m.visibleRows())-1 { m.selected++ }; return m, nil
		case "k", "up": if m.selected > 0 { m.selected-- }; return m, nil
		case "/": m.filter = ""; return m, nil
		}
	case tickMsg:
		return m, m.refresh()
	}
	return m, nil
}

func (m *Model) View() string {
	if m.w == 0 { m.w = 120 }
	if m.h == 0 { m.h = 30 }
	// Header
	title := m.th.title.Render("modfetch • TUI v2 (preview)")
	stats := m.renderStats()
	header := m.th.border.Render(lipgloss.JoinHorizontal(lipgloss.Top, title+"  ", m.th.label.Render(stats)))
	// Left tabs
	left := m.renderTabs()
	// Main table
	main := m.renderTable()
	// Inspector
	insp := m.renderInspector()
	// Compose middle area
	leftW := 22
	inspW := 40
	midW := m.w - leftW - inspW - 4
	if midW < 30 { midW = 30 }
	mid := lipgloss.JoinHorizontal(lipgloss.Top,
		m.th.border.Width(leftW).Render(left),
		m.th.border.Width(midW).Render(main),
		m.th.border.Width(inspW).Render(insp),
	)
	// Footer
	footer := m.th.border.Render(m.th.footer.Render("1 Pending • 2 Active • 3 Completed • 4 Failed • j/k nav • q quit"))
	return lipgloss.JoinVertical(lipgloss.Left, header, mid, footer)
}

func (m *Model) refresh() tea.Cmd {
	rows, err := m.st.ListDownloads()
	if err != nil { _ = logging.New("error", false); }
	m.rows = rows
	m.lastRefresh = time.Now()
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) renderStats() string {
	var pending, active, done, failed int
	for _, r := range m.rows {
		ls := strings.ToLower(r.Status)
		switch ls {
		case "pending","planning": pending++
		case "running": active++
		case "complete": done++
		case "error","checksum_mismatch","verify_failed": failed++
		}
	}
	return fmt.Sprintf("Pending:%d Active:%d Completed:%d Failed:%d", pending, active, done, failed)
}

func (m *Model) renderTabs() string {
	labels := []string{"Pending", "Active", "Completed", "Failed"}
	var sb strings.Builder
	for i, lab := range labels {
		style := m.th.tabInactive
		if i == m.activeTab { style = m.th.tabActive }
		count := len(m.filterRows(i))
		sb.WriteString(style.Render(fmt.Sprintf("%s (%d)", lab, count)))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m *Model) visibleRows() []state.DownloadRow { return m.filterRows(m.activeTab) }

func (m *Model) filterRows(tab int) []state.DownloadRow {
	var out []state.DownloadRow
	for _, r := range m.rows {
		ls := strings.ToLower(r.Status)
		switch tab {
		case 0: if ls=="pending" || ls=="planning" { out = append(out, r) }
		case 1: if ls=="running" { out = append(out, r) }
		case 2: if ls=="complete" { out = append(out, r) }
		case 3: if ls=="error" || ls=="checksum_mismatch" || ls=="verify_failed" { out = append(out, r) }
		}
	}
	return out
}

func (m *Model) renderTable() string {
	rows := m.visibleRows()
	var sb strings.Builder
	sb.WriteString(m.th.head.Render(fmt.Sprintf("%-8s  %-10s  %-s", "STATUS", "SIZE", "DEST")))
	sb.WriteString("\n")
	maxRows := m.h - 10
	if maxRows < 3 { maxRows = len(rows) }
	for i, r := range rows {
		line := fmt.Sprintf("%-8s  %-10d  %s", r.Status, r.Size, r.Dest)
		if i == m.selected { line = m.th.rowSelected.Render(line) }
		sb.WriteString(line+"\n")
		if i+1 >= maxRows { break }
	}
	if len(rows) == 0 { sb.WriteString(m.th.label.Render("(no items)")) }
	return sb.String()
}

func (m *Model) renderInspector() string {
	rows := m.visibleRows()
	if m.selected < 0 || m.selected >= len(rows) { return m.th.label.Render("No selection") }
	r := rows[m.selected]
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("Details"))
	sb.WriteString("\n")
	sb.WriteString(m.th.label.Render("URL:"))
	sb.WriteString("\n")
	sb.WriteString(logging.SanitizeURL(r.URL))
	sb.WriteString("\n\n")
	sb.WriteString(m.th.label.Render("Dest:"))
	sb.WriteString("\n")
	sb.WriteString(r.Dest)
	return sb.String()
}
