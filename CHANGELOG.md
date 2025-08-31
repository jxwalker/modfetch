# Changelog

All notable changes to this project will be documented in this file.

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

