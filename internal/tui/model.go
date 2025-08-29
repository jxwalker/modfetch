package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"modfetch/internal/config"
	"modfetch/internal/state"
	"modfetch/internal/logging"
)

type model struct {
	cfg      *config.Config
	st       *state.DB
	rows     []state.DownloadRow
	selected int
	showInfo bool
	showHelp bool
	filterOn bool
	filter   textinput.Model
	sortMode string // ""|"speed"|"eta"
	err      error
	width   int
	height  int
	refresh time.Duration
	prog    progress.Model
	styles  uiStyles
	// speed state
	prev map[string]obs
}

type obs struct{ bytes int64; t time.Time }

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
	ti := textinput.New()
	ti.Placeholder = "filter (url or dest contains)"
	return &model{cfg: cfg, st: st, refresh: time.Second, prog: p, styles: styles, filter: ti, prev: map[string]obs{}}
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
		case "/":
			m.filterOn = true
			m.filter.Focus()
			return m, nil
		case "enter":
			if m.filterOn { m.filterOn = false; m.filter.Blur(); return m, refreshCmd() }
			return m, nil
		case "esc":
			if m.filterOn { m.filterOn = false; m.filter.SetValue(""); m.filter.Blur(); return m, refreshCmd() }
			return m, nil
		case "j", "down":
			if m.selected < len(m.rows)-1 { m.selected++ }
			return m, nil
		case "k", "up":
			if m.selected > 0 { m.selected-- }
			return m, nil
		case "d":
			m.showInfo = !m.showInfo
			return m, nil
		case "?", "h":
			m.showHelp = !m.showHelp
			return m, nil
		case "s":
			m.sortMode = "speed"
			return m, refreshCmd()
		case "e":
			m.sortMode = "eta"
			return m, refreshCmd()
		case "o":
			m.sortMode = ""
			return m, refreshCmd()
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
	// Update filter if active
	if m.filterOn {
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) View() string {
	var b strings.Builder
	// Header with total completed size and global throughput
	var total int64
	var gRate float64
	for _, r := range m.filteredRows() {
		if r.Status == "complete" { total += r.Size }
		_, _, rate, _ := m.progressFor(r)
		gRate += rate
	}
	help := m.styles.label.Render("(q quit, r refresh, j/k select, d details, / filter)")
	fmt.Fprintf(&b, "%s %s %s\n", m.styles.header.Render("modfetch: downloads"), help, m.styles.label.Render("total="+humanize.Bytes(uint64(total))+" rate="+humanize.Bytes(uint64(gRate))+"/s"))
	if m.filterOn {
		b.WriteString("Filter: "+ m.filter.View()+"\n")
	}
	if m.err != nil {
		fmt.Fprintf(&b, "error: %v\n\n", m.err)
	}
	if m.showHelp {
		b.WriteString(m.helpView())
		b.WriteString("\n")
	}
	// Columns header
	fmt.Fprintf(&b, "%s\n", m.styles.label.Render(fmt.Sprintf("%-8s  %-10s  %-10s  %-8s  %-40s  %s", "STATUS", "PROGRESS", "SPEED", "ETA", "DEST", "URL")))
	rows := m.filteredRows()
	for i, r := range rows {
		prog := m.renderProgress(r)
		_, _, rate, eta := m.progressFor(r)
		fn := r.Dest
		if len(fn) > 40 { fn = fn[len(fn)-40:] }
	u := logging.SanitizeURL(r.URL)
		if len(u) > 60 { u = u[:60] + "…" }
		line := fmt.Sprintf("%-8s  %-10s  %-10s  %-8s  %-40s  %s", r.Status, prog, humanize.Bytes(uint64(rate))+"/s", eta, fn, u)
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
	cur, total, _, _ := m.progressFor(r)
	if total <= 0 { return "--" }
	ratio := float64(cur) / float64(total)
	if ratio < 0 { ratio = 0 }
	if ratio > 1 { ratio = 1 }
	bar := m.prog.ViewAs(ratio)
	return bar
}

func (m *model) renderDetails(r state.DownloadRow) string {
	var sb strings.Builder
	abs := r.Dest
	if abs != "" { abs, _ = filepath.Abs(r.Dest) }
	cur, total, rate, eta := m.progressFor(r)
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("dest:"), abs)
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("size:"), humanize.Bytes(uint64(total)))
	fmt.Fprintf(&sb, "    %s %s/%s\n", m.styles.label.Render("progress:"), humanize.Bytes(uint64(cur)), humanize.Bytes(uint64(total)))
	fmt.Fprintf(&sb, "    %s %s/s\n", m.styles.label.Render("speed:"), humanize.Bytes(uint64(rate)))
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("eta:"), eta)
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("etag:"), r.ETag)
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("last-mod:"), r.LastModified)
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("expected SHA256:"), r.ExpectedSHA256)
	fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("actual SHA256:"), r.ActualSHA256)
	return sb.String()
}

func (m *model) progressFor(r state.DownloadRow) (cur int64, total int64, rate float64, eta string) {
	total = r.Size
	// chunked progress if any chunks exist
	chunks, _ := m.st.ListChunks(r.URL, r.Dest)
	if len(chunks) > 0 {
		for _, c := range chunks {
			if strings.EqualFold(c.Status, "complete") { cur += c.Size }
			total += 0 // already in r.Size
		}
	} else {
		if r.Dest != "" {
			if st, err := os.Stat(r.Dest + ".part"); err == nil {
				cur = st.Size()
			} else if st, err := os.Stat(r.Dest); err == nil {
				cur = st.Size()
			}
		}
	}
	key := r.URL + "|" + r.Dest
	prev := m.prev[key]
	dt := time.Since(prev.t).Seconds()
	if dt > 0 { rate = float64(cur-prev.bytes) / dt }
	m.prev[key] = obs{bytes: cur, t: time.Now()}
	if rate > 0 && total > 0 && cur < total {
		rem := float64(total-cur) / rate
		eta = fmt.Sprintf("%ds", int(rem+0.5))
	} else { eta = "-" }
	return
}

func (m *model) filteredRows() []state.DownloadRow {
	if !m.filterOn && strings.TrimSpace(m.filter.Value()) == "" && m.sortMode == "" { return m.rows }
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	var out []state.DownloadRow
	for _, r := range m.rows {
		if q == "" || strings.Contains(strings.ToLower(r.URL), q) || strings.Contains(strings.ToLower(r.Dest), q) {
			out = append(out, r)
		}
	}
	if m.sortMode != "" {
		sort.SliceStable(out, func(i, j int) bool {
			// compute rate and ETA seconds for both
			ci, ti, ri, _ := m.progressFor(out[i])
			cj, tj, rj, _ := m.progressFor(out[j])
			etaI := etaSeconds(ci, ti, ri)
			etaJ := etaSeconds(cj, tj, rj)
			switch m.sortMode {
			case "speed":
				return ri > rj
			case "eta":
				if etaI == 0 && etaJ == 0 { return ri > rj }
				if etaI == 0 { return false }
				if etaJ == 0 { return true }
				return etaI < etaJ
			default:
				return false
			}
		})
	}
	return out
}

func etaSeconds(cur, total int64, rate float64) float64 {
	if rate <= 0 || total <= 0 || cur >= total { return 0 }
	return float64(total-cur) / rate
}

func (m *model) helpView() string {
	return "Help: j/k up/down • r refresh • d details • / filter • s sort by speed • e sort by ETA • o clear sort • h/? toggle this help"
}

