# TUI Architecture Analysis for modfetch

## Executive Summary

modfetch is a CLI+TUI application for downloading and managing LLM/Stable Diffusion models from Hugging Face and CivitAI. The project has two TUI implementations:

- **TUI v1**: Recently refactored (Sep 24, 2025) into clean MVC components (~930 lines total)
- **TUI v2**: Monolithic model implementation (2658 lines) now set as default

## 1. Project Overview

**What this application does:**
- Fast, resilient downloads with parallel chunked transfers
- SHA256 integrity verification (per-chunk and full file)
- Automatic model classification and placement into app directories
- Batch YAML execution with verify and status
- Rich TUI dashboard with live speed/ETA tracking
- Terminal User Interface for interactive downloads

**Project Structure:**
```
modfetch/
├── cmd/modfetch/          # CLI entry point
│   └── tui_cmd.go         # TUI command handler
├── internal/
│   ├── config/            # YAML configuration
│   ├── downloader/        # Single + chunked engines
│   ├── resolver/          # hf:// and civitai:// resolvers
│   ├── placer/            # Model placement engine
│   ├── state/             # SQLite state database
│   ├── metrics/           # Prometheus metrics
│   └── tui/               # Terminal UI
│       ├── tui_cmd.go     # Entry point
│       ├── model.go       # MVC orchestrator (~65 lines)
│       ├── tui_model.go   # Data/business logic (~298 lines)
│       ├── tui_view.go    # Rendering logic (~249 lines)
│       ├── tui_controller.go  # Event handling (~367 lines)
│       ├── v2/
│       │   ├── model.go   # Monolithic TUI v2 (~2658 lines)
│       │   ├── model_test.go
│       │   └── model_inspector_test.go
│       └── configwizard/  # Interactive config setup
└── docs/                  # Comprehensive documentation
```

## 2. TUI Code Locations

### TUI v1 (Refactored MVC) - Entry: `/home/user/modfetch/internal/tui/`

**Files:**
- `model.go` (65 lines) - Orchestration layer, implements tea.Model interface
- `tui_model.go` (298 lines) - Data model & business logic
- `tui_controller.go` (367 lines) - Event/key handling
- `tui_view.go` (249 lines) - UI rendering with lipgloss styling
- `tui_utils.go` (16 lines) - Helper: tryWrite() for directory checks

**Architecture Pattern: MVC**
- Model: Manages state, database access, download operations
- View: Handles all rendering with Lipgloss styling
- Controller: Processes keyboard events and coordinates updates
- Orchestrator (model.go): Ties components together via tea.Model interface

### TUI v2 (Monolithic) - Entry: `/home/user/modfetch/internal/tui/v2/`

**File:**
- `model.go` (2658 lines) - Everything in one file

**Architecture Pattern: Monolithic**
- Single Model struct containing all state (cfg, rows, filters, modals, themes, etc.)
- All logic (rendering, event handling, downloads) in one file
- Uses Bubble Tea framework
- Frameworks: Bubble Tea, Bubbles (components), Lipgloss (styling)

**Entry Point:** `/home/user/modfetch/cmd/modfetch/tui_cmd.go`
- Default: TUI v2 (line 94)
- Flag: `--v1` to use experimental TUI v1 (line 28)
- Creates config wizard if config doesn't exist

## 3. Current TUI Implementation Approach

### Frameworks & Libraries
- **Bubble Tea**: Terminal UI framework (charmbracelet/bubbletea)
- **Bubbles**: Components library (textinput, progress bars)
- **Lipgloss**: Terminal styling (borders, colors, layouts)
- **Humanize**: Human-readable formatting (file sizes)

### Design Patterns

**TUI v1 (MVC - Good Practice):**
- Single Responsibility Principle applied
- Clear separation: Model↔View↔Controller
- Component testing friendly
- Easier to maintain and extend

**TUI v2 (Monolithic - Pragmatic):**
- All-in-one model for rapid feature development
- Direct state access from anywhere
- Harder to test individual pieces
- Performance optimizations (caching, throttling)

### Key Features

**Both versions support:**
- Multi-tab interface (Pending, Active, Completed, Failed)
- Live speed and ETA calculation with smoothing
- Filtering and sorting (by speed/ETA/remaining bytes)
- New download modal with 4-step wizard
- Batch import from text files
- Auto-placement of downloads
- Toast notifications (v2 only)
- Theme system (v2 only: default, neon, drac, solar)
- Inspector panel (v2 only)

## 4. Obvious Issues & Broken Functionality

### Recent Fixes (Sept 2025)

**1. TUI Version Selection Bug (Fixed: Commit 491e6dc)**
- Issue: Backwards logic - `--v1` flag was actually launching v2
- Fix: Corrected default to v2, made v1 opt-in via `--v1`
- Status: ✅ Fixed

**2. TUI v1 Loading Screen Hang (Fixed: Commit a435afc)**
- Issue: Default window dimensions not set, causing infinite "Loading..." state
- Fix: Set default width=120, height=30 when dimensions uninitialized
- Location: tui_view.go lines 68-71
- Status: ✅ Fixed

**3. Enter Key Handling (Fixed: Commit 7ce5afa)**
- Issue: Bubble Tea sends Enter as 'ctrl+j', causing modal input to hang
- Fix: v1 controller now handles both "enter" and "ctrl+j" (line 211)
- Status: ✅ Fixed in v1
- Note: v2 only handles "enter" - potential issue if Bubble Tea sends ctrl+j

**4. Missing UI Elements (Fixed: Commit a435afc)**
- Issue: v1 lacked borders, colors, and rich theming after MVC refactor
- Fix: Added comprehensive styling, borders, colored status indicators
- Status: ✅ Fixed

### Potential Issues & Gaps

**1. TUI v2 Enter Key Handling**
- Status: ⚠️ Potential Issue
- Details: v2's `updateNewJob()` and `updateBatchMode()` only handle "enter" (lines 402, 492)
- Not clear if v2 also needs "ctrl+j" handling
- Impact: Could cause modal input to fail on some systems

**2. Monolithic v2 Architecture**
- Status: ⚠️ Design Issue
- Details: 2658-line single file hard to test/maintain
- No separation of concerns (rendering, state, logic mixed)
- Makes code reviews and debugging harder
- But: Currently working and feature-complete

**3. Missing Comprehensive Error Handling**
- Status: ⚠️ Minor Issue
- Details: Some async operations (StartDownloadCmd, resolveMetaCmd) don't always bubble up errors
- v1 has basic error handling via errMsg
- v2 has toast-based notifications but could be more consistent

**4. Progress Calculation in v1**
- Status: ⚠️ Potential Inaccuracy
- Details: v1's ProgressFor() always returns 0 for current bytes (tui_model.go line 104)
- This means progress bars won't show intermediate progress
- Impact: Users see "unknown" progress for ongoing downloads in v1

**5. URL Resolution Duplication**
- Status: ⚠️ Code Duplication
- Details: CivitAI URL normalization logic appears in multiple places:
  - tui_controller.go (resolveMetaCmd, lines 338-354)
  - tui_model.go (StartDownload, lines 194-212)
  - v2/model.go (multiple locations)
- Should be centralized

**6. Thread Safety in v1 Model**
- Status: ✅ Good
- Details: Has runningMu RWMutex for concurrent access (tui_model.go lines 25, 75-76)
- Proper protection of running map

## 5. Recent Changes (Git History)

### Major Commits

**Commit a435afc** (Sep 26, 2025)
- Type: Fix/Enhancement
- Title: "restore rich UI elements and borders to TUI v1"
- Changes:
  - Added comprehensive theming with borders
  - Fixed loading screen hang with default dimensions
  - Enhanced help overlay rendering
  - Added vibrant colors to match v2
- Files: tui_view.go, tui_controller.go, tui_cmd.go

**Commit 491e6dc** (Sep 25, 2025)
- Type: Fix
- Title: "correct TUI version selection logic and navigation issues"
- Changes:
  - Fixed backwards --v1 vs default logic
  - Changed default from v1 to v2
  - Updated help text
- Files: tui_cmd.go (5 line changes)

**Commit 7ce5afa** (Sep 24, 2025)
- Type: Refactor
- Title: "split monolithic TUI model into MVC components"
- Changes:
  - Created tui_model.go, tui_view.go, tui_controller.go
  - Split 1084-line model.go into 923 lines of MVC + 65-line orchestrator
  - Fixed Enter key handling (ctrl+j compatibility)
  - Added mutex for thread safety
  - Restored URL resolution logic
  - Files: model.go, tui_controller.go, tui_model.go, tui_view.go, tui_utils.go (930 insertions, 1051 deletions)

**Commit 5bc613b** (Earlier)
- Type: Feature
- Title: "auto-recover running/hold downloads on startup"
- Details: Downloads persist across TUI restarts if marked as running/hold

### Release Highlights (v0.5.2)

- TUI v1 Enhanced with rich UI and borders
- Fixed critical startup issues
- Eliminated terminal escape sequence problems
- Enhanced colorful status indicators
- Fixed arrow key navigation
- Restored help system functionality

## 6. State Management

### TUI v1 (MVC)

**TUIModel state:**
```go
type TUIModel struct {
  cfg       *config.Config      // Configuration
  st        *state.DB           // SQLite database
  rows      []state.DownloadRow // Current downloads
  running   map[string]context.CancelFunc  // Active downloads (with cancel)
  ephems    map[string]ephemeral // Resolving/preflight state
  prev      map[string]obs       // Speed calculation state
}
```

**TUIController state:**
```go
type TUIController struct {
  model        *TUIModel
  view         *TUIView
  teaModel     tea.Model
  selected     int              // Current selection
  showInfo     bool             // Details pane visibility
  showHelp     bool             // Help overlay visibility
  menuOn       bool             // Menu visibility
  filterOn     bool             // Filter input visibility
  newDL        bool             // New download modal visibility
  newStep      int              // 1-4 step progress in modal
  // ... text inputs and other modal state
}
```

### TUI v2 (Monolithic)

**Model struct contains ~40+ fields:**
- Download rows and filters
- UI state (selected, filters, sort mode, grouping)
- Modal states (new job, batch import)
- Caches (rate, ETA, total bytes, host names)
- Theme and styling state
- Authentication status indicators
- Running downloads and retry tracking
- Toast notifications queue

## 7. Event Handling

### TUI v1 (Controller-Based)

Flow: `Update()` → `handleKeyMsg()` → specific handler
- `handleNormalKeys()` - Main navigation
- `handleHelpKeys()` - Help screen
- `handleMenuKeys()` - Menu navigation
- `handleNewDownloadKeys()` - 4-step wizard
- `handleFilterKeys()` - Filter input

### TUI v2 (Model-Based)

Flow: `Update()` → `updateNormal/NewJob/BatchMode/Filter()`
- Direct key handling in each modal context
- Tick messages for UI refresh
- Async commands for long operations

Both use Bubble Tea's message passing:
- `tickMsg` - Refresh timer
- `dlDoneMsg` - Download completion
- `errMsg` - Error notifications
- `metaMsg` - Resolver metadata
- `probeMsg` - Preflight probe results (v2)
- `destSuggestMsg` - Destination suggestions (v2)

## 8. Rendering Logic

### TUI v1 (View Class)

**Main rendering** (tui_view.go):
- `View()` - Main orchestrator
- `renderTable()` - Download list
- `renderDetails()` - Selected item details
- `helpView()` - Help overlay
- `menuView()` - Menu options
- `renderNewDownloadModal()` - Wizard UI
- `renderCommandsBar()` - Key hints

Styling: `uiStyles` struct with lipgloss styles
- Colors: Pink (selection), Green (complete), Red (error)
- Borders: Rounded borders with color 63

### TUI v2 (In-Model Rendering)

Rendering methods embedded in model.go:
- `View()` - Full screen layout
- `renderTable()` - Dynamic columns (URL/DEST/HOST)
- `renderNewJob()` - Modal UI
- `renderBatchMode()` - Batch import UI
- `renderFilter()` - Filter input
- `renderTabs()` - Tab navigation
- `renderTopRightStats()` - Stats panel
- `renderAuthStatus()` - Token status
- `renderToasts()` - Notification queue

Themes: 4 presets (default, neon, drac, solar)
- Customizable via config: `ui.theme`
- Switchable at runtime with 'T' key

## 9. TODO Items & Known Limitations

### From TODO_NEXT.md (Priority 1)

1. **TUI Enhancements**
   - Sort by speed/ETA/remaining ✅ (v2 has this)
   - Actions: retry/cancel/details ✅ (both have this)
   - Warnings for missing token env (partial)
   - Show SuggestedFilename from resolver (partial)
   - Improve colors ✅ (v1.0.5.2 restored these)

2. **Ephemeral Keying Issue** (High Priority)
   - TUI ephemerals should use URL|Dest composite key
   - Better isolation of resolving/preflight state
   - Avoid collisions when multiple jobs use same URL

3. **Path Traversal Prevention**
   - Sanitize `destGuess()` to prevent escaping download_root
   - Reject unsafe basenames: "/", ".", ".."
   - Enforce containment with filepath.Rel

4. **Enter Key Consistency**
   - TUI v2 might need `"ctrl+j"` handling like v1
   - Bubble Tea behavior varies by system

### From TODO.md (Backlog)

**Critical (Sprint 1-2):**
- MF-002: Refactor TUI v1 monolithic model (already done!)
- MF-005: Download progress persistence (auto-recover done)

**High Priority (Sprint 3-4):**
- Placement improvements
- Batch runner enhancements
- Partials management
- Metrics expansion

**Known Gaps:**
- TUI v1 progress bar doesn't show intermediate bytes (ProgressFor returns 0)
- URL resolution logic duplicated across multiple files
- Some error handling could be more consistent
- No mouse support (in backlog)

## 10. Key Findings Summary

### Strengths
1. ✅ Clean MVC architecture in v1 (recently refactored)
2. ✅ Feature-complete v2 with modern UI
3. ✅ Thread-safe state management with mutexes
4. ✅ Comprehensive error handling in downloads
5. ✅ Recent fixes for startup issues and navigation
6. ✅ Auto-recovery of downloads on startup
7. ✅ Rich theming system (v2)
8. ✅ Good documentation and TODOs

### Issues
1. ⚠️ TUI v2 monolithic design (2658 lines in one file)
2. ⚠️ Potential Enter/ctrl+j key handling inconsistency in v2
3. ⚠️ v1 progress bar shows 0 for intermediate progress
4. ⚠️ URL resolution logic duplicated
5. ⚠️ Some ephemeral state keying could be more robust

### Recommendations
1. Consider applying MVC pattern to v2 (or deprecate one version)
2. Verify Enter key handling works consistently across platforms in v2
3. Implement proper progress reporting in v1
4. Centralize URL resolution logic
5. Improve ephemeral state management with composite keys
6. Add more unit tests for TUI logic

---
*Analysis Date: 2025-11-09*
*Current Version: v0.5.2*
*Default Branch: main (commit c0061e2)*
