package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/downloader"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/placer"
	"github.com/jxwalker/modfetch/internal/resolver"
	"github.com/jxwalker/modfetch/internal/state"
	"github.com/jxwalker/modfetch/internal/tui"
	"github.com/jxwalker/modfetch/internal/util"
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

type dlDoneMsg struct {
	url, dest, path string
	err             error
}

type recoverRowsMsg struct{ rows []state.DownloadRow }

type destSuggestMsg struct {
	url  string
	dest string
}

type metaMsg struct {
	url           string
	fileName      string
	suggested     string
	civType       string
	quants        []resolver.Quantization // Available quantizations (HuggingFace)
	selectedQuant string                  // Pre-selected quantization if any
}

type probeMsg struct {
	url       string
	dest      string
	reachable bool
	info      string
}

type obs struct {
	bytes int64
	t     time.Time
}

type toast struct {
	msg  string
	when time.Time
	ttl  time.Duration
}

type Model struct {
	cfg             *config.Config
	st              *state.DB
	th              Theme
	w, h            int
	build           string
	activeTab       int // 0: Pending, 1: Active, 2: Completed, 3: Failed
	rows            []state.DownloadRow
	selected        int
	filterOn        bool
	filterInput     textinput.Model
	sortMode        string // ""|"speed"|"eta"|"rem"
	groupBy         string // ""|"host"
	lastRefresh     time.Time
	prog            progress.Model
	prev            map[string]obs
	prevStatus      map[string]string
	running         map[string]context.CancelFunc
	selectedKeys    map[string]bool
	toasts          []toast
	showToastDrawer bool
	showHelp        bool
	showInspector   bool
	// New download modal state
	newJob           bool
	newStep          int
	newInput         textinput.Model
	newURL           string
	newDest          string //nolint:unused
	newType          string
	newAutoPlace     bool
	newAutoSuggested string // latest auto-suggested dest (to avoid overriding manual edits)
	newTypeDetected  string // detected artifact type (from civitai or heuristics)
	newTypeSource    string // e.g., "civitai", "heuristic"
	newDetectedName  string // suggested filename/name from resolver (for display)
	// Extension suggestion when filename lacks suffix (e.g., .safetensors, .gguf)
	newSuggestExt string
	// Quantization selection state (HuggingFace)
	newAvailableQuants []resolver.Quantization // Available quantization variants
	newSelectedQuant   int                     // Index into newAvailableQuants
	newQuantsDetected  bool                    // Whether quantizations were detected
	newQuantSelecting  bool                    // Currently in quantization selection mode
	// Batch import modal state
	batchMode  bool
	batchInput textinput.Model
	// Overrides for auto-place by key url|dest
	placeType map[string]string
	autoPlace map[string]bool
	// Normalization note (shown after URL entry)
	newNormNote string
	// Types derived from config placement mapping (for hints)
	configTypes []string

	themeIndex int
	columnMode string // dest|url|host
	tickEvery  time.Duration
	// caches for performance
	rateCache     map[string]float64
	etaCache      map[string]string
	totalCache    map[string]int64
	curBytesCache map[string]int64
	// rate history for sparkline
	rateHist map[string][]float64
	// cache for hostnames
	hostCache map[string]string
	// transient retrying indicator per key
	retrying map[string]time.Time
	err      error
	// token/env status
	hfTokenSet  bool
	civTokenSet bool
	hfRejected  bool
	civRejected bool
	// rate limit flags
	hfRateLimited  bool
	civRateLimited bool
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
	fi := textinput.New()
	fi.Placeholder = "filter (url or dest contains)"
	fi.CharLimit = 4096
	// compute refresh
	refresh := time.Second
	if hz := cfg.UI.RefreshHz; hz > 0 {
		if hz > 10 {
			hz = 10
		}
		refresh = time.Second / time.Duration(hz)
	}
	// decide initial column mode
	mode := strings.ToLower(strings.TrimSpace(cfg.UI.ColumnMode))
	if mode != "dest" && mode != "url" && mode != "host" {
		if cfg.UI.ShowURL {
			mode = "url"
		} else {
			mode = "dest"
		}
	}
	m := &Model{
		cfg: cfg, st: st, th: defaultTheme(), activeTab: -1, prog: p, prev: map[string]obs{}, prevStatus: map[string]string{},
		build:   strings.TrimSpace(version),
		running: map[string]context.CancelFunc{}, selectedKeys: map[string]bool{}, filterInput: fi,
		rateCache: map[string]float64{}, etaCache: map[string]string{}, totalCache: map[string]int64{}, curBytesCache: map[string]int64{}, rateHist: map[string][]float64{}, hostCache: map[string]string{},
		retrying:   map[string]time.Time{},
		tickEvery:  refresh,
		columnMode: mode,
		placeType:  map[string]string{}, autoPlace: map[string]bool{},
	}
	// Apply config.theme if provided (initial preset); UI state may override
	if name := strings.TrimSpace(cfg.UI.Theme); name != "" {
		if idx := themeIndexByName(name); idx >= 0 {
			presets := themePresets()
			m.themeIndex = idx
			m.th = presets[idx]
		}
	}
	m.configTypes = computeTypesFromConfig(cfg)
	// Load UI state overrides if present
	m.loadUIState()
	return m
}

func (m *Model) Init() tea.Cmd {
	if m.cfg != nil && m.cfg.General.AutoRecoverOnStart {
		return tea.Batch(
			tea.Tick(m.tickEvery, func(t time.Time) tea.Msg { return tickMsg(t) }),
			m.recoverCmd(),
		)
	}
	return tea.Tick(m.tickEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.newJob {
			return m.updateNewJob(msg)
		}
		if m.batchMode {
			return m.updateBatchMode(msg)
		}
		if m.filterOn {
			return m.updateFilter(msg)
		}
		return m.updateNormal(msg)
	case tickMsg:
		return m, m.refresh()
	case recoverRowsMsg:
		// Start downloads for rows that appear to have been running previously
		rows := msg.rows
		if len(rows) == 0 {
			return m, nil
		}
		cmds := make([]tea.Cmd, 0, len(rows))
		now := time.Now()
		for _, r := range rows {
			key := keyFor(r)
			if _, ok := m.running[key]; ok {
				continue
			}
			ctx, cancel := context.WithCancel(context.Background())
			m.running[key] = cancel
			m.retrying[key] = now
			cmds = append(cmds, m.startDownloadCmdCtx(ctx, r.URL, r.Dest))
		}
		m.addToast(fmt.Sprintf("auto-recover: resumed %d", len(cmds)))
		return m, tea.Batch(cmds...)
	case destSuggestMsg:
		// Update the suggested dest only if the user hasn't edited it since our last suggestion
		if m.newJob && m.newStep == 4 && strings.TrimSpace(m.newURL) == strings.TrimSpace(msg.url) {
			if strings.TrimSpace(m.newInput.Value()) == strings.TrimSpace(m.newAutoSuggested) || strings.TrimSpace(m.newInput.Value()) == "" {
				m.newInput.SetValue(msg.dest)
				m.newAutoSuggested = msg.dest
				// Recompute extension suggestion if missing extension
				if filepath.Ext(msg.dest) == "" {
					m.newSuggestExt = m.inferExt()
				} else {
					m.newSuggestExt = ""
				}
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

			// Store quantization data if detected
			if len(msg.quants) > 0 {
				m.newAvailableQuants = msg.quants
				m.newQuantsDetected = true
				// Find index of selected quantization
				for i, q := range msg.quants {
					if q.Name == msg.selectedQuant {
						m.newSelectedQuant = i
						break
					}
				}
				// Enter quantization selection mode if we're at step 2
				if m.newStep == 2 {
					m.newQuantSelecting = true
				}
			} else {
				m.newAvailableQuants = nil
				m.newQuantsDetected = false
				m.newSelectedQuant = 0
				m.newQuantSelecting = false
			}

			// Pre-fill type field if no quantization selection needed
			if m.newStep == 2 && !m.newQuantSelecting {
				cur := strings.TrimSpace(m.newInput.Value())
				if cur == "" {
					m.newInput.SetValue(t)
				}
			}
		}
		return m, nil
	case probeMsg:
		// Show toast for probe result; refresh table to reflect any hold updates
		host := hostOf(msg.url)
		if msg.reachable {
			m.addToast("probe ok: " + host + " (" + msg.info + ")")
		} else {
			m.addToast("probe failed: unreachable; on hold (" + msg.info + ")")
		}
		return m, m.refresh()
	case dlDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.addToast("failed: " + msg.err.Error())
			// Mark token rejection/rate limit hints on known hosts based on error text
			errTxt := strings.ToLower(m.err.Error())
			if strings.Contains(errTxt, "unauthorized") || strings.Contains(errTxt, "401") || strings.Contains(errTxt, "forbidden") || strings.Contains(errTxt, "403") || strings.Contains(errTxt, "not authorized") {
				if strings.Contains(errTxt, "hugging face") || strings.Contains(errTxt, "huggingface") {
					m.hfRejected = true
				}
				if strings.Contains(errTxt, "civitai") {
					m.civRejected = true
				}
			}
			// Rate limited detection
			if strings.Contains(errTxt, "429") || strings.Contains(errTxt, "rate limited") || strings.Contains(errTxt, "too many requests") {
				u := strings.ToLower(msg.url)
				if strings.Contains(u, "huggingface") {
					m.hfRateLimited = true
					m.addToast("rate limited by huggingface.co; jobs on hold if retried")
				}
				if strings.Contains(u, "civitai") {
					m.civRateLimited = true
					m.addToast("rate limited by civitai.com; jobs may be on hold; try later")
				}
			}
			return m, m.refresh()
		}
		// On success, clear any prior rejection for the host
		u := strings.ToLower(msg.url)
		if strings.HasPrefix(u, "hf://") || strings.Contains(u, "huggingface") {
			m.hfRejected = false
			m.hfRateLimited = false
		}
		if strings.HasPrefix(u, "civitai://") || strings.Contains(u, "civitai") {
			m.civRejected = false
			m.civRateLimited = false
		}
		// Auto place if configured
		key := msg.url + "|" + msg.dest
		if m.autoPlace[key] {
			atype := m.placeType[key]
			if atype == "" {
				atype = ""
			}
			placed, err := placer.Place(m.cfg, msg.path, atype, "")
			if err != nil {
				m.addToast("place failed: " + err.Error())
			} else if len(placed) > 0 {
				m.addToast("placed: " + truncateMiddle(placed[0], 40))
			}
			delete(m.autoPlace, key)
			delete(m.placeType, key)
		}
		return m, m.refresh()
	}
	return m, nil
}

func (m *Model) updateNewJob(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	// Handle quantization selection mode separately
	if m.newQuantSelecting {
		switch s {
		case "esc":
			// Cancel quantization selection, go back to type selection
			m.newQuantSelecting = false
			return m, nil
		case "j", "down":
			// Navigate down in quantization list
			if m.newSelectedQuant < len(m.newAvailableQuants)-1 {
				m.newSelectedQuant++
			}
			return m, nil
		case "k", "up":
			// Navigate up in quantization list
			if m.newSelectedQuant > 0 {
				m.newSelectedQuant--
			}
			return m, nil
		case "enter", "ctrl+j":
			// Confirm quantization selection and update URL with selected file
			if m.newSelectedQuant >= 0 && m.newSelectedQuant < len(m.newAvailableQuants) {
				selected := m.newAvailableQuants[m.newSelectedQuant]

				// Validate and sanitize FilePath
				filePath := strings.TrimSpace(selected.FilePath)
				filePath = strings.TrimPrefix(filePath, "/")
				if filePath == "" {
					m.addToast("error: invalid quantization file path")
					return m, nil
				}

				// Update URL to point to the selected file
				// Parse the current hf:// URL and update the path
				if strings.HasPrefix(m.newURL, "hf://") {
					// Format: hf://owner/repo[/path]?rev=...&quant=...
					// We need to update it to hf://owner/repo/selected.FilePath?rev=...
					// Note: We strip out quant= parameter since we're specifying the exact file
					parts := strings.Split(strings.TrimPrefix(m.newURL, "hf://"), "?")
					pathPart := parts[0]

					// Preserve only rev parameter, drop quant parameter
					revParam := ""
					if len(parts) > 1 {
						if q, err := neturl.ParseQuery(parts[1]); err == nil {
							if rev := q.Get("rev"); rev != "" {
								revParam = "?rev=" + neturl.QueryEscape(rev)
							}
						}
					}

					// Split path into owner/repo[/oldpath]
					pathSegments := strings.Split(pathPart, "/")
					if len(pathSegments) >= 2 {
						owner := pathSegments[0]
						repo := pathSegments[1]
						// Rebuild with selected file path (no quant param needed)
						m.newURL = "hf://" + owner + "/" + repo + "/" + filePath + revParam
					}
				}
			}

			m.newQuantSelecting = false
			m.newInput.SetValue("")
			m.newInput.Placeholder = "Artifact type (optional, e.g. sd.checkpoint)"
			m.newStep = 2
			return m, nil
		}
		return m, nil
	}

	switch s {
	case "esc":
		m.newJob = false
		return m, nil
	case "tab":
		if m.newStep == 4 {
			cur := strings.TrimSpace(m.newInput.Value())
			comp := m.completePath(cur)
			if strings.TrimSpace(comp) != "" {
				m.newInput.SetValue(comp)
				m.newAutoSuggested = comp
			}
			return m, nil
		}
		return m, nil
	case "x", "X":
		if m.newStep == 4 && strings.TrimSpace(m.newSuggestExt) != "" {
			cur := strings.TrimSpace(m.newInput.Value())
			if filepath.Ext(cur) == "" {
				m.newInput.SetValue(cur + m.newSuggestExt)
				m.newSuggestExt = ""
			}
		}
		return m, nil
	case "enter", "ctrl+j":
		val := strings.TrimSpace(m.newInput.Value())
		switch m.newStep {
		case 1:
			if val == "" {
				return m, nil
			}
			m.newURL = val
			if norm, ok := m.normalizeURLForNote(val); ok {
				m.newNormNote = norm
			} else {
				m.newNormNote = ""
			}
			m.newInput.SetValue("")
			m.newInput.Placeholder = "Artifact type (optional, e.g. sd.checkpoint)"
			m.newStep = 2
			return m, m.resolveMetaCmd(m.newURL)
		case 2:
			m.newType = val
			m.newInput.SetValue("")
			m.newInput.Placeholder = "Auto place after download? y/n (default n)"
			m.newStep = 3
			return m, nil
		case 3:
			v := strings.ToLower(strings.TrimSpace(val))
			m.newAutoPlace = v == "y" || v == "yes" || v == "true" || v == "1"
			var cand string
			if m.newAutoPlace {
				cand = m.computePlacementSuggestionImmediate(m.newURL, m.newType)
				m.newAutoSuggested = cand
				m.newInput.SetValue(cand)
				m.newInput.Placeholder = "Destination path (Enter to accept)"
				m.newStep = 4
				if filepath.Ext(cand) == "" {
					m.newSuggestExt = m.inferExt()
				} else {
					m.newSuggestExt = ""
				}
				return m, m.suggestPlacementCmd(m.newURL, m.newType)
			}
			cand = m.immediateDestCandidate(m.newURL)
			m.newAutoSuggested = cand
			m.newInput.SetValue(cand)
			m.newInput.Placeholder = "Destination path (Enter to accept)"
			m.newStep = 4
			if filepath.Ext(cand) == "" {
				m.newSuggestExt = m.inferExt()
			} else {
				m.newSuggestExt = ""
			}
			return m, m.suggestDestCmd(m.newURL)
		case 4:
			urlStr := m.newURL
			dest := strings.TrimSpace(val)
			if dest == "" {
				if m.newAutoPlace {
					dest = m.computePlacementSuggestionImmediate(urlStr, m.newType)
				} else {
					dest = m.computeDefaultDest(urlStr)
				}
			}
			if err := m.preflightDest(dest); err != nil {
				m.addToast("dest not writable: " + err.Error())
				return m, nil
			}
			_ = m.st.UpsertDownload(state.DownloadRow{URL: urlStr, Dest: dest, Status: "pending"})
			cmd := m.startDownloadCmd(urlStr, dest)
			key := urlStr + "|" + dest
			if strings.TrimSpace(m.newType) != "" {
				m.placeType[key] = m.newType
			}
			if m.newAutoPlace {
				m.autoPlace[key] = true
			}
			m.addToast("started: " + truncateMiddle(dest, 40))
			m.newJob = false
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.newInput, cmd = m.newInput.Update(msg)
	return m, cmd
}

func (m *Model) updateBatchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "esc":
		m.batchMode = false
		return m, nil
	case "enter", "ctrl+j":
		path := strings.TrimSpace(m.batchInput.Value())
		var cmd tea.Cmd
		if path != "" {
			cmd = m.importBatchFile(path)
		}
		m.batchMode = false
		return m, cmd
	}
	var cmd tea.Cmd
	m.batchInput, cmd = m.batchInput.Update(msg)
	return m, cmd
}

func (m *Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "enter", "ctrl+j":
		m.filterOn = false
		m.filterInput.Blur()
		return m, nil
	case "esc":
		m.filterOn = false
		m.filterInput.SetValue("")
		m.filterInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

func (m *Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "q", "ctrl+c":
		m.saveUIState()
		return m, tea.Quit
	case "/":
		m.filterOn = true
		m.filterInput.Focus()
		return m, nil
	case "n":
		m.newJob = true
		m.newStep = 1
		m.newInput = textinput.New()
		m.newInput.Placeholder = "Enter URL or resolver URI"
		m.newInput.Focus()
		return m, nil
	case "b":
		m.batchMode = true
		m.batchInput = textinput.New()
		m.batchInput.Placeholder = "Path to text file with URLs"
		m.batchInput.Focus()
		return m, nil
	case "s":
		m.sortMode = "speed"
		return m, nil
	case "e":
		m.sortMode = "eta"
		return m, nil
	case "o":
		m.sortMode = ""
		return m, nil
	case "R":
		m.sortMode = "rem"
		return m, nil
	case "g":
		if m.groupBy == "host" {
			m.groupBy = ""
		} else {
			m.groupBy = "host"
		}
		return m, nil
	case "t":
		switch m.columnMode {
		case "dest":
			m.columnMode = "url"
		case "url":
			m.columnMode = "host"
		default:
			m.columnMode = "dest"
		}
		m.saveUIState()
		return m, nil
	case "v":
		m.compactToggle()
		m.saveUIState()
		return m, nil
	case "T":
		presets := themePresets()
		m.themeIndex = (m.themeIndex + 1) % len(presets)
		m.th = presets[m.themeIndex]
		m.saveUIState()
		return m, nil
	case "H":
		m.showToastDrawer = !m.showToastDrawer
		return m, nil
	case "P":
		rows := m.selectionOrCurrent()
		if len(rows) == 0 {
			return m, nil
		}
		return m, m.probeSelectedCmd(rows)
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	case "i":
		m.showInspector = !m.showInspector
		return m, nil
	case "0":
		m.activeTab = -1
		m.selected = 0
		return m, nil
	case "1":
		m.activeTab = 0
		m.selected = 0
		return m, nil
	case "2":
		m.activeTab = 1
		m.selected = 0
		return m, nil
	case "3":
		m.activeTab = 2
		m.selected = 0
		return m, nil
	case "4":
		m.activeTab = 3
		m.selected = 0
		return m, nil
	case "j", "down":
		if m.selected < len(m.visibleRows())-1 {
			m.selected++
		}
		return m, nil
	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case " ":
		rows := m.visibleRows()
		if m.selected >= 0 && m.selected < len(rows) {
			key := keyFor(rows[m.selected])
			if m.selectedKeys[key] {
				delete(m.selectedKeys, key)
			} else {
				m.selectedKeys[key] = true
			}
		}
		return m, nil
	case "A":
		for _, r := range m.visibleRows() {
			m.selectedKeys[keyFor(r)] = true
		}
		return m, nil
	case "X":
		m.selectedKeys = map[string]bool{}
		return m, nil
	case "r":
		fallthrough
	case "y":
		targets := m.selectionOrCurrent()
		if len(targets) == 0 {
			return m, nil
		}
		cmds := make([]tea.Cmd, 0, len(targets))
		now := time.Now()
		for _, r := range targets {
			ctx, cancel := context.WithCancel(context.Background())
			m.running[keyFor(r)] = cancel
			m.retrying[keyFor(r)] = now
			cmds = append(cmds, m.startDownloadCmdCtx(ctx, r.URL, r.Dest))
		}
		m.addToast(fmt.Sprintf("retrying %d item(s)…", len(targets)))
		return m, tea.Batch(cmds...)
	case "p":
		cnt := 0
		for _, r := range m.selectionOrCurrent() {
			key := keyFor(r)
			if cancel, ok := m.running[key]; ok {
				cancel()
				delete(m.running, key)
				cnt++
			}
		}
		if cnt > 0 {
			m.addToast(fmt.Sprintf("cancelled %d", cnt))
		}
		return m, nil
	case "O":
		rows := m.selectionOrCurrent()
		if len(rows) > 0 {
			_ = openInFileManager(rows[0].Dest, true)
		}
		return m, nil
	case "C":
		rows := m.selectionOrCurrent()
		if len(rows) > 0 {
			_ = copyToClipboard(rows[0].Dest)
		}
		return m, nil
	case "U":
		rows := m.selectionOrCurrent()
		if len(rows) > 0 {
			_ = copyToClipboard(rows[0].URL)
		}
		return m, nil
	case "D":
		rows := m.selectionOrCurrent()
		if len(rows) == 0 {
			return m, nil
		}
		deleted := 0
		for _, r := range rows {
			key := keyFor(r)
			if cancel, ok := m.running[key]; ok {
				cancel()
				delete(m.running, key)
			}
			_ = m.st.DeleteChunks(r.URL, r.Dest)
			if err := m.st.DeleteDownload(r.URL, r.Dest); err == nil {
				deleted++
			}
			delete(m.selectedKeys, key)
		}
		if deleted > 0 {
			m.addToast(fmt.Sprintf("deleted %d", deleted))
		}
		return m, m.refresh()
	}
	return m, nil
}

func (m *Model) View() string {
	if m.w == 0 {
		m.w = 120
	}
	if m.h == 0 {
		m.h = 30
	}
	// Title bar + inline toasts
	ver := m.build
	if ver == "" {
		ver = "dev"
	}
	title := m.th.title.Render(fmt.Sprintf("modfetch • TUI v2 (build %s)", ver))
	toastStr := m.renderToasts()
	titleBar := m.th.border.Width(m.w - 2).Render(lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", toastStr))

	// Horizontal tabs across full width
	tabs := m.th.border.Width(m.w - 2).Render(m.renderTabs())

	// Commands bar for quick reference
	commands := m.th.border.Width(m.w - 2).Render(m.renderCommandsBar())

	// Full-width table
	table := m.th.border.Width(m.w - 2).Render(m.renderTable())

	// Always-on inspector below the table showing highlighted job details
	inspector := m.th.border.Width(m.w - 2).Render(m.renderInspector())

	// Optional overlays
	drawer := ""
	if m.showToastDrawer {
		drawer = m.th.border.Width(m.w - 2).Render(m.renderToastDrawer())
	}
	help := ""
	if m.showHelp {
		help = m.th.border.Width(m.w - 2).Render(m.renderHelp())
	}
	modal := ""
	if m.newJob {
		// Show quantization selection UI if in that mode
		if m.newQuantSelecting {
			modal = m.th.border.Width(m.w - 4).Render(m.renderQuantizationSelection())
		} else {
			modal = m.th.border.Width(m.w - 4).Render(m.renderNewJobModal())
		}
	}
	if m.batchMode {
		modal = m.th.border.Width(m.w - 4).Render(m.renderBatchModal())
	}

	// Minimal footer: show filter input only when active
	filterBar := ""
	if m.filterOn {
		filterBar = "Filter: " + m.filterInput.View()
	}
	footer := ""
	if strings.TrimSpace(filterBar) != "" {
		footer = m.th.border.Width(m.w - 2).Render(m.th.footer.Render(filterBar))
	}

	parts := []string{titleBar, tabs, commands, table, inspector}
	if help != "" {
		parts = append(parts, help)
	}
	if drawer != "" {
		parts = append(parts, drawer)
	}
	if modal != "" {
		parts = append(parts, modal)
	}
	if footer != "" {
		parts = append(parts, footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *Model) recoverCmd() tea.Cmd {
	return func() tea.Msg {
		rows, err := m.st.ListDownloads()
		if err != nil {
			return recoverRowsMsg{rows: nil}
		}
		var todo []state.DownloadRow
		for _, r := range rows {
			st := strings.ToLower(strings.TrimSpace(r.Status))
			if st == "running" || st == "hold" {
				// Skip completed
				todo = append(todo, r)
			}
		}
		return recoverRowsMsg{rows: todo}
	}
}

func (m *Model) refresh() tea.Cmd {
	// Update token env presence status on each refresh tick
	m.updateTokenEnvStatus()
	rows, err := m.st.ListDownloads()
	if err != nil {
		_ = logging.New("error", false)
	}
	m.rows = rows
	m.lastRefresh = time.Now()
	m.gcToasts()
	// determine visible keys to focus updates
	vis := map[string]struct{}{}
	vr := m.visibleRows()
	maxRows := m.maxRowsOnScreen()
	for i, r := range vr {
		if i >= maxRows {
			break
		}
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
		// detect status transitions for rate limit holds
		prev := m.prevStatus[key]
		curr := strings.ToLower(strings.TrimSpace(r.Status)) + "|" + strings.ToLower(strings.TrimSpace(r.LastError))
		if prev != curr {
			// toast when entering hold due to rate limit sleep
			if strings.HasPrefix(curr, "hold|") && strings.Contains(curr, "rate limited") || (strings.HasPrefix(curr, "hold|") && strings.Contains(curr, "waiting") && strings.Contains(curr, "retry-after")) {
				h := hostOf(r.URL)
				m.addToast("on hold due to rate limiting: " + h)
			}
			m.prevStatus[key] = curr
		}
		_, isVis := vis[key]
		status := strings.ToLower(r.Status)
		shouldUpdate := isVis && (status == "running" || status == "planning" || status == "pending")
		var cur, total int64
		if shouldUpdate {
			c2, t2 := m.computeCurAndTotal(r)
			cur, total = c2, t2
			m.totalCache[key] = total
			m.curBytesCache[key] = cur
			prev := m.prev[key]
			dt := time.Since(prev.t).Seconds()
			var rate float64
			if dt > 0 {
				rate = float64(cur-prev.bytes) / dt
			}
			m.prev[key] = obs{bytes: cur, t: time.Now()}
			m.rateCache[key] = rate
			if rate > 0 && total > 0 && cur < total {
				rem := float64(total-cur) / rate
				m.etaCache[key] = fmt.Sprintf("%ds", int(rem+0.5))
			} else {
				// Keep last ETA instead of blanking; initialize to '-' if missing
				if strings.TrimSpace(m.etaCache[key]) == "" {
					m.etaCache[key] = "-"
				}
			}
			m.addRateSample(key, rate)
		}
	}
	return tea.Tick(m.tickEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

//nolint:unused,deadcode
func (m *Model) renderStats() string {
	var pending, active, done, failed int
	var gRate float64
	for _, r := range m.rows {
		ls := strings.ToLower(r.Status)
		switch ls {
		case "pending", "planning":
			pending++
		case "running":
			active++
		case "complete":
			done++
		case "error", "checksum_mismatch", "verify_failed":
			failed++
		}
		rate := m.rateCache[keyFor(r)]
		gRate += rate
	}
	return fmt.Sprintf("Pending:%d Active:%d Completed:%d Failed:%d • Rate:%s/s", pending, active, done, failed, humanize.Bytes(uint64(gRate)))
}

// renderTopLeftHints shows compact key hints in a fixed-size box
//
//nolint:unused,deadcode
func (m *Model) renderTopLeftHints() string {
	lines := []string{
		m.th.head.Render("Hints"),
		"Tabs: 0 All • 1 Pending • 2 Active • 3 Completed • 4 Failed",
		"New: n new download • b batch import (txt file)",
		"Nav: j/k up/down",
		"Filter: / enter • Enter apply • Esc clear",
		"Sort: s speed • e ETA • R remaining • o clear | Group: g host",
		"Select: Space toggle • A all • X clear",
		"Actions: y/r start • p cancel • D delete | Open: O • Copy: C/U | Probe: P",
		"View: t column • v compact • i inspector • T theme • H toasts • ? help • q quit",
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderNewJobModal() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("New Download") + "\n")
	// Token status inline
	sb.WriteString(m.th.label.Render(m.renderAuthStatus()) + "\n\n")
	steps := []string{
		"1) URL/URI",
		"2) Artifact type (optional)",
		"3) Auto place after download (y/n)",
		"4) Destination path",
	}
	sb.WriteString(m.th.label.Render(strings.Join(steps, " • ")) + "\n\n")
	// Step-specific helper text
	switch m.newStep {
	case 1:
		sb.WriteString(m.th.label.Render("Enter an HTTP URL or a resolver URI (hf://owner/repo/path?rev=..., civitai://model/ID?version=...).") + "\n")
	case 2:
		// Show available types from config mapping
		if len(m.configTypes) > 0 {
			sb.WriteString(m.th.label.Render("Types from your config: "+strings.Join(m.configTypes, ", ")) + "\n")
		}
		sb.WriteString(m.th.label.Render("Type helps choose placement directories. Leave blank to skip placement or if unsure.") + "\n")
	case 3:
		sb.WriteString(m.th.label.Render("y: Place into mapped app directories after download • n: Save only to download_root.") + "\n")
	case 4:
		if m.newAutoPlace {
			cand := strings.TrimSpace(m.newAutoSuggested)
			if cand != "" {
				sb.WriteString(m.th.label.Render("Will place into: "+cand) + "\n")
			}
			sb.WriteString(m.th.label.Render("Edit the destination to override.") + "\n")
		} else {
			sb.WriteString(m.th.label.Render("Choose where to save the file. You can place later via 'place'.") + "\n")
		}
		// If we can infer an extension and current input lacks one, suggest adding via X
		curVal := strings.TrimSpace(m.newInput.Value())
		if strings.TrimSpace(m.newSuggestExt) != "" && filepath.Ext(curVal) == "" {
			sb.WriteString(m.th.label.Render("Suggestion: append " + m.newSuggestExt + " (press X)\n"))
		}
	}
	cur := ""
	if m.newStep == 1 {
		cur = "Enter URL or resolver URI"
	}
	if m.newStep == 2 {
		cur = "Artifact type (optional)"
	}
	if m.newStep == 3 {
		cur = "Auto place? y/n"
	}
	if m.newStep == 4 {
		cur = "Destination path (Tab to complete)"
	}
	sb.WriteString(m.th.label.Render(cur) + "\n")
	// Show normalization note once URL entered
	if m.newStep >= 2 && strings.TrimSpace(m.newNormNote) != "" {
		sb.WriteString(m.th.label.Render(m.newNormNote) + "\n")
	}
	// Show detected type/name if available when choosing type
	if m.newStep == 2 && strings.TrimSpace(m.newTypeDetected) != "" {
		from := m.newTypeSource
		if from == "" {
			from = "heuristic"
		}
		sb.WriteString(m.th.label.Render(fmt.Sprintf("Detected: %s (%s)", m.newTypeDetected, from)) + "\n")
	}
	sb.WriteString(m.newInput.View())
	return sb.String()
}

// renderQuantizationSelection renders the quantization selection UI
func (m *Model) renderQuantizationSelection() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("Select Quantization") + "\n\n")

	// Extract repo info from URL if available
	repoInfo := ""
	if strings.HasPrefix(m.newURL, "hf://") {
		parts := strings.Split(strings.TrimPrefix(m.newURL, "hf://"), "/")
		if len(parts) >= 2 {
			repoInfo = parts[0] + "/" + parts[1]
		}
	}
	if repoInfo != "" {
		sb.WriteString(m.th.label.Render("Repository: "+repoInfo) + "\n\n")
	}

	sb.WriteString(m.th.label.Render("Available quantizations (j/k to navigate, Enter to select):") + "\n\n")

	// Render each quantization option
	for i, q := range m.newAvailableQuants {
		prefix := "  "

		// Highlight selected item
		if i == m.newSelectedQuant {
			prefix = "▸ "
		}

		// Format size
		sizeStr := humanize.Bytes(uint64(q.Size))

		// Build line: "  Q4_K_M  (4.1 GB)  - gguf"
		line := fmt.Sprintf("%s%-10s  (%7s)  - %s", prefix, q.Name, sizeStr, q.FileType)

		if i == m.newSelectedQuant {
			sb.WriteString(m.th.ok.Render(line) + "\n")
		} else {
			sb.WriteString(m.th.label.Render(line) + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(m.th.label.Render("j/k: navigate  •  Enter: select  •  Esc: cancel") + "\n")

	return sb.String()
}

func (m *Model) renderBatchModal() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("Batch Import") + "\n")
	sb.WriteString(m.th.label.Render("Enter path to text file with one URL per line (tokens: dest=..., type=..., place=true, mode=copy|symlink|hardlink)") + "\n\n")
	sb.WriteString(m.batchInput.View())
	return sb.String()
}

// renderTopRightStats shows aggregate metrics and recent toasts summary
//
//nolint:unused,deadcode
func (m *Model) renderTopRightStats() string {
	var pending, active, done, failed int
	var gRate float64
	for _, r := range m.rows {
		ls := strings.ToLower(r.Status)
		switch ls {
		case "pending", "planning":
			pending++
		case "running":
			active++
		case "complete":
			done++
		case "error", "checksum_mismatch", "verify_failed":
			failed++
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
	}
	// View indicators
	lines = append(lines, fmt.Sprintf("View: Sort:%s • Group:%s • Column:%s • Theme:%s", m.sortModeLabel(), m.groupByLabel(), m.columnMode, themeNameByIndex(m.themeIndex)))
	// Auth status
	lines = append(lines, m.renderAuthStatus())
	return strings.Join(lines, "\n")
}

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
	}
	var sb strings.Builder
	for i, it := range labels {
		style := m.th.tabInactive
		if it.tab == m.activeTab {
			style = m.th.tabActive
		}
		count := 0
		if it.tab == -1 {
			count = len(m.rows)
		} else {
			count = len(m.filterRows(it.tab))
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s (%d)", it.name, count)))
		if i < len(labels)-1 {
			sb.WriteString("  •  ")
		}
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
		if tab == -1 {
			out = append(out, r)
			continue
		}
		ls := strings.ToLower(r.Status)
		switch tab {
		case 0:
			if ls == "pending" || ls == "planning" || ls == "hold" {
				out = append(out, r)
			}
		case 1:
			if ls == "running" {
				out = append(out, r)
			}
		case 2:
			if ls == "complete" {
				out = append(out, r)
			}
		case 3:
			if ls == "error" || ls == "checksum_mismatch" || ls == "verify_failed" {
				out = append(out, r)
			}
		}
	}
	return out
}

func (m *Model) applySearch(in []state.DownloadRow) []state.DownloadRow {
	q := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if q == "" {
		return in
	}
	var out []state.DownloadRow
	for _, r := range in {
		if strings.Contains(strings.ToLower(r.URL), q) || strings.Contains(strings.ToLower(r.Dest), q) {
			out = append(out, r)
		}
	}
	return out
}

func (m *Model) applySort(in []state.DownloadRow) []state.DownloadRow {
	if m.sortMode == "" {
		return in
	}
	out := make([]state.DownloadRow, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		ci, ti, ri, _ := m.progressFor(out[i])
		cj, tj, rj, _ := m.progressFor(out[j])
		etaI := etaSeconds(ci, ti, ri)
		etaJ := etaSeconds(cj, tj, rj)
		switch m.sortMode {
		case "speed":
			return ri > rj
		case "eta":
			if etaI == 0 && etaJ == 0 {
				return ri > rj
			}
			if etaI == 0 {
				return false
			}
			if etaJ == 0 {
				return true
			}
			return etaI < etaJ
		case "rem":
			// Sort by remaining bytes ascending (unknown totals last)
			remI := int64(1 << 62)
			remJ := int64(1 << 62)
			if ti > 0 {
				remI = ti - ci
			}
			if tj > 0 {
				remJ = tj - cj
			}
			if remI == remJ {
				// Tie-breaker by higher rate
				return ri > rj
			}
			return remI < remJ
		}
		return false
	})
	return out
}

func etaSeconds(cur, total int64, rate float64) float64 {
	if rate <= 0 || total <= 0 || cur >= total {
		return 0
	}
	return float64(total-cur) / rate
}

func (m *Model) renderTable() string {
	rows := m.visibleRows()
	var sb strings.Builder
	lastLabel := "DEST"
	switch m.columnMode {
	case "url":
		lastLabel = "URL"
	case "host":
		lastLabel = "HOST"
	}
	speedLabel := "SPEED"
	etaLabel := "ETA"
	if m.sortMode == "speed" {
		speedLabel = speedLabel + "*"
	}
	if m.sortMode == "eta" {
		etaLabel = etaLabel + "*"
	}
	if m.isCompact() {
		hdr := m.th.head.Render(fmt.Sprintf("%-1s %-8s %-3s  %-16s  %-4s  %-12s  %-8s  %-s", "S", "STATUS", "RT", "PROG", "PCT", "SRC", etaLabel, lastLabel))
		if m.sortMode == "rem" {
			hdr = hdr + "  [sort: remaining]"
		}
		sb.WriteString(hdr)
	} else {
		hdr := m.th.head.Render(fmt.Sprintf("%-1s %-8s %-3s  %-16s  %-4s  %-10s  %-10s  %-12s  %-8s  %-s", "S", "STATUS", "RT", "PROG", "PCT", speedLabel, "THR", "SRC", etaLabel, lastLabel))
		if m.sortMode == "rem" {
			hdr = hdr + "  [sort: remaining]"
		}
		sb.WriteString(hdr)
	}
	sb.WriteString("\n")
	maxRows := m.h - 10
	if maxRows < 3 {
		maxRows = len(rows)
	}
	var prevGroup string
	for i, r := range rows {
		if m.groupBy == "host" {
			host := hostOf(r.URL)
			if i == 0 || host != prevGroup {
				sb.WriteString(m.th.label.Render("// "+host) + "\n")
				prevGroup = host
			}
		}
		// Progress and pct
		prog := m.renderProgress(r)
		cur, total, rate, _ := m.progressFor(r)
		pct := "--%"
		if total > 0 {
			ratio := float64(cur) / float64(total)
			if ratio < 0 {
				ratio = 0
			}
			if ratio > 1 {
				ratio = 1
			}
			pct = fmt.Sprintf("%3.0f%%", ratio*100)
		}
		// For completed jobs, force 100%
		if strings.EqualFold(strings.TrimSpace(r.Status), "complete") {
			pct = "100%"
		}
		eta := m.etaCache[keyFor(r)]
		thr := m.renderSparkline(keyFor(r))
		sel := " "
		if m.selectedKeys[keyFor(r)] {
			sel = "*"
		}
		// status label with transient retrying overlay
		statusLabel := r.Status
		if strings.EqualFold(statusLabel, "hold") && strings.Contains(strings.ToLower(strings.TrimSpace(r.LastError)), "rate limited") {
			statusLabel = "hold(rl)"
		}
		if ts, ok := m.retrying[keyFor(r)]; ok {
			if time.Since(ts) < 4*time.Second {
				statusLabel = "retrying"
			} else {
				delete(m.retrying, keyFor(r))
			}
		}
		last := r.Dest
		switch m.columnMode {
		case "url":
			last = logging.SanitizeURL(r.URL)
		case "host":
			last = hostOf(r.URL)
		}
		src := hostOf(r.URL)
		src = truncateMiddle(src, 12)
		lw := m.lastColumnWidth(m.isCompact())
		last = truncateMiddle(last, lw)
		// Speed column value
		speedStr := humanize.Bytes(uint64(rate)) + "/s"
		if strings.EqualFold(strings.TrimSpace(r.Status), "complete") {
			// Show average speed for completed
			if r.Size > 0 && r.UpdatedAt > 0 && r.CreatedAt > 0 && r.UpdatedAt >= r.CreatedAt {
				dur := time.Duration(r.UpdatedAt-r.CreatedAt) * time.Second
				if dur > 0 {
					avg := float64(r.Size) / dur.Seconds()
					speedStr = humanize.Bytes(uint64(avg)) + "/s"
				}
			} else {
				speedStr = "-"
			}
		}
		var line string
		if m.isCompact() {
			line = fmt.Sprintf("%-1s %-8s %-3d  %-16s  %-4s  %-12s  %-8s  %s", sel, statusLabel, r.Retries, prog, pct, src, eta, last)
		} else {
			line = fmt.Sprintf("%-1s %-8s %-3d  %-16s  %-4s  %-10s  %-10s  %-12s  %-8s  %s", sel, statusLabel, r.Retries, prog, pct, speedStr, thr, src, eta, last)
		}
		if i == m.selected {
			line = m.th.rowSelected.Render(line)
		}
		sb.WriteString(line + "\n")
		if i+1 >= maxRows {
			break
		}
	}
	if len(rows) == 0 {
		sb.WriteString(m.th.label.Render("(no items)"))
	}
	return sb.String()
}

func (m *Model) renderInspector() string {
	rows := m.visibleRows()
	if m.selected < 0 || m.selected >= len(rows) {
		return m.th.label.Render("No selection")
	}
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
	// Retries and status (with transient retrying overlay)
	statusLabel := r.Status
	if ts, ok := m.retrying[keyFor(r)]; ok {
		if time.Since(ts) < 4*time.Second {
			statusLabel = "retrying"
		} else {
			delete(m.retrying, keyFor(r))
		}
	}
	sb.WriteString(fmt.Sprintf("%s %d\n", m.th.label.Render("Retries:"), r.Retries))
	sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Status:"), statusLabel))
	// Completed job duration and average speed
	if strings.EqualFold(strings.TrimSpace(r.Status), "complete") && r.CreatedAt > 0 && r.UpdatedAt >= r.CreatedAt {
		dur := time.Duration((r.UpdatedAt - r.CreatedAt)) * time.Second
		sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Duration:"), dur.String()))
		// Start/End wall times
		startAt := time.Unix(r.CreatedAt, 0).Local().Format("2006-01-02 15:04:05")
		endAt := time.Unix(r.UpdatedAt, 0).Local().Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Started:"), startAt))
		sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Finished:"), endAt))
		if r.Size > 0 && dur > 0 {
			avg := float64(r.Size) / dur.Seconds()
			sb.WriteString(fmt.Sprintf("%s %s/s\n", m.th.label.Render("Avg Speed:"), humanize.Bytes(uint64(avg))))
		}
	} else if strings.EqualFold(strings.TrimSpace(r.Status), "running") && r.CreatedAt > 0 {
		// Show start time for running jobs
		startAt := time.Unix(r.CreatedAt, 0).Local().Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Started:"), startAt))
	}
	// Verification details
	if strings.TrimSpace(r.ExpectedSHA256) != "" || strings.TrimSpace(r.ActualSHA256) != "" {
		sb.WriteString(m.th.label.Render("SHA256:"))
		sb.WriteString("\n")
		if strings.TrimSpace(r.ExpectedSHA256) != "" {
			sb.WriteString(fmt.Sprintf("expected: %s\n", r.ExpectedSHA256))
		}
		if strings.TrimSpace(r.ActualSHA256) != "" {
			sb.WriteString(fmt.Sprintf("actual:   %s\n", r.ActualSHA256))
		}
		if r.ExpectedSHA256 != "" && r.ActualSHA256 != "" {
			if strings.EqualFold(strings.TrimSpace(r.ExpectedSHA256), strings.TrimSpace(r.ActualSHA256)) {
				sb.WriteString(m.th.ok.Render("verified: OK") + "\n")
			} else {
				sb.WriteString(m.th.bad.Render("verified: MISMATCH") + "\n")
			}
		}
	}
	// Show reason for hold/error if available
	if strings.TrimSpace(r.LastError) != "" {
		sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Reason:"), r.LastError))
	}
	return sb.String()
}

func (m *Model) progressFor(r state.DownloadRow) (cur int64, total int64, rate float64, eta string) {
	key := keyFor(r)
	cur = m.curBytesCache[key]
	total = m.totalCache[key]
	// Smooth rate using recent positive samples; fallback to last instantaneous
	rate = m.smoothedRate(key)
	eta = m.etaCache[key]
	// Fallback if cache not populated yet
	if total == 0 && r.Size > 0 {
		total = r.Size
	}
	return
}

// smoothedRate returns a moving average of recent positive samples (up to 5),
// falling back to the last instantaneous rate if no positive samples exist.
func (m *Model) smoothedRate(key string) float64 {
	h := m.rateHist[key]
	if len(h) == 0 {
		return m.rateCache[key]
	}
	sum := 0.0
	count := 0
	for i := len(h) - 1; i >= 0 && count < 5; i-- {
		v := h[i]
		if v > 0 {
			sum += v
			count++
		}
	}
	if count > 0 {
		return sum / float64(count)
	}
	return m.rateCache[key]
}

func (m *Model) computeCurAndTotal(r state.DownloadRow) (cur int64, total int64) {
	total = r.Size
	chunks, _ := m.st.ListChunks(r.URL, r.Dest)
	if len(chunks) > 0 {
		for _, c := range chunks {
			if strings.EqualFold(c.Status, "complete") {
				cur += c.Size
			}
		}
		return cur, total
	}
	if r.Dest != "" {
		p := downloader.StagePartPath(m.cfg, r.URL, r.Dest)
		if st, err := os.Stat(p); err == nil {
			cur = st.Size()
		} else if st, err := os.Stat(r.Dest); err == nil {
			cur = st.Size()
		}
	}
	return cur, total
}

func (m *Model) renderProgress(r state.DownloadRow) string {
	// For completed jobs, render full progress
	if strings.EqualFold(strings.TrimSpace(r.Status), "complete") {
		return m.prog.ViewAs(1)
	}
	cur, total, _, _ := m.progressFor(r)
	if total <= 0 {
		return "--"
	}
	ratio := float64(cur) / float64(total)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return m.prog.ViewAs(ratio)
}

func (m *Model) addRateSample(key string, rate float64) {
	h := m.rateHist[key]
	h = append(h, rate)
	if len(h) > 10 {
		h = h[len(h)-10:]
	}
	m.rateHist[key] = h
}

func (m *Model) renderSparklineKey(key string) string {
	h := m.rateHist[key]
	if len(h) == 0 {
		return ""
	}
	// map rates to 8 levels; normalize by max
	max := 0.0
	for _, v := range h {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		return "──────────"
	}
	levels := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var sb strings.Builder
	// Oldest on the left, newest on the right (rightward growth)
	for _, v := range h {
		r := int((v/max)*float64(len(levels)-1) + 0.5)
		if r < 0 {
			r = 0
		}
		if r >= len(levels) {
			r = len(levels) - 1
		}
		sb.WriteRune(levels[r])
	}
	// pad to 10
	for sb.Len() < 10 {
		sb.WriteRune(' ')
	}
	return sb.String()
}

func (m *Model) renderSparkline(key string) string { return m.renderSparklineKey(key) }

// Preflight: ensure destination's parent is writable
func (m *Model) preflightDest(dest string) error {
	d := strings.TrimSpace(dest)
	if d == "" {
		return fmt.Errorf("empty dest")
	}
	dir := filepath.Dir(d)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return tryWrite(dir)
}

// Path completion for destination
func (m *Model) completePath(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return s
	}
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
	if err != nil {
		return s
	}
	matches := make([]string, 0, len(ents))
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			if e.IsDir() {
				name += "/"
			}
			matches = append(matches, name)
		}
	}
	if len(matches) == 0 {
		return s
	}
	// If single match, replace basename
	var base string
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
	if len(ss) == 0 {
		return ""
	}
	p := ss[0]
	for _, s := range ss[1:] {
		for !strings.HasPrefix(strings.ToLower(s), strings.ToLower(p)) {
			if len(p) == 0 {
				return ""
			}
			p = p[:len(p)-1]
		}
	}
	return p
}

func tryWrite(dir string) error {
	f, err := os.CreateTemp(dir, ".mf-wr-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}

// Immediate (non-blocking) dest candidate based on URL only
func (m *Model) immediateDestCandidate(urlStr string) string {
	s := strings.TrimSpace(urlStr)
	if s == "" {
		return filepath.Join(m.cfg.General.DownloadRoot, "download")
	}
	// If it's a resolver URI, guess based on path base
	if strings.HasPrefix(s, "hf://") || strings.HasPrefix(s, "civitai://") {
		if u, err := neturl.Parse(s); err == nil {
			base := filepath.Base(strings.Trim(u.Path, "/"))
			if base == "" {
				base = "download"
			}
			return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(base))
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
					if base != "" {
						return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(base))
					}
				}
				return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(filepath.Base(u.Path)))
			}
		}
		if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
			// Use model ID as placeholder
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 2 {
				id := parts[1]
				if id != "" {
					return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(id))
				}
			}
		}
		// Fallback to path base
		base := util.URLPathBase(s)
		if base == "" || base == "/" || base == "." {
			base = "download"
		}
		return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(base))
	}
	// raw fallback
	base := util.URLPathBase(s)
	if base == "" || base == "/" || base == "." {
		base = "download"
	}
	return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(base))
}

// Background suggestion using resolver (may perform network I/O)
func (m *Model) suggestDestCmd(urlStr string) tea.Cmd {
	return func() tea.Msg {
		d := m.computeDefaultDest(urlStr)
		// Improve default name for CivitAI direct download endpoints by probing filename via HEAD
		if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
			if u, err := neturl.Parse(urlStr); err == nil {
				h := strings.ToLower(u.Hostname())
				if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/api/download/") {
					if name := m.headFilename(urlStr); strings.TrimSpace(name) != "" {
						name = util.SafeFileName(name)
						d = filepath.Join(m.cfg.General.DownloadRoot, name)
					}
				}
			}
		}
		return destSuggestMsg{url: urlStr, dest: d}
	}
}

// headFilename performs a HEAD request to retrieve a filename from Content-Disposition.
func (m *Model) headFilename(u string) string {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return ""
	}
	// Add CivitAI token if configured
	if m.cfg != nil && m.cfg.Sources.CivitAI.Enabled {
		env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
		if env == "" {
			env = "CIVITAI_TOKEN"
		}
		if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	cd := resp.Header.Get("Content-Disposition")
	if cd == "" {
		return ""
	}
	lcd := strings.ToLower(cd)
	// RFC 5987 filename*
	if idx := strings.Index(lcd, "filename*="); idx >= 0 {
		v := cd[idx+len("filename*="):]
		// expect UTF-8''... optionally
		if strings.HasPrefix(strings.ToLower(v), "utf-8''") {
			v = v[len("utf-8''"):]
		}
		if semi := strings.IndexByte(v, ';'); semi >= 0 {
			v = v[:semi]
		}
		name, _ := neturl.QueryUnescape(strings.Trim(v, "\"' "))
		return name
	}
	// Simple filename=
	if idx := strings.Index(lcd, "filename="); idx >= 0 {
		v := cd[idx+len("filename="):]
		if semi := strings.IndexByte(v, ';'); semi >= 0 {
			v = v[:semi]
		}
		name := strings.Trim(v, "\"' ")
		return name
	}
	return ""
}

// Immediate placement suggestion using artifact type and quick filename
func (m *Model) computePlacementSuggestionImmediate(urlStr, atype string) string {
	atype = strings.TrimSpace(atype)
	if atype == "" {
		return m.immediateDestCandidate(urlStr)
	}
	dirs, err := placer.ComputeTargets(m.cfg, atype)
	if err != nil || len(dirs) == 0 {
		return m.immediateDestCandidate(urlStr)
	}
	base := filepath.Base(m.immediateDestCandidate(urlStr))
	if strings.TrimSpace(base) == "" {
		base = "download"
	}
	return filepath.Join(dirs[0], util.SafeFileName(base))
}

func (m *Model) inferExt() string {
	// Prefer explicit user-provided type
	t := strings.TrimSpace(m.newType)
	if t == "" {
		t = strings.TrimSpace(m.newTypeDetected)
	}
	switch strings.ToLower(t) {
	case "sd.checkpoint", "sd.lora", "sd.vae", "sd.controlnet":
		return ".safetensors"
	case "llm.gguf":
		return ".gguf"
	case "sd.embedding":
		// ambiguous, skip to avoid wrong ext
		return ""
	}
	return ""
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
	if strings.HasSuffix(name, ".gguf") {
		return "llm.gguf"
	}
	if strings.HasSuffix(name, ".safetensors") {
		return "sd.checkpoint"
	}
	return "generic"
}

// Background placement suggestion: resolve filename then join with placement target
func (m *Model) suggestPlacementCmd(urlStr, atype string) tea.Cmd {
	return func() tea.Msg {
		atype = strings.TrimSpace(atype)
		if atype == "" {
			return destSuggestMsg{url: urlStr, dest: m.computeDefaultDest(urlStr)}
		}
		dirs, err := placer.ComputeTargets(m.cfg, atype)
		if err != nil || len(dirs) == 0 {
			return destSuggestMsg{url: urlStr, dest: m.computeDefaultDest(urlStr)}
		}
		// Derive a good filename via resolver-backed default dest
		cand := m.computeDefaultDest(urlStr)
		base := filepath.Base(cand)
		if strings.TrimSpace(base) == "" {
			base = "download"
		}
		d := filepath.Join(dirs[0], util.SafeFileName(base))
		return destSuggestMsg{url: urlStr, dest: d}
	}
}

// Resolve metadata (suggested filename, civitai file type, and quantizations) in background
func (m *Model) resolveMetaCmd(raw string) tea.Cmd {
	return func() tea.Msg {
		s := strings.TrimSpace(raw)
		if s == "" {
			return metaMsg{url: raw}
		}

		// Use centralized URL normalization and resolution
		normalized := tui.NormalizeURL(s)

		// Only resolve URIs (hf:// or civitai://)
		if strings.HasPrefix(normalized, "hf://") || strings.HasPrefix(normalized, "civitai://") {
			res, err := resolver.Resolve(context.Background(), normalized, m.cfg)
			if err == nil {
				return metaMsg{
					url:           raw,
					fileName:      res.FileName,
					suggested:     res.SuggestedFilename,
					civType:       res.FileType,
					quants:        res.AvailableQuantizations,
					selectedQuant: res.SelectedQuantization,
				}
			}
		}
		return metaMsg{url: raw}
	}
}

func computeTypesFromConfig(cfg *config.Config) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range cfg.Placement.Mapping {
		t := strings.TrimSpace(r.Match)
		if t == "" {
			continue
		}
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

func truncateMiddle(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:max])
	}
	left := max / 2
	right := max - left - 1
	return string(runes[:left]) + "…" + string(runes[len(runes)-right:])
}

func (m *Model) lastColumnWidth(compact bool) int {
	// Without side panels, use nearly full width minus borders
	usable := m.w - 2*2 // borders on left/right wrappers
	if usable < 40 {
		usable = 40
	}
	if compact {
		// S(1), space(1), STATUS(8), space(1), RT(3), 2sp, PROG(16), 2sp, PCT(4), 2sp, SRC(12), 2sp, ETA(8), 2sp
		consumed := 1 + 1 + 8 + 1 + 3 + 2 + 16 + 2 + 4 + 2 + 12 + 2 + 8 + 2
		lw := usable - consumed
		if lw < 10 {
			lw = 10
		}
		return lw
	}
	// non-compact consumed widths: + SPEED(10) + THR(10) before SRC
	consumed := 1 + 1 + 8 + 1 + 3 + 2 + 16 + 2 + 4 + 2 + 10 + 2 + 10 + 2 + 12 + 2 + 8 + 2
	lw := usable - consumed
	if lw < 10 {
		lw = 10
	}
	return lw
}

func (m *Model) maxRowsOnScreen() int {
	max := m.h - 10
	if max < 3 {
		return 3
	}
	return max
}

func keyFor(r state.DownloadRow) string { return r.URL + "|" + r.Dest }

func hostOf(urlStr string) string {
	if u, err := neturl.Parse(urlStr); err == nil {
		return u.Hostname()
	}
	return ""
}

func (m *Model) selectionOrCurrent() []state.DownloadRow {
	rows := m.visibleRows()
	if len(m.selectedKeys) == 0 {
		if m.selected >= 0 && m.selected < len(rows) {
			return []state.DownloadRow{rows[m.selected]}
		}
		return nil
	}
	var out []state.DownloadRow
	for _, r := range rows {
		if m.selectedKeys[keyFor(r)] {
			out = append(out, r)
		}
	}
	return out
}

func (m *Model) addToast(s string) {
	m.toasts = append(m.toasts, toast{msg: s, when: time.Now(), ttl: 3 * time.Second})
	if len(m.toasts) > 50 {
		// keep last 50
		m.toasts = m.toasts[len(m.toasts)-50:]
	}
}

func (m *Model) gcToasts() {
	now := time.Now()
	var keep []toast
	for _, t := range m.toasts {
		if now.Sub(t.when) < t.ttl {
			keep = append(keep, t)
		}
	}
	m.toasts = keep
}

func (m *Model) renderToasts() string {
	if len(m.toasts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(m.toasts))
	for _, t := range m.toasts {
		parts = append(parts, t.msg)
	}
	return m.th.label.Render(strings.Join(parts, " • "))
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
	// Apply
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
	p := m.uiStatePath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	st := uiState{ThemeIndex: m.themeIndex, ShowURL: m.columnMode == "url", ColumnMode: m.columnMode, Compact: m.cfg.UI.Compact, GroupBy: m.groupBy, SortMode: m.sortMode}
	b, _ := json.MarshalIndent(st, "", "  ")
	_ = os.WriteFile(p, b, 0o644)
}

func (m *Model) renderCommandsBar() string {
	// concise single-line commands reference
	return m.th.footer.Render("n new • b batch • y/r start • p cancel • D delete • O open • / filter • s/e/R sort • o clear • g group host • t col • v compact • i inspector • H toasts • ? help • q quit")
}

func (m *Model) renderHelp() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("Help (TUI v2)") + "\n")
	sb.WriteString("Tabs: 1 Pending • 2 Active • 3 Completed • 4 Failed\n")
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
	sb.WriteString("Quit: q\n")
	return sb.String()
}

func (m *Model) renderToastDrawer() string {
	if len(m.toasts) == 0 {
		return m.th.label.Render("(no recent notifications)")
	}
	now := time.Now()
	var sb strings.Builder
	for i := len(m.toasts) - 1; i >= 0; i-- { // newest first
		t := m.toasts[i]
		dur := now.Sub(t.when).Round(time.Second)
		sb.WriteString(fmt.Sprintf("%s  %s\n", t.msg, m.th.label.Render(dur.String()+" ago")))
	}
	return sb.String()
}

// URL normalization helper (for modal note only)
func (m *Model) normalizeURLForNote(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return "", false
	}
	u, err := neturl.Parse(s)
	if err != nil {
		return "", false
	}
	h := strings.ToLower(u.Hostname())
	if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 2 {
			modelID := parts[1]
			q := u.Query()
			ver := q.Get("modelVersionId")
			if ver == "" {
				ver = q.Get("version")
			}
			civ := "civitai://model/" + modelID
			if strings.TrimSpace(ver) != "" {
				civ += "?version=" + ver
			}
			return "Normalized to " + civ, true
		}
	}
	if hostIs(h, "huggingface.co") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 5 && parts[2] == "blob" {
			owner := parts[0]
			repo := parts[1]
			rev := parts[3]
			filePath := strings.Join(parts[4:], "/")
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
		if env == "" {
			env = "HF_TOKEN"
		}
		m.hfTokenSet = strings.TrimSpace(os.Getenv(env)) != ""
	} else {
		m.hfTokenSet = false
	}
	if m.cfg.Sources.CivitAI.Enabled {
		env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
		if env == "" {
			env = "CIVITAI_TOKEN"
		}
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
	if envHF == "" {
		envHF = "HF_TOKEN"
	}
	envCIV := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
	if envCIV == "" {
		envCIV = "CIVITAI_TOKEN"
	}
	hfEnabled := m.cfg.Sources.HuggingFace.Enabled
	civEnabled := m.cfg.Sources.CivitAI.Enabled
	var hf string
	var civ string
	if !hfEnabled {
		hf = m.th.label.Render(fmt.Sprintf("HF (%s): disabled", envHF))
	} else if m.hfRateLimited {
		hf = m.th.bad.Render(fmt.Sprintf("HF (%s): rate-limited", envHF))
	} else if m.hfRejected {
		hf = m.th.bad.Render(fmt.Sprintf("HF (%s): rejected", envHF))
	} else if m.hfTokenSet {
		hf = m.th.ok.Render(fmt.Sprintf("HF (%s): set", envHF))
	} else {
		hf = m.th.bad.Render(fmt.Sprintf("HF (%s): unset", envHF))
	}
	if !civEnabled {
		civ = m.th.label.Render(fmt.Sprintf("CivitAI (%s): disabled", envCIV))
	} else if m.civRateLimited {
		civ = m.th.bad.Render(fmt.Sprintf("CivitAI (%s): rate-limited", envCIV))
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
				name = util.URLPathBase(res.URL)
			}
			name = util.SafeFileName(name)
			return filepath.Join(m.cfg.General.DownloadRoot, name)
		}
	}
	// translate civitai model page URLs and Hugging Face blob pages
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		if pu, err := neturl.Parse(u); err == nil {
			h := strings.ToLower(pu.Hostname())
			// For direct CivitAI download endpoints, try HEAD for a better filename first
			if hostIs(h, "civitai.com") && strings.HasPrefix(pu.Path, "/api/download/") {
				if name := m.headFilename(u); strings.TrimSpace(name) != "" {
					name = util.SafeFileName(name)
					return filepath.Join(m.cfg.General.DownloadRoot, name)
				}
			}
			if hostIs(h, "civitai.com") && strings.HasPrefix(pu.Path, "/models/") {
				parts := strings.Split(strings.Trim(pu.Path, "/"), "/")
				if len(parts) >= 2 {
					modelID := parts[1]
					q := pu.Query()
					ver := q.Get("modelVersionId")
					if ver == "" {
						ver = q.Get("version")
					}
					civ := "civitai://model/" + modelID
					if strings.TrimSpace(ver) != "" {
						civ += "?version=" + neturl.QueryEscape(ver)
					}
					if res, err := resolver.Resolve(ctx, civ, m.cfg); err == nil {
						name := res.SuggestedFilename
						if strings.TrimSpace(name) == "" {
							name = filepath.Base(res.URL)
						}
						name = util.SafeFileName(name)
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
						name := util.URLPathBase(res.URL)
						name = util.SafeFileName(name)
						return filepath.Join(m.cfg.General.DownloadRoot, name)
					}
				}
			}
		}
	}
	base := util.URLPathBase(u)
	if base == "" || base == "/" || base == "." {
		base = "download"
	}
	return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(base))
}

func (m *Model) importBatchFile(path string) tea.Cmd {
	f, err := os.Open(path)
	if err != nil {
		m.addToast("batch open failed: " + err.Error())
		return nil
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	count := 0
	cmds := make([]tea.Cmd, 0, 64)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
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
				case "dest":
					dest = v
				case "type":
					typeOv = v
				case "place":
					v2 := strings.ToLower(v)
					place = v2 == "y" || v2 == "yes" || v2 == "true" || v2 == "1"
				}
			}
		}
		if dest == "" {
			dest = m.computeDefaultDest(u)
		}
		cmds = append(cmds, m.startDownloadCmd(u, dest))
		key := u + "|" + dest
		if strings.TrimSpace(typeOv) != "" {
			m.placeType[key] = typeOv
		}
		if place {
			m.autoPlace[key] = true
		}
		count++
	}
	if err := sc.Err(); err != nil {
		m.addToast("batch read error: " + err.Error())
	}
	m.addToast(fmt.Sprintf("batch: started %d", count))
	return tea.Batch(cmds...)
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
					env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv)
					if env == "" {
						env = "HF_TOKEN"
					}
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
						headers["Authorization"] = "Bearer " + tok
					}
				}
				if hostIs(h, "civitai.com") && m.cfg.Sources.CivitAI.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
					if env == "" {
						env = "CIVITAI_TOKEN"
					}
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
						headers["Authorization"] = "Bearer " + tok
					}
				}
			}
			reach, info := downloader.CheckReachable(ctx, m.cfg, row.URL, headers)
			if !reach {
				_ = m.st.UpsertDownload(state.DownloadRow{URL: row.URL, Dest: row.Dest, Status: "hold", LastError: info})
			}
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
						ver := q.Get("modelVersionId")
						if ver == "" {
							ver = q.Get("version")
						}
						civ := "civitai://model/" + modelID
						if strings.TrimSpace(ver) != "" {
							civ += "?version=" + neturl.QueryEscape(ver)
						}
						normToast = "normalized CivitAI page → " + civ
						if res, err := resolver.Resolve(ctx, civ, m.cfg); err == nil {
							resolved = res.URL
							headers = res.Headers
							m.addToast(normToast)
						}
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
						if res, err := resolver.Resolve(ctx, hf, m.cfg); err == nil {
							resolved = res.URL
							headers = res.Headers
							m.addToast(normToast)
						}
					}
				}
			}
		}
		if strings.HasPrefix(resolved, "hf://") || strings.HasPrefix(resolved, "civitai://") {
			res, err := resolver.Resolve(ctx, resolved, m.cfg)
			if err != nil {
				return dlDoneMsg{url: urlStr, dest: dest, path: "", err: err}
			}
			resolved = res.URL
			headers = res.Headers
		} else {
			if u, err := neturl.Parse(resolved); err == nil {
				h := strings.ToLower(u.Hostname())
				if hostIs(h, "huggingface.co") && m.cfg.Sources.HuggingFace.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv)
					if env == "" {
						env = "HF_TOKEN"
					}
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
						headers["Authorization"] = "Bearer " + tok
					}
				}
				if hostIs(h, "civitai.com") && m.cfg.Sources.CivitAI.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
					if env == "" {
						env = "CIVITAI_TOKEN"
					}
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
						headers["Authorization"] = "Bearer " + tok
					}
				}
			}
		}
		// Merge any pre-inserted row under the original URL into the resolved URL key to avoid duplicates
		origKey := urlStr + "|" + dest
		resKey := resolved + "|" + dest
		if origKey != resKey {
			// Mirror placement overrides
			if m.autoPlace[origKey] {
				m.autoPlace[resKey] = true
				delete(m.autoPlace, origKey)
			}
			if t := m.placeType[origKey]; strings.TrimSpace(t) != "" {
				m.placeType[resKey] = t
				delete(m.placeType, origKey)
			}
			// Remove the old pending row and create/refresh the resolved one as pending
			_ = m.st.DeleteDownload(urlStr, dest)
			_ = m.st.UpsertDownload(state.DownloadRow{URL: resolved, Dest: dest, Status: "pending"})
		}
		if start {
			if !m.cfg.Network.DisableAuthPreflight {
				// Quick reachability/auth probe; if network-unreachable or unauthorized, put job on hold and do not start download
				reach, info := downloader.CheckReachable(ctx, m.cfg, resolved, headers)
				if !reach {
					_ = m.st.UpsertDownload(state.DownloadRow{URL: resolved, Dest: dest, Status: "hold", LastError: info})
					// mirror autoplace/type mappings to resolved key so future retries work
					origKey := urlStr + "|" + dest
					resKey := resolved + "|" + dest
					if m.autoPlace[origKey] {
						m.autoPlace[resKey] = true
					}
					if t := m.placeType[origKey]; t != "" {
						m.placeType[resKey] = t
					}
					m.addToast("probe failed: unreachable; job on hold (" + info + ")")
					return dlDoneMsg{url: resolved, dest: dest, path: "", err: fmt.Errorf("hold: unreachable")}
				}
				// If reachable, parse status code for early auth guidance
				code := 0
				if sp := strings.Fields(info); len(sp) > 0 {
					_, _ = fmt.Sscanf(sp[0], "%d", &code)
				}
				if code == 401 || code == 403 {
					host := hostOf(resolved)
					msg := info
					if strings.HasSuffix(strings.ToLower(host), "huggingface.co") {
						env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv)
						if env == "" {
							env = "HF_TOKEN"
						}
						msg = fmt.Sprintf("%s — set %s and ensure repo access/license accepted", info, env)
					} else if strings.HasSuffix(strings.ToLower(host), "civitai.com") {
						env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
						if env == "" {
							env = "CIVITAI_TOKEN"
						}
						msg = fmt.Sprintf("%s — set %s and ensure content is accessible", info, env)
					}
					_ = m.st.UpsertDownload(state.DownloadRow{URL: resolved, Dest: dest, Status: "hold", LastError: msg})
					m.addToast("preflight auth: job on hold (" + msg + ")")
					return dlDoneMsg{url: resolved, dest: dest, path: "", err: fmt.Errorf("hold: unauthorized")}
				}
			}
			log := logging.New("error", false)
			dl := downloader.NewAuto(m.cfg, log, m.st, nil)
			final, _, err := dl.Download(ctx, resolved, dest, "", headers, m.cfg.General.AlwaysNoResume)
			return dlDoneMsg{url: urlStr, dest: dest, path: final, err: err}
		} else {
			// Probe-only path
			reach, info := downloader.CheckReachable(ctx, m.cfg, resolved, headers)
			if !reach {
				_ = m.st.UpsertDownload(state.DownloadRow{URL: resolved, Dest: dest, Status: "hold", LastError: info})
			}
			return probeMsg{url: resolved, dest: dest, reachable: reach, info: info}
		}
	}
}

func openInFileManager(p string, reveal bool) error {
	p = strings.TrimSpace(p)
	if p == "" {
		return fmt.Errorf("empty path")
	}
	// Determine directory to open even if file doesn't exist yet
	var dir string
	if fi, err := os.Stat(p); err == nil {
		if fi.IsDir() {
			dir = p
		} else {
			dir = filepath.Dir(p)
		}
	} else {
		dir = filepath.Dir(p)
	}
	switch runtime.GOOS {
	case "darwin":
		if reveal {
			// Reveal if possible; if that fails, fallback to opening dir
			if err := exec.Command("open", "-R", p).Run(); err == nil {
				return nil
			}
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
	return []Theme{base, neon, drac, solar}
}

func themeIndexByName(name string) int {
	s := strings.ToLower(strings.TrimSpace(name))
	switch s {
	case "", "default":
		return 0
	case "neon":
		return 1
	case "drac", "dracula":
		return 2
	case "solar", "solarized":
		return 3
	default:
		return -1
	}
}

//nolint:unused,deadcode
func themeNameByIndex(idx int) string {
	switch idx {
	case 0:
		return "default"
	case 1:
		return "neon"
	case 2:
		return "drac"
	case 3:
		return "solar"
	default:
		return "default"
	}
}

//nolint:unused,deadcode
func (m *Model) sortModeLabel() string {
	switch strings.ToLower(strings.TrimSpace(m.sortMode)) {
	case "speed":
		return "speed"
	case "eta":
		return "eta"
	case "rem":
		return "remaining"
	default:
		return "none"
	}
}

//nolint:unused,deadcode
func (m *Model) groupByLabel() string {
	if strings.TrimSpace(m.groupBy) == "" {
		return "none"
	}
	return m.groupBy
}

func copyToClipboard(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("empty")
	}
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		in, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		_, _ = in.Write([]byte(s))
		_ = in.Close()
		return cmd.Wait()
	case "linux":
		// try wl-copy then xclip
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd := exec.Command("wl-copy")
			in, err := cmd.StdinPipe()
			if err != nil {
				return err
			}
			if err := cmd.Start(); err != nil {
				return err
			}
			_, _ = in.Write([]byte(s))
			_ = in.Close()
			return cmd.Wait()
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command("xclip", "-selection", "clipboard")
			in, err := cmd.StdinPipe()
			if err != nil {
				return err
			}
			if err := cmd.Start(); err != nil {
				return err
			}
			_, _ = in.Write([]byte(s))
			_ = in.Close()
			return cmd.Wait()
		}
	}
	return fmt.Errorf("no clipboard utility found")
}
