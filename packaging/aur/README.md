# AUR Packaging

This directory stages the `modfetch-bin` AUR package. It packages the published
Linux release binaries instead of rebuilding from source.

Validate metadata and live release checksums from the repository root:

```bash
scripts/check-aur-package.sh
```

Before publishing from an Arch Linux machine:

```bash
cd packaging/aur
makepkg --printsrcinfo > .SRCINFO
makepkg -si
modfetch version
namcap PKGBUILD
```

To publish, push `PKGBUILD` and `.SRCINFO` to:

```bash
ssh://aur@aur.archlinux.org/modfetch-bin.git
```

Publication requires an AUR account with a registered SSH key.
