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

- Live list of downloads
- Throughput and ETA per row
- Keys:
  - Navigation: j/k (select), / (filter), m (menu), h/? (help)
  - Sorting: s (sort by speed), e (sort by ETA), o (clear sort)
  - Actions: n (new), r (refresh), d (details), g (group by status), t (toggle columns)
  - Per-row actions: p (pause/cancel), y (retry), C (copy path), U (copy URL), O (open/reveal), D (delete staged), X (clear row)
- Behavior:
  - Resolving spinner row appears immediately after starting, then transitions to planning → running
  - Live speed and ETA for both chunked and single-stream fallback downloads
  - Supports pasting CivitAI model page URLs; they’re auto-resolved to the correct direct download
- See the full TUI guide: docs/TUI_GUIDE.md

## Tips

- Increase verbosity for debugging: add --log-level debug
- Use --json to emit JSON logs
- Use --summary-json on download to emit a single JSON summary at the end
- Put ./bin on PATH for convenience:

export PATH="$PWD/bin:$PATH"

