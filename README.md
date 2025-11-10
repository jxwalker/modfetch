# modfetch

> **Fast, resilient downloads for AI models** • Download, verify, and organize LLM and Stable Diffusion models from HuggingFace and CivitAI

```
╔═══════════════════════════════════════════════════════════╗
║  🚀 Parallel Chunked Downloads with Auto-Resume          ║
║  ✓  SHA256 Integrity Verification                        ║
║  📚 Rich TUI with Model Library Browser                   ║
║  🎯 Smart Classification & Auto-Placement                 ║
║  ⚡ 10-100x Faster with Indexed Model Discovery          ║
╚═══════════════════════════════════════════════════════════╝
```

## Quick Links

📖 **New User?** Start here: **[Quick Start Guide](docs/QUICKSTART.md)** ← Visual walkthrough in 5 minutes!

📊 **Visual Learner?** See: **[TUI Wireframes](docs/TUI_WIREFRAMES.md)** ← Screenshots and navigation flows

📚 **Documentation:**
- [CLI Reference](docs/CLI_GUIDE.md) - Complete command-line reference
- [TUI Guide](docs/TUI_GUIDE.md) - Interactive terminal interface
- [Library Guide](docs/LIBRARY.md) - Browse and organize your models
- [User Guide](docs/USER_GUIDE.md) - Full feature reference
- [Configuration](docs/CONFIG.md) - Config file options

---

## Features at a Glance

### 🚀 Downloads
- **Parallel chunked downloads** with automatic resume and retries
- **SHA256 verification** (per-chunk and full file)
- **Smart naming** with collision-safe suffixes
- **Resolver support** for `hf://` and `civitai://` URLs
- **Auth preflight** with early failure detection
- **Rate limit handling** with automatic retry

### 📚 Model Library
- **Browse and search** all your downloaded models
- **Rich metadata** from HuggingFace and CivitAI APIs
- **Favorites system** to mark important models
- **Directory scanner** to discover existing models (10-100x faster with indexes)
- **Detailed view** with specs, descriptions, tags, and links
- **Filter by type and source** (LLM, LoRA, VAE, Checkpoint, etc.)

### 🎯 Organization
- **Automatic placement** into app directories
- **Smart classification** by model type
- **Symlink or copy** modes
- **Batch YAML** for bulk operations
- **Dry-run mode** to preview actions

### 🖥️ Rich TUI
- **7 tabs**: All, Pending, Active, Completed, Failed, Library, Settings
- **Live progress** with speed, ETA, and throughput sparklines
- **Interactive actions**: pause, retry, delete, reveal, copy
- **Themes**: Default, Neon, Dracula, Solarized
- **Filter and sort** downloads by various criteria
- **Keyboard shortcuts** for power users

### 🔧 Developer-Friendly
- **Structured logging** (JSON or text)
- **Metrics export** (Prometheus textfile)
- **Resilient SQLite** state tracking
- **Graceful cancellation** (SIGINT/SIGTERM)
- **JSON output** for scripting

---

## TUI Preview

The modfetch TUI provides a beautiful, full-featured interface for managing your AI models:

```
╔═══════════════════════════════════════════════════════════════════════════╗
║  modfetch v0.6.0                    Tab: [2] Active                       ║
╠═══════════════════════════════════════════════════════════════════════════╣
║  🔄 Active: 2   ✓ Completed: 45   ⏳ Pending: 3   ✗ Failed: 1           ║
║  Throughput: 32.5 MB/s   •   Auth: HF ✓  CivitAI ✓                       ║
╠═══════════════════════════════════════════════════════════════════════════╣
║  Status    │ Progress        │ Speed      │ ETA    │ Size  │ File        ║
║  ──────────┼─────────────────┼────────────┼────────┼───────┼─────────    ║
║▶ Running   │ ████████░░░ 45% │ 18.3 MB/s  │ 2m 15s │ 3.8GB │ llama-2...  ║
║  Running   │ ██████░░░░░ 32% │ 14.2 MB/s  │ 3m 42s │ 4.1GB │ mistral...  ║
║  Planning  │ ...             │ -          │ -      │ 2.2GB │ sdxl-base...║
╠═══════════════════════════════════════════════════════════════════════════╣
║  n:New  y:Retry  p:Pause  5/L:Library  6/M:Settings  ?:Help  q:Quit     ║
╚═══════════════════════════════════════════════════════════════════════════╝
```

**Key Features:**
- 🎨 **Multiple themes** (Neon, Dracula, Solarized) - Press `T` to cycle
- ⌨️ **Vim-style navigation** (`j`/`k`) plus arrow keys
- 📊 **Real-time stats** with speed graphs and progress bars
- 🔍 **Search and filter** with `/` key
- 📚 **Browse library** with `5` or `L`
- ⚙️ **View settings** with `6` or `M`

> **See it in action:** Check out the [TUI Wireframes](docs/TUI_WIREFRAMES.md) for detailed visual guides!

---

## Status

✅ **Production Ready:** Core download, verify, and TUI features are stable

🚀 **Active Development:** Library enhancements, bulk operations, advanced filters

📖 **Documentation:** Comprehensive guides with visual examples

---

## What's new in v0.6.0
- **Library View**: Browse all your downloaded models with rich metadata, search, and filters
  - View model details: type, quantization, size, source, tags, descriptions
  - Search by name, filter by type (LLM, LoRA, VAE, etc.) and source (HuggingFace, CivitAI, local)
  - Mark models as favorites for quick access
  - Keyboard shortcuts: `5` or `L` to access Library, `/` to search, `Enter` for details
- **Directory Scanner**: Automatically discover models in your configured directories
  - Scans download_root and placement directories
  - Extracts metadata from filenames (quantization, parameter count, version)
  - O(log n) duplicate detection with database indexes (10-100x faster than linear scan)
  - Press `S` in Library view to trigger a scan
- **Settings Tab**: View your configuration at a glance
  - See directory paths, API token status, placement rules, download settings
  - Visual indicators for HuggingFace and CivitAI token status
  - Press `6` or `M` to access Settings
- **Performance Optimizations**: Added database indexes for 10-100x speedup on large model libraries
- **Comprehensive Testing**: 84 test cases including unit, integration, and performance benchmarks
- **Documentation**: Complete user guides for Library (docs/LIBRARY.md) and Scanner (docs/SCANNER.md)

Previous releases:
- v0.5.2: Enhanced TUI with rich UI elements and vibrant colors
- v0.5.1: Critical installer and TUI navigation fixes
- v0.5.0: Comprehensive installation package with guided setup experience

See full release notes and binaries: https://github.com/jxwalker/modfetch/releases

## Installation

### One-Line Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash
```

**What it does:**
```
✓ Downloads latest release
✓ Verifies SHA256 checksum
✓ Installs to /usr/local/bin
✓ Makes executable
→ Ready to use: modfetch --version
```

### Alternative Methods

<details>
<summary><b>📦 Custom Install Directory</b></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash -s -- --install-dir ~/bin
```
</details>

<details>
<summary><b>🔨 Build from Source</b></summary>

```bash
git clone https://github.com/jxwalker/modfetch
cd modfetch
make build    # Produces ./bin/modfetch
make test     # Run tests
```
</details>

<details>
<summary><b>📥 Download Binary (GitHub Releases)</b></summary>

Download from [Releases](https://github.com/jxwalker/modfetch/releases):
- Linux: amd64, arm64
- macOS: amd64, arm64, universal
- All with SHA256 checksums
</details>

<details>
<summary><b>🍺 Homebrew (Coming Soon)</b></summary>

```bash
brew install jxwalker/tap/modfetch
```
</details>

<details>
<summary><b>🗑️ Uninstall</b></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/uninstall.sh | bash
```
</details>

## Get Started in 3 Steps

> 📖 **Want a detailed walkthrough?** See the **[Quick Start Guide](docs/QUICKSTART.md)** with visual examples!

### 1. Install (1 minute)

```bash
curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash
```

### 2. Configure (1 minute)

```bash
mkdir -p ~/.config/modfetch
cat > ~/.config/modfetch/config.yml << 'YAML'
version: 1
general:
  data_root: "~/modfetch-data"
  download_root: "~/Downloads/modfetch"
  placement_mode: "symlink"
sources:
  huggingface: { enabled: true, token_env: "HF_TOKEN" }
  civitai:     { enabled: true, token_env: "CIVITAI_TOKEN" }
YAML

export MODFETCH_CONFIG=~/.config/modfetch/config.yml
```

### 3. Your First Download

```bash
# Test with a small file
modfetch download --url 'https://proof.ovh.net/files/1Mb.dat'

# Or launch the TUI dashboard
modfetch tui
```

**Result:**
```
✓ Download complete
✓ SHA256 verified
✓ Saved to ~/Downloads/modfetch/1Mb.dat
```

### Optional: API Tokens (for private/gated content)

```bash
export HF_TOKEN="your_token"        # Get from: https://huggingface.co/settings/tokens
export CIVITAI_TOKEN="your_token"   # Get from: https://civitai.com/user/account
```

---

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

## Usage

> 📚 **Full details:** See [CLI Reference](docs/CLI_GUIDE.md) | [TUI Guide](docs/TUI_GUIDE.md) | [User Guide](docs/USER_GUIDE.md)

### Quick Command Reference

```bash
# Launch TUI (recommended for visual management)
modfetch tui

# Download a file
modfetch download --url 'URL'

# Verify downloads
modfetch verify --all

# Place model into app directory
modfetch place --path /path/to/model.safetensors

# Batch downloads
modfetch download --batch jobs.yml --place
```

### Detailed Examples
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

  - **Feature-rich interface** with extensive UX upgrades
    - **7 Tabs**: All, Pending, Active, Completed, Failed, **Library**, **Settings**
    - **Library (Tab 5 or L)**: Browse downloaded models, search, filter, view details
      - Search by name with `/`, filter by type/source
      - View rich metadata: quantization, size, tags, descriptions
      - Mark favorites with `f`, scan directories with `S`
      - See docs/LIBRARY.md for full guide
    - **Settings (Tab 6 or M)**: View configuration and token status
      - See directory paths, placement rules, download settings
      - Check HuggingFace and CivitAI token status
      - Visual indicators for token validation
    - Keys: j/k (select), / (filter/search), m (menu), h/? (help)
    - Sorting: s (sort by speed), e (sort by ETA), R (remaining bytes), o (clear sort)
    - Actions: n (new), r (refresh), d (details), g (group by status), t (toggle columns)
    - Per‑row actions: p (pause/cancel), y (retry), C (copy path), U (copy URL), O (open/reveal), D (delete staged), X (clear row)
    - Live speed and ETA with throughput sparklines and comprehensive status indicators
  - Behavior:
    - Resolving spinner appears immediately, then planning → running
    - Live speed and ETA for both chunked and single‑stream fallback downloads
    - Accepts CivitAI model page URLs (https://civitai.com/models/ID) and rewrites them internally to the correct direct download URL
    - The header marks the active sort (SPEED*/ETA*/[sort: remaining]); the Stats panel shows View indicators (Sort/Group/Column/Theme)
  - See the full TUI guide: docs/TUI_GUIDE.md
  - See the Library guide: docs/LIBRARY.md
  - See the Scanner guide: docs/SCANNER.md
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
- internal/scanner: directory scanner for model discovery
- internal/metadata: metadata fetchers for HuggingFace and CivitAI
- internal/tui: TUI models (Library, Scanner, Settings, Downloads)
- internal/state: SQLite state DB with indexed metadata storage
- internal/metrics: Prometheus textfile metrics
- docs/: configuration, testing, placement, resolvers, library, scanner
- scripts/: smoke test and helpers

Troubleshooting
- Missing tokens: Set `HF_TOKEN` or `CIVITAI_TOKEN` in your environment when accessing private resources.
- TLS or HEAD failures: Downloader falls back to single‑stream when Range/HEAD is unsupported.
- Resume: Re‑running the same download will resume and verify integrity.
- Library not showing models: Press `S` in Library view to scan your directories.
- Slow scanning: First scan may take time; subsequent scans use indexed duplicate detection (10-100x faster).

Roadmap
- See docs/ROADMAP.md for the consolidated, prioritized roadmap
- Further TUI enhancements (bulk operations, advanced filters)
- Placement/classifier refinements and presets
- Release packaging for more distros
- Parallel directory scanning
- Export/import library catalogs
