# TUI Testing Guide

## Comprehensive TUI Test Coverage

### Automated Test Summary

The TUI is now comprehensively tested with automated tests covering all major functionality:

#### 1. Navigation Tests (`internal/tui/navigation_test.go`)

**Tab Switching** - Tests all tab navigation:
- ✅ View All (key `0`) - activeTab = -1
- ✅ Pending (key `1`) - activeTab = 0
- ✅ Active (key `2`) - activeTab = 1
- ✅ Completed (key `3`) - activeTab = 2
- ✅ Failed (key `4`) - activeTab = 3
- ✅ Library (keys `5` or `l`) - activeTab = 4
- ✅ Settings (keys `6` or `m`) - activeTab = 5

**Library Navigation** - Tests movement within library:
- ✅ Navigate down with `j` key
- ✅ Navigate down with down arrow
- ✅ Navigate up with `k` key
- ✅ Navigate up with up arrow
- ✅ Boundary checking (cannot go below 0 or above max)

**Detail View Navigation**:
- ✅ Enter detail view with `Enter` key
- ✅ Exit detail view with `Esc` key

**Search Functionality**:
- ✅ Activate search with `/` key
- ✅ Cancel search with `Esc` key

**UI Toggles**:
- ✅ Help menu toggle with `?` key
- ✅ Inspector panel toggle with `i` key

**System Events**:
- ✅ Window resize handling
- ✅ Quit with `Ctrl+C` or `q`
- ✅ Selection reset on tab change

#### 2. Library Tests (`internal/tui/library_test.go`)

**Data Management**:
- ✅ Refresh library data (empty state)
- ✅ Refresh library data (with models)
- ✅ Library data with filters (source, type, favorites, ratings, tags)
- ✅ Library search functionality
- ✅ Pagination handling

**Rendering**:
- ✅ Render empty library
- ✅ Render library with models
- ✅ Render pagination controls
- ✅ Render selection highlighting
- ✅ Render filter indicators
- ✅ Render detail view
- ✅ Render minimal metadata

**User Interactions**:
- ✅ Search activation and submission
- ✅ Search cancellation
- ✅ Scan directories command
- ✅ Selection bounds checking
- ✅ Favorite display
- ✅ Source color coding
- ✅ Long model name truncation
- ✅ Theme application

#### 3. Library Integration Tests (`internal/tui/library_integration_test.go`)

**End-to-End Workflows**:
- ✅ Scan → Database → Library flow (file discovery to UI display)
- ✅ Metadata fetching and storage
- ✅ Search and filtering integration
- ✅ Duplicate detection
- ✅ Favorite management workflow
- ✅ Metadata registry integration

#### 4. Settings Tests (`internal/tui/settings_test.go`)

**Settings Display**:
- ✅ Basic settings rendering
- ✅ Directory paths display
- ✅ Token status (not set, set, rejected, disabled)
- ✅ Authentication status (compact view)
- ✅ Placement rules display
- ✅ Download settings
- ✅ UI preferences
- ✅ Validation settings
- ✅ Boolean rendering
- ✅ Footer display
- ✅ Custom token environment variables
- ✅ Compact view toggle
- ✅ Theme rendering

#### 5. Inspector Tests (`internal/tui/model_inspector_test.go`)

**Progress Display**:
- ✅ Completed download shows average speed
- ✅ Running download shows start time

#### 6. Metadata Tests (`internal/tui/metadata_test.go`)

**Metadata Operations**:
- ✅ Fetch and store metadata
- ✅ Metadata filtering
- ✅ Usage tracking with timestamps

#### 7. Model Tests (`internal/tui/model_test.go`)

**Modal Handling**:
- ✅ New job modal escape
- ✅ Batch mode escape
- ✅ Filter modal escape
- ✅ Help toggle with `?`

---

## Running the Tests

### Run All TUI Tests
```bash
go test -v ./internal/tui
```

### Run Specific Test Categories

**Navigation Tests Only:**
```bash
go test -v -run TestNavigation ./internal/tui
```

**Library Tests Only:**
```bash
go test -v -run TestLibrary ./internal/tui
```

**Settings Tests Only:**
```bash
go test -v -run TestSettings ./internal/tui
```

**Integration Tests Only:**
```bash
go test -v -run TestIntegration ./internal/tui
```

### Run with Coverage
```bash
go test -v -cover ./internal/tui
```

### Run with Coverage Report
```bash
go test -coverprofile=coverage.out ./internal/tui
go tool cover -html=coverage.out
```

---

## Manual Testing Guide

For manual verification of the TUI, follow this checklist:

### 1. Navigation Testing

**Tab Switching:**
- [ ] Press `0` - View All tab
- [ ] Press `1` - Pending tab
- [ ] Press `2` - Active tab
- [ ] Press `3` - Completed tab
- [ ] Press `4` - Failed tab
- [ ] Press `5` or `l` - Library tab
- [ ] Press `6` or `m` - Settings tab

**Library Navigation:**
- [ ] In Library tab, press `j` to move down
- [ ] Press `k` to move up
- [ ] Press down arrow to move down
- [ ] Press up arrow to move up
- [ ] Try to go past the bottom (should stay at last item)
- [ ] Try to go above the top (should stay at first item)

**Detail View:**
- [ ] In Library, press `Enter` on a model
- [ ] Verify detail view displays
- [ ] Press `Esc` to exit detail view

**Search:**
- [ ] In Library, press `/` to activate search
- [ ] Type a search query
- [ ] Press `Enter` to search
- [ ] Press `/` again and `Esc` to cancel

### 2. UI Features Testing

**Help Menu:**
- [ ] Press `?` to show help
- [ ] Press `?` again to hide help
- [ ] Verify all keybindings are documented

**Inspector Panel:**
- [ ] Press `i` to toggle inspector
- [ ] Verify download status is shown
- [ ] Press `i` again to hide

**Window Resize:**
- [ ] Resize terminal window
- [ ] Verify TUI adapts to new size
- [ ] Check all panels render correctly

### 3. Download Status Testing

**Progress Display:**
- [ ] Start a download
- [ ] Verify progress bar displays
- [ ] Verify speed calculation shows
- [ ] Verify ETA displays
- [ ] Check completed downloads show in Completed tab

**Filtering:**
- [ ] Filter downloads by status (Pending, Active, Completed, Failed)
- [ ] Verify counts are correct in each tab
- [ ] Verify "All" shows everything

### 4. Library Features Testing

**Model Display:**
- [ ] Verify model metadata displays correctly
- [ ] Check file size formatting
- [ ] Check quantization display
- [ ] Check parameter count display

**Filtering:**
- [ ] Test source filter (HuggingFace, CivitAI, etc.)
- [ ] Test model type filter (LLM, LoRA, etc.)
- [ ] Test favorites filter
- [ ] Test rating filter
- [ ] Test tag filter

**Actions:**
- [ ] Press `S` to scan directories
- [ ] Verify scan results appear
- [ ] Toggle favorite status
- [ ] Verify favorites persist

### 5. Settings Testing

**Display:**
- [ ] Verify all settings categories display
- [ ] Check token status indicators
- [ ] Verify directory paths are shown
- [ ] Check placement rules display

**Configuration:**
- [ ] Verify download settings are accurate
- [ ] Check UI preferences display
- [ ] Verify validation settings show

### 6. Stress Testing

**Large Dataset:**
- [ ] Add 100+ models to library
- [ ] Test scrolling performance
- [ ] Test search with large dataset
- [ ] Test filtering with large dataset

**Long Running Downloads:**
- [ ] Start multiple downloads simultaneously
- [ ] Verify all show progress correctly
- [ ] Test cancellation
- [ ] Test resuming

---

## Test Coverage Metrics

Run this to see current test coverage:
```bash
go test -cover ./internal/tui
```

Current coverage (as of latest test run):
- Navigation: ✅ Fully tested
- Library: ✅ Fully tested
- Settings: ✅ Fully tested
- Inspector: ✅ Basic coverage
- Metadata: ✅ Fully tested
- Integration: ✅ Key workflows tested

---

## Continuous Testing

### Pre-commit Testing
Run before committing changes:
```bash
go test ./internal/tui && echo "✅ TUI tests passed"
```

### CI/CD Integration
The test suite runs automatically in CI/CD:
```bash
go test -v -race -coverprofile=coverage.out ./internal/tui
```

---

## Known Limitations

1. **Modal Interactions**: New download modal and batch import modal are tested for escape/cancel but not full workflows
2. **Toast Notifications**: Toast display is not explicitly tested
3. **Themes**: Theme switching is not tested (rendering is tested)
4. **Download Inspector**: Only basic progress display is tested, not all edge cases

---

## Future Test Improvements

- [ ] Add tests for new download modal workflow
- [ ] Add tests for batch import workflow
- [ ] Add tests for toast notification display and timing
- [ ] Add tests for theme switching
- [ ] Add tests for download inspector edge cases (retries, errors, etc.)
- [ ] Add performance benchmarks for large datasets
- [ ] Add visual regression tests (snapshots)
