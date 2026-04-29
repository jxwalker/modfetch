# AUR Packaging

This directory maintains the published `modfetch-bin` AUR package. It packages
the published Linux release binaries instead of rebuilding from source.

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

Updating the package requires an AUR account with a registered SSH public key. A
dedicated key is preferred so it can be revoked independently:

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
ssh -o BatchMode=yes -o ConnectTimeout=5 aur@aur.archlinux.org help
```

The AUR Git remote is:

```bash
ssh://aur@aur.archlinux.org/modfetch-bin.git
```

The repository publishing helper performs validation, auth checking, AUR clone,
file copy, commit, and push:

```bash
scripts/publish-aur.sh vX.Y.Z
```

If authentication fails, paste `~/.ssh/aur.pub` into the AUR account profile and
retry the command.
