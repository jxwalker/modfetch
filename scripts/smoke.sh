#!/usr/bin/env bash
set -euo pipefail
# Simple smoke test for modfetch installation and config
CFG=${1:-${MODFETCH_CONFIG:-}}
if [[ -z "${CFG}" ]]; then
  echo "usage: $0 /path/to/config.yml" >&2
  exit 1
fi

set -x
modfetch version
modfetch config validate --config "$CFG"
modfetch download --config "$CFG" --url 'https://proof.ovh.net/files/1Mb.dat'
modfetch status --config "$CFG" --json | head -n 50 || true
modfetch verify --config "$CFG" --all
set +x

echo "Smoke test completed."

