# modfetch

Fetch, verify, and place LLM and Stable Diffusion models reliably from Hugging Face and CivitAI — with resume, per‑chunk and full SHA256, batch YAML, and a TUI.

A robust CLI/TUI downloader for LLM and Stable Diffusion assets from Hugging Face and CivitAI.

Highlights
- Parallel chunked downloads with resume and retries
- SHA256 integrity verification (per chunk and full file)
- Automatic classification + placement into your app directories
- Batch YAML execution with verify and status
- Rich TUI dashboard and CLI progress bar with throughput and ETA
- Structured logging, metrics, and resilient state (SQLite)

Status: MVP feature-complete for resolvers and downloads; TUI polish and docs ongoing.

Quickstart (≈1 minute)
```bash
# 1) Build
make build
# 2) Minimal config (example paths; see docs/CONFIG.md for full schema)
mkdir -p ~/.config/modfetch
cat >~/.config/modfetch/config.yml <<'YAML'
version: 1
general:
  data_root: "~/modfetch-data"
  download_root: "~/Downloads/modfetch"
  placement_mode: "symlink"
network:
  timeout_seconds: 60
concurrency:
  per_file_chunks: 4
  chunk_size_mb: 8
sources:
  huggingface: { enabled: true, token_env: "HF_TOKEN" }
  civitai:     { enabled: true, token_env: "CIVITAI_TOKEN" }
YAML
# 3) First run (public)
./bin/modfetch download --config ~/.config/modfetch/config.yml --url 'https://proof.ovh.net/files/1Mb.dat'
```

Token env vars (if needed)
- HF_TOKEN — used when accessing gated Hugging Face repos
- CIVITAI_TOKEN — used when accessing gated CivitAI content

Requirements
- Go 1.22+
- Linux (primary), macOS (secondary)

Installation
- From source:
  - Build: `make build`
  - Tests: `make test`
  - Cross-compile: `make release-dist`
- Binaries: via GitHub Releases (tag `vX.Y.Z` to trigger CI)
- Homebrew: see packaging/homebrew/modfetch.rb template
- Deployment (Linux): see docs/DEPLOY_LINUX.md
- Optional: systemd user service for TUI: see docs/SYSTEMD_TUI.md
- Shell completions: see docs/COMPLETIONS.md
Configuration
- All configuration is provided via a YAML file; no secrets in YAML (use env vars).
- Pass the config file path with `--config` or via `MODFETCH_CONFIG`.
- Generate a starter config interactively:
  ```
  modfetch config wizard --out ~/modfetch/config.yml
  ```
- See docs/CONFIG.md for full schema.

User guide
- Validate config:
  ```
  modfetch config validate --config /path/to/config.yml
  ```
- Download with live progress (CLI shows progress bar, speed, ETA):
  ```
  modfetch download --config /path/to/config.yml --url 'https://proof.ovh.net/files/1Mb.dat'
  modfetch download --config /path/to/config.yml --url 'hf://gpt2/README.md?rev=main'
  modfetch download --config /path/to/config.yml --url 'civitai://model/123456?file=vae'
  ```
  - Quiet mode (suppress progress/info logs): add `--quiet`
  - On completion, a summary is printed (dest, size, SHA256, duration, average speed)
- Place artifacts into apps:
  ```
  modfetch place --config /path/to/config.yml --path /path/to/model.safetensors
  ```
- Batch downloads from YAML (optionally place after):
  ```
  modfetch download --config /path/to/config.yml --batch /path/to/jobs.yml --place
  ```
  - See docs/BATCH.md for full batch schema and examples
- TUI dashboard (live status, filter, per-row speed/ETA):
  ```
  modfetch tui --config /path/to/config.yml
  ```
  - Keys: q (quit), r (refresh), j/k (select), d (details), / (filter)
- Verify checksums:
  ```
  modfetch verify --config /path/to/config.yml --all
  ```
- JSON summary (for scripting/CI):
  ```
  modfetch download --config /path/to/config.yml --url 'https://proof.ovh.net/files/1Mb.dat' --summary-json
  ```
- Placement dry-run:
  ```
  modfetch place --config /path/to/config.yml --path /path/to/model.safetensors --dry-run
  ```

Resolvers
- See docs/RESOLVERS.md for hf:// and civitai:// formats, examples, and auth via env tokens.

Logging and metrics
- Log level per command: `--log-level debug|info|warn|error`; `--json` for JSON logs.
- Quiet mode: `--quiet` (download command) hides progress and info logs.
- Metrics: optional Prometheus textfile exporter; configure in YAML (see docs/METRICS.md)
- Troubleshooting: see docs/TROUBLESHOOTING.md

Project layout
- cmd/modfetch: CLI entry point
- internal/config: YAML loader and validation
- internal/downloader: single + chunked engines
- internal/resolver: hf:// and civitai:// resolvers
- internal/placer: placement engine
- internal/tui: TUI models
- internal/state: SQLite state DB
- internal/metrics: Prometheus textfile metrics
- docs/: configuration, testing, placement, resolvers
- scripts/: smoke test and helpers

Troubleshooting
- Missing tokens: Set `HF_TOKEN` or `CIVITAI_TOKEN` in your environment when accessing private resources.
- TLS or HEAD failures: The downloader will fall back to single-stream when range/HEAD is unsupported.
- Resume: Re-running the same download will resume and verify integrity.

Roadmap
- Further TUI enhancements (sorting, action keys)
- Placement/classifier refinements and presets
- Release packaging for more distros

