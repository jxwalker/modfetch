package tui2

import (
	"bufio"
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
	"modfetch/internal/placer"
	"modfetch/internal/resolver"
	"modfetch/internal/state"
)

type Theme struct {
	border      lipgloss.Style
	title       lipgloss.Style
	label       lipgloss.Style
	tabActive   lipgloss.Style
	tabInactive lipgloss.Style
	row         lipgloss.Style
	rowSelected lipgloss.Style
	head        lipgloss.Style
	footer      lipgloss.Style
	ok          lipgloss.Style
	bad         lipgloss.Style
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
		ok:          lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		bad:         lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
	}
}

type tickMsg time.Time

type dlDoneMsg struct{ url, dest, path string; err error }

type destSuggestMsg struct{ url string; dest string }

type metaMsg struct{ url string; fileName string; suggested string; civType string }

type probeMsg struct{ url string; dest string; reachable bool; info string }

type obs struct{ bytes int64; t time.Time }

type toast struct{ msg string; when time.Time; ttl time.Duration }

type Model struct {
	cfg   *config.Config
	st    *state.DB
	th    Theme
	w,h   int
	build string
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
	showInspector bool
	// New download modal state
	newJob bool
	newStep int
	newInput textinput.Model
	newURL string
	newDest string
	newType string
	newAutoPlace bool
	newAutoSuggested string // latest auto-suggested dest (to avoid overriding manual edits)
	newTypeDetected string  // detected artifact type (from civitai or heuristics)
	newTypeSource   string  // e.g., "civitai", "heuristic"
	newDetectedName string  // suggested filename/name from resolver (for display)
	// Batch import modal state
	batchMode bool
	batchInput textinput.Model
	// Overrides for auto-place by key url|dest
	placeType map[string]string
	autoPlace map[string]bool
	// Normalization note (shown after URL entry)
	newNormNote string

	themeIndex int
	columnMode string // dest|url|host
	tickEvery time.Duration
	// caches for performance
	rateCache map[string]float64
	etaCache  map[string]string
	totalCache map[string]int64
	curBytesCache map[string]int64
	// rate history for sparkline
	rateHist map[string][]float64
	// cache for hostnames
	hostCache map[string]string
	err   error
	// token/env status
	hfTokenSet bool
	civTokenSet bool
	hfRejected bool
	civRejected bool
}

type uiState struct {
	ThemeIndex int    `json:"theme_index"`
	ShowURL    bool   `json:"show_url"`
	ColumnMode string `json:"column_mode"`
	Compact    bool   `json:"compact"`
	GroupBy    string `json:"group_by"`
	SortMode   string `json:"sort_mode"`
}

func New(cfg *config.Config, st *state.DB, version string) tea.Model {
p := progress.New(progress.WithDefaultGradient(), progress.WithWidth(16))
	fi := textinput.New(); fi.Placeholder = "filter (url or dest contains)"; fi.CharLimit = 4096
	// compute refresh
	refresh := time.Second
	if hz := cfg.UI.RefreshHz; hz > 0 {
		if hz > 10 { hz = 10 }
		refresh = time.Second / time.Duration(hz)
	}
// decide initial column mode
	mode := strings.ToLower(strings.TrimSpace(cfg.UI.ColumnMode))
	if mode != "dest" && mode != "url" && mode != "host" {
		if cfg.UI.ShowURL { mode = "url" } else { mode = "dest" }
	}
m := &Model{
		cfg: cfg, st: st, th: defaultTheme(), activeTab: -1, prog: p, prev: map[string]obs{},
		build: strings.TrimSpace(version),
		running: map[string]context.CancelFunc{}, selectedKeys: map[string]bool{}, filterInput: fi,
		rateCache: map[string]float64{}, etaCache: map[string]string{}, totalCache: map[string]int64{}, curBytesCache: map[string]int64{}, rateHist: map[string][]float64{}, hostCache: map[string]string{},
		tickEvery: refresh,
		columnMode: mode,
		placeType: map[string]string{}, autoPlace: map[string]bool{},
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
		// New job modal handling
		if m.newJob {
			switch s {
			case "esc": m.newJob = false; return m, nil
			case "tab":
				// Path completion on Destination step
				if m.newStep == 4 {
					cur := strings.TrimSpace(m.newInput.Value())
					comp := m.completePath(cur)
					if strings.TrimSpace(comp) != "" { m.newInput.SetValue(comp); m.newAutoSuggested = comp }
					return m, nil
				}
				return m, nil
			case "enter":
				val := strings.TrimSpace(m.newInput.Value())
				if m.newStep == 1 {
					if val == "" { return m, nil }
					m.newURL = val
					// Show normalization note if applicable
					if norm, ok := m.normalizeURLForNote(val); ok { m.newNormNote = norm } else { m.newNormNote = "" }
					// Next: ask for artifact type
					m.newInput.SetValue("")
					m.newInput.Placeholder = "Artifact type (optional, e.g. sd.checkpoint)"
					m.newStep = 2
					// Resolve metadata in background (detect type, name)
					return m, m.resolveMetaCmd(m.newURL)
				} else if m.newStep == 2 {
					// Got type; ask for autoplace
					m.newType = val
					m.newInput.SetValue("")
					m.newInput.Placeholder = "Auto place after download? y/n (default n)"
					m.newStep = 3
					return m, nil
				} else if m.newStep == 3 {
					// Decide on destination suggestion based on autoplace choice
					v := strings.ToLower(strings.TrimSpace(val))
					m.newAutoPlace = v == "y" || v == "yes" || v == "true" || v == "1"
					var cand string
					if m.newAutoPlace {
						cand = m.computePlacementSuggestionImmediate(m.newURL, m.newType)
						// Background refine using resolver and placer targets
						m.newAutoSuggested = cand
						m.newInput.SetValue(cand)
						m.newInput.Placeholder = "Destination path (Enter to accept)"
						m.newStep = 4
						return m, m.suggestPlacementCmd(m.newURL, m.newType)
					}
					// No autoplace: suggest download_root path
					cand = m.immediateDestCandidate(m.newURL)
					m.newAutoSuggested = cand
					m.newInput.SetValue(cand)
					m.newInput.Placeholder = "Destination path (Enter to accept)"
					m.newStep = 4
					return m, m.suggestDestCmd(m.newURL)
				} else if m.newStep == 4 {
					// Finalize destination and start download
					urlStr := m.newURL
					dest := strings.TrimSpace(val)
					if dest == "" {
						if m.newAutoPlace { dest = m.computePlacementSuggestionImmediate(urlStr, m.newType) } else { dest = m.computeDefaultDest(urlStr) }
					}
					// Preflight: ensure destination directory is writable
					if err := m.preflightDest(dest); err != nil {
						m.addToast("dest not writable: "+err.Error())
						return m, nil
					}
					cmd := m.startDownloadCmd(urlStr, dest)
					key := urlStr+"|"+dest
					if strings.TrimSpace(m.newType) != "" { m.placeType[key] = m.newType }
					if m.newAutoPlace { m.autoPlace[key] = true }
					m.addToast("started: "+truncateMiddle(dest, 40))
					m.newJob = false
					return m, cmd
				}
			}
			var cmd tea.Cmd
			m.newInput, cmd = m.newInput.Update(msg)
			return m, cmd
		}
		// Batch modal handling
		if m.batchMode {
			switch s { case "esc": m.batchMode = false; return m, nil }
			if s == "enter" {
				path := strings.TrimSpace(m.batchInput.Value())
				var cmd tea.Cmd
				if path != "" { cmd = m.importBatchFile(path) }
				m.batchMode = false
				return m, cmd
			}
			var cmd tea.Cmd
			m.batchInput, cmd = m.batchInput.Update(msg)
			return m, cmd
		}
		// If filter mode is on, handle input first (swallow other keys into the input)
		if m.filterOn {
			switch s {
			case "enter": m.filterOn = false; m.filterInput.Blur(); return m, nil
			case "esc": m.filterOn = false; m.filterInput.SetValue(""); m.filterInput.Blur(); return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			return m, cmd
		}
		// Normal key handling
		switch s {
	case "q", "ctrl+c":
			m.saveUIState()
			return m, tea.Quit
		case "/":
			m.filterOn = true; m.filterInput.Focus(); return m, nil
		case "n": // new download wizard
			m.newJob = true; m.newStep = 1; m.newInput = textinput.New(); m.newInput.Placeholder = "Enter URL or resolver URI"; m.newInput.Focus(); return m, nil
		case "b": // batch import from text file
			m.batchMode = true; m.batchInput = textinput.New(); m.batchInput.Placeholder = "Path to text file with URLs"; m.batchInput.Focus(); return m, nil
		case "s": m.sortMode = "speed"; return m, nil
		case "e": m.sortMode = "eta"; return m, nil
		case "o": m.sortMode = ""; return m, nil
		case "g": if m.groupBy=="host" { m.groupBy = "" } else { m.groupBy = "host" }; return m, nil
case "t": // cycle last column DEST->URL->HOST
			switch m.columnMode {
			case "dest": m.columnMode = "url"
			case "url": m.columnMode = "host"
			default: m.columnMode = "dest"
			}
			m.saveUIState(); return m, nil
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
		case "P": // re-probe reachability for selected/current without starting
			rows := m.selectionOrCurrent()
			if len(rows) == 0 { return m, nil }
			return m, m.probeSelectedCmd(rows)
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "i": // toggle inspector panel
			m.showInspector = !m.showInspector
			return m, nil
		case "0": m.activeTab = -1; m.selected = 0; return m, nil
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
		case "r": // start selected (alias of retry)
			fallthrough
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
		case "D": // delete selected rows from DB (even if planning)
			rows := m.selectionOrCurrent()
			if len(rows) == 0 { return m, nil }
			deleted := 0
			for _, r := range rows {
				key := keyFor(r)
				if cancel, ok := m.running[key]; ok { cancel(); delete(m.running, key) }
				_ = m.st.DeleteChunks(r.URL, r.Dest)
				if err := m.st.DeleteDownload(r.URL, r.Dest); err == nil { deleted++ }
				delete(m.selectedKeys, key)
			}
			if deleted > 0 { m.addToast(fmt.Sprintf("deleted %d", deleted)) }
			return m, m.refresh()
		}
	case tickMsg:
		return m, m.refresh()
case destSuggestMsg:
		// Update the suggested dest only if the user hasn't edited it since our last suggestion
		if m.newJob && m.newStep == 2 && strings.TrimSpace(m.newURL) == strings.TrimSpace(msg.url) {
			if strings.TrimSpace(m.newInput.Value()) == strings.TrimSpace(m.newAutoSuggested) || strings.TrimSpace(m.newInput.Value()) == "" {
				m.newInput.SetValue(msg.dest)
				m.newAutoSuggested = msg.dest
			}
		}
		return m, nil
case metaMsg:
		if m.newJob && strings.TrimSpace(m.newURL) == strings.TrimSpace(msg.url) {
			m.newDetectedName = msg.suggested
			// Map civitai type to artifact type; fallback to heuristic from filename
			t := mapCivitFileType(msg.civType, msg.fileName)
			m.newTypeDetected = t
			m.newTypeSource = "civitai"
			if m.newStep == 2 {
				cur := strings.TrimSpace(m.newInput.Value())
				if cur == "" { m.newInput.SetValue(t) }
			}
		}
		return m, nil
case probeMsg:
		// Show toast for probe result; refresh table to reflect any hold updates
		host := hostOf(msg.url)
		if msg.reachable {
			m.addToast("probe ok: "+host+" ("+msg.info+")")
		} else {
			m.addToast("probe failed: unreachable; on hold ("+msg.info+")")
		}
		return m, m.refresh()
case dlDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			// Mark token rejection hints on known hosts based on error text
			errTxt := strings.ToLower(m.err.Error())
			if strings.Contains(errTxt, "unauthorized") || strings.Contains(errTxt, "401") || strings.Contains(errTxt, "forbidden") || strings.Contains(errTxt, "403") || strings.Contains(errTxt, "not authorized") {
				if strings.Contains(errTxt, "hugging face") || strings.Contains(errTxt, "huggingface") {
					m.hfRejected = true
				}
				if strings.Contains(errTxt, "civitai") {
					m.civRejected = true
				}
			}
			return m, m.refresh()
		}
		// On success, clear any prior rejection for the host
		u := strings.ToLower(msg.url)
		if strings.HasPrefix(u, "hf://") || strings.Contains(u, "huggingface") { m.hfRejected = false }
		if strings.HasPrefix(u, "civitai://") || strings.Contains(u, "civitai") { m.civRejected = false }
		// Auto place if configured
		key := msg.url+"|"+msg.dest
		if m.autoPlace[key] {
			atype := m.placeType[key]
			if atype == "" { atype = "" }
			placed, err := placer.Place(m.cfg, msg.path, atype, "")
			if err != nil { m.addToast("place failed: "+err.Error()) } else if len(placed) > 0 { m.addToast("placed: "+truncateMiddle(placed[0], 40)) }
			delete(m.autoPlace, key); delete(m.placeType, key)
		}
		return m, m.refresh()
	}
	return m, nil
}

func (m *Model) View() string {
	if m.w == 0 { m.w = 120 }
	if m.h == 0 { m.h = 30 }
	// Title bar + inline toasts
	ver := m.build
	if ver == "" { ver = "dev" }
	title := m.th.title.Render(fmt.Sprintf("modfetch • TUI v2 (build %s)", ver))
	toastStr := m.renderToasts()
titleBar := m.th.border.Width(m.w-2).Render(lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", toastStr))

	// Top panels: fixed height; left = key hints, right = stats (align right edges)
	topHeight := 8
	// Each bordered panel contributes 2 columns (left+right border)
	topBoxes := 2
	topUsable := m.w - topBoxes*2
	if topUsable < 20 { topUsable = 20 }
	topLeftW := topUsable / 2
	if topLeftW < 40 { topLeftW = 40 }
	if topLeftW > topUsable-20 { topLeftW = topUsable - 20 }
	topRightW := topUsable - topLeftW
	topLeft := m.th.border.Width(topLeftW).Height(topHeight).Render(m.renderTopLeftHints())
	topRight := m.th.border.Width(topRightW).Height(topHeight).Render(m.renderTopRightStats())
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, topLeft, topRight)

	// Bottom panels: queue/table (with tabs), optional inspector (align right edges)
	leftW := 24
	inspW := 0
	boxes := 2
	if m.showInspector { inspW = 42; boxes = 3 }
	usable := m.w - boxes*2
	if usable < 40 { usable = 40 }
	mainW := usable - leftW - inspW
	if mainW < 10 { mainW = 10 }
	left := m.th.border.Width(leftW).Render(m.renderTabs())
	main := m.th.border.Width(mainW).Render(m.renderTable())
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, left, main)
	if m.showInspector {
		insp := m.th.border.Width(inspW).Render(m.renderInspector())
		bottom = lipgloss.JoinHorizontal(lipgloss.Top, left, main, insp)
	}

	// Optional overlays
	drawer := ""
	if m.showToastDrawer { drawer = m.th.border.Width(m.w-2).Render(m.renderToastDrawer()) }
	help := ""
	if m.showHelp { help = m.th.border.Width(m.w-2).Render(m.renderHelp()) }
	modal := ""
	if m.newJob { modal = m.th.border.Width(m.w-4).Render(m.renderNewJobModal()) }
	if m.batchMode { modal = m.th.border.Width(m.w-4).Render(m.renderBatchModal()) }

	// Footer with filter bar
	filterBar := ""
	if m.filterOn { filterBar = "Filter: "+ m.filterInput.View() }
footer := m.th.border.Width(m.w-2).Render(m.th.footer.Render("0 All • 1 Pending • 2 Active • 3 Completed • 4 Failed • j/k nav • y/r start • p cancel • D delete • O open • / filter • s/e sort • o clear • g group host • t last col URL/DEST/HOST • v compact • i inspector • T theme • H toasts • ? help • X clear sel • q quit\n"+filterBar))

	parts := []string{titleBar, topRow, bottom}
	if help != "" { parts = append(parts, help) }
	if drawer != "" { parts = append(parts, drawer) }
	if modal != "" { parts = append(parts, modal) }
	parts = append(parts, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *Model) refresh() tea.Cmd {
	// Update token env presence status on each refresh tick
	m.updateTokenEnvStatus()
	rows, err := m.st.ListDownloads()
	if err != nil { _ = logging.New("error", false) }
	m.rows = rows
	m.lastRefresh = time.Now()
	m.gcToasts()
	// determine visible keys to focus updates
	vis := map[string]struct{}{}
	vr := m.visibleRows()
	maxRows := m.maxRowsOnScreen()
	for i, r := range vr {
		if i >= maxRows { break }
		vis[keyFor(r)] = struct{}{}
	}
	// recompute caches, prioritizing visible, running rows
	for _, r := range m.rows {
		key := keyFor(r)
		// cache host if needed
		if m.columnMode == "host" {
			if _, ok := m.hostCache[key]; !ok {
				m.hostCache[key] = hostOf(r.URL)
			}
		}
		_, isVis := vis[key]
		status := strings.ToLower(r.Status)
		shouldUpdate := isVis && (status == "running" || status == "planning" || status == "pending")
		cur := m.curBytesCache[key]
		total := r.Size
		if shouldUpdate {
			c2, t2 := m.computeCurAndTotal(r)
			cur, total = c2, t2
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

// renderTopLeftHints shows compact key hints in a fixed-size box
func (m *Model) renderTopLeftHints() string {
	lines := []string{
		m.th.head.Render("Hints"),
		"Tabs: 0 All • 1 Pending • 2 Active • 3 Completed • 4 Failed",
		"New: n new download • b batch import (txt file)",
		"Nav: j/k up/down",
		"Filter: / enter • Enter apply • Esc clear",
		"Sort: s speed • e ETA • o clear | Group: g host",
		"Select: Space toggle • A all • X clear",
		"Actions: y/r start • p cancel • D delete | Open: O • Copy: C/U | Probe: P",
		"View: t column • v compact • i inspector • T theme • H toasts • ? help • q quit",
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderNewJobModal() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("New Download")+"\n")
	// Token status inline
	sb.WriteString(m.th.label.Render(m.renderAuthStatus())+"\n\n")
	steps := []string{
		"1) URL/URI",
		"2) Artifact type (optional)",
		"3) Auto place after download (y/n)",
		"4) Destination path",
	}
	sb.WriteString(m.th.label.Render(strings.Join(steps, " • "))+"\n\n")
	cur := ""
	if m.newStep == 1 { cur = "Enter URL or resolver URI" }
	if m.newStep == 2 { cur = "Artifact type (optional)" }
	if m.newStep == 3 { cur = "Auto place? y/n" }
	if m.newStep == 4 { cur = "Destination path (Tab to complete)" }
	sb.WriteString(m.th.label.Render(cur)+"\n")
	// Show normalization note once URL entered
	if m.newStep >= 2 && strings.TrimSpace(m.newNormNote) != "" {
		sb.WriteString(m.th.label.Render(m.newNormNote)+"\n")
	}
	// Show detected type/name if available when choosing type
	if m.newStep == 2 && strings.TrimSpace(m.newTypeDetected) != "" {
		from := m.newTypeSource
		if from == "" { from = "heuristic" }
		sb.WriteString(m.th.label.Render(fmt.Sprintf("Detected: %s (%s)", m.newTypeDetected, from))+"\n")
	}
	sb.WriteString(m.newInput.View())
	return sb.String()
}

func (m *Model) renderBatchModal() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("Batch Import")+"\n")
	sb.WriteString(m.th.label.Render("Enter path to text file with one URL per line (tokens: dest=..., type=..., place=true, mode=copy|symlink|hardlink)" )+"\n\n")
	sb.WriteString(m.batchInput.View())
	return sb.String()
}

// renderTopRightStats shows aggregate metrics and recent toasts summary
func (m *Model) renderTopRightStats() string {
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
		gRate += m.rateCache[keyFor(r)]
	}
	lines := []string{
		m.th.head.Render("Stats"),
		fmt.Sprintf("Pending:   %d", pending),
		fmt.Sprintf("Active:    %d", active),
		fmt.Sprintf("Completed: %d", done),
		fmt.Sprintf("Failed:    %d", failed),
		fmt.Sprintf("Global Rate: %s/s", humanize.Bytes(uint64(gRate))),
		m.renderAuthStatus(),
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderTabs() string {
	labels := []struct{ name string; tab int }{
		{"All", -1},
		{"Pending", 0},
		{"Active", 1},
		{"Completed", 2},
		{"Failed", 3},
	}
	var sb strings.Builder
	for _, it := range labels {
		style := m.th.tabInactive
		if it.tab == m.activeTab { style = m.th.tabActive }
		count := 0
		if it.tab == -1 { count = len(m.rows) } else { count = len(m.filterRows(it.tab)) }
		sb.WriteString(style.Render(fmt.Sprintf("%s (%d)", it.name, count)))
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
		if tab == -1 { out = append(out, r); continue }
		ls := strings.ToLower(r.Status)
		switch tab {
		case 0: if ls=="pending" || ls=="planning" || ls=="hold" { out = append(out, r) }
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
	lastLabel := "DEST"
	if m.columnMode == "url" { lastLabel = "URL" } else if m.columnMode == "host" { lastLabel = "HOST" }
	if m.isCompact() {
		sb.WriteString(m.th.head.Render(fmt.Sprintf("%-1s %-8s  %-16s  %-4s  %-12s  %-8s  %-s", "S", "STATUS", "PROG", "PCT", "SRC", "ETA", lastLabel)))
	} else {
		sb.WriteString(m.th.head.Render(fmt.Sprintf("%-1s %-8s  %-16s  %-4s  %-10s  %-10s  %-12s  %-8s  %-s", "S", "STATUS", "PROG", "PCT", "SPEED", "THR", "SRC", "ETA", lastLabel)))
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
		// Progress and pct
		prog := m.renderProgress(r)
		cur, total, rate, _ := m.progressFor(r)
		pct := "--%"
		if total > 0 {
			ratio := float64(cur) / float64(total)
			if ratio < 0 { ratio = 0 }
			if ratio > 1 { ratio = 1 }
			pct = fmt.Sprintf("%3.0f%%", ratio*100)
		}
		eta := m.etaCache[keyFor(r)]
		thr := m.renderSparkline(keyFor(r))
		sel := " "; if m.selectedKeys[keyFor(r)] { sel = "*" }
		last := r.Dest
		if m.columnMode == "url" { last = logging.SanitizeURL(r.URL) } else if m.columnMode == "host" { last = hostOf(r.URL) }
		src := hostOf(r.URL)
		src = truncateMiddle(src, 12)
		lw := m.lastColumnWidth(m.isCompact())
		last = truncateMiddle(last, lw)
		var line string
		if m.isCompact() {
			line = fmt.Sprintf("%-1s %-8s  %-16s  %-4s  %-12s  %-8s  %s", sel, r.Status, prog, pct, src, eta, last)
		} else {
			line = fmt.Sprintf("%-1s %-8s  %-16s  %-4s  %-10s  %-10s  %-12s  %-8s  %s", sel, r.Status, prog, pct, humanize.Bytes(uint64(rate))+"/s", thr, src, eta, last)
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

// Preflight: ensure destination's parent is writable
func (m *Model) preflightDest(dest string) error {
	d := strings.TrimSpace(dest)
	if d == "" { return fmt.Errorf("empty dest") }
	dir := filepath.Dir(d)
	if err := os.MkdirAll(dir, 0o755); err != nil { return err }
	return tryWrite(dir)
}

// Path completion for destination
func (m *Model) completePath(input string) string {
	s := strings.TrimSpace(input)
	if s == "" { return s }
	// Expand ~ and env for lookup
	h, _ := os.UserHomeDir()
	exp := os.ExpandEnv(s)
	if strings.HasPrefix(exp, "~") {
		exp = strings.Replace(exp, "~", h, 1)
	}
	dir := exp
	prefix := ""
	if fi, err := os.Stat(exp); err == nil && fi.IsDir() {
		// complete inside dir
	} else {
		dir = filepath.Dir(exp)
		prefix = filepath.Base(exp)
	}
	ents, err := os.ReadDir(dir)
	if err != nil { return s }
	matches := make([]string, 0, len(ents))
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			if e.IsDir() { name += "/" }
			matches = append(matches, name)
		}
	}
	if len(matches) == 0 { return s }
	// If single match, replace basename
	base := prefix
	if len(matches) == 1 {
		base = matches[0]
	} else {
		// longest common prefix
		base = longestCommonPrefix(matches)
	}
	out := filepath.Join(dir, base)
	// compress home to ~ for display
	if h != "" && strings.HasPrefix(out, h+string(os.PathSeparator)) {
		out = "~" + strings.TrimPrefix(out, h)
	}
	return out
}

func longestCommonPrefix(ss []string) string {
	if len(ss) == 0 { return "" }
	p := ss[0]
	for _, s := range ss[1:] {
		for !strings.HasPrefix(strings.ToLower(s), strings.ToLower(p)) {
			if len(p) == 0 { return "" }
			p = p[:len(p)-1]
		}
	}
	return p
}

func tryWrite(dir string) error {
	f, err := os.CreateTemp(dir, ".mf-wr-*")
	if err != nil { return err }
	name := f.Name(); _ = f.Close(); _ = os.Remove(name)
	return nil
}

// Immediate (non-blocking) dest candidate based on URL only
func (m *Model) immediateDestCandidate(urlStr string) string {
	s := strings.TrimSpace(urlStr)
	if s == "" { return filepath.Join(m.cfg.General.DownloadRoot, "download") }
	// If it's a resolver URI, guess based on path base
	if strings.HasPrefix(s, "hf://") || strings.HasPrefix(s, "civitai://") {
		if u, err := neturl.Parse(s); err == nil {
			base := filepath.Base(strings.Trim(u.Path, "/"))
			if base == "" { base = "download" }
			return filepath.Join(m.cfg.General.DownloadRoot, utilSafe(base))
		}
	}
	// HTTP(S)
	if u, err := neturl.Parse(s); err == nil {
		h := strings.ToLower(u.Hostname())
		if hostIs(h, "huggingface.co") {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 1 {
				if len(parts) >= 5 && parts[2] == "blob" {
					base := filepath.Base(strings.Join(parts[4:], "/"))
					if base != "" { return filepath.Join(m.cfg.General.DownloadRoot, utilSafe(base)) }
				}
				return filepath.Join(m.cfg.General.DownloadRoot, utilSafe(filepath.Base(u.Path)))
			}
		}
		if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
			// Use model ID as placeholder
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 2 {
				id := parts[1]
				if id != "" { return filepath.Join(m.cfg.General.DownloadRoot, utilSafe(id)) }
			}
		}
		// Fallback to path base
		base := filepath.Base(s)
		if base == "" || base == "/" || base == "." { base = "download" }
		return filepath.Join(m.cfg.General.DownloadRoot, utilSafe(base))
	}
	// raw fallback
	base := filepath.Base(s)
	if base == "" || base == "/" || base == "." { base = "download" }
	return filepath.Join(m.cfg.General.DownloadRoot, utilSafe(base))
}

// Background suggestion using resolver (may perform network I/O)
func (m *Model) suggestDestCmd(urlStr string) tea.Cmd {
	return func() tea.Msg {
		d := m.computeDefaultDest(urlStr)
		return destSuggestMsg{url: urlStr, dest: d}
	}
}

// Immediate placement suggestion using artifact type and quick filename
func (m *Model) computePlacementSuggestionImmediate(urlStr, atype string) string {
	atype = strings.TrimSpace(atype)
	if atype == "" { return m.immediateDestCandidate(urlStr) }
	dirs, err := placer.ComputeTargets(m.cfg, atype)
	if err != nil || len(dirs) == 0 { return m.immediateDestCandidate(urlStr) }
	base := filepath.Base(m.immediateDestCandidate(urlStr))
	if strings.TrimSpace(base) == "" { base = "download" }
return filepath.Join(dirs[0], utilSafe(base))
}

func mapCivitFileType(civType, fileName string) string {
	ct := strings.ToLower(strings.TrimSpace(civType))
	name := strings.ToLower(fileName)
	switch ct {
	case "vae":
		return "sd.vae"
	case "textualinversion":
		return "sd.embedding"
	}
	// Heuristics based on filename
	if strings.Contains(name, "lora") || strings.Contains(name, "lokr") || strings.Contains(name, "locon") {
		return "sd.lora"
	}
	if strings.Contains(name, "control") || strings.Contains(name, "controlnet") {
		return "sd.controlnet"
	}
	if strings.HasSuffix(name, ".gguf") { return "llm.gguf" }
	if strings.HasSuffix(name, ".safetensors") { return "sd.checkpoint" }
	return "generic"
}

// Background placement suggestion: resolve filename then join with placement target
func (m *Model) suggestPlacementCmd(urlStr, atype string) tea.Cmd {
	return func() tea.Msg {
		atype = strings.TrimSpace(atype)
		if atype == "" { return destSuggestMsg{url: urlStr, dest: m.computeDefaultDest(urlStr)} }
		dirs, err := placer.ComputeTargets(m.cfg, atype)
		if err != nil || len(dirs) == 0 { return destSuggestMsg{url: urlStr, dest: m.computeDefaultDest(urlStr)} }
		// Derive a good filename via resolver-backed default dest
		cand := m.computeDefaultDest(urlStr)
		base := filepath.Base(cand)
		if strings.TrimSpace(base) == "" { base = "download" }
		d := filepath.Join(dirs[0], utilSafe(base))
		return destSuggestMsg{url: urlStr, dest: d}
	}
}

// Resolve metadata (suggested filename and civitai file type) in background
func (m *Model) resolveMetaCmd(raw string) tea.Cmd {
	return func() tea.Msg {
		s := strings.TrimSpace(raw)
		if s == "" { return metaMsg{url: raw} }
		// Normalize civitai page URLs to civitai://
		if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
			if u, err := neturl.Parse(s); err == nil {
				h := strings.ToLower(u.Hostname())
				if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
					parts := strings.Split(strings.Trim(u.Path, "/"), "/")
					if len(parts) >= 2 {
						modelID := parts[1]
						q := u.Query(); ver := q.Get("modelVersionId"); if ver == "" { ver = q.Get("version") }
						civ := "civitai://model/" + modelID
						if strings.TrimSpace(ver) != "" { civ += "?version=" + neturl.QueryEscape(ver) }
						s = civ
					}
				}
			}
		}
		if strings.HasPrefix(s, "hf://") || strings.HasPrefix(s, "civitai://") {
			if res, err := resolver.Resolve(context.Background(), s, m.cfg); err == nil {
				return metaMsg{url: raw, fileName: res.FileName, suggested: res.SuggestedFilename, civType: res.FileType}
			}
		}
		return metaMsg{url: raw}
	}
}

func truncateMiddle(s string, max int) string {
	if max <= 0 { return "" }
	runes := []rune(s)
	if len(runes) <= max { return s }
	if max <= 1 { return string(runes[:max]) }
	left := max / 2
	right := max - left - 1
	return string(runes[:left]) + "…" + string(runes[len(runes)-right:])
}

func (m *Model) lastColumnWidth(compact bool) int {
	leftW := 24
	inspW := 0
	if m.showInspector { inspW = 42 }
	// Rough usable width of main table inside borders
	boxes := 2; if m.showInspector { boxes = 3 }
	usable := m.w - leftW - inspW - boxes*2
	if usable < 40 { usable = 40 }
	if compact {
		// S, space, STATUS, 2sp, PROG, 2sp, PCT, 2sp, SRC, 2sp, ETA, 2sp
		consumed := 1 + 1 + 8 + 2 + 16 + 2 + 4 + 2 + 12 + 2 + 8 + 2
		lw := usable - consumed
		if lw < 10 { lw = 10 }
		return lw
	}
	// non-compact consumed widths: add SRC(12) before ETA
	consumed := 1 + 1 + 8 + 2 + 16 + 2 + 4 + 2 + 10 + 2 + 10 + 2 + 12 + 2 + 8 + 2
	lw := usable - consumed
	if lw < 10 { lw = 10 }
	return lw
}

func (m *Model) maxRowsOnScreen() int {
	max := m.h - 10
	if max < 3 { return 3 }
	return max
}

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

func (m *Model) addToast(s string) {
	m.toasts = append(m.toasts, toast{msg: s, when: time.Now(), ttl: 3*time.Second})
	if len(m.toasts) > 50 {
		// keep last 50
		m.toasts = m.toasts[len(m.toasts)-50:]
	}
}

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
if st.ColumnMode != "" { m.columnMode = st.ColumnMode } else if st.ShowURL { m.columnMode = "url" }
	if st.Compact { m.cfg.UI.Compact = true }
	if st.GroupBy != "" { m.groupBy = st.GroupBy }
	if st.SortMode != "" { m.sortMode = st.SortMode }
}

func (m *Model) saveUIState() {
	p := m.uiStatePath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
st := uiState{ThemeIndex: m.themeIndex, ShowURL: m.columnMode == "url", ColumnMode: m.columnMode, Compact: m.cfg.UI.Compact, GroupBy: m.groupBy, SortMode: m.sortMode}
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
	sb.WriteString("Probe: P re-probe reachability of selected/current (does not start download)\n")
sb.WriteString("Columns: t cycle URL/DEST/HOST • v compact view\n")
	sb.WriteString("Actions: y/r start • p cancel • D delete • O open • C copy path • U copy URL\n")
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

// URL normalization helper (for modal note only)
func (m *Model) normalizeURLForNote(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if !(strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")) { return "", false }
	u, err := neturl.Parse(s)
	if err != nil { return "", false }
	h := strings.ToLower(u.Hostname())
	if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 2 {
			modelID := parts[1]
			q := u.Query(); ver := q.Get("modelVersionId"); if ver == "" { ver = q.Get("version") }
			civ := "civitai://model/" + modelID
			if strings.TrimSpace(ver) != "" { civ += "?version=" + ver }
			return "Normalized to " + civ, true
		}
	}
	if hostIs(h, "huggingface.co") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 5 && parts[2] == "blob" {
			owner := parts[0]; repo := parts[1]; rev := parts[3]; filePath := strings.Join(parts[4:], "/")
			hf := "hf://" + owner + "/" + repo + "/" + filePath + "?rev=" + rev
			return "Normalized to " + hf, true
		}
	}
	return "", false
}

// Token/env helpers
func (m *Model) updateTokenEnvStatus() {
	// Presence only (set/unset); rejection is updated upon errors in dlDoneMsg
	if m.cfg.Sources.HuggingFace.Enabled {
		env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv)
		if env == "" { env = "HF_TOKEN" }
		m.hfTokenSet = strings.TrimSpace(os.Getenv(env)) != ""
	} else {
		m.hfTokenSet = false
	}
	if m.cfg.Sources.CivitAI.Enabled {
		env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
		if env == "" { env = "CIVITAI_TOKEN" }
		m.civTokenSet = strings.TrimSpace(os.Getenv(env)) != ""
	} else {
		m.civTokenSet = false
	}
}

// hostIs returns true if h equals root or is a subdomain of root.
func hostIs(h, root string) bool {
	h = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
	root = strings.ToLower(strings.TrimSpace(root))
	return h == root || strings.HasSuffix(h, "."+root)
}

func (m *Model) renderAuthStatus() string {
	// derive statuses with precedence: disabled > rejected > set > unset
	envHF := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv)
	if envHF == "" { envHF = "HF_TOKEN" }
	envCIV := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
	if envCIV == "" { envCIV = "CIVITAI_TOKEN" }
	hfEnabled := m.cfg.Sources.HuggingFace.Enabled
	civEnabled := m.cfg.Sources.CivitAI.Enabled
	var hf string
	var civ string
	if !hfEnabled {
		hf = m.th.label.Render(fmt.Sprintf("HF (%s): disabled", envHF))
	} else if m.hfRejected {
		hf = m.th.bad.Render(fmt.Sprintf("HF (%s): rejected", envHF))
	} else if m.hfTokenSet {
		hf = m.th.ok.Render(fmt.Sprintf("HF (%s): set", envHF))
	} else {
		hf = m.th.bad.Render(fmt.Sprintf("HF (%s): unset", envHF))
	}
	if !civEnabled {
		civ = m.th.label.Render(fmt.Sprintf("CivitAI (%s): disabled", envCIV))
	} else if m.civRejected {
		civ = m.th.bad.Render(fmt.Sprintf("CivitAI (%s): rejected", envCIV))
	} else if m.civTokenSet {
		civ = m.th.ok.Render(fmt.Sprintf("CivitAI (%s): set", envCIV))
	} else {
		civ = m.th.bad.Render(fmt.Sprintf("CivitAI (%s): unset", envCIV))
	}
	return "Auth: " + hf + "  " + civ
}

// Helpers for new download/batch
func (m *Model) computeDefaultDest(urlStr string) string {
	ctx := context.Background()
	u := strings.TrimSpace(urlStr)
	if strings.HasPrefix(u, "hf://") || strings.HasPrefix(u, "civitai://") {
		if res, err := resolver.Resolve(ctx, u, m.cfg); err == nil {
			name := res.SuggestedFilename
			if strings.TrimSpace(name) == "" {
				name = filepath.Base(res.URL)
			}
			name = utilSafe(name)
			return filepath.Join(m.cfg.General.DownloadRoot, name)
		}
	}
	// translate civitai model page URLs and Hugging Face blob pages
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		if pu, err := neturl.Parse(u); err == nil {
			h := strings.ToLower(pu.Hostname())
			if hostIs(h, "civitai.com") && strings.HasPrefix(pu.Path, "/models/") {
				parts := strings.Split(strings.Trim(pu.Path, "/"), "/")
				if len(parts) >= 2 {
					modelID := parts[1]
					q := pu.Query()
					ver := q.Get("modelVersionId"); if ver == "" { ver = q.Get("version") }
					civ := "civitai://model/" + modelID
					if strings.TrimSpace(ver) != "" { civ += "?version=" + neturl.QueryEscape(ver) }
					if res, err := resolver.Resolve(ctx, civ, m.cfg); err == nil {
						name := res.SuggestedFilename
						if strings.TrimSpace(name) == "" { name = filepath.Base(res.URL) }
						name = utilSafe(name)
						return filepath.Join(m.cfg.General.DownloadRoot, name)
					}
				}
			}
			if hostIs(h, "huggingface.co") {
				parts := strings.Split(strings.Trim(pu.Path, "/"), "/")
				if len(parts) >= 5 && parts[2] == "blob" {
					owner := parts[0]
					repo := parts[1]
					rev := parts[3]
					filePath := strings.Join(parts[4:], "/")
					hf := "hf://" + owner + "/" + repo + "/" + filePath + "?rev=" + rev
					if res, err := resolver.Resolve(ctx, hf, m.cfg); err == nil {
						name := filepath.Base(res.URL)
						name = utilSafe(name)
						return filepath.Join(m.cfg.General.DownloadRoot, name)
					}
				}
			}
		}
	}
	base := filepath.Base(u)
	if base == "" || base == "/" || base == "." { base = "download" }
	return filepath.Join(m.cfg.General.DownloadRoot, utilSafe(base))
}

func (m *Model) importBatchFile(path string) tea.Cmd {
	f, err := os.Open(path)
	if err != nil { m.addToast("batch open failed: "+err.Error()); return nil }
	defer f.Close()
	sc := bufio.NewScanner(f)
	count := 0
	cmds := make([]tea.Cmd, 0, 64)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") { continue }
		parts := strings.Fields(line)
		u := parts[0]
		dest := ""
		typeOv := ""
		place := false
		for _, tok := range parts[1:] {
			if i := strings.IndexByte(tok, '='); i > 0 {
				k := strings.ToLower(strings.TrimSpace(tok[:i]))
				v := strings.TrimSpace(tok[i+1:])
				switch k {
				case "dest": dest = v
				case "type": typeOv = v
				case "place": v2 := strings.ToLower(v); place = v2=="y"||v2=="yes"||v2=="true"||v2=="1"
				}
			}
		}
		if dest == "" { dest = m.computeDefaultDest(u) }
		cmds = append(cmds, m.startDownloadCmd(u, dest))
		key := u+"|"+dest
		if strings.TrimSpace(typeOv) != "" { m.placeType[key] = typeOv }
		if place { m.autoPlace[key] = true }
		count++
	}
	if err := sc.Err(); err != nil { m.addToast("batch read error: "+err.Error()) }
	m.addToast(fmt.Sprintf("batch: started %d", count))
	return tea.Batch(cmds...)
}

// utilSafe mirrors util.SafeFileName without import cycle here
func utilSafe(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	if name == "" { return "download" }
	return name
}

func (m *Model) startDownloadCmd(urlStr, dest string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.running[urlStr+"|"+dest] = cancel
	return m.downloadOrHoldCmd(ctx, urlStr, dest, true)
}

// probeSelectedCmd: probe reachability for rows without starting
func (m *Model) probeSelectedCmd(rows []state.DownloadRow) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(rows))
	for _, r := range rows {
		row := r
		cmds = append(cmds, func() tea.Msg {
			ctx := context.Background()
			headers := map[string]string{}
			if u, err := neturl.Parse(row.URL); err == nil {
				h := strings.ToLower(u.Hostname())
				if hostIs(h, "huggingface.co") && m.cfg.Sources.HuggingFace.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv); if env == "" { env = "HF_TOKEN" }
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" { headers["Authorization"] = "Bearer "+tok }
				}
				if hostIs(h, "civitai.com") && m.cfg.Sources.CivitAI.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv); if env == "" { env = "CIVITAI_TOKEN" }
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" { headers["Authorization"] = "Bearer "+tok }
				}
			}
			reach, info := downloader.CheckReachable(ctx, m.cfg, row.URL, headers)
			if !reach { _ = m.st.UpsertDownload(state.DownloadRow{URL: row.URL, Dest: row.Dest, Status: "hold"}) }
			return probeMsg{url: row.URL, dest: row.Dest, reachable: reach, info: info}
		})
	}
	return tea.Batch(cmds...)
}

func (m *Model) startDownloadCmdCtx(ctx context.Context, urlStr, dest string) tea.Cmd {
	return m.downloadOrHoldCmd(ctx, urlStr, dest, true)
}

// downloadOrHoldCmd optionally starts download; when start=false it only probes and updates hold status
func (m *Model) downloadOrHoldCmd(ctx context.Context, urlStr, dest string, start bool) tea.Cmd {
	return func() tea.Msg {
		resolved := urlStr
		headers := map[string]string{}
		normToast := ""
		// Translate civitai model pages and Hugging Face blob pages into resolver URIs for proper resolution
		if strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://") {
			if u, err := neturl.Parse(resolved); err == nil {
				h := strings.ToLower(u.Hostname())
				// CivitAI model page -> civitai://
				if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
					parts := strings.Split(strings.Trim(u.Path, "/"), "/")
					if len(parts) >= 2 {
						modelID := parts[1]
						q := u.Query()
						ver := q.Get("modelVersionId"); if ver == "" { ver = q.Get("version") }
						civ := "civitai://model/" + modelID; if strings.TrimSpace(ver) != "" { civ += "?version=" + neturl.QueryEscape(ver) }
						normToast = "normalized CivitAI page → " + civ
						if res, err := resolver.Resolve(ctx, civ, m.cfg); err == nil { resolved = res.URL; headers = res.Headers; m.addToast(normToast) }
					}
				}
				// Hugging Face blob page -> hf://owner/repo/path?rev=...
				if hostIs(h, "huggingface.co") {
					parts := strings.Split(strings.Trim(u.Path, "/"), "/")
					if len(parts) >= 5 && parts[2] == "blob" {
						owner := parts[0]
						repo := parts[1]
						rev := parts[3]
						filePath := strings.Join(parts[4:], "/")
						hf := "hf://" + owner + "/" + repo + "/" + filePath + "?rev=" + rev
						normToast = "normalized HF blob → " + hf
						if res, err := resolver.Resolve(ctx, hf, m.cfg); err == nil { resolved = res.URL; headers = res.Headers; m.addToast(normToast) }
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
				if hostIs(h, "huggingface.co") && m.cfg.Sources.HuggingFace.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv); if env == "" { env = "HF_TOKEN" }
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" { headers["Authorization"] = "Bearer "+tok }
				}
				if hostIs(h, "civitai.com") && m.cfg.Sources.CivitAI.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv); if env == "" { env = "CIVITAI_TOKEN" }
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" { headers["Authorization"] = "Bearer "+tok }
				}
			}
		}
		if start {
			// Quick reachability probe; if network-unreachable, put job on hold and do not start download
			reach, info := downloader.CheckReachable(ctx, m.cfg, resolved, headers)
			if !reach {
				_ = m.st.UpsertDownload(state.DownloadRow{URL: resolved, Dest: dest, Status: "hold"})
				// mirror autoplace/type mappings to resolved key so future retries work
				origKey := urlStr+"|"+dest
				resKey := resolved+"|"+dest
				if m.autoPlace[origKey] { m.autoPlace[resKey] = true }
				if t := m.placeType[origKey]; t != "" { m.placeType[resKey] = t }
				m.addToast("probe failed: unreachable; job on hold ("+info+")")
				return dlDoneMsg{url: resolved, dest: dest, path: "", err: fmt.Errorf("hold: unreachable")}
			}
			log := logging.New("error", false)
			dl := downloader.NewAuto(m.cfg, log, m.st, nil)
			final, _, err := dl.Download(ctx, resolved, dest, "", headers, m.cfg.General.AlwaysNoResume)
			return dlDoneMsg{url: urlStr, dest: dest, path: final, err: err}
		}
		// Probe-only path
		reach, info := downloader.CheckReachable(ctx, m.cfg, resolved, headers)
		if !reach { _ = m.st.UpsertDownload(state.DownloadRow{URL: resolved, Dest: dest, Status: "hold"}) }
		return probeMsg{url: resolved, dest: dest, reachable: reach, info: info}
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
