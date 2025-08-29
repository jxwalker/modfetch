#!/usr/bin/env bash
set -euo pipefail

# scripts/uat_linux.sh
# End-to-end UAT helper for a Linux dev server.
# - Builds the binary (if run from repo root)
# - Validates config
# - Runs one Hugging Face download (public)
# - Optionally runs a CivitAI download if CIVITAI_UAT_URI is set (and token present if required)
# - Prints JSON status and runs verify --all
#
# Usage:
#   bash scripts/uat_linux.sh [/path/to/config.yml]
#
# Environment:
#   MODFETCH_CONFIG       Fallback config path if arg not given
#   HF_TOKEN              For gated Hugging Face resources (not required for public)
#   CIVITAI_TOKEN         For gated CivitAI resources
#   CIVITAI_UAT_URI       civitai:// URI to test in your environment (optional)
#

CONFIG=${1:-${MODFETCH_CONFIG:-${XDG_CONFIG_HOME:-$HOME/.config}/modfetch/config.yml}}

log()  { printf "\033[1;34m[uat]\033[0m %s\n" "$*"; }
warn() { printf "\033[1;33m[uat]\033[0m %s\n" "$*"; }
err()  { printf "\033[1;31m[uat]\033[0m %s\n" "$*"; }

have_cmd() { command -v "$1" >/dev/null 2>&1; }

BIN=""
if [[ -x ./bin/modfetch ]]; then
  BIN=./bin/modfetch
elif have_cmd modfetch; then
  BIN=$(command -v modfetch)
else
  err "modfetch binary not found. Build it first (make build)."; exit 1
fi

# Build if in repo and binary missing
if [[ -f Makefile && ! -x ./bin/modfetch ]]; then
  log "Building modfetch"
  make build
fi

log "Using config: $CONFIG"
if [[ ! -f "$CONFIG" ]]; then
  err "Config file does not exist: $CONFIG"; exit 1
fi

log "Version"
"$BIN" version || true

log "Validate config"
"$BIN" config validate --config "$CONFIG"

log "Public HTTP download"
"$BIN" download --config "$CONFIG" --url 'https://proof.ovh.net/files/1Mb.dat'

log "Hugging Face public download"
"$BIN" download --config "$CONFIG" --url 'hf://gpt2/README.md?rev=main'

if [[ -n "${CIVITAI_UAT_URI:-}" ]]; then
  log "CivitAI download: ${CIVITAI_UAT_URI}"
  "$BIN" download --config "$CONFIG" --url "$CIVITAI_UAT_URI"
else
  warn "Skipping CivitAI step (set CIVITAI_UAT_URI to test; CIVITAI_TOKEN may be required for gated content)"
fi

log "Status (JSON, first 50 lines)"
"$BIN" status --config "$CONFIG" --json | head -n 50 || true

log "Verify all completed downloads"
"$BIN" verify --config "$CONFIG" --all

log "UAT completed"

