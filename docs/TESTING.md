# Testing strategy and UAT plan

This document describes how to test modfetch locally, in CI, and on a Linux UAT server.

## 1. Test levels
- Unit/behavior (no network): config parsing, classifier, placement, status/verify against temp files and DB.
- Integration (networked): resolvers (hf://, civitai://) and downloaders (single/chunked), batch, placement.
- UAT/stress: long downloads, resume/repair, rate limits, TLS, performance, disk pressure.

## 2. Requirements
- Set tokens via environment:
  - `HF_TOKEN` (for gated Hugging Face repos)
  - `CIVITAI_TOKEN` (for CivitAI API)
- A YAML config (see CONFIG.md) and writable paths in `general.data_root`, `general.download_root`.

## 3. Local smoke tests
```
scripts/smoke.sh ./assets/sample-config/config.example.yml
```

## 4. Integration tests (Go)
- Execute: `go test ./...`
- Networked tests are minimal and use public small files; gated tests should be added gated by tokens.

## 5. UAT on Ubuntu server
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

## 6. UAT scenarios and acceptance
- Reliability: resume (kill mid-download and restart), checksum repair (tamper `.part`, re-run), Range vs non-Range servers.
- Functionality: classification, placement (symlink/hardlink/copy); batch overrides.
- Observability: metrics written; logs helpful and secret-safe; status/verify truth.
- Performance: throughput >= 70–80% of available bandwidth; CPU/mem within acceptable bounds.

## 7. CI
- Add jobs for `golangci-lint`, `govulncheck`, integration tests gated by secrets, release workflow on tag.


