# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

Added
- Added AUR `modfetch-bin` packaging metadata for the published Linux release
  binaries.
- Added portable AUR package validation that checks `PKGBUILD`, `.SRCINFO`, and
  published release checksums.
- Added `library sync push` and `library sync pull` for `file://` catalog sync
  targets, including dry-run and JSON output support.
- Added HTTP(S) catalog pull targets for `library sync pull`, allowing a
  published catalog URL to be imported with the same dry-run/conflict handling.
- Added ModelScope metadata enrichment for `modelscope.cn` model URLs, including
  source detection, API-backed metadata, and best-effort fallback metadata.

Docs
- Documented Arch Linux AUR package maintenance and release checklist coverage.
- Aligned README, installation, release, roadmap, and TUI docs with the shipped
  v0.7.0 state and the current AUR SSH publication blocker.
- Removed obsolete top-level sprint and test status reports that described old
  sandbox limits instead of the current validation workflow.

## v0.7.0 — 2026-04-28

Added
- Added Homebrew tap installation documentation and release checklist coverage.
- Added `library export` and `library import` for portable JSON catalog backups, dry-runs, conflict reporting, and idempotent restores.
- Added TUI library filtering, multi-select bulk operations, confirmation summaries, and selected catalog export.
- Added placement presets for common local AI tools, config wizard preset selection, `place --preset`, and dry-run placement previews.
- Added bounded parallel library scanning with CLI/TUI progress reporting and optional stale-record repair.

Changed
- Improved classifier confidence reporting for ambiguous artifacts.
- Hardened release validation with a docs drift check in CI and the release checklist.
- Replaced material metadata HTTP test doubles and fake 7z execution with real local HTTP and archive integration coverage.

Testing
- Added scanner performance benchmarks and cancellation consistency tests.
- Expanded real CLI, SQLite, TUI command, metadata HTTP, archive, and docs drift validation.

Docs
- Refreshed README, installation, CLI, TUI, library, scanner, and release workflow documentation for the v0.7.0 feature set.

## v0.6.3 — 2026-04-27

Fixes
- Fixed Hugging Face shorthand alias downloads such as `hf://gpt2/README.md?rev=main`.
- Preserved canonical namespaced repo parsing for dotted repository names such as `hf://acme/model.v1`.

Docs
- Clarified Hugging Face resolver forms for repo-only and optional-path URIs.

## v0.6.2 — 2026-04-26

Added
- S3-compatible storage destinations using local resumable staging plus SigV4 upload.
- Archive extraction support for zip, tar, tar.gz, tgz, and optional 7z backends.
- Duplicate reporting and `dedupe` link modes for verified duplicate artifacts.
- Event-driven TUI state refresh with polling fallback.
- Strict config validation mode and documented enum/range validation.
- SQLite schema version baseline with explicit legacy v0-to-v1 migration.

Changed
- Centralized config path resolution and common CLI flag registration.
- Synced shell completions with current commands and flags.
- Improved chunked download verification, adaptive chunk sizing, and downloader HTTP client behavior.
- Refined TUI state helpers, grouping/preferences, filter selection persistence, and progress reporting.

Testing
- Expanded resolver, state, placer, downloader, metrics, config, completion, S3, archive, and TUI test coverage.
- Added migration, schema validation, completion, rate-limit, and race-suite coverage.

Docs
- Closed out the consolidated roadmap and marked old sprint/test status reports as historical.
- Added database schema documentation and refreshed config, completion, batch, and CLI docs.

## v0.6.1 — 2025-11-11

Testing
- Added real Hugging Face and CivitAI API integration tests with retry/backoff handling.
- Expanded TUI navigation and inspector test coverage.
- Added test guidance for manual TUI validation and external API behavior.

Fixes
- Fixed resolver path parsing, scanner version extraction, scanner format detection, state timestamp precision, and TUI test fixtures.
- Improved test reliability for flaky external API calls.

## v0.5.2 — 2025-09-26

Fixes
- **TUI v1**: Restore rich UI elements, vibrant colors, and proper borders to TUI v1 (refactored MVC)
- **TUI v1**: Fix critical startup issue where TUI was stuck at "Loading..." due to uninitialized window dimensions
- **TUI v1**: Eliminate terminal escape sequences "10;?11;?" by removing problematic tea.WithMouseCellMotion()
- **TUI v1**: Enhance visual feedback with colorful status indicators (green for completed, red for failed, pink for active)
- **TUI v1**: Improve help system overlay rendering and navigation experience
- **TUI v1**: Add comprehensive color scheme matching TUI v2's visual appeal

## v0.5.1 — 2025-09-25

Fixes
- **Installer**: Fix 404 error when downloading binaries - handle raw binaries instead of tar archives
- **Installer**: Initialize SKIP_CONFIG_WIZARD variable to prevent unbound variable error
- **TUI Navigation**: Fix backwards version selection logic - default to working TUI v2, use --v1 for refactored version
- **TUI Navigation**: Restore proper arrow key navigation and help system functionality

## v0.5.0 — 2025-01-24

Installation & Deployment
- **Comprehensive installation package**: One-liner curl installer with guided setup experience
- **Cross-platform support**: Automated Linux/macOS binary detection and installation
- **Config wizard integration**: Interactive setup with existing `modfetch config wizard`
- **Uninstaller**: Clean removal script with optional data preservation
- **Developer setup**: Enhanced `scripts/setup-dev.sh` with git hooks and VS Code configuration

Improvements
- CLI: `status` supports `--only-errors` (filter) and `--summary` (totals and error count). JSON includes `{ total, errors, rows }` when `--summary` is set.
- CLI: `download` accepts `--sha256-file` to read an expected hash from a file (supports .sha256 "hash  filename" format).
- CLI: `clean` adds `--sidecars` to remove orphan `.sha256` sidecar files; JSON output now includes `sidecars_removed`.
- CLI: `verify` adds `--fix-sidecar` to rewrite `<dest>.sha256` for verified files.
- TUI v2: clearer surfacing of rate limits and view indicators; table header marks active sort (SPEED*/ETA*/[sort: remaining]); Stats panel shows Sort/Group/Column/Theme.
- CI: introduce `golangci-lint` job and cache setup; keep vet/test/build/govulncheck; Makefile adds `fmt`, `fmt-check`, `vet`, `lint`, and `ci` targets.

New
- Config: `network.retry_on_rate_limit` (bool) to respect `Retry-After` on HTTP 429; `network.rate_limit_max_delay_seconds` to cap the wait (defaults to 600s if unset; 0 uses the default).

## v0.3.3 — 2025-08-31

Fixes
- Naming: strip URL query/fragment from default filenames and sanitize more aggressively; applies to CLI, TUI, and batch importer fallbacks (e.g., CivitAI direct links no longer include ?type=… in filenames).
- TUI v2: computeDefaultDest now tries a HEAD request for CivitAI direct download URLs (/api/download/…) to use Content-Disposition filename when available; falls back to safe base.

## v0.3.2 — 2025-08-31

Maintenance
- CI: Add macOS universal (fat) binary to release artifacts via new macOS job. Checksums included.
- Docs: README updated to note CI-built universal binary and checksums.

## v0.3.1 — 2025-08-31

Maintenance
- CI: Fix release workflow to avoid duplicate asset uploads that caused a softprops/action-gh-release error. No code changes since v0.3.0.

## v0.3.0 — 2025-08-31

Highlights
- TUI v2 is now the default with extensive UX upgrades:
  - Live progress, speed, ETA with smoothing; throughput sparkline
  - Filtering, sorting, grouping; multi-select; toasts drawer; help/commands bar
  - New download wizard with smarter filename suggestions (resolver + HEAD Content-Disposition), detected artifact type, and placement hints
  - Auth status indicators for Hugging Face and CivitAI (presence + rejection detection)
  - Reachability probe (P), duplicate row merge, and immediate pending-row visibility
  - Persisted UI state (theme/columns/layout) and configurable refresh rate (ui.refresh_hz)
- Batch/import
  - Import URLs from text with preprocessing (redirects, filename/dest inference); integration tests and CI smoke
  - Parallel batch download and duplicate/collision handling
- Resolver/classifier
  - CivitAI suggested filename de-duplication and slugification while preserving extension
  - GGUF magic detection (case-insensitive)
  - Resolver cache with TTL and CLI purge
- Downloader/state/metrics
  - Friendlier HTTP auth/permission error messages (401/403/404) persisted to state.last_error
  - Context-based cancellation and cleanup; atomic counters; periodic metrics write
  - State DB indices and COALESCE fixes; new last_error column and CreatedAt timestamps
- CLI
  - Default config path fallback to ~/.config/modfetch/config.yml
  - Safer host boundary checks and URL normalization
- Docs/CI
  - CHANGELOG introduced; expanded TUI guide and docs refresh
  - Release workflow builds OS/arch artifacts and attaches checksums

Assets
- Linux: modfetch_linux_amd64, modfetch_linux_arm64
- macOS: modfetch_darwin_amd64, modfetch_darwin_arm64
- Checksums: SHA256SUMS
## v0.2.1 — 2025-08-30

Highlights
- TUI
  - Ephemeral rows keyed by URL|dest to avoid collisions and enable precise clearing
  - New shortcuts: D (delete staged data) and X (clear stuck row)
  - Open/Reveal actions execute synchronously to surface errors
  - Live speed/ETA for both chunked and single-stream fallbacks; smoother sampling
- Downloader
  - Treat HTTP 416 on resume-beyond-EOF as successful completion for single-stream fallback
  - Safe finalize and verification polish for .safetensors
  - Clearer messages for missing auth tokens and 401s
- Docs
  - README and USER_GUIDE refreshed with simpler quickstart, TUI keymap, and resolver URL examples
  - Sample config includes sane defaults; CivitAI model page URLs auto-resolve
- Misc
  - Metrics writes guarded when disabled
- State
  - downloads table indexed on status and dest columns

Assets
- Linux: modfetch_linux_amd64, modfetch_linux_arm64
- macOS: modfetch_darwin_amd64, modfetch_darwin_arm64, modfetch_darwin_universal
- Checksums: SHA256SUMS

## v0.2.0 — 2025-08-29

- Initial TUI UX improvements and context actions
- Composite ephemeral handling in TUI
- Downloader fixes and documentation updates

## v0.1.0 — 2025-08-20

- Initial release: CLI download, verify, placement; hf:// and civitai:// resolvers; chunked + single-stream fallback
