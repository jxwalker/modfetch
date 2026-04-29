#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
bin="${MODFETCH_BIN:-$root/bin/modfetch}"

if [[ -n "${MODFETCH_BIN:-}" ]]; then
  if [[ ! -x "$bin" ]]; then
    printf 'Configured MODFETCH_BIN is not executable: %s\n' "$bin" >&2
    exit 1
  fi
elif [[ ! -x "$bin" ]]; then
  (cd "$root" && make build)
fi

workdir="$(mktemp -d "${TMPDIR:-/tmp}/modfetch-real-uat.XXXXXX")"
cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT

cfg="$workdir/config.yml"
cat >"$cfg" <<EOF
version: 1
general:
  data_root: "$workdir/data"
  download_root: "$workdir/downloads"
sources:
  huggingface:
    enabled: true
    token_env: HF_TOKEN
  civitai:
    enabled: true
    token_env: CIVITAI_TOKEN
EOF

mkdir -p "$workdir/data" "$workdir/downloads"

run_case() {
  local name="$1"
  local uri="$2"
  printf '\n==> %s\n' "$name" >&2
  "$bin" download --config "$cfg" --url "$uri" --summary-json --quiet
}

run_case "direct public HTTP" "https://proof.ovh.net/files/1Mb.dat"
run_case "starter alias via resolver" "starter://gpt2-tokenizer"
run_case "public Hugging Face resolver" "hf://gpt2/config.json?rev=607a30d783dfa663caf39e06633721c8d4cfcd7e"

if [[ -n "${MODFETCH_UAT_CIVITAI_URI:-}" ]]; then
  run_case "configured CivitAI URI" "$MODFETCH_UAT_CIVITAI_URI"
else
  printf '\nSkipping CivitAI real download: set MODFETCH_UAT_CIVITAI_URI to a known-small public or token-gated URI.\n' >&2
fi

if [[ -n "${MODFETCH_UAT_EXTRA_URI:-}" ]]; then
  run_case "configured extra URI" "$MODFETCH_UAT_EXTRA_URI"
fi

"$bin" verify --config "$cfg" --all
"$bin" status --config "$cfg" --summary --json >/dev/null

printf '\nReal download matrix passed. Downloads were written under %s during the run.\n' "$workdir/downloads" >&2
