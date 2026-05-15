# modfetch

[![CI](https://github.com/jxwalker/modfetch/actions/workflows/ci.yml/badge.svg)](https://github.com/jxwalker/modfetch/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/jxwalker/modfetch)](https://github.com/jxwalker/modfetch/releases)
[![Licence](https://img.shields.io/github/license/jxwalker/modfetch)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8)](go.mod)

**The model downloader that helps you choose, fetch, verify, resume, and
organize AI model files.**

modfetch is built for the real local-model workflow: you need the right GGUF,
safetensors, tokenizer, LoRA, checkpoint, or archive; you may not know the exact
URL; downloads can be huge; partials must resume; and once the file lands it
needs to be usable in Ollama, llama.cpp, MLX, ComfyUI, AUTOMATIC1111/Forge,
Transformers, vLLM, or another local runtime.

```text
Choose a model      Resume huge files      Verify integrity      Organize library
recommend / TUI --> adaptive download --> SHA256/safetensors --> place / scan / sync
```

## Why Use It

- **No more copied mystery URLs**: search Hugging Face, CivitAI, and ModelScope
  with `discover`, or let `recommend` rank real downloadable files for your
  task and hardware.
- **Built for large model transfers**: chunked resume, retries, rate-limit
  handling, Hugging Face/Xet-friendly tuning, and live adaptive ramp-up/backoff.
- **Better than a raw downloader for model work**: resolvers, auth preflight,
  metadata, checksums, placement presets, library scanning, and SQLite state are
  all part of the same workflow.
- **Beginner-friendly, power-user capable**: start with curated starter
  downloads or the guided TUI, then move to JSON output, batch YAML, catalog
  sync, and benchmark history when you need automation.
- **Local-first and transparent**: config is YAML, secrets stay in environment
  variables, and dry-runs show the planned destination and transfer metadata
  before bytes are written.

## Install

```bash
brew tap jxwalker/tap
brew install jxwalker/tap/modfetch
```

Other supported install paths:

```bash
# One-line installer
curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash

# Arch Linux / AUR
git clone https://aur.archlinux.org/modfetch-bin.git
cd modfetch-bin
makepkg -si

# From source
git clone https://github.com/jxwalker/modfetch
cd modfetch
make build
```

See [docs/INSTALLATION.md](docs/INSTALLATION.md) for release binaries, custom
install directories, shell completions, and troubleshooting.

## Quick Start

If you do not know what to download yet, start here:

```bash
# One-time setup for Homebrew/source installs. The one-line installer runs this
# for you, but the command is harmless if the config already exists.
mkdir -p ~/.config/modfetch
modfetch config wizard --out ~/.config/modfetch/config.yml
export MODFETCH_CONFIG=~/.config/modfetch/config.yml

# Beginner path: choose a small coding model that fits this machine.
modfetch get coding --small

# Preview the exact selected download without writing files.
modfetch get coding --small --dry-run --summary-json

# Download the top result through the resumable transfer pipeline.
modfetch get coding --small --download
```

If you already know the source:

```bash
# Direct URL, Hugging Face resolver, CivitAI resolver, or starter alias.
modfetch download --url 'https://example.com/model.safetensors'
modfetch download --url 'hf://gpt2/README.md?rev=main'
modfetch download --url 'civitai://model/123456'
modfetch download --url 'starter://gpt2-tokenizer'
```

If you want a visual workflow:

```bash
modfetch tui
```

Press `G` in the TUI to choose a model by task, hardware, provider, runtime,
and maximum file size. Press `i` on a result to inspect ranking rationale,
runtime setup, placement readiness, and transfer settings. Press `p` to probe
remote size, range support, server filename, validators, and learned host
history before starting a large download.

## Common Workflows

### Find and Download a Real Model

```bash
modfetch get coding --small
modfetch get coding --small --download
modfetch discover search "tiny gpt2"
modfetch discover download "sshleifer/tiny-gpt2" --select 1
```

`get` is the beginner path: task and size presets feed the recommendation
engine, then optional `--download` delegates to the normal transfer pipeline.
Use `discover` when you want to search a provider by name and select a concrete
artifact yourself.

### Download a Huge GGUF Without Hand-Tuning

```bash
modfetch download --url 'hf://owner/repo/model.gguf?rev=main' --profile auto
```

`--profile auto` promotes large range-capable objects to large-model tuning.
modfetch starts from persisted per-host history when available, ramps up while
throughput is healthy, and backs off on stalls or HTTP 429s.

You can still force aria2-style settings for a specific transfer:

```bash
modfetch download --url 'https://huggingface.co/owner/repo/resolve/main/model.gguf' \
  --connections 16 --chunk-size-mb 64
```

### Benchmark Before a Multi-Hour Download

```bash
modfetch bench --url 'hf://owner/repo/model.gguf?rev=main' \
  --tools modfetch,aria2 --duration 30s --json

modfetch bench --history
```

Benchmark runs disposable samples against the same URL and records host/tuning
history that later adaptive downloads can reuse.

### Place Models Where Local Apps Expect Them

```bash
modfetch place --path ~/Downloads/modfetch/model.gguf --preset ollama --dry-run
modfetch place --path ~/Downloads/modfetch/model.safetensors --preset comfyui
```

Placement presets cover Ollama, ComfyUI, AUTOMATIC1111, Forge, and generic
Hugging Face cache exports. GGUF recommendations still include runtime guidance
for llama.cpp-style tools; use symlink, hardlink, or copy mode depending on your
filesystem.

### Manage the Library

```bash
modfetch library scan --repair-stale
modfetch library export --output modfetch-catalog.json
modfetch library import --input modfetch-catalog.json --dry-run
modfetch library sync push --target file:///srv/modfetch/catalog.json
modfetch library sync pull --target https://example.com/modfetch-catalog.json --dry-run
```

The library keeps metadata, favorites, source URLs, checksums, placement hints,
and scan results in a local SQLite state database.

### Verify and Repair

```bash
modfetch verify --all --summary
modfetch verify --scan-dir ~/models --safetensors-deep --only-errors --summary
modfetch verify --scan-dir ~/models --safetensors-deep --repair --quarantine-incomplete
```

modfetch can verify recorded SHA256 values, refresh sidecars, deep-check
safetensors structure, trim safe trailing bytes, and quarantine incomplete
files for redownload.

## Feature Tour

| Area | What You Get |
| --- | --- |
| Model selection | `recommend`, `discover`, starter aliases, task presets, hardware fit, runtime hints, local learning history |
| Download engine | direct HTTPS, `starter://`, `hf://`, `civitai://`, chunked resume, auth preflight, retries, rate-limit handling, SHA256 sidecars |
| Large transfers | `--profile auto`, `--profile large-model`, explicit connections/chunk size, adaptive ramp-up/backoff, persisted per-host history |
| Benchmarking | modfetch-vs-aria2 samples on the same URL, JSON output, benchmark history |
| TUI | downloads, library, settings, guided recommendations, result inspection, live metadata probing, themes, filters, bulk actions |
| Library | indexed scans, rich metadata, favorites, type/source filters, stale repair, portable export/import/sync |
| Placement | presets for Ollama, ComfyUI, AUTOMATIC1111/Forge, Hugging Face cache exports, symlink/hardlink/copy modes |
| Verification | SHA256, sidecars, safetensors structural checks, repair/quarantine flows |
| Automation | batch YAML, JSON summaries, JSON logs, TUI snapshots, shell completions, Prometheus textfile metrics |
| Distribution | Homebrew/Linuxbrew, AUR `modfetch-bin`, GitHub Release binaries, one-line installer, source builds |

## TUI Preview

```text
+------------------------------ modfetch v0.8.1 ------------------------------+
| Tab: [2] Active       Completed: 45   Active: 2   Pending: 3   Failed: 1     |
+------------------------------------------------------------------------------+
| Status    Progress           Speed       ETA       Size      File             |
| Running   ########.... 45%    18.3 MB/s   2m15s     3.8 GB    llama-8b.gguf   |
| Running   ######...... 32%    14.2 MB/s   3m42s     4.1 GB    mistral.gguf    |
| Planning  ...                 -           -         2.2 GB    sdxl-base.sft   |
+------------------------------------------------------------------------------+
| G recommend | n new | b batch | y/r start | p cancel | / filter | ? help      |
+------------------------------------------------------------------------------+
```

The TUI is not just a progress display. It includes:

- Guided recommendations with `G`
- Result inspection and live metadata probing before large downloads
- Library search, filters, details, favorites, and bulk actions
- Settings and token-status views
- Download sorting by speed, ETA, or remaining bytes
- Non-interactive snapshots for scripts: `modfetch tui --snapshot --json`

See [docs/TUI_GUIDE.md](docs/TUI_GUIDE.md) and
[docs/TUI_WIREFRAMES.md](docs/TUI_WIREFRAMES.md).

## Configuration

Create a starter config with the wizard:

```bash
mkdir -p ~/.config/modfetch
modfetch config wizard --out ~/.config/modfetch/config.yml
```

Or create a minimal config:

```yaml
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
```

Use tokens only when needed for private, gated, or restricted content:

```bash
export HF_TOKEN="..."
export CIVITAI_TOKEN="..."
export MODFETCH_CONFIG=~/.config/modfetch/config.yml
```

Secrets should stay in environment variables. modfetch redacts secrets from
logs and dry-run output.

## Command Map

```text
config      validate, print, or generate YAML config
download    fetch one URL/resolver URI or a batch file
bench       compare modfetch and aria2, or inspect transfer history
discover    search providers and download a selected result
get         beginner task presets for choosing and downloading models
recommend   rank model files for task, hardware, runtime, and memory fit
starter     list or download beginner-safe starter artifacts
status      show persisted download status
tui         open the terminal dashboard or print a snapshot
library     scan, export, import, and sync the model catalog
batch       import URLs from a text file and produce a YAML batch
place       link/copy models into app-specific directories
verify      check SHA256 and safetensors integrity
clean       prune staged partials and orphan sidecars
dedupe      replace duplicate completed downloads with links
completion  generate shell completions
```

Run `modfetch help` or read [docs/CLI_GUIDE.md](docs/CLI_GUIDE.md) for the full
reference.

## Documentation

Start here:

- [Documentation Index](docs/README.md): every guide in one place
- [Quick Start](docs/QUICKSTART.md): installation, config, first download, TUI
- [User Guide](docs/USER_GUIDE.md): end-to-end workflows
- [CLI Guide](docs/CLI_GUIDE.md): command and flag reference
- [TUI Guide](docs/TUI_GUIDE.md): keyboard controls and visual workflow
- [Installation](docs/INSTALLATION.md): Homebrew, AUR, installer, binaries, source

Deep dives:

- [Batch Jobs](docs/BATCH.md)
- [Configuration](docs/CONFIG.md)
- [Resolvers](docs/RESOLVERS.md)
- [Placement](docs/PLACEMENT.md)
- [Library](docs/LIBRARY.md)
- [Scanner](docs/SCANNER.md)
- [Metrics](docs/METRICS.md)
- [Database](docs/DATABASE.md)
- [Completions](docs/COMPLETIONS.md)
- [Linux Deployment](docs/DEPLOY_LINUX.md)
- [Systemd TUI](docs/SYSTEMD_TUI.md)
- [Testing Philosophy](docs/TESTING.md)
- [Testing Command Guide](docs/TESTING_GUIDE.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [Release Checklist](docs/RELEASE.md)
- [Roadmap](docs/ROADMAP.md)
- [TUI Wireframes](docs/TUI_WIREFRAMES.md)
- [TUI Analysis Summary](docs/TUI_ANALYSIS_SUMMARY.txt)
- [AUR Packaging](packaging/aur/README.md)

## Status

Current release: **v0.8.1**, tagged 2026-05-14.

Shipped highlights:

- v0.8.1: guided TUI recommendations, result inspection, live transfer metadata
  probing, and recommendation release hardening.
- v0.8.0: hardware-aware recommendations, local learning history, runtime
  hints, benchmark-driven tuning, and adaptive transfer behavior.
- v0.7.1: starter downloads, real-provider discovery, ModelScope discovery, AUR
  packaging, catalog sync, metadata enrichment, and TUI snapshots.
- v0.7.0: Homebrew distribution, portable catalogs, TUI bulk operations,
  placement presets, scanner repair, and docs drift validation.

The v0.8.x release line is closed. The next roadmap item is v0.9 planning based
on real user feedback.

## Development

```bash
git clone https://github.com/jxwalker/modfetch
cd modfetch
make build
go test -count=1 ./...
make lint
scripts/check-docs-drift.sh
```

Project layout:

```text
cmd/modfetch         CLI entry point
internal/downloader  direct, chunked, adaptive, and resumable transfers
internal/resolver    starter, Hugging Face, and CivitAI resolver support
internal/recommend   hardware-aware recommendation engine
internal/discovery   provider search and result selection
internal/tui         Bubble Tea dashboard, library, settings, recommendations
internal/state       SQLite state and metadata storage
internal/metadata    Hugging Face, CivitAI, ModelScope, Ollama enrichment
internal/placer      app placement and preset logic
docs/                user and maintainer documentation
scripts/             installer, release, UAT, and validation helpers
```

Please keep PRs focused, update docs for user-visible behavior, avoid logging
secrets, and include a real smoke test for provider/download changes.

## Licence

MIT. See [LICENCE](LICENSE).
