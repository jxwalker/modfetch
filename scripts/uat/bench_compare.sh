#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
bin="$repo_root/bin/modfetch"
if [[ ! -x "$bin" ]]; then
  bin="$repo_root/modfetch"
fi
if [[ ! -x "$bin" ]]; then
  echo "modfetch binary not found; run make build first" >&2
  exit 1
fi

workdir="$(mktemp -d "${TMPDIR:-/tmp}/modfetch-bench-uat.XXXXXX")"
trap 'rm -rf "$workdir"' EXIT
cfg="$workdir/config.yml"
cat >"$cfg" <<YAML
version: 1
general:
  data_root: "$workdir/data"
  download_root: "$workdir/downloads"
concurrency:
  per_file_chunks: 4
  per_host_requests: 4
  chunk_size_mb: 1
YAML
mkdir -p "$workdir/data" "$workdir/downloads"

url="${MODFETCH_BENCH_UAT_URL:-https://proof.ovh.net/files/1Mb.dat}"
tools="modfetch"
if command -v aria2c >/dev/null 2>&1; then
  tools="modfetch,aria2"
else
  echo "aria2c not found; running modfetch-only bench UAT" >&2
fi

"$bin" bench --config "$cfg" --url "$url" --tools "$tools" --duration "${MODFETCH_BENCH_UAT_DURATION:-3s}" --profile default --json

echo "bench compare UAT passed for $tools against $url" >&2
