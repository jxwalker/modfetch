# Testing

Use this file as the short maintainer checklist. The longer UAT plan lives in
`docs/TESTING.md`.

## Local Validation

Run from the repository root:

```bash
scripts/check-docs-drift.sh
scripts/check-aur-package.sh
go test -count=1 ./...
go vet ./...
make build
```

`scripts/check-aur-package.sh` validates the staged AUR metadata and published
release checksums. On macOS, it reports that `makepkg` is unavailable and skips
only the Arch-specific `.SRCINFO` regeneration check.

## Auth-Gated Coverage

Most tests use local fixtures, temporary databases, local HTTP servers, or public
small files. Tests that need gated provider access read tokens from the
environment and skip when they are not set:

```bash
export HF_TOKEN="hf_..."
export CIVITAI_TOKEN="..."
go test -count=1 ./internal/resolver ./internal/metadata
```

Do not print token values in logs or test output.

## Focused TUI Checks

```bash
go test -count=1 ./internal/tui/... ./cmd/modfetch
```

These cover navigation, filter persistence, library filters, multi-select bulk
actions, selected catalog export, settings rendering, config wizard validation,
non-interactive TUI snapshots backed by a real SQLite state database, and shell
completion drift for removed TUI selector flags. The command package tests also
exercise HTTP(S) catalog sync push/pull with real `httptest` servers and bearer
headers.

The metadata package exercises registry fetchers against local HTTP/TLS servers,
including Hugging Face, CivitAI, ModelScope, and Ollama Library behavior.

## Manual Smoke

After `make build`, verify non-gated behavior with a public URL:

```bash
export MODFETCH_CONFIG="${MODFETCH_CONFIG:-$HOME/.config/modfetch/config.yml}"
./bin/modfetch version
./bin/modfetch download --config "$MODFETCH_CONFIG" --url 'https://proof.ovh.net/files/1Mb.dat' --summary-json
./bin/modfetch verify --config "$MODFETCH_CONFIG" --all
```

For TUI validation:

```bash
./bin/modfetch tui --config ~/.config/modfetch/config.yml
./bin/modfetch tui --config ~/.config/modfetch/config.yml --snapshot --json
```
