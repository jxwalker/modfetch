#!/usr/bin/env bash
set -euo pipefail

# Demo for batch import + batch download
# Usage: scripts/uat/batch_import_demo.sh /path/to/config.yaml

CFG="${1:-${MODFETCH_CONFIG:-}}"
if [[ -z "$CFG" ]]; then
  echo "Usage: $0 /path/to/config.yaml (or export MODFETCH_CONFIG)" >&2
  exit 1
fi

ROOT_DIR="${HOME}/Downloads/modfetch"
mkdir -p "$ROOT_DIR"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

INPUT="$TMPDIR/input_urls.txt"
BATCH="$TMPDIR/batch.yaml"

cat > "$INPUT" <<'EOF'
# Sample URLs for import
https://speed.hetzner.de/1MB.bin
# Hugging Face example (adjust to a small public file you have access to)
# hf://owner/repo/path/to/file.bin?rev=main
# CivitAI model page (auto-normalized) – replace with a public example if available
# https://civitai.com/models/12345?modelVersionId=67890
EOF

echo "Importing URLs from $INPUT ..." >&2
./bin/modfetch batch import --config="$CFG" --input="$INPUT" --output="$BATCH" --dest-dir="$ROOT_DIR" --sha-mode=none

echo "Batch file generated at: $BATCH" >&2
cat "$BATCH"

echo "Running batch download..." >&2
./bin/modfetch download --config="$CFG" --batch="$BATCH" --summary-json || true

