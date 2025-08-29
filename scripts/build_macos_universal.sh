#!/usr/bin/env bash
set -euo pipefail
if [[ $# -ne 3 ]]; then
  echo "usage: $0 <amd64_binary> <arm64_binary> <out_universal_binary>" >&2
  exit 1
fi
AMD64="$1"
ARM64="$2"
OUT="$3"
if ! command -v lipo >/dev/null; then
  echo "lipo not found; cannot build universal binary" >&2
  exit 1
fi
lipo -create -output "$OUT" "$AMD64" "$ARM64"
chmod +x "$OUT"
echo "created universal binary: $OUT"

