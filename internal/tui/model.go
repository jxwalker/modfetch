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
	"github.com/jxwalker/modfetch/internal/metadata"
	"github.com/jxwalker/modfetch/internal/placer"
	"github.com/jxwalker/modfetch/internal/resolver"
	"github.com/jxwalker/modfetch/internal/scanner"
	"github.com/jxwalker/modfetch/internal/state"
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

type tickMsg time.Time

type dlDoneMsg struct {
	url, dest, path string
	err             error
	// Placement metadata to avoid concurrent map access
	origKey       string
	resKey        string
	autoPlace     bool
	placeType     string
	needsPlaceMap bool // true if placement metadata should be applied
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
	activeTab       int // 0: Pending, 1: Active, 2: Completed, 3: Failed, 4: Library
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
	// Library view state
	libraryRows          []state.ModelMetadata
	librarySelected      int
	libraryFilter        state.MetadataFilters
	librarySearch        string
	libraryViewingDetail bool
	libraryDetailModel   *state.ModelMetadata
	libraryFilterType    string          // Filter by model type: "", "LLM", "LoRA", etc.
	libraryFilterSource  string          // Filter by source: "", "huggingface", "civitai"
	libraryShowFavorites bool            // Show only favorites
	libraryNeedsRefresh  bool            // Flag to reload library data
	librarySearchInput   textinput.Model // Search input for library
	librarySearchActive  bool            // Search input is active
	libraryScanning      bool            // Currently scanning directories
	libraryScanProgress  string          // Scan progress message
	log                  *logging.Logger
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
	// Create library search input
	libSearch := textinput.New()
	libSearch.Placeholder = "search models..."
	libSearch.CharLimit = 256

	m := &Model{
		cfg: cfg, st: st, th: defaultTheme(), activeTab: -1, prog: p, prev: map[string]obs{}, prevStatus: map[string]string{},
		build:   strings.TrimSpace(version),
		running: map[string]context.CancelFunc{}, selectedKeys: map[string]bool{}, filterInput: fi,
		rateCache: map[string]float64{}, etaCache: map[string]string{}, totalCache: map[string]int64{}, curBytesCache: map[string]int64{}, rateHist: map[string][]float64{}, hostCache: map[string]string{},
		retrying:   map[string]time.Time{},
		tickEvery:  refresh,
		columnMode: mode,
		placeType:  map[string]string{}, autoPlace: map[string]bool{},
		// Library state
		libraryRows:         []state.ModelMetadata{},
		libraryNeedsRefresh: true,
		librarySearchInput:  libSearch,
		log:                 logging.New("info", false),
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
		if m.librarySearchActive {
			return m.updateLibrarySearch(msg)
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
			// Capture placement data before spawning goroutine (avoid concurrent map access)
			autoPlace := m.autoPlace[key]
			placeType := m.placeType[key]
			ctx, cancel := context.WithCancel(context.Background())
			m.running[key] = cancel
			m.retrying[key] = now
			cmds = append(cmds, m.startDownloadCmdCtx(ctx, r.URL, r.Dest, autoPlace, placeType))
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
	case scanCompleteMsg:
		// Handle directory scan completion
		m.libraryScanning = false
		m.libraryScanProgress = ""

		if msg.err != nil {
			m.addToast(fmt.Sprintf("scan error: %v", msg.err))
		} else if msg.result != nil {
			m.addToast(fmt.Sprintf("scan complete: %d files scanned, %d models added, %d skipped",
				msg.result.FilesScanned, msg.result.ModelsAdded, msg.result.ModelsSkipped))
			// Refresh library to show new models
			m.libraryNeedsRefresh = true
			m.refreshLibraryData()
		}
		return m, nil
	case dlDoneMsg:
		// Apply placement metadata if needed (safe concurrent map access from Update thread)
		if msg.needsPlaceMap && msg.origKey != msg.resKey {
			if msg.autoPlace {
				m.autoPlace[msg.resKey] = true
			}
			if strings.TrimSpace(msg.placeType) != "" {
				m.placeType[msg.resKey] = msg.placeType
			}
			// Clean up old keys
			delete(m.autoPlace, msg.origKey)
			delete(m.placeType, msg.origKey)
		}
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
		// Fetch and store metadata asynchronously
		go m.fetchAndStoreMetadata(msg.url, msg.dest, msg.path)
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

func (m *Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	// Handle library-specific escape key
	if s == "esc" && m.activeTab == 4 && m.libraryViewingDetail {
		m.libraryViewingDetail = false
		m.libraryDetailModel = nil
		return m, nil
	}

	switch s {
	case "q", "ctrl+c":
		m.saveUIState()
		return m, tea.Quit
	case "/":
		if m.activeTab == 4 {
			// Library search
			m.librarySearchActive = true
			m.librarySearchInput.Focus()
		} else {
			// Download filter
			m.filterOn = true
			m.filterInput.Focus()
		}
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
	case "S":
		// Scan directories for existing models
		if m.activeTab == 4 && !m.libraryScanning {
			return m, m.scanDirectoriesCmd()
		}
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
	case "5", "l":
		// Switch to Library tab (key 5 or 'l' for library)
		m.activeTab = 4
		m.librarySelected = 0
		m.libraryViewingDetail = false
		// Load library data if needed
		if m.libraryNeedsRefresh || len(m.libraryRows) == 0 {
			m.refreshLibraryData()
		}
		return m, nil
	case "6", "m":
		// Switch to Settings tab (key 6 or 'm' for menu/settings)
		m.activeTab = 5
		return m, nil
	case "j", "down":
		if m.activeTab == 4 {
			// Library navigation
			if !m.libraryViewingDetail && m.librarySelected < len(m.libraryRows)-1 {
				m.librarySelected++
			}
		} else {
			// Download table navigation
			if m.selected < len(m.visibleRows())-1 {
				m.selected++
			}
		}
		return m, nil
	case "k", "up":
		if m.activeTab == 4 {
			// Library navigation
			if !m.libraryViewingDetail && m.librarySelected > 0 {
				m.librarySelected--
			}
		} else {
			// Download table navigation
			if m.selected > 0 {
				m.selected--
			}
		}
		return m, nil
	case "enter", "ctrl+j":
		if m.activeTab == 4 {
			// Library view: open detail view
			if !m.libraryViewingDetail && m.librarySelected < len(m.libraryRows) {
				m.libraryDetailModel = &m.libraryRows[m.librarySelected]
				m.libraryViewingDetail = true
			}
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
			// Capture placement data before spawning goroutine (avoid concurrent map access)
			key := keyFor(r)
			autoPlace := m.autoPlace[key]
			placeType := m.placeType[key]
			ctx, cancel := context.WithCancel(context.Background())
			m.running[key] = cancel
			m.retrying[key] = now
			cmds = append(cmds, m.startDownloadCmdCtx(ctx, r.URL, r.Dest, autoPlace, placeType))
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
	title := m.th.title.Render(fmt.Sprintf("modfetch • TUI (build %s)", ver))
	toastStr := m.renderToasts()
	titleBar := m.th.border.Width(m.w - 2).Render(lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", toastStr))

	// Horizontal tabs across full width
	tabs := m.th.border.Width(m.w - 2).Render(m.renderTabs())

	// Commands bar for quick reference
	commands := ""
	if m.activeTab == 4 {
		// Library-specific commands
		commands = m.th.border.Width(m.w - 2).Render(m.renderLibraryCommandsBar())
	} else if m.activeTab == 5 {
		// Settings tab commands
		commands = m.th.border.Width(m.w - 2).Render(m.renderSettingsCommandsBar())
	} else {
		// Download table commands
		commands = m.th.border.Width(m.w - 2).Render(m.renderCommandsBar())
	}

	// Main content: table, library, or settings view
	var mainContent string
	if m.activeTab == 4 {
		// Library view
		mainContent = m.th.border.Width(m.w - 2).Render(m.renderLibrary())
	} else if m.activeTab == 5 {
		// Settings view
		mainContent = m.th.border.Width(m.w - 2).Render(m.renderSettings())
	} else {
		// Download table view
		mainContent = m.th.border.Width(m.w - 2).Render(m.renderTable())
	}

	// Inspector (only for download tabs, not library or settings)
	inspector := ""
	if m.activeTab != 4 && m.activeTab != 5 {
		inspector = m.th.border.Width(m.w - 2).Render(m.renderInspector())
	}

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

	// Minimal footer: show filter or search input when active
	inputBar := ""
	if m.filterOn {
		inputBar = "Filter: " + m.filterInput.View()
	} else if m.librarySearchActive {
		inputBar = "Search: " + m.librarySearchInput.View()
	}
	footer := ""
	if strings.TrimSpace(inputBar) != "" {
		footer = m.th.border.Width(m.w - 2).Render(m.th.footer.Render(inputBar))
	}

	parts := []string{titleBar, tabs}
	if commands != "" {
		parts = append(parts, commands)
	}
	parts = append(parts, mainContent)
	if inspector != "" {
		parts = append(parts, inspector)
	}
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
		// Set placement data before spawning goroutine
		key := u + "|" + dest
		if strings.TrimSpace(typeOv) != "" {
			m.placeType[key] = typeOv
		}
		if place {
			m.autoPlace[key] = true
		}
		// Capture placement data to pass (avoid concurrent map access)
		autoPlace := m.autoPlace[key]
		placeType := m.placeType[key]
		cmds = append(cmds, m.startDownloadCmd(u, dest, autoPlace, placeType))
		count++
	}
	if err := sc.Err(); err != nil {
		m.addToast("batch read error: " + err.Error())
	}
	m.addToast(fmt.Sprintf("batch: started %d", count))
	return tea.Batch(cmds...)
}

func (m *Model) startDownloadCmd(urlStr, dest string, autoPlace bool, placeType string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.running[urlStr+"|"+dest] = cancel
	return m.downloadOrHoldCmd(ctx, urlStr, dest, true, autoPlace, placeType)
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

func (m *Model) startDownloadCmdCtx(ctx context.Context, urlStr, dest string, autoPlace bool, placeType string) tea.Cmd {
	return m.downloadOrHoldCmd(ctx, urlStr, dest, true, autoPlace, placeType)
}

// downloadOrHoldCmd optionally starts download; when start=false it only probes and updates hold status
func (m *Model) downloadOrHoldCmd(ctx context.Context, urlStr, dest string, start bool, autoPlace bool, placeType string) tea.Cmd {
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
				return dlDoneMsg{url: urlStr, dest: dest, path: "", err: err, needsPlaceMap: false}
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
		// Use placement metadata passed as parameters (safe - no concurrent map access)
		needsMap := false
		autoPlaceVal := autoPlace
		placeTypeVal := placeType
		if origKey != resKey {
			// Mark that placement data needs to be remapped in Update()
			needsMap = true
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
					// Return placement data to Update() for safe map handling
					m.addToast("probe failed: unreachable; job on hold (" + info + ")")
					return dlDoneMsg{
						url: resolved, dest: dest, path: "", err: fmt.Errorf("hold: unreachable"),
						origKey: origKey, resKey: resKey, autoPlace: autoPlaceVal, placeType: placeTypeVal, needsPlaceMap: needsMap,
					}
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
					return dlDoneMsg{
						url: resolved, dest: dest, path: "", err: fmt.Errorf("hold: unauthorized"),
						origKey: origKey, resKey: resKey, autoPlace: autoPlaceVal, placeType: placeTypeVal, needsPlaceMap: needsMap,
					}
				}
			}
			log := logging.New("error", false)
			dl := downloader.NewAuto(m.cfg, log, m.st, nil)
			final, _, err := dl.Download(ctx, resolved, dest, "", headers, m.cfg.General.AlwaysNoResume)
			return dlDoneMsg{
				url: urlStr, dest: dest, path: final, err: err,
				origKey: origKey, resKey: resKey, autoPlace: autoPlaceVal, placeType: placeTypeVal, needsPlaceMap: needsMap,
			}
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

// Theme presets

// This is fire-and-forget - errors are logged but don't affect the UI.
func (m *Model) fetchAndStoreMetadata(url, dest, path string) {
	if m.st == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create metadata registry
	registry := metadata.NewRegistry()

	// Fetch metadata from appropriate source
	meta, err := registry.FetchMetadata(ctx, url)
	if err != nil {
		// Log error but don't fail - metadata is optional
		if m.log != nil {
			m.log.Debugf("metadata fetch failed for %s: %v", url, err)
		}
		return
	}

	// Set destination path
	meta.DownloadURL = url
	meta.Dest = dest

	// Get file size if available
	if path != "" {
		if info, err := os.Stat(path); err == nil {
			meta.FileSize = info.Size()
		}
	}

	// Store metadata in database
	if err := m.st.UpsertMetadata(meta); err != nil {
		if m.log != nil {
			m.log.Debugf("metadata storage failed for %s: %v", url, err)
		}
		return
	}

	// Mark library as needing refresh
	m.libraryNeedsRefresh = true

	if m.log != nil {
		m.log.Debugf("stored metadata for %s (%s)", meta.ModelName, url)
	}
}

// refreshLibraryData loads model metadata from the database
