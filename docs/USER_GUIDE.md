# User Guide

This guide walks you through configuration, common workflows, and tips to get the most out of modfetch.

- Build: `make build` (produces `./bin/modfetch`)
- Config file path can be passed with `--config` or via the environment variable `MODFETCH_CONFIG`.

## Configure

Create a config file:

1) Minimal example (see docs/CONFIG.md for full schema)

version: 1
general:
  data_root: "~/modfetch-data"
  download_root: "~/Downloads/modfetch"
  placement_mode: "symlink"
sources:
  huggingface: { enabled: true, token_env: "HF_TOKEN" }
  civitai:     { enabled: true, token_env: "CIVITAI_TOKEN" }
validation:
  safetensors_deep_verify_after_download: true

2) Use the wizard

modfetch config wizard --out ~/.config/modfetch/config.yml

3) Export tokens (only if needed for gated content)

export HF_TOKEN=...    # Hugging Face
export CIVITAI_TOKEN=...  # CivitAI

## Core workflows

Set a default config if you like:

export MODFETCH_CONFIG=~/.config/modfetch/config.yml

### Download

Default naming
- civitai:// URIs default to `<ModelName> - <OriginalFileName>` (sanitized) under your download_root.
- Direct URLs default to the basename of the final URL with query/fragment removed and the name sanitized.
- The TUI (and importer) try a HEAD request for CivitAI direct endpoints to honor server-provided filenames via Content-Disposition when available.

modfetch download --config ~/.config/modfetch/config.yml \
  --url 'https://proof.ovh.net/files/1Mb.dat'

- Shows a live progress bar with speed and ETA.
- Supports resume (Range) and retries. Optionally, enable honoring server-provided Retry-After for HTTP 429 by setting `network.retry_on_rate_limit: true` in your config.
- On completion, prints a summary and writes a .sha256 sidecar.

For Hugging Face / CivitAI resolvers:

modfetch download --config ~/.config/modfetch/config.yml \
  --url 'hf://org/repo/path/to/file?rev=main'
modfetch download --config ~/.config/modfetch/config.yml \
  --url 'civitai://model/123456?file=myfile.safetensors'

#### Dry-run planning

Use `--dry-run` to plan without downloading. It resolves resolver URIs and direct URLs, computes the default destination (respecting resolver SuggestedFilename and `--naming-pattern`), and probes remote metadata (filename, size, Accept-Range). It performs no database or file writes.

- Secrets are never printed; only a boolean `auth_attached` is shown.
- If `network.disable_auth_preflight: true` is set in the config, the probe is skipped.

Examples:

modfetch download --config ~/.config/modfetch/config.yml \
  --url 'https://example.com/file.bin' --dry-run

# JSON plan (machine-readable)
modfetch download --config ~/.config/modfetch/config.yml \
  --url 'hf://org/repo/path?rev=main' --dry-run --summary-json

### Verify (state-based)

Verify a specific previously downloaded item recorded in the DB:

modfetch verify --config ~/.config/modfetch/config.yml --path /path/to/file

Verify all completed downloads in state:

modfetch verify --config ~/.config/modfetch/config.yml --all

Tips:
- Add `--only-errors` to show only problematic files
- Add `--summary` to print total scanned, error count, and a list of error paths
- Combine with `--safetensors-deep` to include deep validation in state-backed checks

### Deep-verify safetensors and directory scan/repair

Scan a directory of .safetensors/.sft for structural correctness and exact coverage:

modfetch verify --config ~/.config/modfetch/config.yml \
  --scan-dir /path/to/models --safetensors-deep

Show only errors and a summary:

modfetch verify --config ~/.config/modfetch/config.yml \
  --scan-dir /path/to/models --safetensors-deep \
  --only-errors --summary

Repair files with extra trailing bytes (safe, lossless) and quarantine incomplete ones:

modfetch verify --config ~/.config/modfetch/config.yml \
  --scan-dir /path/to/models --safetensors-deep --repair --quarantine-incomplete

Notes:
- Extra bytes: file is larger than the header-declared size; repair truncates to the exact declared size.
- Incomplete: file is smaller than declared; cannot be repaired in place. Quarantine and re-download.
- With validation.safetensors_deep_verify_after_download: true, deep verification runs right after each .safetensors download and fails the command if invalid.
- Automatic trimming of trailing bytes is always applied to .safetensors files after download finalize.

### Placement

Place a downloaded artifact into your app’s directory per your mapping rules:

modfetch place --config ~/.config/modfetch/config.yml --path /path/to/model.safetensors

Use --dry-run to preview without writing.

See docs/PLACEMENT.md and docs/RESOLVERS.md for details and examples.

### TUI dashboard

modfetch tui --config ~/.config/modfetch/config.yml

The TUI provides **7 tabs** for managing downloads and your model library:

**Download Tabs (0-4):**
- **All** (Tab 0): All downloads regardless of status
- **Pending** (Tab 1): Downloads waiting to start
- **Active** (Tab 2): Currently downloading files
- **Completed** (Tab 3): Successfully completed downloads
- **Failed** (Tab 4): Failed downloads

**Library & Settings (5-6):**
- **Library** (Tab 5 or `L`): Browse and search your downloaded models
  - View rich metadata: type, quantization, size, tags, descriptions
  - Search by name with `/`, filter by type and source
  - Mark favorites with `f`, scan directories with `S`
  - See docs/LIBRARY.md for complete guide
- **Settings** (Tab 6 or `M`): View configuration and token status
  - Directory paths, placement rules, download settings
  - HuggingFace and CivitAI token validation status
  - Read-only view (edit config file to make changes)

**Key Features:**
- Live speed and ETA with throughput tracking
- Sorting: s (speed), e (ETA), R (remaining bytes), o (clear)
- Actions: n (new), b (batch), y (retry), p (pause), D (delete)
- Per-row: C (copy path), U (copy URL), O (open/reveal)
- Auto-resolves CivitAI model page URLs
- Auto-recovery of running downloads on startup

See the full TUI guide: docs/TUI_GUIDE.md

## Tips

- Increase verbosity for debugging: add --log-level debug
- Use --json to emit JSON logs
- Use --summary-json on download to emit a single JSON summary at the end
- Put ./bin on PATH for convenience:

export PATH="$PWD/bin:$PATH"

