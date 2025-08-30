# Next TODO (Improvements)

Priority 1
- Configurable naming patterns for resolvers
  - Add `sources.civitai.naming.pattern` and `sources.huggingface.naming.pattern` with tokens: `{model_name}`, `{version_name}`, `{version_id}`, `{file_name}`, `{repo}`, `{path}`. CLI override `--naming-pattern`.
- Preflight auth and early 401 detection
  - Pre-download HEAD or 0–0 probe with Authorization; if 401 and token missing, fail early with guidance. `--no-auth-preflight` to disable.
- TUI enhancements
  - Sort by speed/ETA/remaining, actions (retry/cancel/details), warnings for missing token env, show SuggestedFilename/hostcaps, improve colors.

Priority 2
- Placement and classifier improvements
  - Better SD/LLM heuristics, presets for ComfyUI/A1111, `place --write-paths` to generate extra_model_paths.yaml, diff in dry-run.
- Batch runner improvements
  - Per-job naming/placement overrides, global concurrency, batch summary JSON, retry policies, stop-on-failure.
- Partials management polish
  - `clean --parts-only`, `--older-than`, show reclaimed space, startup warnings when stale .part present; docs for partials_root.
- Metrics expansion
  - Counters for 401s, retries by host; gauges for active downloads, per-host throughput; include build version.
- Logging and UX improvements
  - Wider `--quiet`, consistent JSON fields, rigorous secret redaction, resolver debug toggles, global throttle of repeated warnings.

Priority 3
- Safetensors verification outputs
  - `--json-out FILE` / `--csv-out FILE` for scan results; repaired/quarantined flags; non-zero exit on critical unless `--allow-partial`.
- CI and release automation
  - GitHub Actions: tests on Linux/macOS; build and attach artifacts on tag; publish SHA256SUMS. Consider goreleaser later.
- Packaging: Homebrew formula
  - Finalize formula/tap; validate arm64/amd64; completions installation.
- Docs
  - Examples for naming patterns and tokens; token setup; extra_model_paths.yaml recipes; 401/gated FAQ; v0.2.0 upgrade notes.
- Test coverage expansion
  - Auth preflight and warnings, naming token expansion, UniquePath edge cases, prealloc/progress interaction, placement dry-run.

Exploratory
- Extensible sources (design doc)
  - Plan plugin-like resolvers for `s3://`, `gcs://`, `local://`; define minimal API and safety considerations.
- Security hardening audit
  - Review logs and error paths for token leakage; add central `redact()` helper used by logging; ensure headers are never logged.

---

CodeRabbit follow-ups (2025-08-30)

Priority 1
- TUI ephemerals keyed by URL|Dest and exact clearing
  - Key resolving/preflight ephemerals by composite key url|dest to avoid collisions; clear only on matching DB row by URL or cleaned Dest; render resolving spinner based on the composite key.
  - Add helper ephemeralKey(url, dest) and update all call sites (addEphemeral, View spinner check, refresh clearing, dlDone handling).
  - Ensure dlDoneMsg includes both url and dest so ephemerals can be cleared precisely. [PR #8]
- Consistent dest use when starting downloads from TUI
  - When dest is blank, compute dg := destGuess(url, dest) once and use dg for preflight, addEphemeral, and startDownloadCmd so pause/cancel lookup keys match the eventual DB row. [PR #8]
- Single-stream downloader: treat HTTP 416 as already complete
  - If resuming and the server returns 416 Requested Range Not Satisfiable and local .part size >= reported size, finalize (fsync, rename/copy to final, adjust safetensors), recompute SHA, upsert DB row status=complete, and return success instead of error. [PR #8]
- Sanitize destGuess and enforce containment
  - Prevent path traversal and ensure computed default filenames stay under download_root. Reject unsafe basenames ("/", ".", ".."), clean and validate with filepath.Rel; fall back to a safe default when needed. [PR #8]

Priority 2
- TUI open/reveal: use Run() and surface errors
  - Switch from exec.Command(...).Start() to .Run() so non-zero exit codes are caught; optionally wrap in an async command to avoid blocking UI; provide user-visible error on failure. [PR #8]
- Metrics manager: guard writes when disabled
  - Skip creating/writing Prometheus textfile metrics when enabled=false or path is empty to avoid os.CreateTemp failures; audit all write sites for guards. [PR #8]
- Docstrings and tests
  - Add docstrings for new TUI helpers (preflightForDownload, destGuess, addEphemeral, openInFileManager) and downloader changes.
  - Unit tests: speed/ETA sampling behavior, ephemeral clearing logic, destGuess sanitation, and single-stream 416 finalize path.

---

Additional Backlog (MF-001..MF-039)

1. CRITICAL PRIORITY ITEMS (Sprint 1-2)
1.1 Code Quality & Testing

- MF-001: Implement comprehensive unit test coverage for all resolver packages (target: >80% coverage)
- MF-002: Refactor internal/tui/model.go to comply with single responsibility principle (max 200 lines per file)
- MF-003: Establish integration test suite for all download scenarios
- MF-004: Implement automated test execution in CI/CD pipeline

1.2 Core Functionality

- MF-005: Implement download progress persistence with automatic recovery
- MF-006: Add bandwidth throttling capability with configurable limits per download
- MF-007: Refactor fetchChunk method in chunked.go into composable, testable units

2. HIGH PRIORITY ITEMS (Sprint 3-4)
2.1 Feature Enhancements

- MF-008: Implement mirror/fallback URL support for all resolver types
- MF-009: Develop batch job scheduling system with cron-like syntax
- MF-010: Add connection pooling for HTTP clients
- MF-011: Implement streaming hash verification to reduce memory footprint

2.2 Documentation

- MF-012: Generate comprehensive godoc documentation for all public APIs
- MF-013: Create Architecture Decision Records (ADR) for key technical decisions
- MF-014: Develop Kubernetes deployment guide with Helm charts
- MF-015: Establish API versioning and deprecation policy documentation

3. MEDIUM PRIORITY ITEMS (Sprint 5-6)
3.1 User Experience

- MF-016: Implement mouse support in TUI interface
- MF-017: Develop theme system with minimum dark/light mode options
- MF-018: Enhance error messages with contextual guidance and resolution steps
- MF-019: Create interactive configuration wizard for first-time setup

3.2 Architecture

- MF-020: Design and implement plugin architecture for custom resolvers
- MF-021: Implement download queue with priority management
- MF-022: Add automatic archive extraction post-download (zip, tar.gz, 7z)
- MF-023: Refactor context propagation to use context-aware client pattern

4. STANDARD PRIORITY ITEMS (Sprint 7-8)
4.1 Security Enhancements

- MF-024: Integrate with system keychain/secret managers for token storage
- MF-025: Implement GPG/signature verification for downloaded artifacts
- MF-026: Add audit logging for all download activities
- MF-027: Implement rate limiting per source to prevent API abuse

4.2 Performance Optimisation

- MF-028: Optimise database queries with proper indexing strategy
- MF-029: Implement adaptive chunk sizing based on network conditions
- MF-030: Add metrics collection for performance monitoring
- MF-031: Implement memory-mapped file operations for large downloads


