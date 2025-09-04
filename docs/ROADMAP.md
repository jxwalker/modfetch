# Roadmap (Consolidated)

This document consolidates the project backlog and roadmap from multiple sources into a single, prioritized list. Source docs: docs/BACKLOG.md, docs/TODO.md, docs/TODO_NEXT.md, README.md (Roadmap), docs/PRD.md, docs/backlog/.

Status key: [NEW] newly captured, [IMPL] implemented already (kept here for traceability), [WIP] in progress.

Priority 1 — Critical reliability and performance
- Concurrent download recovery [NEW]
  - Persist/reattach TUI-initiated downloads after process restart; graceful recovery of work-in-progress.
- Database transaction boundaries [NEW]
  - Group related state updates in transactions to avoid inconsistent rows on crashes.
- Streaming hashing [IMPL]
  - Single and chunked downloaders perform streaming SHA256; keep for traceability.
- Chunk corruption recovery [IMPL]
  - Self-check and targeted re-fetch of dirty chunks is implemented in chunked downloader.

Priority 2 — Core functionality gaps
- Bandwidth throttling (per-download and global) [NEW]
- Mirror/fallback URLs with ordered failover [NEW]
- Partial verification during single-stream downloads [NEW]
  - Periodic checkpoints; chunked mode already verifies per-chunk.
- Connection pool management [NEW]
  - Reuse a shared HTTP client/transport across downloaders; per-host limits.

Priority 3 — User experience
- TUI refactor and feature polish [NEW]
  - Factor large models into smaller components; refine sort/group/columns; persistence of selection when filtering; progress accuracy during planning.
- Progress persistence across sessions [NEW]
- Adaptive retry/backoff by error type [NEW]
- Download queue management with priorities [NEW]

Priority 4 — Quality improvements
- Rich, structured error context with remediation hints [NEW]
- Test coverage expansion (resolvers/state/placer/downloaders) [NEW]
- Metrics expansion (per-download stats, percentiles) [NEW]
- Configuration validation hardening [NEW]

Priority 5 — Advanced features
- Archive extraction post-download (zip/tar/7z) [NEW]
- Duplicate detection / content-addressable storage [NEW]
- S3-compatible backend for storage [NEW]
- Download scheduling (cron-like windows) [NEW]

Priority 6 — Architecture
- Context propagation pattern improvements [NEW]
- Plugin architecture for resolvers [NEW]
- Event-driven updates vs polling [NEW]

Quick wins (remaining)
- Add a flag to skip SHA256 verification intentionally for trusted sources (implemented as --force) [IMPL]
- Fix TUI selected item persistence when filtering [NEW]
- Fix progress bar showing 100% during chunk planning [NEW]

Technical debt
- Remove duplicate SafeFileName implementations (ensure all call util.SafeFileName) [NEW]
- Consolidate HTTP client creation to a shared pool [NEW]
- Standardize error wrapping and logging redaction [NEW]
- Trim dead code in legacy TUI model [NEW]
- Audit metrics writes/guards when disabled [NEW]

Performance optimizations
- Pre-allocate/truncate files in chunked mode (done) and consider parallel chunk verification [NEW]
- DNS result caching for repeated hosts [NEW]
- Adaptive chunk size based on throughput [NEW]

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

Notes
- See docs/TODO_NEXT.md for CodeRabbit follow-ups and granular TUI tasks.
- README “Roadmap” bullets map to the Priority 3/4 buckets here.

