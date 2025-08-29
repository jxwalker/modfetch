package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"
	"modfetch/internal/config"
	"modfetch/internal/state"
)

type model struct {
	cfg   *config.Config
	st    *state.DB
	rows  []state.DownloadRow
	err   error
	width int
	height int
	refresh time.Duration
}

type tickMsg time.Time

type refreshMsg struct{}

type errMsg struct{ err error }

func New(cfg *config.Config, st *state.DB) tea.Model {
	return &model{cfg: cfg, st: st, refresh: time.Second}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(refreshCmd(), tickCmd(m.refresh))
}

func refreshCmd() tea.Cmd {
	return func() tea.Msg { return refreshMsg{} }
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

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
			return m, tea.Batch(refreshCmd())
		}
	case tickMsg:
		return m, refreshCmd()
	case refreshMsg:
		rows, err := m.st.ListDownloads()
		if err != nil { return m, func() tea.Msg { return errMsg{err} } }
		m.rows = rows
		return m, tickCmd(m.refresh)
	case errMsg:
		m.err = msg.err
		return m, tickCmd(m.refresh)
	}
	return m, nil
}

func (m *model) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "modfetch: downloads (q to quit, r to refresh)\n")
	if m.err != nil {
		fmt.Fprintf(&b, "error: %v\n\n", m.err)
	}
	fmt.Fprintf(&b, "%-8s  %-10s  %-40s  %s\n", "STATUS", "SIZE", "DEST", "URL")
	for _, r := range m.rows {
		sz := humanize.Bytes(uint64(r.Size))
		fn := r.Dest
		if len(fn) > 40 { fn = fn[len(fn)-40:] }
		u := r.URL
		if len(u) > 60 { u = u[:60] + "…" }
		fmt.Fprintf(&b, "%-8s  %-10s  %-40s  %s\n", r.Status, sz, fn, u)
	}
	return b.String()
}

