package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	neturl "net/url"
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
	"modfetch/internal/downloader"
	"modfetch/internal/resolver"
)

type model struct {
	cfg      *config.Config
	st       *state.DB
	rows     []state.DownloadRow
	selected int
	showInfo bool
	showHelp bool
	menuOn   bool
	filterOn bool
	filter   textinput.Model
	sortMode string // ""|"speed"|"eta"
	filterPreset string // all|active|errors
	groupOn  bool
	showURLFirst bool
	// new download modal
	newDL    bool
	newURL   textinput.Model
	newDest  textinput.Model
	newFocus int
	newMsg   string
	err      error
	width   int
	height  int
	refresh time.Duration
	prog    progress.Model
	styles  uiStyles
	// speed state
	prev map[string]obs
}

type dlDoneMsg struct { path string; err error }

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
	m := &model{cfg: cfg, st: st, refresh: time.Second, prog: p, styles: styles, filter: ti, prev: map[string]obs{}, filterPreset: "all"}
	// new download inputs
	m.newURL = textinput.New(); m.newURL.Placeholder = "URL (https://..., hf://..., civitai://...)"; m.newURL.CharLimit = 4096
	m.newDest = textinput.New(); m.newDest.Placeholder = "Destination path (optional)"; m.newDest.CharLimit = 4096
	return m
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
		// Handle new download modal key events first
		if m.newDL {
			sw := msg.String()
			switch sw {
			case "esc":
				m.newDL = false; m.newMsg = ""; return m, nil
			case "tab":
				m.newFocus = (m.newFocus + 1) % 2
				return m, nil
			case "shift+tab":
				m.newFocus = (m.newFocus + 1 + 2 - 1) % 2
				return m, nil
			case "enter":
				url := strings.TrimSpace(m.newURL.Value())
				dest := strings.TrimSpace(m.newDest.Value())
				if url == "" { m.newMsg = "URL required"; return m, nil }
				m.newMsg = "starting download..."
				return m, m.startDownloadCmd(url, dest)
			}
			// route typing to focused input
			if m.newFocus == 0 { var cmd tea.Cmd; m.newURL, cmd = m.newURL.Update(msg); return m, cmd }
			{ var cmd tea.Cmd; m.newDest, cmd = m.newDest.Update(msg); return m, cmd }
		}
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
		case "n":
			m.newDL = true; m.newURL.SetValue(""); m.newDest.SetValue(""); m.newFocus = 0; m.newURL.Focus(); m.newDest.Blur(); m.newMsg = ""; return m, nil
		case "f":
			if m.filterPreset == "all" { m.filterPreset = "active" } else if m.filterPreset == "active" { m.filterPreset = "errors" } else { m.filterPreset = "all" }
			return m, refreshCmd()
		case "g":
			m.groupOn = !m.groupOn; return m, refreshCmd()
		case "t":
			m.showURLFirst = !m.showURLFirst; return m, nil
		case "d":
			m.showInfo = !m.showInfo
			return m, nil
		case "?", "h":
			m.showHelp = !m.showHelp
			return m, nil
		case "m":
			m.menuOn = !m.menuOn
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
	case dlDoneMsg:
		if msg.err != nil {
			m.newMsg = fmt.Sprintf("download failed: %v", msg.err)
		} else {
			m.newMsg = fmt.Sprintf("downloaded: %s", msg.path)
		}
		m.newDL = false
		return m, refreshCmd()
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
	// Header with totals and throughput
	var total int64
	var gRate float64
	counts := map[string]int{"complete":0, "running":0, "pending":0, "planning":0, "error":0}
	for _, r := range m.filteredRows() {
		if r.Status == "complete" { total += r.Size }
		_, _, rate, _ := m.progressFor(r)
		gRate += rate
		ls := strings.ToLower(r.Status)
		if _, ok := counts[ls]; ok { counts[ls]++ }
	}
	help := m.styles.label.Render("(q quit, r refresh, j/k select, n new, f filter preset, g group, t toggle cols, d details, / filter, m menu)")
	fmt.Fprintf(&b, "%s %s %s\n", m.styles.header.Render("modfetch: downloads"), help, m.styles.label.Render("total="+humanize.Bytes(uint64(total))+" rate="+humanize.Bytes(uint64(gRate))+"/s"))
	// Status counts & preset
	fmt.Fprintf(&b, "%s\n", m.styles.label.Render(fmt.Sprintf("completed:%d running:%d pending:%d planning:%d errors:%d • view:%s", counts["complete"], counts["running"], counts["pending"], counts["planning"], counts["error"], m.filterPreset)) )
	// Token env warnings
	if m.cfg.Sources.HuggingFace.Enabled {
		env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv)
		if env == "" { env = "HF_TOKEN" }
		if strings.TrimSpace(os.Getenv(env)) == "" {
			b.WriteString(m.styles.label.Render("Warning: Hugging Face token env "+env+" not set; gated repos may 401")+"\n")
		}
	}
	if m.cfg.Sources.CivitAI.Enabled {
		env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
		if env == "" { env = "CIVITAI_TOKEN" }
		if strings.TrimSpace(os.Getenv(env)) == "" {
			b.WriteString(m.styles.label.Render("Warning: CivitAI token env "+env+" not set; gated content may 401")+"\n")
		}
	}
	if m.filterOn {
		b.WriteString("Filter: "+ m.filter.View()+"\n")
	}
	if m.menuOn {
		b.WriteString(m.menuView())
		b.WriteString("\n")
	}
	if m.newMsg != "" {
		b.WriteString(m.styles.label.Render(m.newMsg)+"\n")
	}
	// New download modal overlay
	if m.newDL {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("New Download")+"\n")
		if m.newFocus == 0 { m.newURL.Focus(); m.newDest.Blur() } else { m.newDest.Focus(); m.newURL.Blur() }
		b.WriteString("  URL:  "+m.newURL.View()+"\n")
		b.WriteString("  Dest: "+m.newDest.View()+"\n")
		b.WriteString(m.styles.label.Render("Enter to start • Tab to switch • Esc to cancel")+"\n\n")
	}
	if m.err != nil {
		fmt.Fprintf(&b, "error: %v\n\n", m.err)
	}
	if m.showHelp {
		b.WriteString(m.helpView())
		b.WriteString("\n")
	}
	// Columns header with dynamic widths
	avail := m.width
	if avail <= 0 { avail = 120 }
	fixed := 8 + 2 + 10 + 2 + 10 + 2 + 8 + 2 + 2 // status, spaces, progress label, speed, eta, spaces
	rest := avail - fixed
	if rest < 20 { rest = 20 }
	destW := rest/2
	urlW := rest - destW
	if m.showURLFirst { destW, urlW = urlW, destW }
	// Build header labels
	labelLeft := "DEST"; labelRight := "URL"; if m.showURLFirst { labelLeft, labelRight = "URL", "DEST" }
	fmt.Fprintf(&b, "%s\n", m.styles.label.Render(fmt.Sprintf("%-8s  %-10s  %-10s  %-8s  %-*s  %-*s", "STATUS", "PROGRESS", "SPEED", "ETA", destW, labelLeft, urlW, labelRight)))
	rows := m.filteredRows()
	var prevStatus string
	for i, r := range rows {
		if m.groupOn {
			if i == 0 || !strings.EqualFold(prevStatus, r.Status) {
				b.WriteString(m.styles.label.Render(fmt.Sprintf("== %s ==", strings.ToUpper(r.Status)))+"\n")
				prevStatus = r.Status
			}
		}
		prog := m.renderProgress(r)
		_, _, rate, eta := m.progressFor(r)
		d := r.Dest; if len(d) > destW { if destW > 1 { d = d[len(d)-destW:] } }
		u := logging.SanitizeURL(r.URL); if len(u) > urlW { if urlW > 1 { u = u[:urlW-1] + "…" } }
		left := d; right := u
		if m.showURLFirst { left, right = u, d }
		line := fmt.Sprintf("%-8s  %-10s  %-10s  %-8s  %-*s  %-*s", r.Status, prog, humanize.Bytes(uint64(rate))+"/s", eta, destW, left, urlW, right)
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
	return "Help: j/k up/down • r refresh • n new download • f filter preset • g group by status • t toggle columns • d details • / filter • s sort by speed • e sort by ETA • o clear sort • m menu • h/? toggle this help"
}

func (m *model) menuView() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Menu") + "\n")
	b.WriteString("  n: New download      f: Filter preset (all/active/errors)    g: Group by status    t: Toggle columns\n")
	b.WriteString("  d: Toggle details    /: Filter                           s: Sort by speed     e: Sort by ETA\n")
	b.WriteString("  o: Clear sort        h: Help                             r: Refresh           q: Quit\n")
	return b.String()
}

func (m *model) startDownloadCmd(urlStr, dest string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		resolved := urlStr
		headers := map[string]string{}
		if strings.HasPrefix(resolved, "hf://") || strings.HasPrefix(resolved, "civitai://") {
			res, err := resolver.Resolve(ctx, resolved, m.cfg)
			if err != nil { return dlDoneMsg{"", err} }
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
		return dlDoneMsg{final, err}
	}
}

// Modal overlay for new download (render within View)

