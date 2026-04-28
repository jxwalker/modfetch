#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
aur_dir="$root/packaging/aur"
pkgbuild="$aur_dir/PKGBUILD"
srcinfo="$aur_dir/.SRCINFO"

if [[ ! -s "$pkgbuild" || ! -s "$srcinfo" ]]; then
  echo "AUR packaging files are missing or empty" >&2
  exit 1
fi

# shellcheck source=/dev/null
source "$pkgbuild"

expected_tag="${1:-v${pkgver}}"
expected_pkgver="${expected_tag#v}"
release_base="https://github.com/jxwalker/modfetch/releases/download/v${pkgver}"

failures=0
fail() {
  echo "AUR package check: $*" >&2
  failures=$((failures + 1))
}

require_equal() {
  local actual="$1"
  local expected="$2"
  local label="$3"
  if [[ "$actual" != "$expected" ]]; then
    fail "$label expected '$expected' but found '$actual'"
  fi
}

require_file_contains() {
  local file="$1"
  local pattern="$2"
  local label="$3"
  if ! grep -Eq "$pattern" "$file"; then
    fail "$label"
  fi
}

fetch_sha256_file() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" | awk '{ print $1; exit }'
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$url" | awk '{ print $1; exit }'
  else
    echo "curl or wget is required for AUR checksum validation" >&2
    exit 1
  fi
}

fetch_sha256_body() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    if command -v sha256sum >/dev/null 2>&1; then
      curl -fsSL "$url" | sha256sum | awk '{ print $1; exit }'
    else
      curl -fsSL "$url" | shasum -a 256 | awk '{ print $1; exit }'
    fi
  elif command -v wget >/dev/null 2>&1; then
    if command -v sha256sum >/dev/null 2>&1; then
      wget -qO- "$url" | sha256sum | awk '{ print $1; exit }'
    else
      wget -qO- "$url" | shasum -a 256 | awk '{ print $1; exit }'
    fi
  else
    echo "curl or wget is required for AUR checksum validation" >&2
    exit 1
  fi
}

array_has() {
  local needle="$1"
  shift
  local item
  for item in "$@"; do
    [[ "$item" == "$needle" ]] && return 0
  done
  return 1
}

require_equal "$pkgname" "modfetch-bin" "pkgname"
require_equal "$_pkgname" "modfetch" "_pkgname"
require_equal "$pkgver" "$expected_pkgver" "pkgver"
require_equal "${pkgrel}" "1" "pkgrel"
array_has "x86_64" "${arch[@]}" || fail "arch must include x86_64"
array_has "aarch64" "${arch[@]}" || fail "arch must include aarch64"
array_has "modfetch" "${provides[@]}" || fail "provides must include modfetch"
array_has "modfetch" "${conflicts[@]}" || fail "conflicts must include modfetch"

require_equal "${source[0]}" "LICENSE::https://raw.githubusercontent.com/jxwalker/modfetch/v${pkgver}/LICENSE" "license source"
require_equal "${source_x86_64[0]}" "modfetch-${pkgver}-linux-amd64::${release_base}/modfetch_linux_amd64" "x86_64 source"
require_equal "${source_aarch64[0]}" "modfetch-${pkgver}-linux-arm64::${release_base}/modfetch_linux_arm64" "aarch64 source"

require_equal "${sha256sums[0]}" "$(fetch_sha256_body "https://raw.githubusercontent.com/jxwalker/modfetch/v${pkgver}/LICENSE")" "LICENSE checksum"
require_equal "${sha256sums_x86_64[0]}" "$(fetch_sha256_file "${release_base}/modfetch_linux_amd64.sha256")" "x86_64 checksum"
require_equal "${sha256sums_aarch64[0]}" "$(fetch_sha256_file "${release_base}/modfetch_linux_arm64.sha256")" "aarch64 checksum"

require_file_contains "$pkgbuild" 'install -Dm755 "\$binary" "\$\{pkgdir\}/usr/bin/\$\{_pkgname\}"' "PKGBUILD must install modfetch into /usr/bin"
require_file_contains "$pkgbuild" 'install -Dm644 "\$\{srcdir\}/LICENSE" "\$\{pkgdir\}/usr/share/licenses/\$\{pkgname\}/LICENSE"' "PKGBUILD must install the license"

require_file_contains "$srcinfo" '^pkgbase = modfetch-bin$' ".SRCINFO pkgbase mismatch"
require_file_contains "$srcinfo" "pkgver = ${pkgver}$" ".SRCINFO pkgver mismatch"
require_file_contains "$srcinfo" "source_x86_64 = modfetch-${pkgver}-linux-amd64::${release_base}/modfetch_linux_amd64$" ".SRCINFO x86_64 source mismatch"
require_file_contains "$srcinfo" "sha256sums_x86_64 = ${sha256sums_x86_64[0]}$" ".SRCINFO x86_64 checksum mismatch"
require_file_contains "$srcinfo" "source_aarch64 = modfetch-${pkgver}-linux-arm64::${release_base}/modfetch_linux_arm64$" ".SRCINFO aarch64 source mismatch"
require_file_contains "$srcinfo" "sha256sums_aarch64 = ${sha256sums_aarch64[0]}$" ".SRCINFO aarch64 checksum mismatch"

if command -v makepkg >/dev/null 2>&1; then
  tmp_srcinfo="$(mktemp)"
  (cd "$aur_dir" && makepkg --printsrcinfo > "$tmp_srcinfo")
  if ! diff -u "$srcinfo" "$tmp_srcinfo"; then
    fail ".SRCINFO does not match makepkg --printsrcinfo"
  fi
  rm -f "$tmp_srcinfo"
else
  echo "makepkg not found; completed portable AUR metadata and checksum validation"
fi

if [[ "$failures" -gt 0 ]]; then
  exit 1
fi

echo "AUR package check passed for ${expected_tag}"
