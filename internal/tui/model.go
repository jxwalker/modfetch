package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"modfetch/internal/config"
	"modfetch/internal/state"
)

type model struct {
	cfg      *config.Config
	st       *state.DB
	rows     []state.DownloadRow
	selected int
	showInfo bool
	err      error
	width   int
	height  int
	refresh time.Duration
	prog    progress.Model
	styles  uiStyles
}

type uiStyles struct {
	header lipgloss.Style
	col    lipgloss.Style
	label  lipgloss.Style
}

type tickMsg time.Time

type refreshMsg struct{}

type errMsg struct{ err error }

func New(cfg *config.Config, st *state.DB) tea.Model {
	p := progress.New(progress.WithDefaultGradient())
	styles := uiStyles{
		header: lipgloss.NewStyle().Bold(true),
		col:    lipgloss.NewStyle().Padding(0, 1),
		label:  lipgloss.NewStyle().Faint(true),
	}
	return &model{cfg: cfg, st: st, refresh: time.Second, prog: p, styles: styles}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(refreshCmd(), tickCmd(m.refresh))
}

func refreshCmd() tea.Cmd { return func() tea.Msg { return refreshMsg{} } }

func tickCmd(d time.Duration) tea.Cmd { return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) }) }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			return m, refreshCmd()
		case "j", "down":
			if m.selected < len(m.rows)-1 { m.selected++ }
			return m, nil
		case "k", "up":
			if m.selected > 0 { m.selected-- }
			return m, nil
		case "d":
			m.showInfo = !m.showInfo
			return m, nil
		}
	case tickMsg:
		return m, refreshCmd()
	case refreshMsg:
		rows, err := m.st.ListDownloads()
		if err != nil { return m, func() tea.Msg { return errMsg{err} } }
		m.rows = rows
		if m.selected >= len(m.rows) { m.selected = len(m.rows) - 1 }
		if m.selected < 0 { m.selected = 0 }
		return m, tickCmd(m.refresh)
	case errMsg:
		m.err = msg.err
		return m, tickCmd(m.refresh)
	}
	return m, nil
}

func (m *model) View() string {
	var b strings.Builder
	// Header with total completed size
	var total int64
	for _, r := range m.rows { if r.Status == "complete" { total += r.Size } }
	fmt.Fprintf(&b, "%s %s\n", m.styles.header.Render("modfetch: downloads"), m.styles.label.Render("(q quit, r refresh, j/k select, d details) total="+humanize.Bytes(uint64(total))))
	if m.err != nil {
		fmt.Fprintf(&b, "error: %v\n\n", m.err)
	}
	// Columns header
	fmt.Fprintf(&b, "%s\n", m.styles.label.Render(fmt.Sprintf("%-8s  %-10s  %-40s  %s", "STATUS", "PROGRESS", "DEST", "URL")))
	for i, r := range m.rows {
		prog := m.renderProgress(r)
		fn := r.Dest
		if len(fn) > 40 { fn = fn[len(fn)-40:] }
		u := r.URL
		if len(u) > 60 { u = u[:60] + "…" }
		line := fmt.Sprintf("%-8s  %-10s  %-40s  %s", r.Status, prog, fn, u)
		if i == m.selected {
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
		if m.showInfo && i == m.selected {
			b.WriteString(m.renderDetails(r))
		}
	}
	return b.String()
}

func (m *model) renderProgress(r state.DownloadRow) string {
	// Determine current bytes by checking .part or final file size
	cur := int64(0)
	if r.Dest != "" {
		if st, err := os.Stat(r.Dest + ".part"); err == nil {
			cur = st.Size()
		} else if st, err := os.Stat(r.Dest); err == nil {
			cur = st.Size()
		}
	}
	if r.Size <= 0 {
		return "--"
	}
	ratio := float64(cur) / float64(r.Size)
	if ratio < 0 { ratio = 0 }
	if ratio > 1 { ratio = 1 }
	bar := m.prog.ViewAs(ratio)
	return bar
}

func (m *model) renderDetails(r state.DownloadRow) string {
	var sb strings.Builder
	abs := r.Dest
	if abs != "" { abs, _ = filepath.Abs(r.Dest) }
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("dest:"), abs)
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("etag:"), r.ETag)
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("last-mod:"), r.LastModified)
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("expected SHA256:"), r.ExpectedSHA256)
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("actual SHA256:"), r.ActualSHA256)
	return sb.String()
}

