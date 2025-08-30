# Contributing to modfetch

Thanks for your interest in contributing! This guide will help you set up your environment and submit high‑quality changes.

## Prerequisites
- Go 1.22+
- Git
- Optional: GitHub CLI (`gh`) for creating releases

## Getting started
1) Fork and clone

   git clone https://github.com/<you>/modfetch
   cd modfetch

2) Build and test

   make build
   make test

3) Run a quick smoke test
- Prepare a minimal config (example paths):

   mkdir -p ~/.config/modfetch
   cat >~/.config/modfetch/config.yml <<'YAML'
   version: 1
   general:
     data_root: "~/modfetch-data"
     download_root: "~/Downloads/modfetch"
     placement_mode: "symlink"
   YAML

- Download a public file and verify

   ./bin/modfetch config validate --config ~/.config/modfetch/config.yml
   ./bin/modfetch download --config ~/.config/modfetch/config.yml --url 'https://proof.ovh.net/files/1Mb.dat'
   ./bin/modfetch verify --config ~/.config/modfetch/config.yml --all --summary

## Development workflow
- Create a feature branch for your change
- Keep PRs focused and small; include context/rationale in the description
- Update docs for user‑visible changes (README, docs/USER_GUIDE.md)
- Ensure tests pass: `make test`
- Perform a quick manual smoke test (as above)

## Coding style
- Follow idiomatic Go (gofmt/go vet defaults); keep functions cohesive and well‑named
- Log messages should be actionable and avoid leaking secrets
- Prefer small, testable units; add tests for new behavior

## Commit messages
- Use clear, imperative subject lines (e.g., "tui: add sort by ETA")
- Reference issues/PRs when applicable

## Pull request checklist
- [ ] Tests pass (go test ./...)
- [ ] Docs updated (README/USER_GUIDE/etc.)
- [ ] Manual smoke test completed for at least one public URL
- [ ] No secrets in configs or logs

## Local release (maintainers)
- Tag a release: `git tag -a vX.Y.Z -m "modfetch vX.Y.Z" && git push origin vX.Y.Z`
- Build artifacts: `make release-dist`
- Create macOS Universal binary and update checksums:

  make macos-universal && make checksums

- Upload artifacts to GitHub:

  gh release upload vX.Y.Z dist/* --clobber

- Draft release notes in CHANGELOG.md and on GitHub

