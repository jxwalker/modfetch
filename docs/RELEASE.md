# Release Checklist

Use this checklist from a clean `main` checkout before tagging a modfetch release.

## Before Tagging

- Confirm `CHANGELOG.md` has a section for the exact release version.
- Run the local validation suite:
  ```bash
  scripts/check-docs-drift.sh
  go test ./...
  make build
  ```
- Confirm installer smoke tests still pass for the current platform:
  ```bash
  scripts/install.sh --install-dir "$(mktemp -d)" --skip-config-wizard
  ```
- Confirm README.md, docs/QUICKSTART.md, docs/USER_GUIDE.md, docs/CLI_GUIDE.md,
  docs/TUI_WIREFRAMES.md, and docs/INSTALLATION.md mention the current release
  behavior and clearly label any staged-but-unpublished installation channel.

## GitHub Release

- Tag the release as `vX.Y.Z`.
- Confirm the release workflow publishes Linux and macOS binaries plus `.sha256`
  files for every artifact.
- Confirm GitHub Release notes were extracted from the matching `CHANGELOG.md`
  section.

## Homebrew Tap

- Update `jxwalker/homebrew-tap` after release assets are published.
- Set the formula release string to the new tag.
- Update SHA256 values from the published release assets.
- Validate the tap locally:
  ```bash
  HOMEBREW_NO_AUTO_UPDATE=1 brew audit --strict --online jxwalker/tap/modfetch
  HOMEBREW_NO_AUTO_UPDATE=1 brew install jxwalker/tap/modfetch
  "$(brew --prefix)/bin/modfetch" version
  HOMEBREW_NO_AUTO_UPDATE=1 brew test jxwalker/tap/modfetch
  HOMEBREW_NO_AUTO_UPDATE=1 brew uninstall modfetch
  ```
- Open and merge a tap PR, then confirm the tap branch was deleted.

## AUR Package

- Update `packaging/aur/PKGBUILD` and `packaging/aur/.SRCINFO` after release
  assets are published.
- Set `pkgver` to the release version without the leading `v`.
- Update SHA256 values from the published Linux release assets and LICENSE file.
- Confirm the publishing machine can authenticate with AUR before release-day
  publication:
  ```bash
  ssh -o BatchMode=yes -o ConnectTimeout=5 aur@aur.archlinux.org help
  ```
- If authentication fails with `Permission denied (publickey)`, create an AUR
  account, generate a dedicated SSH key, and paste the public key into the AUR
  account profile before retrying:
  ```bash
  ssh-keygen -t ed25519 -f ~/.ssh/aur -C "aur-modfetch"
  ```
  Then configure the private key locally:
  ```sshconfig
  Host aur.archlinux.org
    User aur
    IdentityFile ~/.ssh/aur
    IdentitiesOnly yes
  ```
- Validate the packaged metadata and published checksums:
  ```bash
  scripts/check-aur-package.sh vX.Y.Z
  ```
- On an Arch Linux machine, validate the source package before pushing to AUR:
  ```bash
  cd packaging/aur
  makepkg --printsrcinfo > .SRCINFO
  makepkg -si
  modfetch version
  namcap PKGBUILD
  ```
- Push `PKGBUILD` and `.SRCINFO` to
  `ssh://aur@aur.archlinux.org/modfetch-bin.git`.

## Final Verification

- Ensure `git status --short --branch` is clean on `main`.
- Verify the release tag exists on GitHub.
- Inspect `gh release view vX.Y.Z` for the expected assets and notes.
- Check there are no stale release PRs or `codex/*` branches remaining.
