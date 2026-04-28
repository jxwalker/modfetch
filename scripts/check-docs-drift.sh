#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

latest_tag="${1:-}"
if [[ -z "$latest_tag" ]]; then
  latest_tag="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
fi
if [[ -z "$latest_tag" ]]; then
  latest_tag="$(awk '/^## v[0-9]+\.[0-9]+\.[0-9]+/ { print $2; exit }' CHANGELOG.md)"
fi
if [[ -z "$latest_tag" ]]; then
  echo "could not determine latest release tag from git tags or CHANGELOG.md; pass one explicitly" >&2
  exit 1
fi

failures=0
check_contains() {
  local file="$1"
  local pattern="$2"
  local message="$3"
  if ! grep -Eq "$pattern" "$file"; then
    echo "docs drift: $message ($file)" >&2
    failures=$((failures + 1))
  fi
}

check_not_contains() {
  local file="$1"
  local pattern="$2"
  local message="$3"
  if grep -Eiq "$pattern" "$file"; then
    echo "docs drift: $message ($file)" >&2
    failures=$((failures + 1))
  fi
}

core_docs=(
  README.md
  docs/QUICKSTART.md
  docs/USER_GUIDE.md
  docs/CLI_GUIDE.md
  docs/INSTALLATION.md
  CHANGELOG.md
)

for file in "${core_docs[@]}"; do
  if [[ ! -s "$file" ]]; then
    echo "docs drift: required release doc is missing or empty ($file)" >&2
    failures=$((failures + 1))
  fi
done

check_contains CHANGELOG.md "^## ${latest_tag//./\\.}(\\b|[[:space:]]|—|-)" "CHANGELOG.md is missing a section for ${latest_tag}"
check_contains docs/ROADMAP.md "Current release: ${latest_tag//./\\.}," "roadmap current release does not match ${latest_tag}"
check_contains scripts/install.sh "using ${latest_tag//./\\.} as fallback" "installer fallback version does not match ${latest_tag}"
check_contains docs/RELEASE.md "scripts/check-docs-drift\\.sh" "release checklist does not run docs drift validation"

for file in README.md docs/QUICKSTART.md docs/USER_GUIDE.md docs/CLI_GUIDE.md docs/INSTALLATION.md docs/RELEASE.md; do
  check_not_contains "$file" "homebrew.*(coming soon|unpublished|not yet|TODO)" "stale Homebrew publication claim"
done

if [[ "$failures" -gt 0 ]]; then
  exit 1
fi

echo "docs drift check passed for ${latest_tag}"
