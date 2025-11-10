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
	activeTab       int // 0: All, 1: Pending, 2: Active, 3: Completed, 4: Failed, 5: Library, 6: Settings
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
	case metadataStoredMsg:
		// Metadata was successfully stored, mark library as needing refresh
		m.libraryNeedsRefresh = true
		return m, nil
	case scanCompleteMsg:
		// Handle directory scan completion
		m.libraryScanning = false
		m.libraryScanProgress = ""

		if msg.err != nil {
			m.addToast(fmt.Sprintf("scan error: %v", msg.err))
		} else if msg.result != nil {
			m.addToast(fmt.Sprintf("scan complete: %d files scanned, %d models added, %d skipped",
				msg.result.FilesScanned, msg.result.ModelsAdded, msg.result.ModelsSkipped))
			// Refresh library to show new models - will be loaded when user views library tab
			m.libraryNeedsRefresh = true
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
		// Fetch and store metadata asynchronously via command
		cmds = append(cmds, m.fetchAndStoreMetadataCmd(msg.url, msg.dest, msg.path))
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
