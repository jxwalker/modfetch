# Prioritised Enhancement List for modfetch

This document tracks the backlog of enhancements, grouped by priority. It mirrors the current product backlog and is intended to persist outside any session.

## Priority 1: Critical Reliability & Performance

1. Memory-Efficient Hashing
   - Current: Loading entire files into memory for SHA256 verification
   - Impact: Cannot handle files larger than available RAM
   - Fix: Stream-based hashing with fixed buffer size

2. Concurrent Download Recovery
   - Current: TUI-initiated downloads lose context on restart
   - Impact: Lost progress if TUI crashes
   - Fix: Persist download contexts in state DB with recovery on startup

3. Chunk Corruption Recovery
   - Current: Re-downloads entire chunk on corruption
   - Impact: Wastes bandwidth on large chunks
   - Fix: Binary search within corrupted chunks to find exact corruption point

4. Database Transaction Boundaries
   - Current: Individual operations without transaction grouping
   - Impact: Potential inconsistent state on crashes
   - Fix: Wrap related operations in transactions

## Priority 2: Core Functionality Gaps

5. Download Bandwidth Throttling
   - Current: No bandwidth limits
   - Impact: Saturates connection, affects other users
   - Fix: Token bucket rate limiter per download and global

6. Mirror/Fallback URLs
   - Current: Single URL per download
   - Impact: Fails if primary source unavailable
   - Fix: Ordered list of URLs with automatic failover

7. Partial File Verification
   - Current: Only verifies complete files
   - Impact: Cannot detect corruption until download completes
   - Fix: Periodic checkpoints with partial verification

8. Connection Pool Management
   - Current: Creates new HTTP clients per downloader
   - Impact: Unnecessary overhead and connection establishment
   - Fix: Shared connection pool with per-host limits

## Priority 3: User Experience

9. TUI Model Refactoring
   - Current: 600+ line monolithic model.go
   - Impact: Hard to maintain and test
   - Fix: Split into model, view, controller components

10. Progress Persistence Across Sessions
    - Current: Progress resets on restart
    - Impact: Cannot track long-term download statistics
    - Fix: Historical progress tracking in state DB

11. Smart Retry Logic
    - Current: Fixed exponential backoff
    - Impact: Not optimal for different failure types
    - Fix: Adaptive retry based on error type (network vs server)

12. Download Queue Management
    - Current: No queue prioritisation
    - Impact: Cannot control download order
    - Fix: Priority queue with drag-and-drop reordering in TUI

## Priority 4: Quality Improvements

13. Comprehensive Error Context
    - Current: Generic error messages
    - Impact: Hard to diagnose issues
    - Fix: Structured errors with context, suggestions, and error codes

14. Test Coverage
    - Current: ~30% coverage estimated
    - Impact: Regressions likely
    - Fix: Unit tests for resolvers, state, placer packages

15. Metrics Collection
    - Current: Basic Prometheus metrics
    - Impact: Limited observability
    - Fix: Detailed per-download metrics, success rates, performance percentiles

16. Configuration Validation
    - Current: Basic validation
    - Impact: Runtime failures from bad config
    - Fix: Comprehensive validation with helpful error messages

## Priority 5: Advanced Features

17. Archive Extraction
    - Current: No post-download processing
    - Impact: Manual extraction required
    - Fix: Automatic extraction with progress tracking

18. Duplicate Detection
    - Current: Re-downloads existing files
    - Impact: Wasted bandwidth and storage
    - Fix: Content-addressable storage with deduplication

19. S3-Compatible Backend
    - Current: Local filesystem only
    - Impact: Limited to single machine
    - Fix: S3 backend for distributed storage

20. Download Scheduling
    - Current: Immediate execution only
    - Impact: Cannot schedule for off-peak
    - Fix: Cron-like scheduler with rate limits per time window

## Priority 6: Code Architecture

21. Context Propagation Pattern
    - Current: Context passed individually
    - Impact: Verbose and error-prone
    - Fix: Context-aware client pattern

22. Plugin Architecture
    - Current: Hard-coded resolvers
    - Impact: Cannot extend without modifying core
    - Fix: Plugin interface for custom resolvers

23. Event-Driven Architecture
    - Current: Polling for status updates
    - Impact: Inefficient and delayed updates
    - Fix: Event bus for real-time updates

## Quick Wins (Can be done immediately)

- Add `--dry-run` flag to download command
- Add `--force` flag to skip SHA256 verification
- Add download time estimation to CLI output
- Fix TUI selected item persistence when filtering
- Add Ctrl+C graceful shutdown handler
- Add `--quiet` flag that actually suppresses all non-error output
- Fix progress bar showing 100% during chunk planning phase

## Technical Debt Items

- Remove duplicate `SafeFileName` implementations
- Consolidate HTTP client creation
- Standardise error wrapping patterns
- Remove dead code in TUI model
- Fix inconsistent mutex usage in metrics package
- Resolve TODO comments (14 found)
- Fix potential goroutine leak in progress display

## Performance Optimisations

- Pre-allocate file space to prevent fragmentation
- Use `sendfile` syscall for copies where available
- Implement parallel chunk verification
- Cache DNS resolutions for repeated downloads
- Use HTTP/2 server push where supported
- Implement adaptive chunk sizing based on throughput

## Breaking Changes to Consider for v1.0

- Restructure config schema for clarity
- Change state DB schema for better querying
- Standardise CLI flags across commands
- Move from positional to flag-based arguments
- Rename packages for clarity (e.g., `placer` -> `placement`)

