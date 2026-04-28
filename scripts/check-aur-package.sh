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

extract_pkgbuild_scalar() {
  local file="$1"
  local var_name="$2"
  local line value

  line="$(grep -E "^[[:space:]]*${var_name}=" "$file" | head -n 1 || true)"
  if [[ -z "$line" ]]; then
    return 1
  fi

  value="${line#*=}"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  value="${value#\"}"
  value="${value%\"}"
  value="${value#\'}"
  value="${value%\'}"
  printf '%s\n' "$value"
}

extract_pkgbuild_array_first() {
  local file="$1"
  local var_name="$2"
  local line

  line="$(grep -E "^[[:space:]]*${var_name}=\\(" "$file" | head -n 1 || true)"
  if [[ -z "$line" ]]; then
    return 1
  fi

  printf '%s\n' "$line" | sed -E "s/^[^(]*\\([[:space:]]*['\"]([^'\"]+)['\"].*/\\1/"
}

pkgname="$(extract_pkgbuild_scalar "$pkgbuild" "pkgname")"
_pkgname="$(extract_pkgbuild_scalar "$pkgbuild" "_pkgname")"
pkgver="$(extract_pkgbuild_scalar "$pkgbuild" "pkgver")"
pkgrel="$(extract_pkgbuild_scalar "$pkgbuild" "pkgrel")"
pkgbuild_license_sha="$(extract_pkgbuild_array_first "$pkgbuild" "sha256sums")"
pkgbuild_x86_sha="$(extract_pkgbuild_array_first "$pkgbuild" "sha256sums_x86_64")"
pkgbuild_arm_sha="$(extract_pkgbuild_array_first "$pkgbuild" "sha256sums_aarch64")"

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
  local expected_text="$2"
  local label="$3"
  if ! grep -Fq -- "$expected_text" "$file"; then
    fail "$label"
  fi
}

fetch_url() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL \
      --retry 3 \
      --retry-delay 2 \
      --connect-timeout 10 \
      --max-time 60 \
      "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- \
      --tries=3 \
      --waitretry=2 \
      --timeout=10 \
      "$url"
  else
    echo "curl or wget is required for AUR checksum validation" >&2
    exit 1
  fi
}

fetch_sha256_file() {
  local url="$1"
  fetch_url "$url" | awk '{ print $1; exit }'
}

fetch_sha256_body() {
  local url="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    fetch_url "$url" | sha256sum | awk '{ print $1; exit }'
  else
    fetch_url "$url" | shasum -a 256 | awk '{ print $1; exit }'
  fi
}

require_equal "$pkgname" "modfetch-bin" "pkgname"
require_equal "$_pkgname" "modfetch" "_pkgname"
require_equal "$pkgver" "$expected_pkgver" "pkgver"
require_equal "${pkgrel}" "1" "pkgrel"

require_file_contains "$pkgbuild" "arch=('x86_64' 'aarch64')" "arch must include x86_64 and aarch64"
require_file_contains "$pkgbuild" "license=('MIT')" "license must be MIT"
require_file_contains "$pkgbuild" "provides=('modfetch')" "provides must include modfetch"
require_file_contains "$pkgbuild" "conflicts=('modfetch')" "conflicts must include modfetch"
require_file_contains "$pkgbuild" 'source=("LICENSE::https://raw.githubusercontent.com/jxwalker/modfetch/v${pkgver}/LICENSE")' "license source mismatch"
require_file_contains "$pkgbuild" 'source_x86_64=("${_pkgname}-${pkgver}-linux-amd64::https://github.com/jxwalker/modfetch/releases/download/v${pkgver}/${_pkgname}_linux_amd64")' "x86_64 source mismatch"
require_file_contains "$pkgbuild" 'source_aarch64=("${_pkgname}-${pkgver}-linux-arm64::https://github.com/jxwalker/modfetch/releases/download/v${pkgver}/${_pkgname}_linux_arm64")' "aarch64 source mismatch"
require_file_contains "$pkgbuild" 'install -Dm755 "$binary" "${pkgdir}/usr/bin/${_pkgname}"' "PKGBUILD must install modfetch into /usr/bin"
require_file_contains "$pkgbuild" 'install -Dm644 "${srcdir}/LICENSE" "${pkgdir}/usr/share/licenses/${pkgname}/LICENSE"' "PKGBUILD must install the license"

require_equal "$pkgbuild_license_sha" "$(fetch_sha256_body "https://raw.githubusercontent.com/jxwalker/modfetch/v${pkgver}/LICENSE")" "LICENSE checksum"
require_equal "$pkgbuild_x86_sha" "$(fetch_sha256_file "${release_base}/modfetch_linux_amd64.sha256")" "x86_64 checksum"
require_equal "$pkgbuild_arm_sha" "$(fetch_sha256_file "${release_base}/modfetch_linux_arm64.sha256")" "aarch64 checksum"

require_file_contains "$srcinfo" "pkgbase = modfetch-bin" ".SRCINFO pkgbase mismatch"
require_file_contains "$srcinfo" "pkgver = ${pkgver}" ".SRCINFO pkgver mismatch"
require_file_contains "$srcinfo" "source_x86_64 = modfetch-${pkgver}-linux-amd64::${release_base}/modfetch_linux_amd64" ".SRCINFO x86_64 source mismatch"
require_file_contains "$srcinfo" "sha256sums_x86_64 = ${pkgbuild_x86_sha}" ".SRCINFO x86_64 checksum mismatch"
require_file_contains "$srcinfo" "source_aarch64 = modfetch-${pkgver}-linux-arm64::${release_base}/modfetch_linux_arm64" ".SRCINFO aarch64 source mismatch"
require_file_contains "$srcinfo" "sha256sums_aarch64 = ${pkgbuild_arm_sha}" ".SRCINFO aarch64 checksum mismatch"

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
