#!/usr/bin/env bash
set -euo pipefail

tag="${1:-}"
changelog="${2:-CHANGELOG.md}"

if [[ -z "$tag" ]]; then
    printf 'usage: %s <tag> [changelog]\n' "$0" >&2
    exit 2
fi

if [[ ! -f "$changelog" ]]; then
    printf 'changelog not found: %s\n' "$changelog" >&2
    exit 2
fi

notes=$(
    awk -v tag="$tag" '
        $1 == "##" && $2 == tag {
            found = 1
            next
        }
        found && $0 ~ "^##[[:space:]]+" {
            exit
        }
        found {
            print
        }
        END {
            if (!found) {
                exit 42
            }
        }
    ' "$changelog"
) || {
    status=$?
    if [[ "$status" -eq 42 ]]; then
        printf 'release notes for %s not found in %s\n' "$tag" "$changelog" >&2
    fi
    exit "$status"
}

notes=$(printf '%s\n' "$notes" | sed '/[^[:space:]]/,$!d')
if ! grep -q '[^[:space:]]' <<<"$notes"; then
    printf 'release notes for %s are empty in %s\n' "$tag" "$changelog" >&2
    exit 42
fi

printf '%s\n' "$notes"
