#!/usr/bin/env bash
set -euo pipefail

# UAT prepare script: creates download root and checks config variable

if [[ -z "${MODFETCH_CONFIG:-}" ]]; then
  echo "Note: MODFETCH_CONFIG is not set. Pass --config to commands or export MODFETCH_CONFIG=/path/to/config.yaml" >&2
else
  echo "Using MODFETCH_CONFIG=$MODFETCH_CONFIG" >&2
fi

# Create default download root under ~/Downloads/modfetch if config not provided
DEFAULT_ROOT="${HOME}/Downloads/modfetch"
mkdir -p "$DEFAULT_ROOT"
echo "Ensured download root exists: $DEFAULT_ROOT" >&2

echo "Tokens (if needed) should be exported before running UAT: HF_TOKEN, CIVITAI_TOKEN (values not displayed)." >&2

