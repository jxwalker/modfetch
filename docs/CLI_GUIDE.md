# CLI Reference Guide

Complete command-line reference for modfetch. For the visual TUI interface, see [TUI Guide](TUI_GUIDE.md).

## Table of Contents

- [Overview](#overview)
- [Global Flags](#global-flags)
- [Commands](#commands)
  - [download](#download)
  - [verify](#verify)
  - [place](#place)
  - [clean](#clean)
  - [config](#config)
  - [tui](#tui)
- [URL Formats](#url-formats)
- [Examples](#examples)
- [Scripting](#scripting)

---

## Overview

modfetch provides a rich CLI for downloading, verifying, and managing AI models. All commands support:

- **Structured output** with `--summary-json` for scripting
- **Flexible logging** with `--log-level` and `--json`
- **Configuration** via `--config` flag or `MODFETCH_CONFIG` environment variable

**Quick command summary:**
```bash
modfetch download --url URL       # Download a file
modfetch verify --all              # Verify all downloads
modfetch place --path FILE         # Place model into app
modfetch clean --days 7            # Clean old partials
modfetch config validate           # Validate config
modfetch tui                       # Launch visual TUI
```

---

## Global Flags

These flags work across all commands:

| Flag | Description | Default |
|------|-------------|---------|
| `--config PATH` | Path to YAML config file | `$MODFETCH_CONFIG` or `~/.config/modfetch/config.yml` |
| `--log-level LEVEL` | Log verbosity: `debug`, `info`, `warn`, `error` | `info` |
| `--json` | Output logs as JSON | `false` |
| `--help` | Show help for command | - |

**Examples:**
```bash
# Use specific config
modfetch download --config ~/my-config.yml --url URL

# Debug logging
modfetch download --log-level debug --url URL

# JSON logs for parsing
modfetch verify --all --json
```

---

## Commands

### download

Download a file from a URL with resume, chunking, and verification.

**Syntax:**
```bash
modfetch download --url URL [OPTIONS]
```

**Required:**
- `--url URL` - URL to download (supports `https://`, `hf://`, `civitai://`)

**Optional:**
- `--dest PATH` - Destination file path (default: auto-generated from URL)
- `--sha256 HEX` - Expected SHA256 hash for verification
- `--sha256-file PATH` - File containing expected hash (`.sha256` format)
- `--batch PATH` - YAML file with multiple downloads (see [BATCH.md](BATCH.md))
- `--place` - Automatically place files after download (with `--batch`)
- `--batch-parallel N` - Concurrent downloads in batch mode
- `--dry-run` - Preview download without actually downloading
- `--summary-json` - Output JSON summary instead of human-readable
- `--quiet` - Suppress human-readable output (keeps errors)
- `--no-auth-preflight` - Skip authentication check (override config)

**URL Formats:**

```bash
# Direct HTTPS
modfetch download --url 'https://example.com/model.safetensors'

# HuggingFace (requires HF_TOKEN for private repos)
modfetch download --url 'hf://TheBloke/Llama-2-7B-GGUF/llama-2-7b.Q4_K_M.gguf?rev=main'

# CivitAI (requires CIVITAI_TOKEN for restricted content)
modfetch download --url 'civitai://model/123456'
modfetch download --url 'civitai://model/123456?version=456789'
modfetch download --url 'civitai://model/123456?file=specific-file.safetensors'
```

**Examples:**

```bash
# Simple download
modfetch download --url 'https://proof.ovh.net/files/1Mb.dat'

# Download with verification
modfetch download --url 'https://example.com/model.bin' \
  --sha256 'abc123...'

# Download to specific path
modfetch download --url 'hf://org/repo/model.gguf' \
  --dest ~/models/my-model.gguf

# Batch download
modfetch download --batch jobs.yml --place

# Dry run (preview without downloading)
modfetch download --url 'hf://org/repo/model.gguf' --dry-run

# JSON output for scripts
modfetch download --url 'https://example.com/file.bin' --summary-json
```

**Output:**

Human-readable (default):
```
Downloading: model.safetensors
Progress: ████████████████████ 100%
Speed: 45.2 MB/s
✓ Download complete
✓ SHA256 verified
✓ Size: 3.8 GB
✓ Time: 1m 24s
✓ Saved: ~/Downloads/modfetch/model.safetensors
```

JSON (`--summary-json`):
```json
{
  "url": "https://example.com/model.safetensors",
  "dest": "/home/user/Downloads/modfetch/model.safetensors",
  "size": 4089730000,
  "sha256": "abc123...",
  "duration_seconds": 84.5,
  "avg_speed_mbps": 45.2,
  "status": "completed"
}
```

---

### verify

Verify checksums of downloaded files.

**Syntax:**
```bash
modfetch verify [OPTIONS]
```

**Modes:**

**1. Verify specific file:**
```bash
modfetch verify --path FILE
```

**2. Verify all downloads:**
```bash
modfetch verify --all
```

**3. Scan directory:**
```bash
modfetch verify --scan-dir PATH
```

**Optional:**
- `--only-errors` - Show only files with verification errors
- `--summary` - Print summary with totals and error paths
- `--safetensors-deep` - Deep verify `.safetensors` files (structure + coverage)
- `--repair` - Repair safetensors with trailing bytes (trim to exact size)
- `--quarantine-incomplete` - Move incomplete safetensors to `.quarantine/`
- `--fix-sidecar` - Write/refresh `.sha256` sidecar files after verification

**Examples:**

```bash
# Verify specific file
modfetch verify --path ~/models/llama-2-7b.gguf

# Verify all downloads in state
modfetch verify --all

# Show only errors with summary
modfetch verify --all --only-errors --summary

# Deep-verify safetensors directory
modfetch verify --scan-dir ~/models/sd --safetensors-deep

# Repair safetensors with extra bytes
modfetch verify --scan-dir ~/models \
  --safetensors-deep --repair --quarantine-incomplete

# Verify and update sidecar files
modfetch verify --all --fix-sidecar
```

**Output:**

```
Verifying: ~/models/llama-2-7b.gguf
✓ SHA256 matches (abc123...)
✓ Size: 3.8 GB

Verifying: ~/models/model.safetensors
✗ Extra bytes: file is 12 bytes larger than header declares
  (use --repair to trim)

Summary:
  Total: 42 files
  Passed: 41
  Failed: 1
  Errors:
    - ~/models/model.safetensors
```

---

### place

Place downloaded models into app directories based on placement rules.

**Syntax:**
```bash
modfetch place --path FILE [OPTIONS]
```

**Required:**
- `--path FILE` - Path to file to place

**Optional:**
- `--dry-run` - Preview placement without writing

**Examples:**

```bash
# Place a model
modfetch place --path ~/Downloads/llama-2-7b.gguf

# Preview placement
modfetch place --path ~/models/sdxl.safetensors --dry-run
```

**Output:**

```
Placing: ~/Downloads/llama-2-7b.gguf
Matched rule: *.gguf → /opt/ollama/models
Placement mode: symlink
✓ Created symlink: /opt/ollama/models/llama-2-7b.gguf → ~/Downloads/llama-2-7b.gguf
```

See [PLACEMENT.md](PLACEMENT.md) for configuration details.

---

### clean

Clean up partial downloads and orphaned sidecar files.

**Syntax:**
```bash
modfetch clean [OPTIONS]
```

**Optional:**
- `--days N` - Clean partials older than N days (default: 7)
- `--include-next-to-dest` - Also clean `.part` files next to destinations
- `--sidecars` - Clean orphaned `.sha256` sidecar files

**Examples:**

```bash
# Clean partials older than 7 days
modfetch clean --days 7

# Clean everything including sidecars
modfetch clean --days 3 --include-next-to-dest --sidecars

# Immediate cleanup (0 days)
modfetch clean --days 0
```

**Output:**

```
Cleaning partial downloads older than 7 days...
✓ Removed: ~/Downloads/modfetch/file1.part (14 days old)
✓ Removed: ~/Downloads/modfetch/file2.part (30 days old)

Cleaned: 2 files, freed 1.2 GB
```

---

### config

Configuration management commands.

**Subcommands:**

**1. Validate config:**
```bash
modfetch config validate [--config PATH]
```

**2. Generate config wizard:**
```bash
modfetch config wizard --out PATH
```

**Examples:**

```bash
# Validate config
modfetch config validate --config ~/modfetch/config.yml

# Interactive config wizard
modfetch config wizard --out ~/.config/modfetch/config.yml
```

**Output (validate):**

```
✓ Config is valid
  - Data root: ~/modfetch-data
  - Download root: ~/Downloads/modfetch
  - Placement mode: symlink
  - Sources: huggingface (enabled), civitai (enabled)
```

See [CONFIG.md](CONFIG.md) for full configuration reference.

---

### tui

Launch the interactive Terminal User Interface.

**Syntax:**
```bash
modfetch tui [--config PATH]
```

**Examples:**

```bash
# Launch TUI with default config
modfetch tui

# Use specific config
modfetch tui --config ~/modfetch/config.yml
```

See [TUI Guide](TUI_GUIDE.md) and [TUI Wireframes](TUI_WIREFRAMES.md) for full documentation.

---

## URL Formats

### Direct HTTPS

Any valid HTTPS URL:
```bash
https://example.com/path/to/model.safetensors
https://example.com/files/model.bin?query=param
```

**Filename resolution:**
- Uses clean basename (strips query/fragment)
- Sanitizes for filesystem safety
- Adds collision-safe suffix if file exists

---

### HuggingFace (hf://)

**Format:**
```
hf://org/repo/path/to/file?rev=REVISION
```

**Components:**
- `org` - Organization or username
- `repo` - Repository name
- `path/to/file` - File path within repo
- `rev` - Git revision (branch, tag, or commit SHA)

**Examples:**
```bash
# Latest from main
hf://TheBloke/Llama-2-7B-GGUF/llama-2-7b.Q4_K_M.gguf

# Specific revision
hf://TheBloke/Llama-2-7B-GGUF/llama-2-7b.Q4_K_M.gguf?rev=main

# Specific commit
hf://meta/llama-2-7b/model.safetensors?rev=abc123def456
```

**Authentication:**
```bash
export HF_TOKEN="your_huggingface_token"
modfetch download --url 'hf://private/repo/model.gguf'
```

Get token: https://huggingface.co/settings/tokens

---

### CivitAI (civitai://)

**Format:**
```
civitai://model/MODEL_ID[?version=VERSION_ID][?file=FILENAME]
```

**Components:**
- `MODEL_ID` - Numeric model ID
- `version` (optional) - Specific version ID (defaults to latest)
- `file` (optional) - Specific file when version has multiple files

**Examples:**
```bash
# Latest version, primary file
civitai://model/123456

# Specific version
civitai://model/123456?version=789012

# Specific file
civitai://model/123456?file=model-fp16.safetensors

# Both version and file
civitai://model/123456?version=789012&file=vae.safetensors
```

**Authentication:**
```bash
export CIVITAI_TOKEN="your_civitai_token"
modfetch download --url 'civitai://model/123456'
```

Get token: https://civitai.com/user/account

**Note:** Also accepts direct CivitAI model page URLs:
```bash
# Auto-resolves to civitai://model/ID
modfetch download --url 'https://civitai.com/models/123456'
```

---

## Examples

### Basic Workflows

**Download and place a model:**
```bash
# 1. Download
modfetch download --url 'hf://TheBloke/Llama-2-7B-GGUF/llama-2-7b.Q4_K_M.gguf'

# 2. Verify
modfetch verify --path ~/Downloads/modfetch/llama-2-7b.Q4_K_M.gguf

# 3. Place into app
modfetch place --path ~/Downloads/modfetch/llama-2-7b.Q4_K_M.gguf
```

**Batch download with placement:**
```bash
# Create batch file
cat > jobs.yml << 'YAML'
items:
  - url: hf://TheBloke/Llama-2-7B-GGUF/llama-2-7b.Q4_K_M.gguf
  - url: civitai://model/123456
  - url: https://example.com/model.safetensors
    sha256: abc123...
YAML

# Download and place all
modfetch download --batch jobs.yml --place --batch-parallel 3
```

**Verify and repair safetensors:**
```bash
# Scan, verify, and repair
modfetch verify --scan-dir ~/models/sd \
  --safetensors-deep \
  --repair \
  --quarantine-incomplete \
  --only-errors \
  --summary
```

---

### Scripting

**Check if download needed:**
```bash
#!/bin/bash
URL="https://example.com/model.bin"
DEST="$HOME/models/model.bin"

if [ ! -f "$DEST" ]; then
  modfetch download --url "$URL" --dest "$DEST"
else
  echo "File already exists, verifying..."
  modfetch verify --path "$DEST" || {
    echo "Verification failed, re-downloading..."
    rm "$DEST"
    modfetch download --url "$URL" --dest "$DEST"
  }
fi
```

**Parse JSON output:**
```bash
#!/bin/bash
RESULT=$(modfetch download --url "$URL" --summary-json)

SIZE=$(echo "$RESULT" | jq -r '.size')
SHA256=$(echo "$RESULT" | jq -r '.sha256')
SPEED=$(echo "$RESULT" | jq -r '.avg_speed_mbps')

echo "Downloaded: $SIZE bytes"
echo "SHA256: $SHA256"
echo "Average speed: ${SPEED} MB/s"
```

**Automated cleanup:**
```bash
#!/bin/bash
# Daily cleanup cron job
0 2 * * * /usr/local/bin/modfetch clean --days 7 --sidecars
```

**Monitor download status:**
```bash
#!/bin/bash
# Download with status tracking
modfetch download --url "$URL" --summary-json > result.json

if [ $? -eq 0 ]; then
  echo "✓ Download successful"
  jq . result.json
else
  echo "✗ Download failed"
  exit 1
fi
```

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `MODFETCH_CONFIG` | Default config file path |
| `HF_TOKEN` | HuggingFace API token |
| `CIVITAI_TOKEN` | CivitAI API token |

**Example:**
```bash
export MODFETCH_CONFIG=~/.config/modfetch/config.yml
export HF_TOKEN="hf_..."
export CIVITAI_TOKEN="..."

modfetch download --url 'hf://private/repo/model.gguf'
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |
| `2` | Configuration error |
| `3` | Network error |
| `4` | Verification error (SHA256 mismatch) |
| `5` | Authentication error (401/403) |

---

## Tips

**Performance:**
```bash
# Increase concurrency in config.yml
concurrency:
  per_file_chunks: 8      # More chunks (default: 4)
  chunk_size_mb: 16       # Larger chunks (default: 8)
```

**Debugging:**
```bash
# Debug level logging
modfetch download --url URL --log-level debug

# JSON logs for analysis
modfetch download --url URL --log-level debug --json | jq
```

**CI/CD Integration:**
```bash
# Quiet mode + JSON summary
modfetch download --url URL --quiet --summary-json
```

**Dry-run planning:**
```bash
# Preview without downloading
modfetch download --url 'hf://org/repo/file' --dry-run --summary-json
```

---

## See Also

- [User Guide](USER_GUIDE.md) - Workflows and use cases
- [TUI Guide](TUI_GUIDE.md) - Interactive terminal interface
- [TUI Wireframes](TUI_WIREFRAMES.md) - Visual TUI guide
- [Configuration](CONFIG.md) - Config file reference
- [Batch Downloads](BATCH.md) - Batch YAML format
- [Placement](PLACEMENT.md) - Automatic file placement
- [Troubleshooting](TROUBLESHOOTING.md) - Common issues
