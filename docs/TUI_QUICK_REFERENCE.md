# TUI Quick Reference Guide

## File Locations

### TUI v1 (Refactored MVC - Experimental)
```
internal/tui/
├── model.go              # 65 lines - MVC orchestrator
├── tui_model.go          # 298 lines - Business logic & state
├── tui_view.go           # 249 lines - Rendering
├── tui_controller.go     # 367 lines - Event handling
└── tui_utils.go          # 16 lines - Helpers
```

### TUI v2 (Monolithic - Default)
```
internal/tui/v2/
└── model.go              # 2658 lines - Everything
```

### Entry Point
```
cmd/modfetch/tui_cmd.go   # Default: v2, Flag --v1 for v1
```

## Architecture Comparison

| Aspect | TUI v1 | TUI v2 |
|--------|--------|--------|
| Pattern | MVC | Monolithic |
| Size | ~930 lines | 2658 lines |
| Default | No (--v1 flag) | Yes (default) |
| Thread Safe | ✅ Mutex protected | ✅ (needs verification) |
| Progress Display | ❌ Shows 0 bytes | ✅ Real-time |
| Themes | 1 (default) | 4 presets |
| Testing | ✅ Component-level | ⚠️ Full integration |

## Key Features

### Both Support
- Multi-tab interface (Pending, Active, Completed, Failed)
- Live speed/ETA with smoothing
- Filtering & sorting
- New download modal (4 steps)
- Batch import from text files
- Auto-placement
- Arrow keys & Vim navigation (j/k)

### TUI v2 Only
- Toast notifications
- Theme switching at runtime ('T' key)
- Inspector panel ('i' key)
- Rate limit detection
- Authentication status display

## Frameworks Used

```
github.com/charmbracelet/bubbletea     # UI framework
github.com/charmbracelet/bubbles       # Components (textinput, progress)
github.com/charmbracelet/lipgloss      # Terminal styling
github.com/dustin/go-humanize          # Human-readable sizes
```

## Known Issues

### Critical
- ❌ TUI v1 progress bar shows 0 bytes (technical limitation)
- ⚠️ TUI v2 may not handle ctrl+j key (potential Bubble Tea incompatibility)

### Design
- ⚠️ TUI v2 is monolithic (hard to test, maintain)
- ⚠️ URL resolution logic duplicated across files

### Minor
- ⚠️ Some error cases could bubble up better
- ⚠️ Ephemeral state keying could be more robust

## Recent Fixes (v0.5.2 - Sept 2025)

| Commit | Issue | Fix |
|--------|-------|-----|
| a435afc | v1 loading hang | Set default window dimensions |
| a435afc | Missing UI elements | Added borders, colors, styling |
| 491e6dc | Backwards v1/v2 logic | Fixed default to v2, --v1 flag |
| 7ce5afa | Enter key hang | Added ctrl+j handling |

## State Management Patterns

### TUI v1 (Separated)
- **Model**: Config, database, downloads, running tasks
- **Controller**: User input, UI state (selected, modals)
- **View**: Pure rendering functions

### TUI v2 (Unified)
- Single Model struct with 40+ fields
- State includes: rows, filters, modals, caches, themes, auth status

## Rendering Methods

### TUI v1
```
View() → renderTable() + renderDetails() + renderCommandsBar()
         + menuView() (if menu) + helpView() (if help)
```

### TUI v2
```
View() → renderTabs() + renderTable() + renderNewJob() + renderFilter()
         + renderToasts() + renderAuthStatus()
```

## Message Types (Bubble Tea)

```go
tickMsg            // UI refresh timer (1Hz default)
dlDoneMsg          // Download completed
metaMsg            // URL metadata from resolver
errMsg             // Error notification
destSuggestMsg     // Destination suggestion (v2)
probeMsg           // Preflight probe result (v2)
recoverRowsMsg     // Auto-recovery at startup (v2)
```

## Command Handler Flow

```
modfetch tui [--config CONFIG] [--v1]
    ↓
handleTUI() in tui_cmd.go
    ↓
Load/create config
    ↓
Open state database (SQLite)
    ↓
Select v1 or v2
    ↓
tea.NewProgram(model).Run()
```

## Caching & Performance (v2)

TUI v2 implements several caches to avoid re-computation:
- `rateCache[key]` - Download speed
- `etaCache[key]` - Estimated time remaining
- `totalCache[key]` - Total bytes
- `curBytesCache[key]` - Current bytes downloaded
- `rateHist[key]` - Speed history for sparklines
- `hostCache[key]` - Hostname lookups

## Default Keybindings

| Key | Action |
|-----|--------|
| j/↓ | Move down |
| k/↑ | Move up |
| n | New download |
| b | Batch import |
| / | Filter |
| ? | Help |
| q | Quit |
| m | Menu (v1) |
| s | Sort by speed (v2) |
| e | Sort by ETA (v2) |
| g | Group by status (v2) |
| t | Toggle column (v2) |
| T | Change theme (v2) |
| i | Inspector (v2) |
| H | Toast drawer (v2) |
| y/r | Retry download |
| p | Pause/cancel |
| O | Open in file manager |
| C | Copy path to clipboard |
| U | Copy URL to clipboard |
| D | Delete staged file |
| X | Clear/reset row |

## Testing & Development

### Run TUI v1
```bash
./bin/modfetch tui --config ~/.config/modfetch/config.yml --v1
```

### Run TUI v2 (default)
```bash
./bin/modfetch tui --config ~/.config/modfetch/config.yml
```

### Run Tests
```bash
go test ./internal/tui/...
go test ./internal/tui/v2/...
```

## TODO & Future Work

**High Priority:**
1. Add ctrl+j handling to TUI v2 for Bubble Tea consistency
2. Fix TUI v1 progress reporting (currently shows 0 bytes)
3. Refactor TUI v2 with MVC pattern (or deprecate v1)
4. Centralize URL resolution logic
5. Improve ephemeral state keying with composite keys

**Medium Priority:**
1. Add more unit tests for rendering logic
2. Document theme system better
3. Add mouse support
4. Improve error message bubbling

---
See `/home/user/modfetch/docs/TUI_ARCHITECTURE_ANALYSIS.md` for detailed analysis.
