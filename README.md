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

What's new in v0.5.2
- **TUI v1 Enhanced**: Restored rich UI elements, vibrant colors, and proper borders to TUI v1 (refactored MVC)
- **TUI Stability**: Fixed critical startup issues and eliminated terminal escape sequences
- **Visual Feedback**: Enhanced colorful status indicators and comprehensive theming
- **Installation Improvements**: Fixed installer 404 errors and unbound variable issues
- **Navigation**: Restored proper arrow key navigation and help system functionality

Previous releases:
- v0.5.1: Critical installer and TUI navigation fixes
- v0.5.0: Comprehensive installation package with guided setup experience

See full release notes and binaries: https://github.com/jxwalker/modfetch/releases

Installation
- **One-liner install** (recommended):
  ```bash
  curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash
  ```
- **Custom install directory**:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash -s -- --install-dir ~/bin
  ```
- **From source**:
  - Build: `make build` (produces `./bin/modfetch`)
  - Test: `make test`
  - Cross‑platform artifacts: `make release-dist`
- **Binaries**: via GitHub Releases (Linux/macOS with SHA256 checksums)
- **Homebrew**: `brew install jxwalker/tap/modfetch` (coming soon)
- **Uninstall**: `curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/uninstall.sh | bash`
- See CHANGELOG.md for what's new each release

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
    - direct URLs use the basename of the final resolved URL; query/fragment is stripped and the name is sanitized
    - TUI and importer try a HEAD request for CivitAI direct endpoints to use server‑provided filenames when available
  - SHA256 expectation:
    - pass `--sha256 <HEX>` or `--sha256-file <path>` (.sha256 "hash  filename" format supported)
  - Quiet mode: add `--quiet`
  - Auth preflight: runs a lightweight HEAD/0–0 probe and fails early on 401/403 with guidance; disable with `--no-auth-preflight` or set `network.disable_auth_preflight: true` in config
  - Dry-run planning: use `--dry-run` to resolve URLs/URIs, compute the default destination, and probe remote metadata (filename, size, Accept-Range) without downloading or writing. Combine with `--summary-json` for machine-readable output.
    - Secrets are never printed (only a boolean `auth_attached`).
    - If `network.disable_auth_preflight: true` is set, the metadata probe is skipped.
    
        modfetch download --config /path/to/config.yml --url 'hf://org/repo/path?rev=main' --dry-run
        modfetch download --config /path/to/config.yml --url 'https://example.com/file.bin' --dry-run --summary-json
  - On completion, a summary is printed (dest, size, SHA256, duration, average speed)
  - Cancel with Ctrl+C (SIGINT/SIGTERM); partial files are cleaned up
- Place artifacts into apps:
  
  modfetch place --config /path/to/config.yml --path /path/to/model.safetensors
  
- Batch downloads from YAML (optionally place after):
  
  modfetch download --config /path/to/config.yml --batch /path/to/jobs.yml --place
  
  - See docs/BATCH.md for the schema and examples
- TUI dashboard (live list, filters, per‑row speed/ETA):
  
  modfetch tui --config /path/to/config.yml
  
  - **TUI v2 (default)**: Feature-rich interface with extensive UX upgrades
    - Keys: j/k (select), / (filter), m (menu), h/? (help)
    - Sorting: s (sort by speed), e (sort by ETA), R (remaining bytes), o (clear sort)
    - Actions: n (new), r (refresh), d (details), g (group by status), t (toggle columns)
    - Per‑row actions: p (pause/cancel), y (retry), C (copy path), U (copy URL), O (open/reveal), D (delete staged), X (clear row)
    - Live speed and ETA with throughput sparklines and comprehensive status indicators
  - **TUI v1 (refactored)**: Enhanced MVC architecture with vibrant colors and rich theming
    
    modfetch tui --config /path/to/config.yml --v1
    
    - Colorful status indicators (green for completed, red for failed, pink for active)
    - Clean borders and enhanced visual feedback
    - Full navigation support with discoverable help system
  - Behavior:
    - Resolving spinner appears immediately, then planning → running
    - Live speed and ETA for both chunked and single‑stream fallback downloads
    - Accepts CivitAI model page URLs (https://civitai.com/models/ID) and rewrites them internally to the correct direct download URL
    - The header marks the active sort (SPEED*/ETA*/[sort: remaining]); the Stats panel shows View indicators (Sort/Group/Column/Theme)
  - See the full TUI guide: docs/TUI_GUIDE.md
- Verify checksums in state:
  
  modfetch verify --config /path/to/config.yml --all
  
  - Use `--only-errors` to show only problematic files; add `--summary` for totals and paths
  - Write/refresh a sidecar: add `--fix-sidecar` to rewrite `<dest>.sha256` once verified
- Deep‑verify safetensors and scan/repair a directory:
  
  modfetch verify --config /path/to/config.yml --scan-dir /path/to/models --safetensors-deep
  modfetch verify --config /path/to/config.yml --scan-dir /path/to/models --safetensors-deep --repair --quarantine-incomplete
  
  - Only errors + summary: add `--only-errors --summary`
- JSON summary (for scripting/CI):
  
  modfetch download --config /path/to/config.yml --url 'https://proof.ovh.net/files/1Mb.dat' --summary-json
  
- Placement dry‑run:
  
  modfetch place --config /path/to/config.yml --path /path/to/model.safetensors --dry-run
  
- Clean partials and orphan sidecars:
  
  modfetch clean --config /path/to/config.yml --days 7 --include-next-to-dest --sidecars
  

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
  - CI will build and publish artifacts automatically for:
    - Linux: amd64, arm64
    - macOS: amd64, arm64, universal (fat) binary
    - Checksums (.sha256) for all artifacts
  - See CHANGELOG.md for release notes
  - Optional (local): `make release-dist` and `make macos-universal` if you want to reproduce artifacts locally

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
- See docs/ROADMAP.md for the consolidated, prioritized roadmap
- Further TUI enhancements
- Placement/classifier refinements and presets
- Release packaging for more distros
