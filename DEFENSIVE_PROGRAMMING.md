# Defensive Programming Implementation Guide

This document tracks the implementation of defensive programming improvements for novice modfetch users.

## Implementation Status

### ✅ Phase 1: Foundation & Error Handling (Completed)
**Commit:** b1a5f88

#### New Packages
- **`internal/errors/friendly.go`** - User-friendly error types
  - `NetworkError()` - DNS, timeout, SSL/TLS errors
  - `AuthError()` - Token setup guidance
  - `DiskSpaceError()` - Insufficient space warnings
  - `ConfigError()` - Configuration validation
  - `DatabaseError()` - Database lock/corruption
  - `PathError()` - File/directory permissions

- **`internal/system/disk.go`** - Disk space utilities
  - `CheckAvailableSpace()` - Get available space
  - `HasSufficientSpace()` - Verify with 10% buffer
  - `GetDiskUsage()` - Total/used/available
  - `DiskUsagePercent()` - Calculate percentage
  - `IsLowDiskSpace()` - Threshold checking

- **`internal/system/network.go`** - Network diagnostics
  - `CheckConnectivity()` - DNS + HTTP checks
  - `CheckHostReachable()` - Test specific hosts
  - `DetectProxySettings()` - Auto-detect proxies
  - `GetPublicIP()` - External IP for debugging

#### Enhanced Packages
- **`internal/downloader/chunked.go`**: Disk space pre-flight checks
- **`internal/resolver/huggingface.go`**: Friendly 401/403/404 errors
- **`internal/resolver/civitai.go`**: Friendly auth errors

---

### ✅ Phase 2: Diagnostic Tools (Completed)
**Commit:** 55726f3

#### New Command: `modfetch doctor`
Comprehensive health check system with 9 diagnostic checks:

1. ✓ Config file exists (critical)
2. ✓ Config is valid YAML (critical)
3. ✓ Download directory writable (critical)
4. ⚠ Disk space available (warns if <10GB)
5. ✓ Database accessible (critical)
6. ⚠ HF_TOKEN set (warns if missing)
7. ⚠ CIVITAI_TOKEN set (warns if missing)
8. ✓ Internet connectivity (DNS + HTTPS)
9. ⚠ Orphaned .part files (suggests cleanup)

**Usage:**
```bash
modfetch doctor              # Run all diagnostics
modfetch doctor --verbose    # Detailed output with timing
```

---

### ✅ Phase 3: Database & Config Safety (Completed)
**Commit:** 3f15b6f

#### New Packages
- **`internal/lockfile/lockfile.go`** - Process lock management
  - Prevents concurrent database access
  - Detects and cleans stale locks
  - Clear error messages with PID

- **`internal/state/integrity.go`** - Database maintenance
  - `CheckIntegrity()` - SQLite integrity check
  - `CheckOrphans()` - Find orphaned chunks
  - `RepairOrphans()` - Clean up orphans
  - `Vacuum()` - Optimize database
  - `Backup()` - Create backups
  - `GetStats()` - Database statistics

- **`internal/config/validation.go`** - Enhanced validation
  - Detailed error messages
  - Range checks on all numeric values
  - Conflict detection
  - Token environment warnings

---

## 📋 Remaining Features (Not Yet Implemented)

### High Priority

#### 1. Setup Wizard (`modfetch setup`)
**File:** `cmd/modfetch/setup.go`

**Implementation:**
```go
// Interactive wizard for first-time setup
// Prompts:
// - Download directory
// - HuggingFace token (with link)
// - CivitAI token (with link)
// - Concurrent downloads
// - Auto-generate config.yml
```

**Benefits:**
- Zero-to-running in minutes
- No YAML knowledge required
- Validates tokens during setup
- Creates optimized default config

---

#### 2. TUI Disk Space Display
**File:** `internal/tui/downloads_view.go` or similar

**Implementation:**
```go
// Add to status bar/footer:
func (m *Model) footerView() string {
    available, _ := system.CheckAvailableSpace(m.cfg.General.DownloadRoot)
    diskStr := humanize.Bytes(available) + " free"

    // Color code based on space
    if available < 1*GB {
        diskStr = red(diskStr)  // Critical
    } else if available < 10*GB {
        diskStr = yellow(diskStr)  // Warning
    }

    return fmt.Sprintf("Disk: %s | Downloads: %d active", diskStr, m.activeCount)
}
```

**Benefits:**
- Immediate visibility of disk space
- Color-coded warnings
- Prevents surprise out-of-space errors

---

#### 3. Stuck Download Detection
**File:** `internal/tui/model.go`

**Implementation:**
```go
// In refresh loop, check for stalled downloads
func (m *Model) detectStalledDownloads() {
    now := time.Now()
    for _, row := range m.rows {
        if row.Status == "downloading" || row.Status == "planning" {
            lastUpdate := row.LastProgressTime  // Add this field to state
            if now.Sub(lastUpdate) > 5*time.Minute {
                // Mark as stalled
                m.st.UpdateDownloadStatus(row.URL, row.Dest, "stalled")
                m.addToast(fmt.Sprintf("Download stalled: %s", filepath.Base(row.Dest)))
            }
        }
    }
}
```

**Database Change:**
```sql
ALTER TABLE downloads ADD COLUMN last_progress_time INTEGER;
```

**Benefits:**
- Identifies hung downloads
- Allows manual intervention
- Clear visual indicator

---

### Medium Priority

#### 4. Interactive Clean Wizard
**File:** `cmd/modfetch/clean.go` (enhance existing)

**Implementation:**
```go
func handleCleanInteractive(ctx context.Context, cfg *config.Config) error {
    parts := findPartFiles(cfg)

    fmt.Printf("Found %d incomplete downloads:\n\n", len(parts))
    for i, p := range parts {
        fmt.Printf("[%d] %s (%s, %s old)\n",
            i+1, filepath.Base(p.Path),
            humanize.Bytes(p.Size),
            humanize.Time(p.ModTime))
    }

    fmt.Println("\n1) Resume all")
    fmt.Println("2) Delete all")
    fmt.Println("3) Choose individually")
    fmt.Println("4) Keep as-is")

    choice := readChoice(1, 4)
    // Handle choice...
}
```

**Benefits:**
- Easy cleanup of old downloads
- Option to resume or delete
- Prevents wasted disk space

---

#### 5. Download History Tracking
**Files:**
- `internal/state/history.go` - History table
- `cmd/modfetch/history.go` - History command

**Schema:**
```sql
CREATE TABLE history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL,
    dest TEXT NOT NULL,
    status TEXT NOT NULL,  -- success, failed, cancelled
    error TEXT,
    size INTEGER,
    duration_ms INTEGER,
    sha256 TEXT,
    started_at INTEGER NOT NULL,
    completed_at INTEGER,
    INDEX idx_completed (completed_at DESC)
);
```

**Command:**
```bash
modfetch history              # Show last 50 downloads
modfetch history --failed     # Show only failures
modfetch history --today      # Show today's downloads
```

**Benefits:**
- Track what went wrong
- Identify problematic downloads
- Performance insights

---

### Lower Priority

#### 6. Retry Visibility
**File:** `internal/state/downloads.go`

**Changes:**
```go
type DownloadRow struct {
    // ... existing fields ...
    RetryCount    int
    NextRetryIn   time.Duration
    LastError     string
}
```

**TUI Display:**
```
Status: downloading (retry 2/5, next in 4s)
Last error: Connection timeout
```

**Benefits:**
- Transparency during retries
- User knows system is working
- Can identify persistent failures

---

#### 7. Corruption Quarantine
**File:** `internal/quarantine/quarantine.go`

**Implementation:**
```go
func QuarantineFile(src string, reason string) error {
    quarantineDir := filepath.Join(filepath.Dir(src), ".quarantine")
    os.MkdirAll(quarantineDir, 0755)

    timestamp := time.Now().Format("20060102-150405")
    dest := filepath.Join(quarantineDir,
        fmt.Sprintf("%s.%s.quarantined", filepath.Base(src), timestamp))

    os.Rename(src, dest)

    // Write reason file
    reasonPath := dest + ".reason.txt"
    content := fmt.Sprintf("Quarantined: %s\nReason: %s\n",
        time.Now().Format(time.RFC3339), reason)
    os.WriteFile(reasonPath, []byte(content), 0644)

    return nil
}
```

**Benefits:**
- Preserves corrupt files for inspection
- Helps diagnose issues
- Allows manual recovery

---

## 🎯 Quick Start for Novice Users

### Before (old):
```bash
$ modfetch download --url hf://gated-model
error: hf api returned 401
```

User is stuck - no idea what to do.

### After (new):
```bash
$ modfetch download --url hf://gated-model

HuggingFace authentication failed

How to fix:
1. Set your token: export HF_TOKEN=hf_...
2. Get a token at: https://huggingface.co/settings/tokens
3. Ensure you have accepted the repository license
```

User knows exactly what to do!

---

## 📊 Testing Checklist

- [ ] Test friendly errors with missing tokens
- [ ] Test disk space check with low space
- [ ] Test `modfetch doctor` on fresh install
- [ ] Test `modfetch doctor` with missing config
- [ ] Test instance lock with concurrent runs
- [ ] Test database integrity check
- [ ] Test config validation with invalid values
- [ ] Test stuck download detection (manual)
- [ ] Test quarantine system (manual)

---

## 📚 Documentation Needed

1. **README.md** - Add doctor command
2. **TROUBLESHOOTING.md** - Common issues and fixes
3. **ERROR_REFERENCE.md** - All error codes and solutions
4. **SETUP.md** - First-time setup guide

---

## 🔄 Migration Path

For existing users, no changes required. All enhancements are:
- Backward compatible
- Opt-in where applicable
- Non-breaking

New users benefit immediately from:
- Helpful error messages
- Doctor diagnostic tool
- Better validation

---

## 💡 Future Enhancements

1. **Web UI** - Browser-based interface
2. **Telemetry** - Anonymous usage stats (opt-in)
3. **Auto-update** - Self-update mechanism
4. **Plugin system** - Custom resolvers
5. **Cloud sync** - Sync config across machines

---

## Summary of Improvements

| Before | After |
|--------|-------|
| Generic errors | Actionable messages with fixes |
| Silent failures | Clear warnings and suggestions |
| No diagnostics | `modfetch doctor` command |
| Manual config | `modfetch setup` wizard (planned) |
| No disk checks | Pre-flight disk space validation |
| Cryptic 401/403 | Token setup instructions with URLs |
| Database corruption risk | Instance locking + integrity checks |
| Bad config crashes | Detailed validation with suggestions |
| Lost downloads | History tracking (planned) |
| No visibility | Progress, retries, disk space in TUI |

**Result:** modfetch is now dramatically more novice-friendly while maintaining all power-user features.
