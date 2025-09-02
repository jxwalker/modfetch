#!/usr/bin/env bash
set -euo pipefail

# scripts/dev_server_setup.sh
# Bootstrap a Linux dev server for modfetch.
# - Installs Go 1.22+ if not present (Ubuntu/Debian via apt + manual tarball install)
# - Ensures config directories and a starter config exist
# - Builds modfetch if the repo is present
# - Runs basic validation commands
#
# Usage: run from anywhere. If run from the repo root, it will also build modfetch.
# Environment overrides:
#   GO_VERSION   (default: 1.22.10)
#   INSTALL_GO   (default: auto; set to 0 to skip, 1 to force)
#   CONFIG_PATH  (default: $HOME/.config/modfetch/config.yml)

GO_VERSION=${GO_VERSION:-1.22.10}
INSTALL_GO=${INSTALL_GO:-auto}
CONFIG_PATH=${CONFIG_PATH:-"${XDG_CONFIG_HOME:-$HOME/.config}/modfetch/config.yml"}

log()  { printf "\033[1;32m[modfetch-setup]\033[0m %s\n" "$*"; }
warn() { printf "\033[1;33m[modfetch-setup]\033[0m %s\n" "$*"; }
err()  { printf "\033[1;31m[modfetch-setup]\033[0m %s\n" "$*"; }

have_cmd() { command -v "$1" >/dev/null 2>&1; }

parse_go_version_ok() {
  if ! have_cmd go; then return 1; fi
  local ver; ver=$(go version | awk '{print $3}' | sed 's/go//')
  # rudimentary major.minor check
  local major minor
  major=$(echo "$ver" | cut -d. -f1)
  minor=$(echo "$ver" | cut -d. -f2)
  if [[ "${major:-0}" -gt 1 ]] || { [[ "${major:-0}" -eq 1 ]] && [[ "${minor:-0}" -ge 22 ]]; }; then
    return 0
  fi
  return 1
}

install_go_linux() {
  # Requires sudo to write /usr/local
  if ! have_cmd curl; then
    if have_cmd apt-get; then
      log "Installing curl via apt-get"
      sudo apt-get update -y
      sudo apt-get install -y curl ca-certificates tar
    else
      err "curl is required. Please install curl and rerun."
      exit 1
    fi
  fi
  local os=linux arch
  case "$(uname -m)" in
    x86_64) arch=amd64;;
    aarch64|arm64) arch=arm64;;
    *) err "Unsupported architecture: $(uname -m)"; exit 1;;
  esac
  local tarball="go${GO_VERSION}.${os}-${arch}.tar.gz"
  local url="https://go.dev/dl/${tarball}"
  log "Downloading Go ${GO_VERSION} from ${url}"
  curl -fsSL "$url" -o "/tmp/${tarball}"
  log "Installing to /usr/local (requires sudo)"
  sudo rm -rf /usr/local/go || true
  sudo tar -C /usr/local -xzf "/tmp/${tarball}"
  rm -f "/tmp/${tarball}"
  if ! grep -q "/usr/local/go/bin" <<<":${PATH}:"; then
    warn "Go installed to /usr/local/go. Ensure /usr/local/go/bin is on your PATH."
  fi
}

maybe_install_go() {
  case "$INSTALL_GO" in
    0|false|no)  log "Skipping Go install (INSTALL_GO=$INSTALL_GO)"; return;;
    1|true|yes)  log "Forcing Go install"; install_go_linux; return;;
    auto)        ;;
    *)           warn "Unknown INSTALL_GO=$INSTALL_GO; defaulting to auto";;
  esac
  if parse_go_version_ok; then
    log "Go $(go version) is OK (>= 1.22)"
  else
    log "Installing Go ${GO_VERSION} (>=1.22 required)"
    install_go_linux
  fi
}

ensure_config() {
  local cfg_dir; cfg_dir=$(dirname "$CONFIG_PATH")
  mkdir -p "$cfg_dir"
  if [[ -f "$CONFIG_PATH" ]]; then
    log "Config exists: $CONFIG_PATH"
    return
  fi
  # If running from repo root and sample exists, copy it, else write a minimal config
  if [[ -f "assets/sample-config/config.example.yml" ]]; then
    log "Seeding config from assets/sample-config/config.example.yml"
    cp "assets/sample-config/config.example.yml" "$CONFIG_PATH"
  else
    log "Writing minimal starter config to $CONFIG_PATH"
    cat >"$CONFIG_PATH" <<'YAML'
version: 1
general:
  data_root: "~/modfetch-data"
  download_root: "~/Downloads/modfetch"
  placement_mode: "symlink"
  quarantine: true
  allow_overwrite: false
network:
  timeout_seconds: 60
  max_redirects: 10
  tls_verify: true
  user_agent: "modfetch/setup"
concurrency:
  global_files: 4
  per_file_chunks: 4
  chunk_size_mb: 16
  max_retries: 8
  backoff:
    min_ms: 200
    max_ms: 30000
    jitter: true
sources:
  huggingface:
    enabled: true
    token_env: "HF_TOKEN"
  civitai:
    enabled: true
    token_env: "CIVITAI_TOKEN"
placement:
  apps: {}
  mapping: []
logging:
  level: "info"
  format: "human"
  file:
    enabled: false
    path: ""
    max_megabytes: 50
    max_backups: 3
    max_age_days: 14
metrics:
  prometheus_textfile:
    enabled: false
    path: ""
validation:
  require_sha256: false
  accept_md5_sha1_if_provided: false
YAML
  fi
}

build_if_repo_present() {
  if [[ -f Makefile && -d cmd/modfetch ]]; then
    log "Building modfetch (make build)"
    make build
  else
    warn "Not in repo root; skipping build. To build: make build"
  fi
}

post_checks() {
  local bin
  if [[ -x ./bin/modfetch ]]; then
    bin=./bin/modfetch
  elif have_cmd modfetch; then
    bin=$(command -v modfetch)
  else
    warn "modfetch binary not found. Build with 'make build' inside the repo."
    return
  fi
  "$bin" version || true
  "$bin" config validate --config "$CONFIG_PATH" || true
  log "Try a smoke download (public):"
  log "  $bin download --config $CONFIG_PATH --url 'https://proof.ovh.net/files/1Mb.dat'"
  log "For Hugging Face (public):"
  log "  $bin download --config $CONFIG_PATH --url 'hf://gpt2/README.md?rev=main'"
  log "If accessing private/gated content, export HF_TOKEN/CIVITAI_TOKEN in your shell (do not print them)."
}

main() {
  log "Starting dev server setup"
  if [[ -f /etc/os-release ]]; then
    . /etc/os-release
    log "Detected OS: ${NAME:-unknown} ${VERSION_ID:-}"
  fi
  maybe_install_go
  ensure_config
  build_if_repo_present
  post_checks
  log "Setup complete. Review $CONFIG_PATH, export tokens as needed, and run smoke tests."
}

main "$@"

