# Release Checklist

Use this checklist from a clean `main` checkout before tagging a modfetch release.

## Before Tagging

- Confirm `CHANGELOG.md` has a section for the exact release version.
- Run the local validation suite:
  ```bash
  go test ./...
  make build
  ```
- Confirm installer smoke tests still pass for the current platform:
  ```bash
  scripts/install.sh --install-dir "$(mktemp -d)" --skip-config-wizard
  ```
- Confirm README.md, docs/QUICKSTART.md, docs/USER_GUIDE.md, docs/CLI_GUIDE.md,
  and docs/INSTALLATION.md mention the current release behavior and do not refer
  to unpublished installation channels.

## GitHub Release

- Tag the release as `vX.Y.Z`.
- Confirm the release workflow publishes Linux and macOS binaries plus `.sha256`
  files for every artifact.
- Confirm GitHub Release notes were extracted from the matching `CHANGELOG.md`
  section.

## Homebrew Tap

- Update `jxwalker/homebrew-tap` after release assets are published.
- Set the formula release token to the new tag.
- Update SHA256 values from the published release assets.
- Validate the tap locally:
  ```bash
  HOMEBREW_NO_AUTO_UPDATE=1 brew audit --strict --online jxwalker/tap/modfetch
  HOMEBREW_NO_AUTO_UPDATE=1 brew install jxwalker/tap/modfetch
  modfetch version
  HOMEBREW_NO_AUTO_UPDATE=1 brew test jxwalker/tap/modfetch
  HOMEBREW_NO_AUTO_UPDATE=1 brew uninstall modfetch
  ```
- Open and merge a tap PR, then confirm the tap branch was deleted.

## Final Verification

- Confirm `git status --short --branch` is clean on `main`.
- Confirm the release tag exists on GitHub.
- Confirm `gh release view vX.Y.Z` shows the expected assets and notes.
- Confirm there are no stale release PRs or `codex/*` branches left open.
