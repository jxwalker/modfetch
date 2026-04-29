# Testing Strategy and UAT Plan

This document describes how to test modfetch locally, in CI, and on a Linux UAT server.

## 1. Test Levels

- Unit and behavior tests: config parsing, classifier, placement, archive,
  downloader, status/verify, catalog import/export, and TUI state against temp
  files, local databases, fixtures, and local HTTP servers.
- Integration tests: resolver behavior, download flows, archive extraction,
  catalog import/export, placement, batch, scanner, and TUI workflows.
- UAT and stress: long downloads, resume/repair, rate limits, TLS, performance,
  disk pressure, installer smoke tests, and package metadata validation.

## 2. Requirements

- Go 1.22 or newer.
- Set tokens via environment:
  - `HF_TOKEN` (for gated Hugging Face repos)
  - `CIVITAI_TOKEN` (for CivitAI API)
- A YAML config (see CONFIG.md) and writable paths in `general.data_root`, `general.download_root`.

## 3. Local Validation

Run the real local validation suite from the repository root:

```bash
scripts/check-docs-drift.sh
scripts/check-aur-package.sh
go test -count=1 ./...
go vet ./...
make build
```

On macOS, `scripts/check-aur-package.sh` validates the portable metadata and
published release checksums, then skips only the Arch-specific `makepkg`
regeneration check when `makepkg` is unavailable.

## 4. Local Smoke Tests

```bash
scripts/smoke.sh ./assets/sample-config/config.example.yml
scripts/uat/real_download_matrix.sh
```

The real download matrix builds the local binary if needed, creates an isolated
temporary config, then performs live public downloads through direct HTTP,
`starter://`, Hugging Face resolver paths, and a tiny real Hugging Face model
selected through `discover`. It verifies the resulting download records before
removing its temporary workspace.

Set `MODFETCH_UAT_CIVITAI_URI` to add a known-small CivitAI download to the
matrix. This is intentionally opt-in because CivitAI availability, gating, and
acceptable test assets change more often than the public HTTP and Hugging Face
smoke targets.

## 5. Auth-Gated Tests

Most tests run without provider tokens. Tests that require gated provider access
read tokens from the environment and skip when they are unset:

```bash
HF_TOKEN="hf_..." CIVITAI_TOKEN="..." go test -count=1 ./internal/resolver ./internal/metadata
```

## 6. UAT on Ubuntu Server

- Install `modfetch` binary (from Release) to `/usr/local/bin`.
- Place config at `/etc/modfetch/config.yml` (or `$HOME/.config/modfetch/config.yml`).
- Export tokens in the shell or in a protected env file sourced by your shell.
- Run:
```
modfetch config validate --config /etc/modfetch/config.yml
modfetch download --config /etc/modfetch/config.yml --url 'hf://gpt2/README.md?rev=main'
modfetch status --config /etc/modfetch/config.yml --json
modfetch verify --config /etc/modfetch/config.yml --all
```
- Batch UAT:
```
modfetch download --config /etc/modfetch/config.yml --batch /opt/modfetch/jobs/uat.yml --place
```
- TUI:
```
tmux new -s modfetch_tui 'modfetch tui --config /etc/modfetch/config.yml'
```

## 7. UAT Scenarios and Acceptance

- Reliability: resume (kill mid-download and restart), checksum repair (tamper `.part`, re-run), Range vs non-Range servers.
- Functionality: classification, placement (symlink/hardlink/copy); batch overrides.
- Observability: metrics written; logs helpful and secret-safe; status/verify truth.
- Performance: throughput >= 70-80% of available bandwidth; CPU/mem within acceptable bounds.

## 8. CI

CI runs docs drift validation, AUR metadata validation, Go tests, cross-platform
builds, and release artifact publication on tags. Add new CI coverage when a
feature introduces a new external service, package channel, or release artifact.
