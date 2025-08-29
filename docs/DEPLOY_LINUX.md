# Linux deployment and dev-server setup

This guide helps you build, configure, and smoke test modfetch on a Linux dev server (Ubuntu 22.04+ recommended).

## 1) Prerequisites
- OS: Ubuntu 22.04 LTS (or similar)
- Packages: git, curl, ca-certificates, build tools
- Go: 1.22+

Example (Ubuntu):
- Install Go via official tarball (adjust version/arch as needed):
  - Download: https://go.dev/dl
  - Add /usr/local/go/bin to PATH (profile or system-wide)
- Or install from your distro if it provides Go 1.22+.

Verify:
- `go version` shows 1.22 or newer.

## 2) Get modfetch and build
Option A — build directly on the dev server (recommended):
- Clone your repo and build:
  - `git clone <your-remote> modfetch && cd modfetch`
  - `make build` (produces `bin/modfetch`)

Option B — cross-compile on another machine:
- From your workstation:
  - `make linux` (produces artifacts in `dist/`)
  - Copy the `modfetch_linux_<arch>` binary to the server and rename to `modfetch`.

Check version:
- `./bin/modfetch version` should print a version (CI/Make injects this at build time).

## 3) Configure
- Create a YAML config at one of:
  - User: `$HOME/.config/modfetch/config.yml`
  - System: `/etc/modfetch/config.yml` (ensure the process has read permission)
- Start from the example at `assets/sample-config/config.example.yml` and adjust paths:
  - `general.data_root` (DB/logs/metrics root)
  - `general.download_root` (where downloads are staged)
  - `placement.apps` and `placement.mapping` for your tools (ComfyUI, A1111, Ollama, etc.)

Tokens (if needed):
- Provide private-access tokens via environment variables (do not store secrets in YAML):
  - `HF_TOKEN` for Hugging Face (gated repos)
  - `CIVITAI_TOKEN` for CivitAI
- Export them in your shell profile or a secure env file you source (avoid printing them back to screen).

Validate config:
- `modfetch config validate --config /path/to/config.yml`

## 4) Smoke test
- Direct HTTP test (public):
  - `modfetch download --config /path/to/config.yml --url 'https://proof.ovh.net/files/1Mb.dat'`
- Hugging Face resolver (public):
  - `modfetch download --config /path/to/config.yml --url 'hf://gpt2/README.md?rev=main'`
- Status and verify:
  - `modfetch status --config /path/to/config.yml --json | head -n 50`
  - `modfetch verify --config /path/to/config.yml --all`

## 5) Batch jobs (optional)
- See `assets/sample-config/jobs.example.yml` for a starter file.
- Run jobs and optionally place:
  - `modfetch download --config /path/to/config.yml --batch /path/to/jobs.yml --place`

## 6) TUI (optional)
- Live dashboard:
  - `modfetch tui --config /path/to/config.yml`
- Suggested: run inside tmux/screen if you want it to persist.

## 7) Metrics (optional)
- If enabled in config (`metrics.prometheus_textfile`), modfetch will write a Prometheus textfile to the configured path. Point a node exporter textfile collector to that directory.

## 8) Troubleshooting
- "Range unsupported" or HEAD failures: the downloader will automatically fall back to single-stream.
- Resume: re-running the same download resumes and verifies integrity.
- Placement conflicts: if `allow_overwrite` is false and a different file exists at the destination, modfetch errors out to protect data. Either remove the file, allow overwrite, or adjust mapping.

