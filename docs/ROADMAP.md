# Roadmap

This is the active project roadmap after the v0.8.0 release. Historical backlog
items that shipped in v0.6.x and v0.7.x are summarized at the end for traceability.

Status key:
- [PLANNED] not started
- [NEXT] next implementation slice
- [IN PROGRESS] active work
- [DONE] shipped on main

## Current Baseline

Current release: v0.8.0, tagged 2026-05-13.

Shipped baseline:
- Reliable direct, Hugging Face, and CivitAI downloads with resume, retries,
  SHA256 verification, auth preflight, rate-limit handling, archive extraction,
  S3-compatible destinations, and duplicate linking.
- TUI download management with downloads, library, settings, themes, sorting,
  grouping, filtering, state persistence, and event-driven refresh.
- Library metadata storage, indexed scanning, favorites, metadata fetchers, and
  documented configuration, placement, resolver, CLI, and installer workflows.
- Release automation that builds Linux/macOS artifacts, publishes checksums, and
  uses `CHANGELOG.md` sections for GitHub Release notes.
- Beginner-friendly starter aliases and real-provider discovery commands for
  users who do not already know the exact model URL to download.
- Hardware-aware recommendation commands that detect or accept RAM/VRAM
  constraints, rank live provider results by task and memory fit, and hand the
  selected URI to the same resumable download path.
- Recommendation history and runtime hints, so repeated choices for the same
  task, query, and hardware class influence ranking, and recommendation output
  explains likely local runtimes and placement presets.

## v0.7.0 Goal [DONE]

Make modfetch easier to adopt and easier to use for real model-library
maintenance. v0.7.0 should focus on distribution polish, library portability,
bulk TUI operations, and placement presets rather than another downloader core
rewrite.

## v0.7.0 Implementation Plan

### 1. Package Distribution [DONE]

Outcome: users can install through a maintained package channel instead of only
the curl installer or manual release binaries.

- [DONE] Homebrew tap formula for macOS and Linuxbrew.
- [DONE] Document the package install path in README and docs/INSTALLATION.md.
- [DONE] Add release checklist coverage, so formula updates are part of tagging.
- [DONE] Decide whether an AUR package is in scope for v0.7.0 or v0.7.x:
  defer to v0.7.x after Homebrew usage settles.

Acceptance checks:
- Formula installs the latest GitHub Release artifact and verifies checksum.
- README and installation docs no longer describe Homebrew as unpublished once
  the tap exists.
- `docs/RELEASE.md` includes the tap update and validation steps.

### 2. Library Catalog Export and Import [DONE]

Outcome: users can move or back up their model library index without manually
copying SQLite state.

- [DONE] Add `library export --format json` for model metadata, favorites,
  source URLs, checksums, and placement hints.
- [DONE] Add `library import` with dry-run, conflict reporting, and idempotent
  updates.
- [DONE] Include schema version metadata in exported catalogs.
- [DONE] Document backup/restore and machine migration workflows.

Acceptance checks:
- Export/import round trip preserves model identity, favorite status, source,
  destination, checksum, and core metadata.
- Import dry-run reports creates, updates, skips, and conflicts without writes.

### 3. TUI Bulk Operations and Filter Menu [DONE]

Outcome: common library maintenance tasks are possible from the TUI without
dropping to separate CLI commands.

- [DONE] Implement the documented `F` filter menu for library type/source,
  favorite status, and text search.
- [DONE] Add multi-select bulk actions for retry, delete staged data, verify,
  place, favorite/unfavorite, and export selected catalog entries.
- [DONE] Show bulk-action confirmation summaries before destructive actions.
- [DONE] Add tests for selection persistence across filters and tabs.

Acceptance checks:
- Selection state remains stable when filters change.
- Destructive actions require confirmation and show the exact affected count.
- Keyboard help and TUI guide match the implemented controls.

### 4. Placement and Classifier Presets [DONE]

Outcome: first-time setup for common local AI tools needs less custom YAML.

- [DONE] Add named placement presets for common targets such as Ollama,
  ComfyUI, AUTOMATIC1111/Forge-style Stable Diffusion layouts, and generic
  Hugging Face cache exports where appropriate.
- [DONE] Add `modfetch config wizard` support for selecting presets.
- [DONE] Add `modfetch place --preset NAME --dry-run` preview behavior.
- [DONE] Improve classifier confidence reporting for ambiguous artifacts.

Acceptance checks:
- Preset output is explicit YAML that users can inspect and edit.
- `--dry-run` explains every planned link/copy and every skipped artifact.

### 5. Scanner Performance and Repair UX [DONE]

Outcome: large model directories scan faster and failed scans are easier to
recover from.

- [DONE] Add bounded parallel directory scanning with serialized DB writes.
- [DONE] Add scan progress reporting for CLI and TUI.
- [DONE] Add optional stale-record repair for files moved or deleted outside
  modfetch.
- [DONE] Benchmark sequential versus parallel scans on representative trees.

Acceptance checks:
- Parallel scan results match sequential scan results.
- Scan cancellation leaves the database in a consistent state.
- Benchmarks show the change helps on large directories without regressing small
  scans materially.

### 6. Release and Documentation Hardening [DONE]

Outcome: release quality stays repeatable as install surfaces expand.

- [DONE] Add a release checklist that covers changelog section, installer,
  package formula, release notes extraction, artifacts, checksums, and smoke
  installs.
- [DONE] Add a docs drift check for current version strings and stale installation
  claims.
- [DONE] Keep README.md, docs/QUICKSTART.md, docs/USER_GUIDE.md,
  docs/CLI_GUIDE.md, docs/INSTALLATION.md, and CHANGELOG.md aligned before
  each tag.

Acceptance checks:
- [DONE] A release candidate can be validated from a clean checkout using documented
  commands.
- [DONE] CI or a local script catches missing changelog release notes before tagging.

## v0.7.x Candidates

These are useful, but not required for the first v0.7.0 release:

- [DONE] AUR packaging after Homebrew is stable:
  `modfetch-bin` is published to AUR, package metadata and validation live under
  `packaging/aur/`, and updates are automated by `scripts/publish-aur.sh`.
- [DONE] Remote catalog sync foundation:
  `library sync push/pull --target file://...` reuses the portable catalog
  schema for shared folders, mounted drives, and local filesystem sync tests.
- [DONE] Read-only remote catalog pull:
  `library sync pull --target https://...` imports published catalogs with the
  same dry-run and conflict handling as local catalog imports.
- [DONE] ModelScope metadata enrichment:
  ModelScope URLs are recognized as `modelscope` sources and enrich library
  records from the ModelScope model API with best-effort offline fallback.
- [DONE] Ollama Library metadata enrichment beyond ModelScope:
  `ollama.com/library/...` pages are recognized as `ollama` sources and enrich
  library records with page metadata plus best-effort fallback.
- [DONE] Authenticated and writable remote catalog sync targets:
  HTTP(S) sync targets can use bearer tokens from `--token-env`, and
  `library sync push --target https://...` publishes catalogs with `PUT`.
- More archive formats if users report concrete needs.
- [DONE] Non-interactive TUI scripting hooks:
  `modfetch tui --snapshot` exits without launching the Bubble Tea interface
  and `--snapshot --json` emits script-friendly downloads, library, and config
  state.
- [DONE] Hardware-aware model recommendation:
  `modfetch recommend` provides an llmfit-style path from user task and local
  hardware to ranked provider results, with JSON output and one-step dry-run or
  download handoff.

## v0.8.x Direction

- [DONE] Recommendation quality feedback loop:
  persist accepted/skipped recommendations, benchmark results, and completed
  download outcomes so future recommendations learn from real local use.
- [DONE] Runtime-aware guidance:
  expose placement/runtime hints for Ollama, llama.cpp, MLX, ComfyUI, and
  Stable Diffusion UIs so recommendation output explains where a model can run,
  not just whether it can be downloaded.
- [NEXT] Interactive recommendation flow in the TUI:
  add a guided "choose a model" workflow that filters by task, hardware, file
  size, source, and placement target before starting a download.

## Completed Release History

- v0.8.0: delivered benchmark-driven transfer tuning, live adaptive
  ramp-up/backoff, persisted per-host transfer history, hardware-aware
  recommendations, learned recommendation history, and runtime/placement hints
  for local model execution.
- v0.7.1: delivered beginner-safe starter downloads, real-provider discovery
  search/download, ModelScope discovery support, AUR release automation,
  catalog sync HTTP targets, metadata enrichment for ModelScope and Ollama
  pages, non-interactive TUI snapshots, and release hardening.
- v0.7.0: delivered Homebrew distribution docs, portable library catalog
  export/import, TUI bulk library operations, placement presets, scanner
  performance and stale repair UX, real integration test hardening, and
  release docs drift validation.
- v0.6.3: fixed Hugging Face shorthand alias downloads and refreshed resolver
  documentation.
- v0.6.2: delivered storage, archive extraction, duplicate linking, schema,
  CLI/completion, release workflow, and broad test coverage improvements.
- v0.6.1: expanded real API integration tests, TUI tests, and reliability fixes.
- v0.6.0: introduced library view, directory scanner, settings tab, indexed
  metadata storage, and core TUI/library documentation.

## Completed Backlog Summary

The previous consolidated backlog is closed as of v0.6.x. Completed areas
include:

- Concurrent recovery and transaction boundaries.
- Streaming hashing and chunk corruption recovery.
- Bandwidth throttling and mirror/fallback URLs.
- Partial verification and shared HTTP clients.
- TUI refactors and retry/backoff improvements.
- Queue priority handling and structured error remediation.
- Expanded metrics and strict config validation.
- Plugin-style resolvers and v1.0 schema migration baseline.
