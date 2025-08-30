#!/usr/bin/env bash
set -euo pipefail

# scripts/ci/run_all.sh - Local CI runner to mirror CI checks
# Usage: scripts/ci/run_all.sh

# 1) Static and unit tests
command -v go >/dev/null || { echo "Go toolchain not found" >&2; exit 1; }

go vet ./...
go test ./... -cover

# 2) Build CLI
mkdir -p bin
go build -ldflags "-s -w" -o bin/modfetch ./cmd/modfetch

# 3) CLI smoke + importer (no network dependency)
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

cat > "$TMPDIR/config.yaml" <<EOF
version: 1
general:
  data_root: $TMPDIR/data
  download_root: $TMPDIR/dl
EOF
mkdir -p "$TMPDIR/data" "$TMPDIR/dl"

cat > "$TMPDIR/input.txt" <<'EOF'
https://example.com/a.bin
https://example.com/b.bin
EOF

./bin/modfetch version
./bin/modfetch completion bash >/dev/null
./bin/modfetch completion zsh >/dev/null
./bin/modfetch completion fish >/dev/null

./bin/modfetch batch import --config="$TMPDIR/config.yaml" --input="$TMPDIR/input.txt" --output="$TMPDIR/batch.yaml" --dest-dir="$TMPDIR/dl" --sha-mode=none

grep -q "version: 1" "$TMPDIR/batch.yaml"
grep -q "jobs:" "$TMPDIR/batch.yaml"
grep -q "uri: https://example.com/a.bin" "$TMPDIR/batch.yaml"
grep -q "uri: https://example.com/b.bin" "$TMPDIR/batch.yaml"

echo "All local CI checks passed." >&2

