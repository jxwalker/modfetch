# AUR Packaging

This directory stages the `modfetch-bin` AUR package. It packages the published
Linux release binaries instead of rebuilding from source. The package has not
been published to AUR until a maintainer account with an AUR-registered SSH key
pushes these files.

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

Publishing requires an AUR account with a registered SSH public key. A dedicated
key is preferred so it can be revoked independently:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/aur -C "aur-modfetch"
```

Add the contents of `~/.ssh/aur.pub` to the SSH public key field in the AUR
account profile, then configure the private key locally:

```sshconfig
Host aur.archlinux.org
  User aur
  IdentityFile ~/.ssh/aur
  IdentitiesOnly yes
```

Verify auth before publishing:

```bash
ssh aur@aur.archlinux.org help
```

To publish, push `PKGBUILD` and `.SRCINFO` to:

```bash
ssh://aur@aur.archlinux.org/modfetch-bin.git
```
