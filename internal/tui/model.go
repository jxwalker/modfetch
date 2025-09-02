package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
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
	menuIndex int
	menuItems []string
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
	// onboarding tips
	tipsOn   bool
	// running downloads control (only those started via TUI)
	running map[string]context.CancelFunc
	// ephemeral starters (resolving/preflight), keyed by URL|Dest
	ephems  map[string]ephemeral
	spin   int
	err      error
	width   int
	height  int
	refresh time.Duration
	prog    progress.Model
	styles  uiStyles
	// speed state
	prev map[string]obs
}

// ephemeral represents a starting download before DB row appears

type ephemeral struct {
	URL  string
	Dest string
	When time.Time
}

type dlDoneMsg struct { url string; dest string; path string; err error }

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
m := &model{cfg: cfg, st: st, refresh: time.Second, prog: p, styles: styles, filter: ti, prev: map[string]obs{}, filterPreset: "all", tipsOn: true, running: map[string]context.CancelFunc{}, ephems: map[string]ephemeral{}, menuItems: []string{"New download","Filter preset","Group by status","Toggle columns","Sort by speed","Sort by ETA","Clear sort","Help","Refresh","Quit"}}
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
		// Handle menu navigation
		if m.menuOn && !m.newDL {
			sw := msg.String()
			switch sw {
			case "up": if m.menuIndex>0 { m.menuIndex-- }; return m, nil
			case "down": if m.menuIndex < len(m.menuItems)-1 { m.menuIndex++ }; return m, nil
			case "enter": m.menuOn=false; return m, m.applyMenuChoice(m.menuIndex)
			case "esc","m": m.menuOn=false; return m, nil
			}
		}
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
				dg := strings.TrimSpace(m.destGuess(url, dest))
				if err := m.preflightForDownload(url, dg); err != nil { m.newMsg = "preflight failed: "+err.Error(); return m, nil }
				m.addEphemeral(url, dg)
				m.newMsg = "starting download..."
				// close the modal immediately so focus returns to the table
				m.newDL = false
				return m, m.startDownloadCmd(url, dg)
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
			switch m.filterPreset {
			case "all":
				m.filterPreset = "active"
			case "active":
				m.filterPreset = "errors"
			default:
				m.filterPreset = "all"
			}
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
			if m.menuOn { m.menuIndex = 0 }
			return m, nil
case "p":
			// pause/cancel selected if running under TUI
			if m.selected >=0 && m.selected < len(m.rows) {
				key := m.rows[m.selected].URL+"|"+m.rows[m.selected].Dest
				if cancel, ok := m.running[key]; ok { cancel(); delete(m.running, key); m.newMsg = "cancelled: "+m.rows[m.selected].Dest }
			}
			return m, nil
case "y":
			// retry selected
			if m.selected >=0 && m.selected < len(m.rows) {
				u := m.rows[m.selected].URL; d := m.rows[m.selected].Dest
				ctx, cancel := context.WithCancel(context.Background())
				m.running[u+"|"+d] = cancel
				m.newMsg = "retrying..."
				return m, m.startDownloadCmdCtx(ctx, u, d)
			}
			return m, nil
case "C":
			// copy dest path
			if m.selected >=0 && m.selected < len(m.rows) {
				if err := copyToClipboard(m.rows[m.selected].Dest); err != nil { m.newMsg = "copy failed: "+err.Error() } else { m.newMsg = "copied path to clipboard" }
			}
			return m, nil
case "U":
			// copy URL
			if m.selected >=0 && m.selected < len(m.rows) {
				if err := copyToClipboard(m.rows[m.selected].URL); err != nil { m.newMsg = "copy failed: "+err.Error() } else { m.newMsg = "copied URL to clipboard" }
			}
			return m, nil
case "O":
			// open folder (or reveal on macOS)
			if m.selected >=0 && m.selected < len(m.rows) {
				p := m.rows[m.selected].Dest
				if p != "" {
					if err := openInFileManager(p, true); err != nil { m.newMsg = "open failed: "+err.Error() } else { m.newMsg = "opened in file manager" }
				}
			}
			return m, nil
case "D":
			// delete staged data (.part) and clear chunk state
			if m.selected >=0 && m.selected < len(m.rows) {
				r := m.rows[m.selected]
				deleted := []string{}
				// hashed staged path
				p1 := downloader.StagePartPath(m.cfg, r.URL, r.Dest)
				if fi, err := os.Stat(p1); err == nil && !fi.IsDir() { if err := os.Remove(p1); err == nil { deleted = append(deleted, p1) } }
				// next-to-dest .part
				p2 := r.Dest + ".part"
				if p2 != p1 { if fi, err := os.Stat(p2); err == nil && !fi.IsDir() { if err := os.Remove(p2); err == nil { deleted = append(deleted, p2) } } }
				_ = m.st.DeleteChunks(r.URL, r.Dest)
				if len(deleted) > 0 { m.newMsg = "deleted staged data: " + strings.Join(deleted, ", ") } else { m.newMsg = "no staged data found" }
				return m, refreshCmd()
			}
			return m, nil
case "X":
			// clear download row from DB (useful for stuck planning rows)
			if m.selected >=0 && m.selected < len(m.rows) {
				r := m.rows[m.selected]
				_ = m.st.DeleteChunks(r.URL, r.Dest)
				if err := m.st.DeleteDownload(r.URL, r.Dest); err != nil { m.newMsg = "clear failed: "+err.Error() } else { m.newMsg = "cleared row" }
				return m, refreshCmd()
			}
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
		// Clear ephemerals when a matching DB row appears by URL or Dest.
		if len(m.ephems) > 0 {
			for k, e := range m.ephems {
				cleared := false
				for _, r := range rows {
					if r.URL == e.URL { cleared = true; break }
					if e.Dest != "" && r.Dest != "" && filepath.Clean(r.Dest) == filepath.Clean(e.Dest) { cleared = true; break }
				}
				if cleared { delete(m.ephems, k) }
			}
		}
		if m.selected >= len(m.rows) { m.selected = len(m.rows) - 1 }
		if m.selected < 0 { m.selected = 0 }
		// Auto-hide tips once we have any rows
		if len(m.rows) > 0 { m.tipsOn = false }
		// advance spinner
		m.spin++
		return m, tickCmd(m.refresh)
	case errMsg:
		m.err = msg.err
		return m, tickCmd(m.refresh)
case dlDoneMsg:
		// clear ephemeral if still present
		if strings.TrimSpace(msg.url) != "" { delete(m.ephems, ephemeralKey(msg.url, msg.dest)) }
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
	if m.tipsOn && len(m.rows) == 0 {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("Welcome to modfetch")+"\n")
		b.WriteString("- Press n to add your first download (URL and optional dest)\n")
		b.WriteString("- Press m for Menu, h for Help, / to Filter\n")
		b.WriteString("- Config: ~/.config/modfetch/config.yml (auto-created on first run)\n\n")
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
// Prepend ephemeral starters so UI never looks idle
if len(m.ephems) > 0 {
	keys := make([]string, 0, len(m.ephems))
	for k := range m.ephems { keys = append(keys, k) }
	sort.Strings(keys)
	for _, k := range keys {
		e := m.ephems[k]
		rows = append([]state.DownloadRow{{URL: e.URL, Dest: e.Dest, Status: "pending", Size: 0}}, rows...)
	}
}
// apply preset filters
switch m.filterPreset {
case "active":
	rows = filterByStatuses(rows, []string{"planning","pending","running"})
case "errors":
	rows = filterByStatuses(rows, []string{"error","checksum_mismatch","verify_failed"})
}
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
	// Ephemeral starter: show resolving spinner
	if _, ok := m.ephems[ephemeralKey(r.URL, r.Dest)]; ok {
		return "resolving " + m.spinnerChar()
	}
	if strings.EqualFold(r.Status, "planning") {
		return "planning " + m.spinnerChar()
	}
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
	// Guidance if stuck in planning for more than ~5s without chunks
	if strings.EqualFold(r.Status, "planning") {
		chunks, _ := m.st.ListChunks(r.URL, r.Dest)
		if len(chunks) == 0 && r.UpdatedAt > 0 && time.Since(time.Unix(r.UpdatedAt, 0)) > 5*time.Second {
			fmt.Fprintf(&sb, "    %s %s\n", m.styles.label.Render("note:"), "still planning… this can happen if the server blocks HEAD; it will auto-fallback to single. If this persists, check CIVITAI_TOKEN/HF_TOKEN and network, or press y to retry.")
		}
	}
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
		// No chunk info (likely single-stream). Use the staged .part path consistent with downloader.
		if r.Dest != "" {
			p := downloader.StagePartPath(m.cfg, r.URL, r.Dest)
			if st, err := os.Stat(p); err == nil {
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

func filterByStatuses(in []state.DownloadRow, want []string) []state.DownloadRow {
	set := map[string]struct{}{}
	for _, s := range want { set[strings.ToLower(s)] = struct{}{} }
	var out []state.DownloadRow
	for _, r := range in {
		if _, ok := set[strings.ToLower(r.Status)]; ok { out = append(out, r) }
	}
	return out
}

func (m *model) helpView() string {
	return "Help: j/k up/down • r refresh • n new download • f filter preset • g group by status • t toggle columns • d details • / filter • s sort by speed • e sort by ETA • o clear sort • C copy path • U copy URL • O open folder • D delete staged data • X clear row • m menu • h/? toggle help"
}

func (m *model) menuView() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Menu") + "\n")
		for i, it := range m.menuItems {
			var line string
			if i == m.menuIndex { line = "> "+it } else { line = "  "+it }
			b.WriteString(line+"\n")
		}
	b.WriteString(m.styles.label.Render("Use ↑/↓ to select, Enter to apply, m or Esc to close")+"\n")
	return b.String()
}

func (m *model) applyMenuChoice(i int) tea.Cmd {
	if i < 0 || i >= len(m.menuItems) { return nil }
	switch m.menuItems[i] {
	case "New download": m.newDL = true; m.newURL.SetValue(""); m.newDest.SetValue(""); m.newFocus=0; m.newURL.Focus(); m.newMsg=""
	case "Filter preset":
		switch m.filterPreset {
		case "all":
			m.filterPreset = "active"
		case "active":
			m.filterPreset = "errors"
		default:
			m.filterPreset = "all"
		}
	case "Group by status": m.groupOn = !m.groupOn
	case "Toggle columns": m.showURLFirst = !m.showURLFirst
	case "Sort by speed": m.sortMode = "speed"
	case "Sort by ETA": m.sortMode = "eta"
	case "Clear sort": m.sortMode = ""
	case "Help": m.showHelp = !m.showHelp
	case "Refresh": return refreshCmd()
	case "Quit": return tea.Quit
	}
	return nil
}

func (m *model) startDownloadCmd(urlStr, dest string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	key := urlStr+"|"+dest
	m.running[key] = cancel
	return m.startDownloadCmdCtx(ctx, urlStr, dest)
}

func (m *model) startDownloadCmdCtx(ctx context.Context, urlStr, dest string) tea.Cmd {
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

// Helpers

func (m *model) spinnerChar() string {
	frames := []rune{'-', '\\', '|', '/'}
	return string(frames[m.spin%len(frames)])
}

// composite key helper for ephemerals
func ephemeralKey(url, dest string) string { return url + "|" + dest }

func (m *model) addEphemeral(url, dest string) {
	m.ephems[ephemeralKey(url, dest)] = ephemeral{URL: url, Dest: dest, When: time.Now()}
}

func (m *model) destGuess(urlStr, dest string) string {
	d := strings.TrimSpace(dest)
	if d != "" { return d }
	root := strings.TrimSpace(m.cfg.General.DownloadRoot)
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		if u, err := neturl.Parse(urlStr); err == nil {
			b := path.Base(u.Path)
			if b == "/" || b == "." || b == "" || b == ".." { b = "download" }
			candidate := filepath.Join(root, b)
			if rel, err := filepath.Rel(root, candidate); err == nil && !strings.HasPrefix(rel, "..") {
				return candidate
			}
			return filepath.Join(root, "download")
		}
	}
	// resolver URI; we don't know yet
	return filepath.Join(root, "(resolving)")
}

func (m *model) preflightForDownload(urlStr, dest string) error {
	// Ensure download_root exists and is writable
	if err := os.MkdirAll(m.cfg.General.DownloadRoot, 0o755); err != nil { return err }
	if err := tryWrite(m.cfg.General.DownloadRoot); err != nil { return fmt.Errorf("download_root not writable: %w", err) }
	// Ensure staging area exists and is writable depending on stage_partials
	if m.cfg.General.StagePartials {
		parts := m.cfg.General.PartialsRoot
		if strings.TrimSpace(parts) == "" { parts = filepath.Join(m.cfg.General.DownloadRoot, ".parts") }
		if err := os.MkdirAll(parts, 0o755); err != nil { return fmt.Errorf("create parts dir: %w", err) }
		if err := tryWrite(parts); err != nil { return fmt.Errorf("parts dir not writable: %w", err) }
	} else {
		// part lives next to dest; ensure its parent if dest provided
		d := strings.TrimSpace(dest)
		if d != "" {
			if err := os.MkdirAll(filepath.Dir(d), 0o755); err != nil { return fmt.Errorf("create dest dir: %w", err) }
			if err := tryWrite(filepath.Dir(d)); err != nil { return fmt.Errorf("dest dir not writable: %w", err) }
		}
	}
	return nil
}

func tryWrite(dir string) error {
	f, err := os.CreateTemp(dir, ".mf-wr-*")
	if err != nil { return err }
	name := f.Name(); _ = f.Close(); _ = os.Remove(name)
	return nil
}

// Modal overlay for new download (render within View)

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

func openInFileManager(p string, reveal bool) error {
	p = strings.TrimSpace(p)
	if p == "" { return fmt.Errorf("empty path") }
	// Determine directory to open even if file doesn't exist yet
	var dir string
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

