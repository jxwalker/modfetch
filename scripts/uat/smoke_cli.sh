#!/usr/bin/env bash
set -euo pipefail

CFG="${1:-${MODFETCH_CONFIG:-}}"
if [[ -z "$CFG" ]]; then
  echo "Usage: $0 /path/to/config.yaml (or export MODFETCH_CONFIG)" >&2
  exit 1
fi

# Basic config checks
./bin/modfetch config validate --config="$CFG"
./bin/modfetch config print --config="$CFG" | head -n 20

# Show version
./bin/modfetch version

# Show completion scripts exist
./bin/modfetch completion bash >/dev/null
./bin/modfetch completion zsh >/dev/null
./bin/modfetch completion fish >/dev/null

echo "Smoke CLI checks completed." >&2

