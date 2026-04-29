#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
aur_dir="$root/packaging/aur"
pkgbase="${AUR_PKGBASE:-modfetch-bin}"
remote="${AUR_REMOTE:-ssh://aur@aur.archlinux.org/${pkgbase}.git}"
branch="${AUR_BRANCH:-master}"
tag="${1:-}"

if [[ -z "$tag" ]]; then
  pkgver="$(grep -E '^[[:space:]]*pkgver=' "$aur_dir/PKGBUILD" | head -n 1 | cut -d= -f2-)"
  tag="v${pkgver}"
fi

if [[ "$tag" != v* ]]; then
  echo "release tag must start with v, got: $tag" >&2
  exit 1
fi

"$root/scripts/check-aur-package.sh" "$tag"

if [[ "$remote" == ssh://aur@aur.archlinux.org/* ]] && ! ssh -o BatchMode=yes -o ConnectTimeout=10 aur@aur.archlinux.org help >/dev/null 2>&1; then
  cat >&2 <<'EOF'
AUR SSH authentication failed.

Register this machine's AUR public key in the AUR account profile, then retry:

  cat ~/.ssh/aur.pub

Expected local SSH config:

  Host aur.archlinux.org
    User aur
    IdentityFile ~/.ssh/aur
    IdentitiesOnly yes

EOF
  exit 1
fi

workdir="$(mktemp -d "${TMPDIR:-/tmp}/modfetch-aur.XXXXXX")"
cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT

git -c init.defaultBranch="$branch" clone --depth 1 "$remote" "$workdir/$pkgbase"

cp "$aur_dir/PKGBUILD" "$aur_dir/.SRCINFO" "$workdir/$pkgbase/"
cd "$workdir/$pkgbase"

git add PKGBUILD .SRCINFO
if git diff --cached --quiet; then
  echo "AUR package ${pkgbase} is already up to date for ${tag}"
  exit 0
fi

git commit -m "Update ${pkgbase} to ${tag}"
git push origin "HEAD:${branch}"

echo "Published ${pkgbase} ${tag} to AUR"
