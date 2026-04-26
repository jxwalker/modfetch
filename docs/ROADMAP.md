# Roadmap (Consolidated)

This document consolidates the project backlog and roadmap from multiple sources into a single, prioritized list. Source docs: docs/BACKLOG.md, docs/TODO.md, docs/TODO_NEXT.md, README.md (Roadmap), docs/PRD.md, docs/backlog/.

Status key: [NEW] newly captured, [IMPL] implemented already (kept here for traceability), [WIP] in progress.

Priority 1 — Critical reliability and performance
- Concurrent download recovery [IMPL]
  - Persist/reattach TUI-initiated downloads after process restart; graceful recovery of work-in-progress.
- Database transaction boundaries [IMPL]
  - Group related state updates in transactions to avoid inconsistent rows on crashes.
- Streaming hashing [IMPL]
  - Single and chunked downloaders perform streaming SHA256; keep for traceability.
- Chunk corruption recovery [IMPL]
  - Self-check and targeted re-fetch of dirty chunks is implemented in chunked downloader.

Priority 2 — Core functionality gaps
- Bandwidth throttling (per-download and global) [IMPL]
- Mirror/fallback URLs with ordered failover [IMPL]
- Partial verification during single-stream downloads [IMPL]
  - Periodic checkpoints; chunked mode already verifies per-chunk.
- Connection pool management [IMPL]
  - Reuse a shared HTTP client/transport across downloaders; per-host limits.

Priority 3 — User experience
- TUI refactor and feature polish [IMPL]
  - TUI state persistence, state-event handling, downloads/library/settings views, and modal handling are split across focused files; sort/group/columns, filter selection persistence, and planning progress accuracy are implemented.
- Progress persistence across sessions [IMPL]
- Adaptive retry/backoff by error type [IMPL]
- Download queue management with priorities [IMPL]

Priority 4 — Quality improvements
- Rich, structured error context with remediation hints [IMPL]
- Test coverage expansion (resolvers/state/placer/downloaders) [IMPL]
- Metrics expansion (per-download stats, percentiles) [IMPL]
- Configuration validation hardening [IMPL]

Priority 5 — Advanced features
- Archive extraction post-download (zip/tar/7z) [IMPL]
  - zip, tar, tar.gz, and tgz use native extraction; 7z uses `7zz`, `7z`, or `7za` when available on PATH.
- Duplicate detection / content-addressable storage [IMPL]
  - Duplicate reporting by completed SHA256 is implemented; `dedupe` can replace verified duplicates with hardlinks or symlinks to canonical content.
- S3-compatible backend for storage [NEW]
- Download scheduling (cron-like windows) [IMPL]

Priority 6 — Architecture
- Context propagation pattern improvements [IMPL]
- Plugin architecture for resolvers [IMPL]
- Event-driven TUI updates with polling fallback [IMPL]

Quick wins (remaining)
- Add a flag to skip SHA256 verification intentionally for trusted sources (implemented as --force) [IMPL]
- Fix TUI selected item persistence when filtering [IMPL]
- Fix progress bar showing 100% during chunk planning [IMPL]

Technical debt
- Remove duplicate SafeFileName implementations (ensure all call util.SafeFileName) [IMPL]
- Consolidate HTTP client creation to a shared pool [IMPL]
- Standardize error wrapping and logging redaction [IMPL]
- Trim dead code in legacy TUI model [IMPL]
- Audit metrics writes/guards when disabled [IMPL]

Performance optimizations
- Pre-allocate/truncate files in chunked mode and parallel chunk verification [IMPL]
- DNS result caching for repeated hosts [IMPL]
- Adaptive chunk size based on configured throughput and file size [IMPL]

Breaking changes to consider for v1.0
- Config schema tidy-up
- State DB schema simplification
- Standardize CLI flags and naming

Completed (from prior docs)
- CivitAI model-aware default filenames and naming patterns (sources.*.naming.pattern) [IMPL]
- Auth preflight/early 401 guidance with opt-out via network.disable_auth_preflight [IMPL]
- Batch: --batch-parallel support; importer and summary behaviors [IMPL]
- Single-stream: treat HTTP 416 on resume as completion [IMPL]
- --quiet behavior aligned to suppress human summaries [IMPL]

Completed v0.6.0 (November 2025)
- Library View: Browse and search downloaded models with rich metadata [IMPL]
  - Search by name, filter by type (LLM, LoRA, VAE) and source (HuggingFace, CivitAI, local)
  - View detailed information: quantization, size, tags, descriptions
  - Mark models as favorites
  - Tab 5 or `L` keyboard shortcut
- Directory Scanner: Auto-discover models in configured directories [IMPL]
  - Scans download_root and placement directories
  - Extracts metadata from filenames (quantization, parameter count, version)
  - O(log n) duplicate detection with database indexes (10-100x faster than linear scan)
  - Press `S` in Library view to trigger scan
- Settings Tab: View configuration at a glance [IMPL]
  - Display directory paths, API token status, placement rules, download settings
  - Visual indicators for HuggingFace and CivitAI token validation
  - Tab 6 or `M` keyboard shortcut
- Database Performance: Added indexes for 10-100x speedup on large model libraries [IMPL]
  - idx_metadata_dest and idx_metadata_model_name indexes
  - Optimized duplicate detection and search queries
- Comprehensive Testing: 84 test cases including unit, integration, and performance benchmarks [IMPL]

Notes
- README "Roadmap" bullets map to the Priority 3/4 buckets here.
- v0.6.0 features fully documented in docs/LIBRARY.md and docs/SCANNER.md
