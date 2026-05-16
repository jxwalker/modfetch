# modfetch Documentation

This directory is the detailed reference for modfetch. Start with the README if
you want the product overview; use these docs when you are configuring,
automating, troubleshooting, or maintaining a release.

## Start Here

- [Quick Start](QUICKSTART.md): install, configure, run a first download, and
  open the TUI.
- [User Guide](USER_GUIDE.md): end-to-end workflows for `get`, recommendations,
  packs, snapshots, downloads, verification, placement, and library management.
- [CLI Guide](CLI_GUIDE.md): command and flag reference.
- [TUI Guide](TUI_GUIDE.md): dashboard layout, keyboard controls, guided
  recommendations, and library actions.
- [Installation](INSTALLATION.md): Homebrew, AUR, installer, release binaries,
  and source builds.

## Core Workflows

- [Batch Jobs](BATCH.md): YAML batch downloads and imported URL lists.
- [Configuration](CONFIG.md): config schema, source tokens, network settings,
  validation, placement, and metrics.
- [Resolvers](RESOLVERS.md): `starter://`, `hf://`, CivitAI, direct URLs, and
  auth behavior.
- [Placement](PLACEMENT.md): symlink, hardlink, copy, and named app presets.
- [Library](LIBRARY.md): metadata, favorites, scanning, export/import, and sync.
- [Scanner](SCANNER.md): indexed local model discovery and stale-record repair.
- [Troubleshooting](TROUBLESHOOTING.md): common download, auth, filesystem, and
  safetensors issues.

## Operations and Automation

- [Metrics](METRICS.md): Prometheus textfile output.
- [Database](DATABASE.md): local SQLite state layout.
- [Completions](COMPLETIONS.md): shell completion generation.
- [Linux Deployment](DEPLOY_LINUX.md): Linux host setup notes.
- [Systemd TUI](SYSTEMD_TUI.md): running the TUI under systemd.
- [Testing](TESTING.md): validation philosophy and real-execution coverage.
- [Testing Guide](TESTING_GUIDE.md): practical test commands.

## Maintainers

- [Release Checklist](RELEASE.md): changelog, artifacts, package channels, docs
  drift, and post-publish verification.
- [Roadmap](ROADMAP.md): shipped baseline and next planning area.
- [TUI Wireframes](TUI_WIREFRAMES.md): visual interaction reference.
- [TUI Analysis Summary](TUI_ANALYSIS_SUMMARY.txt): current TUI architecture and
  maintenance notes.
- [AUR Packaging](../packaging/aur/README.md): Arch package publishing notes.

## Current Release

Current release: v0.8.1.

The v0.8.x line shipped hardware-aware recommendations, runtime hints, learned
recommendation history, benchmark-driven adaptive transfer tuning, guided TUI
recommendations, recommendation inspection, and live transfer metadata probes.

Unreleased main adds beginner `get`, `--run-help`, curated multi-file task
packs, and Hugging Face snapshot manifests.
