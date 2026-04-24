---
phase: 01-first-class-hf-snapshots-and-manifest-planner
plan: 01
subsystem: infra
tags: [downloader, sqlite, tui, lifecycle, finalization, recovery]
requires: []
provides:
  - Terminal download state now converges to `complete` even if post-finalization sidecar or directory fsync bookkeeping fails.
  - TUI active-job tracking is cleared on both success and failure.
  - Auto-recovery reconciles obviously finalized `running` rows instead of restarting them.
affects: [snapshot-planner, verification, recovery, tui]
tech-stack:
  added: []
  patterns: [terminal-state persistence helper, startup reconciliation of stale running rows]
key-files:
  created: [.planning/phases/01-first-class-hf-snapshots-and-manifest-planner/01-01-SUMMARY.md, internal/downloader/finalization_state_test.go, internal/tui/recovery_state_test.go]
  modified: [internal/downloader/chunked.go, internal/downloader/fsutil.go, internal/downloader/single.go, internal/tui/actions.go, internal/tui/model.go]
key-decisions:
  - "Treat sidecar write and directory fsync failures after a valid final artifact as nonterminal warnings, not reasons to strand the row in running."
  - "Clear TUI running-map state on every dlDoneMsg path, not just user-initiated cancellation."
  - "Do not auto-recover hold rows; only reconcile or resume rows that were marked running."
patterns-established:
  - "Downloader finalization should persist a terminal state before returning from nonfatal bookkeeping errors."
  - "Startup recovery must reconcile stale state with the filesystem before restarting work."
requirements-completed: [STATE-01, STATE-02]
duration: 39min
completed: 2026-04-23
---

# Phase 1 Plan 1 Summary

**Downloader terminal state now converges after finalization, with TUI cleanup and stale-running recovery hardened for real completed files**

## Performance

- **Duration:** 39 min
- **Started:** 2026-04-23T20:00:00Z
- **Completed:** 2026-04-23T20:39:00Z
- **Tasks:** 4
- **Files modified:** 7

## Accomplishments
- Successful finalized artifacts no longer remain stuck in `running` when post-finalization sidecar or directory fsync work fails.
- TUI `running` bookkeeping is cleared on both success and failure, fixing the “process still running” symptom.
- Startup recovery now reconciles stale `running` rows with finalized on-disk artifacts and avoids auto-resuming `hold` rows.
- Regression coverage was added for downloader terminal-state persistence and TUI recovery/cleanup behavior.

## Files Created/Modified
- `internal/downloader/single.go` - marks finalized downloads complete even when noncritical sidecar/fsync bookkeeping fails
- `internal/downloader/chunked.go` - applies the same terminal-state rule to chunked finalization
- `internal/downloader/fsutil.go` - exposes hookable helpers for finalization-path tests
- `internal/tui/actions.go` - reconciles stale running rows from finalized artifacts and stops auto-recovering hold rows
- `internal/tui/model.go` - clears in-memory running/retrying state when downloads finish
- `internal/downloader/finalization_state_test.go` - regression test for finalized file plus sidecar failure
- `internal/tui/recovery_state_test.go` - regression tests for TUI cleanup and stale-running recovery

## Decisions Made
- Noncritical bookkeeping failures after a valid final artifact should be recorded as warnings while preserving terminal success state.
- TUI running-state cleanup belongs in `dlDoneMsg` handling so all completion paths converge through one place.
- Startup recovery should reconcile filesystem truth before restarting background work.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- The likely lifecycle bug was split across downloader finalization ordering and separate TUI in-memory tracking, so both layers needed coordinated fixes.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Terminal lifecycle state is now trustworthy enough to support richer snapshot planner/state work.
- Phase 1 can proceed to manifest domain modeling and Hugging Face snapshot planning.

---
*Phase: 01-first-class-hf-snapshots-and-manifest-planner*
*Completed: 2026-04-23*
