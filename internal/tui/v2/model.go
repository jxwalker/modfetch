package tui2

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	neturl "net/url"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"modfetch/internal/config"
	"modfetch/internal/downloader"
	"modfetch/internal/logging"
	"modfetch/internal/resolver"
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

type dlDoneMsg struct{ url, dest, path string; err error }

type obs struct{ bytes int64; t time.Time }

type toast struct{ msg string; when time.Time; ttl time.Duration }

type Model struct {
	cfg   *config.Config
	st    *state.DB
	th    Theme
	w,h   int
	activeTab int // 0: Pending, 1: Active, 2: Completed, 3: Failed
	rows  []state.DownloadRow
	selected int
	filterOn bool
	filterInput textinput.Model
	sortMode string // ""|"speed"|"eta"
	groupBy string  // ""|"host"
	lastRefresh time.Time
	prog  progress.Model
	prev  map[string]obs
	running map[string]context.CancelFunc
	selectedKeys map[string]bool
	toasts []toast
	showToastDrawer bool
	showHelp bool
	themeIndex int
	showURL bool
	tickEvery time.Duration
	// caches for performance
	rateCache map[string]float64
	etaCache  map[string]string
	totalCache map[string]int64
	curBytesCache map[string]int64
	// rate history for sparkline
	rateHist map[string][]float64
	err   error
}

type uiState struct {
	ThemeIndex int    `json:"theme_index"`
	ShowURL    bool   `json:"show_url"`
	Compact    bool   `json:"compact"`
	GroupBy    string `json:"group_by"`
	SortMode   string `json:"sort_mode"`
}

func New(cfg *config.Config, st *state.DB) tea.Model {
	p := progress.New(progress.WithDefaultGradient())
	fi := textinput.New(); fi.Placeholder = "filter (url or dest contains)"; fi.CharLimit = 4096
	// compute refresh
	refresh := time.Second
	if hz := cfg.UI.RefreshHz; hz > 0 {
		if hz > 10 { hz = 10 }
		refresh = time.Second / time.Duration(hz)
	}
m := &Model{
		cfg: cfg, st: st, th: defaultTheme(), activeTab: 1, prog: p, prev: map[string]obs{},
		running: map[string]context.CancelFunc{}, selectedKeys: map[string]bool{}, filterInput: fi,
		rateCache: map[string]float64{}, etaCache: map[string]string{}, totalCache: map[string]int64{}, curBytesCache: map[string]int64{}, rateHist: map[string][]float64{},
		tickEvery: refresh,
		showURL: cfg.UI.ShowURL,
	}
	// Load UI state overrides if present
	m.loadUIState()
	return m
}

func (m *Model) Init() tea.Cmd {
return tea.Tick(m.tickEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
case tea.KeyMsg:
		s := msg.String()
		// If filter mode is on, handle input first
		if m.filterOn {
			switch s {
			case "enter": m.filterOn = false; m.filterInput.Blur(); return m, nil
			case "esc": m.filterOn = false; m.filterInput.SetValue(""); m.filterInput.Blur(); return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			return m, cmd
		}
		switch s {
	case "q", "ctrl+c":
			m.saveUIState()
			return m, tea.Quit
		case "/":
			m.filterOn = true; m.filterInput.Focus(); return m, nil
		case "s": m.sortMode = "speed"; return m, nil
		case "e": m.sortMode = "eta"; return m, nil
		case "o": m.sortMode = ""; return m, nil
		case "g": if m.groupBy=="host" { m.groupBy = "" } else { m.groupBy = "host" }; return m, nil
case "t": // toggle last column between DEST and URL
			m.showURL = !m.showURL; m.saveUIState(); return m, nil
case "v": // compact view toggle
			m.compactToggle(); m.saveUIState()
			return m, nil
case "T": // theme cycle presets
			presets := themePresets()
			m.themeIndex = (m.themeIndex + 1) % len(presets)
			m.th = presets[m.themeIndex]
			m.saveUIState()
			return m, nil
		case "H": // toggle toast drawer
			m.showToastDrawer = !m.showToastDrawer
			return m, nil
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "1": m.activeTab = 0; m.selected = 0; return m, nil
		case "2": m.activeTab = 1; m.selected = 0; return m, nil
		case "3": m.activeTab = 2; m.selected = 0; return m, nil
		case "4": m.activeTab = 3; m.selected = 0; return m, nil
		case "j", "down": if m.selected < len(m.visibleRows())-1 { m.selected++ }; return m, nil
		case "k", "up": if m.selected > 0 { m.selected-- }; return m, nil
		case " ": // toggle selection
			rows := m.visibleRows()
			if m.selected >=0 && m.selected < len(rows) {
				key := keyFor(rows[m.selected])
				if m.selectedKeys[key] { delete(m.selectedKeys, key) } else { m.selectedKeys[key] = true }
			}
			return m, nil
		case "A": // select all visible
			for _, r := range m.visibleRows() { m.selectedKeys[keyFor(r)] = true }
			return m, nil
		case "X": // clear selection
			m.selectedKeys = map[string]bool{}
			return m, nil
case "y": // retry (batch-aware)
			targets := m.selectionOrCurrent()
			if len(targets) == 0 { return m, nil }
			cmds := make([]tea.Cmd, 0, len(targets))
			for _, r := range targets {
				ctx, cancel := context.WithCancel(context.Background())
				m.running[keyFor(r)] = cancel
				cmds = append(cmds, m.startDownloadCmdCtx(ctx, r.URL, r.Dest))
			}
			m.addToast(fmt.Sprintf("retrying %d item(s)…", len(targets)))
			return m, tea.Batch(cmds...)
		case "p": // cancel (batch-aware)
			cnt := 0
			for _, r := range m.selectionOrCurrent() {
				key := keyFor(r)
				if cancel, ok := m.running[key]; ok { cancel(); delete(m.running, key); cnt++ }
			}
			if cnt > 0 { m.addToast(fmt.Sprintf("cancelled %d", cnt)) }
			return m, nil
		case "O": // open/reveal in file manager
			rows := m.selectionOrCurrent()
			if len(rows) > 0 {
				_ = openInFileManager(rows[0].Dest, true)
			}
			return m, nil
		case "C": // copy dest of current
			rows := m.selectionOrCurrent()
			if len(rows) > 0 { _ = copyToClipboard(rows[0].Dest) }
			return m, nil
		case "U": // copy URL of current
			rows := m.selectionOrCurrent()
			if len(rows) > 0 { _ = copyToClipboard(rows[0].URL) }
			return m, nil
		}
	case tickMsg:
		return m, m.refresh()
	case dlDoneMsg:
		if msg.err != nil { m.err = msg.err }
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
	// Toasts inline
	toastStr := m.renderToasts()
	header := m.th.border.Render(lipgloss.JoinHorizontal(lipgloss.Top, title+"  ", m.th.label.Render(stats), "  ", toastStr))
	// Left tabs
	left := m.renderTabs()
	// Main table
	main := m.renderTable()
	// Inspector
	insp := m.renderInspector()
	// Compose middle area
	leftW := 24
	inspW := 42
	midW := m.w - leftW - inspW - 4
	if midW < 30 { midW = 30 }
	mid := lipgloss.JoinHorizontal(lipgloss.Top,
		m.th.border.Width(leftW).Render(left),
		m.th.border.Width(midW).Render(main),
		m.th.border.Width(inspW).Render(insp),
	)
	// Optional toast drawer
	drawer := ""
	if m.showToastDrawer {
		drawer = m.th.border.Width(m.w-2).Render(m.renderToastDrawer())
	}
	// Optional help modal
	help := ""
	if m.showHelp {
		help = m.th.border.Width(m.w-2).Render(m.renderHelp())
	}
	// Footer
	filterBar := ""
	if m.filterOn { filterBar = "Filter: "+ m.filterInput.View() }
footer := m.th.border.Render(m.th.footer.Render("1 Pending • 2 Active • 3 Completed • 4 Failed • j/k nav • y retry • p cancel • O open • / filter • s/e sort • o clear • g group host • t URL/DEST • v compact • T theme • H toasts • ? help • X clear sel • q quit\n"+filterBar))
	parts := []string{header, mid}
	if help != "" { parts = append(parts, help) }
	if drawer != "" { parts = append(parts, drawer) }
	parts = append(parts, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *Model) refresh() tea.Cmd {
	rows, err := m.st.ListDownloads()
	if err != nil { _ = logging.New("error", false) }
	m.rows = rows
	m.lastRefresh = time.Now()
	m.gcToasts()
	// recompute caches once per tick
	for _, r := range m.rows {
		key := keyFor(r)
		cur, total := m.computeCurAndTotal(r)
		m.totalCache[key] = total
		m.curBytesCache[key] = cur
		prev := m.prev[key]
		dt := time.Since(prev.t).Seconds()
		var rate float64
		if dt > 0 { rate = float64(cur - prev.bytes) / dt }
		m.prev[key] = obs{bytes: cur, t: time.Now()}
		m.rateCache[key] = rate
		if rate > 0 && total > 0 && cur < total {
			rem := float64(total-cur) / rate
			m.etaCache[key] = fmt.Sprintf("%ds", int(rem+0.5))
		} else { m.etaCache[key] = "-" }
		m.addRateSample(key, rate)
	}
return tea.Tick(m.tickEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) renderStats() string {
	var pending, active, done, failed int
	var gRate float64
	for _, r := range m.rows {
		ls := strings.ToLower(r.Status)
		switch ls {
		case "pending","planning": pending++
		case "running": active++
		case "complete": done++
		case "error","checksum_mismatch","verify_failed": failed++
		}
		rate := m.rateCache[keyFor(r)]
		gRate += rate
	}
	return fmt.Sprintf("Pending:%d Active:%d Completed:%d Failed:%d • Rate:%s/s", pending, active, done, failed, humanize.Bytes(uint64(gRate)))
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

func (m *Model) visibleRows() []state.DownloadRow {
	rows := m.filterRows(m.activeTab)
	rows = m.applySearch(rows)
	rows = m.applySort(rows)
	return rows
}

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

func (m *Model) applySearch(in []state.DownloadRow) []state.DownloadRow {
	q := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if q == "" { return in }
	var out []state.DownloadRow
	for _, r := range in {
		if strings.Contains(strings.ToLower(r.URL), q) || strings.Contains(strings.ToLower(r.Dest), q) { out = append(out, r) }
	}
	return out
}

func (m *Model) applySort(in []state.DownloadRow) []state.DownloadRow {
	if m.sortMode == "" { return in }
	out := make([]state.DownloadRow, len(in)); copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		ci, ti, ri, _ := m.progressFor(out[i])
		cj, tj, rj, _ := m.progressFor(out[j])
		etaI := etaSeconds(ci, ti, ri)
		etaJ := etaSeconds(cj, tj, rj)
		switch m.sortMode {
		case "speed": return ri > rj
		case "eta":
			if etaI == 0 && etaJ == 0 { return ri > rj }
			if etaI == 0 { return false }
			if etaJ == 0 { return true }
			return etaI < etaJ
		}
		return false
	})
	return out
}

func etaSeconds(cur, total int64, rate float64) float64 {
	if rate <= 0 || total <= 0 || cur >= total { return 0 }
	return float64(total-cur) / rate
}

func (m *Model) renderTable() string {
	rows := m.visibleRows()
	var sb strings.Builder
	lastLabel := "DEST"; if m.showURL { lastLabel = "URL" }
	if m.isCompact() {
		sb.WriteString(m.th.head.Render(fmt.Sprintf("%-2s %-8s  %-10s  %-8s  %-s", "S", "STATUS", "PROGRESS", "ETA", lastLabel)))
	} else {
		sb.WriteString(m.th.head.Render(fmt.Sprintf("%-2s %-8s  %-10s  %-10s  %-10s  %-8s  %-s", "S", "STATUS", "PROGRESS", "SPEED", "THR", "ETA", lastLabel)))
	}
	sb.WriteString("\n")
	maxRows := m.h - 10
	if maxRows < 3 { maxRows = len(rows) }
	var prevGroup string
	for i, r := range rows {
		if m.groupBy == "host" {
			host := hostOf(r.URL)
			if i == 0 || host != prevGroup {
				sb.WriteString(m.th.label.Render("// "+host)+"\n")
				prevGroup = host
			}
		}
		prog := m.renderProgress(r)
		rate := m.rateCache[keyFor(r)]
		eta := m.etaCache[keyFor(r)]
		thr := m.renderSparkline(keyFor(r))
		sel := " "; if m.selectedKeys[keyFor(r)] { sel = "*" }
		last := r.Dest
		if m.showURL { last = logging.SanitizeURL(r.URL) }
		var line string
		if m.isCompact() {
			line = fmt.Sprintf("%-2s %-8s  %-10s  %-8s  %s", sel, r.Status, prog, eta, last)
		} else {
			line = fmt.Sprintf("%-2s %-8s  %-10s  %-10s  %-10s  %-8s  %s", sel, r.Status, prog, humanize.Bytes(uint64(rate))+"/s", thr, eta, last)
		}
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
	sb.WriteString("\n\n")
	// Basic metrics
	cur := m.curBytesCache[keyFor(r)]
	total := m.totalCache[keyFor(r)]
	rate := m.rateCache[keyFor(r)]
	eta := m.etaCache[keyFor(r)]
	sb.WriteString(fmt.Sprintf("%s %s/%s\n", m.th.label.Render("Progress:"), humanize.Bytes(uint64(cur)), humanize.Bytes(uint64(total))))
	sb.WriteString(fmt.Sprintf("%s %s/s\n", m.th.label.Render("Speed:"), humanize.Bytes(uint64(rate))))
	sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("ETA:"), eta))
	sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Throughput:"), m.renderSparkline(keyFor(r))))
	return sb.String()
}

func (m *Model) progressFor(r state.DownloadRow) (cur int64, total int64, rate float64, eta string) {
	key := keyFor(r)
	cur = m.curBytesCache[key]
	total = m.totalCache[key]
	rate = m.rateCache[key]
	eta = m.etaCache[key]
	// Fallback if cache not populated yet
	if total == 0 && r.Size > 0 { total = r.Size }
	return
}

func (m *Model) computeCurAndTotal(r state.DownloadRow) (cur int64, total int64) {
	total = r.Size
	chunks, _ := m.st.ListChunks(r.URL, r.Dest)
	if len(chunks) > 0 {
		for _, c := range chunks {
			if strings.EqualFold(c.Status, "complete") { cur += c.Size }
		}
		return cur, total
	}
	if r.Dest != "" {
		p := downloader.StagePartPath(m.cfg, r.URL, r.Dest)
		if st, err := os.Stat(p); err == nil { cur = st.Size() } else if st, err := os.Stat(r.Dest); err == nil { cur = st.Size() }
	}
	return cur, total
}

func (m *Model) renderProgress(r state.DownloadRow) string {
	cur, total, _, _ := m.progressFor(r)
	if total <= 0 { return "--" }
	ratio := float64(cur) / float64(total)
	if ratio < 0 { ratio = 0 }
	if ratio > 1 { ratio = 1 }
	return m.prog.ViewAs(ratio)
}

func (m *Model) addRateSample(key string, rate float64) {
	h := m.rateHist[key]
	h = append(h, rate)
	if len(h) > 10 { h = h[len(h)-10:] }
	m.rateHist[key] = h
}

func (m *Model) renderSparklineKey(key string) string {
	h := m.rateHist[key]
	if len(h) == 0 { return "" }
	// map rates to 8 levels; normalize by max
	max := 0.0
	for _, v := range h { if v > max { max = v } }
	if max <= 0 { return "──────────" }
	levels := []rune{'▁','▂','▃','▄','▅','▆','▇','█'}
	var sb strings.Builder
	for _, v := range h {
		r := int((v/max)*float64(len(levels)-1) + 0.5)
		if r < 0 { r = 0 }; if r >= len(levels) { r = len(levels)-1 }
		sb.WriteRune(levels[r])
	}
	// pad to 10
	for sb.Len() < 10 { sb.WriteRune(' ') }
	return sb.String()
}

func (m *Model) renderSparkline(key string) string { return m.renderSparklineKey(key) }

func keyFor(r state.DownloadRow) string { return r.URL+"|"+r.Dest }

func hostOf(urlStr string) string {
	if u, err := neturl.Parse(urlStr); err == nil {
		return u.Hostname()
	}
	return ""
}

func (m *Model) selectionOrCurrent() []state.DownloadRow {
	rows := m.visibleRows()
	if len(m.selectedKeys) == 0 {
		if m.selected >= 0 && m.selected < len(rows) { return []state.DownloadRow{ rows[m.selected] } }
		return nil
	}
	var out []state.DownloadRow
	for _, r := range rows { if m.selectedKeys[keyFor(r)] { out = append(out, r) } }
	return out
}

func (m *Model) addToast(s string) { m.toasts = append(m.toasts, toast{msg: s, when: time.Now(), ttl: 3*time.Second}) }

func (m *Model) gcToasts() {
	now := time.Now()
	var keep []toast
	for _, t := range m.toasts { if now.Sub(t.when) < t.ttl { keep = append(keep, t) } }
	m.toasts = keep
}

func (m *Model) renderToasts() string {
	if len(m.toasts) == 0 { return "" }
	parts := make([]string, 0, len(m.toasts))
	for _, t := range m.toasts { parts = append(parts, t.msg) }
	return m.th.label.Render(strings.Join(parts, " • "))
}

func (m *Model) compactToggle() { m.cfg.UI.Compact = !m.cfg.UI.Compact }
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
	if err != nil { return }
	var st uiState
	if err := json.Unmarshal(b, &st); err != nil { return }
	// Apply
	m.themeIndex = st.ThemeIndex
	presets := themePresets()
	if m.themeIndex >= 0 && m.themeIndex < len(presets) { m.th = presets[m.themeIndex] }
	m.showURL = st.ShowURL
	if st.Compact { m.cfg.UI.Compact = true }
	if st.GroupBy != "" { m.groupBy = st.GroupBy }
	if st.SortMode != "" { m.sortMode = st.SortMode }
}

func (m *Model) saveUIState() {
	p := m.uiStatePath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	st := uiState{ThemeIndex: m.themeIndex, ShowURL: m.showURL, Compact: m.cfg.UI.Compact, GroupBy: m.groupBy, SortMode: m.sortMode}
	b, _ := json.MarshalIndent(st, "", "  ")
	_ = os.WriteFile(p, b, 0o644)
}

func (m *Model) renderHelp() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("Help (TUI v2)")+"\n")
	sb.WriteString("Tabs: 1 Pending • 2 Active • 3 Completed • 4 Failed\n")
	sb.WriteString("Nav: j/k up/down\n")
	sb.WriteString("Filter: / to enter; Enter to apply; Esc to clear\n")
	sb.WriteString("Sort: s speed • e ETA • o clear\n")
	sb.WriteString("Group: g group by host\n")
	sb.WriteString("Theme: T cycle presets\n")
	sb.WriteString("Toasts: H toggle drawer\n")
	sb.WriteString("Select: Space toggle • A select all • X clear selection\n")
	sb.WriteString("Columns: t toggle URL/DEST • v compact view\n")
	sb.WriteString("Actions: y retry • p cancel • O open • C copy path • U copy URL\n")
	sb.WriteString("Quit: q\n")
	return sb.String()
}

func (m *Model) renderToastDrawer() string {
	if len(m.toasts) == 0 { return m.th.label.Render("(no recent notifications)") }
	now := time.Now()
	var sb strings.Builder
	for i := len(m.toasts)-1; i >= 0; i-- { // newest first
		t := m.toasts[i]
		dur := now.Sub(t.when).Round(time.Second)
		sb.WriteString(fmt.Sprintf("%s  %s\n", t.msg, m.th.label.Render(dur.String()+" ago")))
	}
	return sb.String()
}

func (m *Model) startDownloadCmd(urlStr, dest string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.running[urlStr+"|"+dest] = cancel
	return m.startDownloadCmdCtx(ctx, urlStr, dest)
}

func (m *Model) startDownloadCmdCtx(ctx context.Context, urlStr, dest string) tea.Cmd {
	return func() tea.Msg {
		resolved := urlStr
		headers := map[string]string{}
		// Translate civitai model page URLs into civitai:// URIs for proper resolution
		if strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://") {
			if u, err := neturl.Parse(resolved); err == nil {
				h := strings.ToLower(u.Hostname())
				if strings.HasSuffix(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
					parts := strings.Split(strings.Trim(u.Path, "/"), "/")
					if len(parts) >= 2 {
						modelID := parts[1]
						q := u.Query()
						ver := q.Get("modelVersionId"); if ver == "" { ver = q.Get("version") }
						civ := "civitai://model/" + modelID; if strings.TrimSpace(ver) != "" { civ += "?version=" + ver }
						if res, err := resolver.Resolve(ctx, civ, m.cfg); err == nil { resolved = res.URL; headers = res.Headers }
					}
				}
			}
		}
		if strings.HasPrefix(resolved, "hf://") || strings.HasPrefix(resolved, "civitai://") {
			res, err := resolver.Resolve(ctx, resolved, m.cfg)
			if err != nil { return dlDoneMsg{url: urlStr, dest: dest, path: "", err: err} }
			resolved = res.URL
			headers = res.Headers
		} else {
			if u, err := neturl.Parse(resolved); err == nil {
				h := strings.ToLower(u.Hostname())
				if strings.HasSuffix(h, "huggingface.co") && m.cfg.Sources.HuggingFace.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv); if env == "" { env = "HF_TOKEN" }
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" { headers["Authorization"] = "Bearer "+tok }
				}
				if strings.HasSuffix(h, "civitai.com") && m.cfg.Sources.CivitAI.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv); if env == "" { env = "CIVITAI_TOKEN" }
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" { headers["Authorization"] = "Bearer "+tok }
				}
			}
		}
		log := logging.New("error", false)
		dl := downloader.NewAuto(m.cfg, log, m.st, nil)
		final, _, err := dl.Download(ctx, resolved, dest, "", headers, m.cfg.General.AlwaysNoResume)
		return dlDoneMsg{url: urlStr, dest: dest, path: final, err: err}
	}
}

func openInFileManager(p string, reveal bool) error {
	p = strings.TrimSpace(p)
	if p == "" { return fmt.Errorf("empty path") }
	// Determine directory to open even if file doesn't exist yet
	dir := p
	if fi, err := os.Stat(p); err == nil {
		if fi.IsDir() { dir = p } else { dir = filepath.Dir(p) }
	} else {
		dir = filepath.Dir(p)
	}
	switch runtime.GOOS {
	case "darwin":
		if reveal {
			// Reveal if possible; if that fails, fallback to opening dir
			if err := exec.Command("open", "-R", p).Run(); err == nil { return nil }
		}
		return exec.Command("open", dir).Run()
	case "linux":
		if _, err := exec.LookPath("xdg-open"); err == nil {
			return exec.Command("xdg-open", dir).Run()
		}
	}
	return fmt.Errorf("cannot open file manager on %s", runtime.GOOS)
}

// Theme presets
func themePresets() []Theme {
	base := defaultTheme()
	neon := base
	neon.title = neon.title.Foreground(lipgloss.Color("46")) // neon green
	neon.head = neon.head.Foreground(lipgloss.Color("49"))
	neon.border = neon.border.BorderForeground(lipgloss.Color("51"))
	neon.tabActive = neon.tabActive.Foreground(lipgloss.Color("48"))

	drac := base
	drac.title = drac.title.Foreground(lipgloss.Color("213"))
	drac.head = drac.head.Foreground(lipgloss.Color("219"))
	drac.border = drac.border.BorderForeground(lipgloss.Color("135"))
	drac.tabActive = drac.tabActive.Foreground(lipgloss.Color("204"))

	solar := base
	solar.title = solar.title.Foreground(lipgloss.Color("136"))
	solar.head = solar.head.Foreground(lipgloss.Color("178"))
	solar.border = solar.border.BorderForeground(lipgloss.Color("136"))
	solar.tabActive = solar.tabActive.Foreground(lipgloss.Color("166"))
	return []Theme{ base, neon, drac, solar }
}

func copyToClipboard(s string) error {
	s = strings.TrimSpace(s)
	if s == "" { return fmt.Errorf("empty") }
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		in, err := cmd.StdinPipe(); if err != nil { return err }
		if err := cmd.Start(); err != nil { return err }
		_, _ = in.Write([]byte(s)); _ = in.Close()
		return cmd.Wait()
	case "linux":
		// try wl-copy then xclip
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd := exec.Command("wl-copy")
			in, err := cmd.StdinPipe(); if err != nil { return err }
			if err := cmd.Start(); err != nil { return err }
			_, _ = in.Write([]byte(s)); _ = in.Close()
			return cmd.Wait()
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command("xclip", "-selection", "clipboard")
			in, err := cmd.StdinPipe(); if err != nil { return err }
			if err := cmd.Start(); err != nil { return err }
			_, _ = in.Write([]byte(s)); _ = in.Close()
			return cmd.Wait()
		}
	}
	return fmt.Errorf("no clipboard utility found")
}
