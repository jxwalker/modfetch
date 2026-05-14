# User Guide

modfetch helps you choose the right model artifact, download it reliably, verify
it, and organize it for local runtimes such as Ollama, llama.cpp, MLX, ComfyUI,
AUTOMATIC1111/Forge, Transformers, and vLLM.

Use this guide when you want the end-to-end workflow. For a shorter product
overview, start with [README.md](../README.md); for flags and subcommands, use
[CLI_GUIDE.md](CLI_GUIDE.md).

- Current release: v0.8.1.
- Build from source: `make build` produces `./bin/modfetch`.
- Config path: pass `--config` or set `MODFETCH_CONFIG`.
- Secrets: keep provider tokens in environment variables, not YAML.

## Configure

Create a config file:

1. Minimal example. See [CONFIG.md](CONFIG.md) for the full schema.

```yaml
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
```

2. Use the wizard.

```bash
modfetch config wizard --out ~/.config/modfetch/config.yml
```

3. Export tokens only when needed for private, gated, or restricted content.

```bash
export HF_TOKEN=...    # Hugging Face
export CIVITAI_TOKEN=...  # CivitAI
```

## Core workflows

Set a default config if you like:

```bash
export MODFETCH_CONFIG=~/.config/modfetch/config.yml
```

### Download

Default naming
- civitai:// URIs default to `<ModelName> - <OriginalFileName>` (sanitized) under your download_root.
- Direct URLs default to the basename of the final URL with query/fragment removed and the name sanitized.
- The TUI (and importer) try a HEAD request for CivitAI direct endpoints to honor server-provided filenames via Content-Disposition when available.

```bash
modfetch download --config ~/.config/modfetch/config.yml \
  --url 'https://proof.ovh.net/files/1Mb.dat'
```

- Shows a live progress bar with speed and ETA.
- Supports resume (Range) and retries. Optionally, enable honoring server-provided Retry-After for HTTP 429 by setting `network.retry_on_rate_limit: true` in your config.
- On completion, prints a summary and writes a .sha256 sidecar.

For Hugging Face / CivitAI resolvers:

```bash
modfetch download --config ~/.config/modfetch/config.yml \
  --url 'hf://gpt2/README.md?rev=main'
modfetch download --config ~/.config/modfetch/config.yml \
  --url 'hf://org/repo/path/to/file?rev=main'
modfetch download --config ~/.config/modfetch/config.yml \
  --url 'hf://org/repo?rev=main&quant=Q4_K_M'
modfetch download --config ~/.config/modfetch/config.yml \
  --url 'civitai://model/123456?file=myfile.safetensors'
```

### Recommend a model for your hardware

If you do not know which repo or quantization to choose, start with
`recommend`. It detects the current machine, searches live providers, estimates
memory fit from file size, parameter count, and quantization metadata, and shows
the command it would use to download each result. The output also includes
runtime hints, such as llama.cpp/Ollama for GGUF files or ComfyUI/Stable
Diffusion WebUI for image safetensors.

```bash
modfetch recommend --task chat
modfetch recommend --task coding
modfetch recommend "llama 8b gguf" --ram-gb 32 --unified-memory --json
```

The `--task` flag tunes ranking for `chat`, `coding`, `embedding`, or `image`
models. To plan for a different host, use `--ram-gb`, `--vram-gb`, and
`--unified-memory`.

When you like a result, download it through the same resumable pipeline:

```bash
modfetch recommend --task coding --download --select 1
```

Use `--dry-run --summary-json` first when you want to verify the selected URI,
destination, remote size, range support, and attached auth state without writing
files.

modfetch remembers selected and skipped recommendations per task, query, and
hardware class. That history nudges future ranking without hiding fresh provider
results:

```bash
modfetch recommend --history
modfetch recommend --task coding --no-learn
```

The same selection path is available in the TUI. Launch `modfetch tui`, press
`G`, choose the task, hardware budget, provider, runtime or placement target,
maximum file size, and optional query, then press `Enter` on a recommendation
to start the normal resumable download. On the recommendation result list,
press `i` to inspect ranking rationale, runtime setup, placement prerequisites,
and the dry-run destination/transfer plan. Press `p` to run a live metadata
probe that checks the resolved URL, remote size, range support, and any learned
per-host transfer history before you commit to a large download.

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

### Library backup and sync

Export a portable catalog from your local library:

modfetch library export --config ~/.config/modfetch/config.yml \
  --output modfetch-catalog.json

Preview an import before writing to your local library:

modfetch library import --config ~/.config/modfetch/config.yml \
  --input modfetch-catalog.json --dry-run

Push or pull the same catalog through a sync target:

modfetch library sync push --config ~/.config/modfetch/config.yml \
  --target file:///srv/modfetch/catalog.json
modfetch library sync pull --config ~/.config/modfetch/config.yml \
  --target file:///srv/modfetch/catalog.json --dry-run
modfetch library sync pull --config ~/.config/modfetch/config.yml \
  --target https://example.com/modfetch-catalog.json --dry-run

Push supports `file://` and plain filesystem paths, which is useful for mounted shares or a local backup directory. Pull supports those local targets plus read-only HTTP(S) catalog URLs.

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

For automation, use the non-interactive snapshot path:

```bash
modfetch tui --config ~/.config/modfetch/config.yml --snapshot --json
```

This reports download status totals, library totals, favorite counts, source and
type counts, and configured roots without opening the TUI.

See the full TUI guide: docs/TUI_GUIDE.md

## Tips

- Increase verbosity for debugging: add --log-level debug
- Use --json to emit JSON logs
- Use --summary-json on download to emit a single JSON summary at the end
- Put ./bin on PATH for convenience:

export PATH="$PWD/bin:$PATH"
