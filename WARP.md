# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

Repository: modfetch — a Go CLI/TUI to reliably fetch, verify, and place models from Hugging Face and CivitAI.

Common commands
- Requirements: Go 1.22+ (Linux/macOS). Auth tokens are only needed for gated content: export HF_TOKEN and/or CIVITAI_TOKEN in your shell.
- Build (produces ./bin/modfetch)
  - make build
- Run (public URL examples)
  - ./bin/modfetch download --config ~/.config/modfetch/config.yml --url 'https://proof.ovh.net/files/1Mb.dat'
  - ./bin/modfetch download --config ~/.config/modfetch/config.yml --url 'hf://gpt2/README.md?rev=main'
  - TUI: ./bin/modfetch tui --config ~/.config/modfetch/config.yml
- Tests
  - All tests: make test (go test ./...)
  - With coverage: go test ./... -cover
  - Single package: go test ./internal/downloader -v
  - Single test: go test ./internal/downloader -run 'TestName' -v
- Lint/format/static analysis
  - Format: make fmt
  - Check formatting: make fmt-check (fails if gofmt would change files)
  - Vet: make vet
  - Lint: make lint (requires golangci-lint to be installed; see https://golangci-lint.run/)
  - CI-style local check: make ci (runs vet, fmt-check, test)
- Local CI mirror
  - scripts/ci/run_all.sh (runs vet, tests with coverage, builds, and exercises non-network CLI paths)
- Packaging/cross builds (artifacts in dist/)
  - Linux: make linux (amd64, arm64)
  - macOS: make darwin (amd64, arm64); make macos-universal to lipo a fat binary
  - Checksums: make checksums; Full set: make release-dist
- Smoke test and dev helpers
  - Quick smoke: scripts/smoke.sh /path/to/config.yml
  - Sample configs: assets/sample-config/config.example.yml and jobs.example.yml
  - Linux dev bootstrap: scripts/dev_server_setup.sh

Configuration and environment
- Config file: pass with --config or set MODFETCH_CONFIG. Default resolution if unset attempts ~/.config/modfetch/config.yml.
- YAML schema highlights (see docs/CONFIG.md for full details): general (data_root, download_root, placement_mode, stage_partials/partials_root, always_no_resume), network (timeouts, Retry-After handling, disable_auth_preflight), concurrency (per_file_chunks, per_host_requests, chunk_size_mb, retries/backoff), sources (huggingface, civitai with token_env), placement mapping, classifier overrides, metrics (Prometheus textfile), validation (safetensors deep verify), ui (refresh_hz, column_mode, compact/theme).

Big-picture architecture
- CLI entrypoint: cmd/modfetch (standard flag-based subcommands)
  - Commands: config (validate|print|wizard), download, status, place, verify, tui, batch import, completion, hostcaps, clean, version.
  - Global flags per command: --config, --log-level, --json. Some commands also support --quiet, --dry-run, --summary-json.
- Resolver layer (internal/resolver)
  - Supports hf://owner/repo/path?rev=... and civitai://model/{id}[?version=...&file=...].
  - Attaches Authorization from env tokens when enabled in config; normalizes common page URLs to resolver URIs (e.g., huggingface blob pages, civitai model pages).
  - Results cached to data_root/resolver-cache.json with configurable TTL.
  - For CivitAI, returns SuggestedFilename based on config patterns or “ModelName - FileName”.
- Downloader (internal/downloader)
  - Prefers chunked parallel downloads with resume and per-chunk SHA256; falls back to single-stream when servers don’t honor Range or HEAD.
  - Stages data to a .part file (either download_root/.parts or partials_root), verifies integrity, and atomically finalizes to dest; writes a .sha256 sidecar.
  - Self-checks and repairs dirty chunks; optional deep verification for .safetensors; respects HTTP 429 Retry-After with a configurable cap.
  - Concurrency governed by config (per_file_chunks, per_host_requests, backoff, retries).
- State (internal/state)
  - SQLite database under data_root/state.db tracks downloads (status, retries, sizes, errors), chunk plans/progress, and host capability cache (HEAD OK, Accept-Ranges).
- Placement and classification
  - internal/classifier detects artifact types (e.g., sd.checkpoint/lora/vae/controlnet, llm.gguf) with optional YAML regex overrides.
  - internal/placer maps files into app directories via placement.mapping; supports symlink, hardlink, or copy per config/general. Placement fails if overwriting different content unless allow_overwrite is true.
- TUI (internal/tui, internal/tui/v2)
  - Bubble Tea–based dashboard; v2 adds richer sorting/grouping and theming. Refresh cadence and column mode configurable via ui.* in YAML. See docs/TUI_GUIDE.md.
- Logging and metrics
  - Text or JSON logs with level control; URLs are sanitized to avoid leaking secrets.
  - Optional Prometheus textfile exporter writes counters/gauges to metrics.prometheus_textfile.path.

Pointers to important docs
- README.md: install/quickstart, project layout, TUI keys overview, and release notes pointer.
- docs/CONFIG.md: full YAML schema and examples; token/env guidance; placement/classifier details; UI options.
- docs/RESOLVERS.md: URI formats and auth behavior.
- docs/TUI_GUIDE.md: keyboard shortcuts and workflow.
- docs/TESTING.md: local/integration/UAT guidance and environment requirements.

Notes for future Warp sessions
- When working on features that touch resolvers or downloader, consider running a quick public download for smoke validation and use --dry-run/--summary-json to inspect plan output without network transfer.
- If make lint fails due to missing golangci-lint, install it first or run only vet/fmt/test.

