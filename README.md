# modfetch

Fetch, verify, and place LLM and Stable Diffusion models reliably from Hugging Face and CivitAI — with resume, per‑chunk and full SHA256, batch YAML, and a TUI.

A fast, resilient CLI + TUI for getting models where they belong. Parallel chunked downloads with resume, exact SHA256 verification, smart default naming, and placement into your app directories.

Highlights
- Parallel chunked downloads with resume and retries
- SHA256 integrity verification (per‑chunk and full file)
- Automatic classification + placement into your app directories
- Batch YAML execution with verify and status
- Rich TUI dashboard and CLI progress with throughput and ETA
- Structured logging, metrics, and resilient SQLite state
- Graceful cancellation on SIGINT/SIGTERM cleans up partial downloads

Status: MVP feature‑complete for resolvers and downloads; ongoing polish in TUI and docs.

Installation
- From source
  - Build: `make build` (produces `./bin/modfetch`)
  - Test: `make test`
  - Cross‑platform artifacts: `make release-dist`
- Binaries: via GitHub Releases (after v0.2.0)
  - macOS Universal binary is also provided in releases
- Homebrew: planned
- See CHANGELOG.md for what’s new each release

Quickstart (≈1 minute)
```bash
# 1) Build
make build

# 2) Minimal config (see docs/CONFIG.md for full schema)
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

Tokens (only for gated content)
- HF_TOKEN — Hugging Face
- CIVITAI_TOKEN — CivitAI

Requirements
- Go 1.22+
- Linux (primary), macOS (secondary)

Configuration
- Provide config via YAML; don’t put secrets in YAML (use env vars).
- Pass with `--config` or set `MODFETCH_CONFIG`.
- Generate a starter config interactively:
  ```
  modfetch config wizard --out ~/modfetch/config.yml
  ```
- See docs/CONFIG.md for full schema.

Usage (see docs/USER_GUIDE.md for details)
- Validate config:
  
  modfetch config validate --config /path/to/config.yml
  
- Download with live progress (speed, ETA):
  
  modfetch download --config /path/to/config.yml --url 'https://proof.ovh.net/files/1Mb.dat'
  modfetch download --config /path/to/config.yml --url 'hf://org/repo/path?rev=main'
  modfetch download --config /path/to/config.yml --url 'civitai://model/123456?file=vae'
  
  - URL forms:
    - civitai://model/{id}[?version=...] is supported; base page URLs like https://civitai.com/models/{id} auto‑resolve to the latest version’s primary file
    - hf://org/repo/path?rev=... is supported
  - Default filename:
    - civitai:// uses `<ModelName> - <OriginalFileName>` if `--dest` is omitted (with collision‑safe suffixes)
    - others use the basename of the resolved URL
  - Quiet mode: add `--quiet`
  - On completion, a summary is printed (dest, size, SHA256, duration, average speed)
  - Cancel with Ctrl+C (SIGINT/SIGTERM); partial files are cleaned up
- Place artifacts into apps:
  
  modfetch place --config /path/to/config.yml --path /path/to/model.safetensors
  
- Batch downloads from YAML (optionally place after):
  
  modfetch download --config /path/to/config.yml --batch /path/to/jobs.yml --place
  
  - See docs/BATCH.md for the schema and examples
- TUI dashboard (live list, filters, per‑row speed/ETA):
  
  modfetch tui --config /path/to/config.yml
  
  - Keys:
    - Navigation: j/k (select), / (filter), m (menu), h/? (help)
    - Sorting: s (sort by speed), e (sort by ETA), o (clear sort)
    - Actions: n (new), r (refresh), d (details), g (group by status), t (toggle columns)
    - Per‑row actions: p (pause/cancel), y (retry), C (copy path), U (copy URL), O (open/reveal), D (delete staged), X (clear row)
  - Behavior:
    - Resolving spinner appears immediately, then planning → running
    - Live speed and ETA for both chunked and single‑stream fallback downloads
    - Accepts CivitAI model page URLs (https://civitai.com/models/ID) and rewrites them internally to the correct direct download URL
  - See the full TUI guide: docs/TUI_GUIDE.md
  - Preview the next‑gen TUI v2 (experimental):
    
    modfetch tui --config /path/to/config.yml --v2
- Verify checksums in state:
  
  modfetch verify --config /path/to/config.yml --all
  
  - Use `--only-errors` to show only problematic files; add `--summary` for totals and paths
- Deep‑verify safetensors and scan/repair a directory:
  
  modfetch verify --config /path/to/config.yml --scan-dir /path/to/models --safetensors-deep
  modfetch verify --config /path/to/config.yml --scan-dir /path/to/models --safetensors-deep --repair --quarantine-incomplete
  
  - Only errors + summary: add `--only-errors --summary`
- JSON summary (for scripting/CI):
  
  modfetch download --config /path/to/config.yml --url 'https://proof.ovh.net/files/1Mb.dat' --summary-json
  
- Placement dry‑run:
  
  modfetch place --config /path/to/config.yml --path /path/to/model.safetensors --dry-run
  

Resolvers
- See docs/RESOLVERS.md for hf:// and civitai:// formats, examples, and auth via env tokens.

Logging and metrics
- Control verbosity: `--log-level debug|info|warn|error`; `--json` for JSON logs
- Quiet mode: `--quiet` (download command)
- Metrics: Prometheus textfile exporter (see docs/METRICS.md)
- Troubleshooting: see docs/TROUBLESHOOTING.md

Contributing
- Requirements: Go 1.22+; GitHub CLI (gh) optional for releases
- Getting started:
  
  git clone https://github.com/<you>/modfetch
  cd modfetch
  make build && make test
  
- Development workflow:
  - Create a feature branch per change
  - Keep PRs focused and small; include rationale in the PR description
  - Update docs for user‑visible changes
  - Ensure tests pass: make test
  - Run a quick smoke test locally:
    
    ./bin/modfetch download --config /path/to/config.yml --url 'https://proof.ovh.net/files/1Mb.dat'
    
- PR checklist:
  - [ ] Tests pass (go test ./...)
  - [ ] Docs updated (README/USER_GUIDE as applicable)
  - [ ] No secrets in configs or logs
  - [ ] Manual smoke test completed for at least one public URL
- Release process (maintainers):
  - Tag: git tag -a vX.Y.Z -m "modfetch vX.Y.Z" && git push origin vX.Y.Z
  - Build artifacts: make release-dist (includes Linux/macOS binaries)
  - macOS Universal: make macos-universal && make checksums
  - Upload: gh release upload vX.Y.Z dist/* --clobber
  - See CHANGELOG.md for release notes

See CONTRIBUTING.md for full guidelines.

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
- TLS or HEAD failures: Downloader falls back to single‑stream when Range/HEAD is unsupported.
- Resume: Re‑running the same download will resume and verify integrity.

Roadmap
- Further TUI enhancements
- Placement/classifier refinements and presets
- Release packaging for more distros
