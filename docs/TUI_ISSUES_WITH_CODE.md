# TUI Issues - With Code References

This document provides specific code snippets for each identified issue.

## Issue #1: TUI v1 Progress Bar Shows 0 Bytes

**Severity:** HIGH  
**File:** `/home/user/modfetch/internal/tui/tui_model.go`  
**Lines:** 101-108

**Problem:**
```go
// ProgressFor returns the progress information for a specific download.
func (m *TUIModel) ProgressFor(url, dest string) (int64, int64, string) {
    for _, row := range m.rows {
        if row.URL == url && row.Dest == dest {
            return 0, row.Size, row.Status  // ❌ ALWAYS returns 0 for current bytes!
        }
    }
    return 0, 0, "unknown"
}
```

**Impact:**
- Progress bars in TUI v1 never show intermediate progress
- Users always see "unknown" or 0% progress
- Only total size is shown, not completed bytes

**Why This Happens:**
- The method signature expects current bytes but returns hardcoded 0
- Progress calculation should come from download state, not just row status
- v2 has proper implementation with caches

**Recommended Fix:**
- Get actual bytes from state database (if tracked per chunk)
- Or use the `ephems` map to track in-progress bytes
- Track bytes downloaded per download_row

---

## Issue #2: TUI v2 May Not Handle ctrl+j Key

**Severity:** MEDIUM (Depends on Bubble Tea behavior)  
**File:** `/home/user/modfetch/internal/tui/v2/model.go`  
**Lines:** 402, 492

**Problem:**
```go
// In updateNewJob() method
func (m *Model) updateNewJob(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    s := msg.String()
    switch s {
    case "esc":
        m.newJob = false
        return m, nil
    // ... other cases ...
    case "enter":  // ❌ Only handles "enter", not "ctrl+j"
        val := strings.TrimSpace(m.newInput.Value())
        switch m.newStep {
        // ... handle steps ...
        }
    }
    // ...
}

// In updateBatchMode() method
func (m *Model) updateBatchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    s := msg.String()
    switch s {
    case "esc":
        m.batchMode = false
        return m, nil
    case "enter":  // ❌ Same issue here
        path := strings.TrimSpace(m.batchInput.Value())
        // ...
    }
}
```

**Comparison with TUI v1 (Fixed):**
```go
// In tui_controller.go - properly handles both
func (c *TUIController) handleNewDownloadKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "esc":
        c.newDL = false
        return c.wrapModel(), nil
    case "enter", "ctrl+j":  // ✅ Handles BOTH
        val := strings.TrimSpace(c.newInput.Value())
        // ...
    }
}
```

**Impact:**
- On some systems/terminals, Bubble Tea sends "ctrl+j" for Enter key
- TUI v2 modal dialogs could fail to respond to Enter key
- Users would be stuck in the modal unable to proceed

**Recommended Fix:**
```go
case "enter", "ctrl+j":  // Handle both variants
    // ... existing logic ...
```

---

## Issue #3: URL Resolution Logic Duplicated

**Severity:** MEDIUM (Code Quality)

### Location 1: `tui_controller.go` lines 329-363

```go
func (c *TUIController) resolveMetaCmd(raw string) tea.Cmd {
    return func() tea.Msg {
        s := strings.TrimSpace(raw)
        if s == "" {
            return metaMsg{url: raw}
        }
        if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
            if u, err := url.Parse(s); err == nil {
                h := strings.ToLower(u.Hostname())
                if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
                    // ... CivitAI URL normalization code ...
                }
            }
        }
        if strings.HasPrefix(s, "hf://") || strings.HasPrefix(s, "civitai://") {
            if res, err := resolver.Resolve(context.Background(), s, c.model.cfg); err == nil {
                return metaMsg{url: raw, fileName: res.FileName, suggested: res.SuggestedFilename, civType: res.FileType}
            }
        }
        return metaMsg{url: raw}
    }
}
```

### Location 2: `tui_model.go` lines 182-238

```go
func (m *TUIModel) StartDownload(ctx context.Context, urlStr, dest, sha string, headers map[string]string) error {
    // ... preflight checks ...
    
    resolved := urlStr
    if headers == nil {
        headers = map[string]string{}
    }

    if strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://") {
        if u, err := neturl.Parse(resolved); err == nil {
            h := strings.ToLower(u.Hostname())
            if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
                // ... DUPLICATED CivitAI URL normalization code ...
            }
            if hostIs(h, "huggingface.co") {
                // ... DUPLICATED HF URL normalization code ...
            }
        }
    }

    if strings.HasPrefix(resolved, "hf://") || strings.HasPrefix(resolved, "civitai://") {
        res, err := resolver.Resolve(ctx, resolved, m.cfg)
        if err != nil {
            return err
        }
        resolved = res.URL
        headers = res.Headers
    } else {
        // ... DUPLICATED auth token handling ...
    }
    // ...
}
```

### Location 3: `v2/model.go` - Multiple locations

Similar duplication exists in the v2 model for URL resolution.

**Problem:**
- Same CivitAI URL normalization appears in at least 3 places
- Same auth token handling logic duplicated
- Changes to URL handling must be made in multiple places
- Risk of inconsistency between versions

**Recommended Solution:**
Create a shared utility in `internal/tui/url_resolution.go`:

```go
// internal/tui/url_resolution.go
package tui

import (
    "net/url"
    "strings"
    "github.com/jxwalker/modfetch/internal/resolver"
)

// NormalizeURL converts civitai.com URLs to civitai:// URIs
func NormalizeURL(raw string) string {
    s := strings.TrimSpace(raw)
    if s == "" {
        return s
    }
    
    if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
        if u, err := url.Parse(s); err == nil {
            h := strings.ToLower(u.Hostname())
            if (h == "civitai.com" || strings.HasSuffix(h, ".civitai.com")) && 
               strings.HasPrefix(u.Path, "/models/") {
                // Convert to civitai://model/ID format
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
                        civ += "?version=" + url.QueryEscape(ver)
                    }
                    return civ
                }
            }
        }
    }
    return s
}
```

---

## Issue #4: TUI v2 Monolithic Design

**Severity:** MEDIUM (Architecture/Maintainability)  
**File:** `/home/user/modfetch/internal/tui/v2/model.go`  
**Lines:** All 2658 lines

**Problem:**
The entire TUI v2 is in one file with a Model struct containing 40+ fields:

```go
type Model struct {
    cfg             *config.Config      // Configuration
    st              *state.DB           // Database
    th              Theme               // Theming
    w, h            int                 // Dimensions
    build           string              // Build version
    activeTab       int                 // Current tab (0-4)
    rows            []state.DownloadRow // All downloads
    selected        int                 // Selected row index
    filterOn        bool                // Is filter active
    filterInput     textinput.Model     // Filter textbox
    sortMode        string              // "", "speed", "eta", "rem"
    groupBy         string              // "", "host"
    lastRefresh     time.Time           // Last UI refresh
    prog            progress.Model      // Progress bar
    prev            map[string]obs      // Speed calculation state
    prevStatus      map[string]string   // Previous status tracking
    running         map[string]context.CancelFunc  // Active downloads
    selectedKeys    map[string]bool     // Multi-select state
    toasts          []toast             // Notification queue
    showToastDrawer bool
    showHelp        bool
    showInspector   bool
    // ... 20+ more fields ...
    newJob          bool                // New download modal
    newStep         int                 // Step in wizard
    newInput        textinput.Model     // Modal input
    // ... many more ...
}
```

**Mixed Concerns:**
- Rendering methods mixed with state management
- Event handling mixed with business logic
- All state (UI, data, caches) in one place
- Makes unit testing very difficult

**Impact:**
- Difficult to test individual components
- Hard to code review
- Increased cognitive load to understand
- Risky to modify without breaking something else

**Contrast with TUI v1:**
TUI v1 separated these cleanly:
```
TUIModel (298 lines)  → Data & Business Logic
TUIView (249 lines)   → Pure Rendering
TUIController (367)   → Event Handling & State Transitions
model.go (65 lines)   → Orchestration via tea.Model interface
```

**Recommended Solution:**
Apply the TUI v1 pattern to v2:
1. Extract rendering to TUIViewV2 (all `render*()` methods)
2. Extract event handling to TUIControllerV2 (all `update*()` methods)
3. Extract data access to TUIModelV2
4. Keep a thin orchestrator to tie them together

---

## Issue #5: Ephemeral State Keying Could Be More Robust

**Severity:** LOW-MEDIUM (Edge Case)  
**File:** `/home/user/modfetch/internal/tui/tui_model.go` lines 26-27, 95-98

**Current Implementation:**
```go
type TUIModel struct {
    // ...
    ephems map[string]ephemeral  // ❌ Keyed only by URL
}

// AddEphemeral adds an ephemeral download state
func (m *TUIModel) AddEphemeral(url, dest string, headers map[string]string, sha string) {
    m.ephems[url+"|"+dest] = ephemeral{url: url, dest: dest, headers: headers, sha: sha}
    // ✅ Actually DOES use composite key!
}
```

Wait, I see the code actually DOES use composite keys (`url+"|"+dest`)!  
But this is documented in TODO_NEXT.md as something to verify/improve.

**Concern:**
The documentation suggests this needs better isolation to avoid:
- Collisions when multiple jobs use same URL
- Proper clearing when rows complete
- Race conditions during transitions

**Current Safeguard:** ✅ Uses composite key `url|dest`

**Could Be Improved:**
- Add helper function `func ephemeralKey(url, dest string) string`
- Ensure all call sites use the helper
- Add tests for collision scenarios

---

## Issue #6: Inconsistent Error Handling

**Severity:** LOW (UX Issue)

**Examples:**

### TUI v1 - Basic error handling:
```go
case errMsg:
    return c.wrapModel(), nil  // ❌ Just swallows the error
```

### TUI v2 - Better error handling with toasts:
```go
case dlDoneMsg:
    if msg.err != nil {
        m.err = msg.err
        m.addToast("failed: " + msg.err.Error())  // ✅ Shows user
        // ... log and handle ...
        return m, m.refresh()
    }
```

**Recommendation:**
- TUI v1 should implement toast-like notifications for errors
- All async operations should properly bubble errors
- User should see meaningful error messages

---

## Summary Table

| Issue | Severity | Type | File | Lines | Fix Effort |
|-------|----------|------|------|-------|-----------|
| #1: Progress shows 0 bytes | HIGH | Bug | tui_model.go | 101-108 | Medium |
| #2: ctrl+j key handling | MEDIUM | Bug | v2/model.go | 402,492 | Low |
| #3: URL logic duplicated | MEDIUM | Quality | multiple | multiple | Medium |
| #4: v2 monolithic | MEDIUM | Design | v2/model.go | all | High |
| #5: Ephemeral keying | LOW | Design | tui_model.go | 26-98 | Low |
| #6: Error handling | LOW | UX | both | both | Low |

---

**Analysis Date:** Nov 9, 2025  
**Version:** v0.5.2  
**Status:** All issues documented with code references for easy fixing
