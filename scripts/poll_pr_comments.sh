#!/usr/bin/env bash
set -euo pipefail

PR_NUMBER="${1:-18}"
INTERVAL="${INTERVAL:-120}"
STATE_DIR=".pr_monitor"
BACKLOG="docs/backlog/pr-${PR_NUMBER}.md"
AUTO_COMMIT="${AUTO_COMMIT:-1}"
POST_COMMENT="${POST_COMMENT:-1}"
AUTHORS="${AUTHORS:-coderabbitai,codex}"
AUTHORS_PATTERN="$(printf '%s' "$AUTHORS" | sed 's/,/|/g')"

# Ensure we operate from the repo root
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT"

mkdir -p "$STATE_DIR" "$(dirname "$BACKLOG")"
LAST_COUNT_FILE="${STATE_DIR}/pr_${PR_NUMBER}_coderabbit_count.txt"

get_count() {
  gh pr view "$PR_NUMBER" --json comments 2>/dev/null | jq -r --arg re "$AUTHORS_PATTERN" '((.comments | map(select(.author.login|test($re))) | length) // 0)' || echo "0"
}

get_last_body() {
  gh pr view "$PR_NUMBER" --json comments 2>/dev/null | jq -r --arg re "$AUTHORS_PATTERN" '((.comments | map(select(.author.login|test($re))) | last | .body) // "")' || echo ""
}

get_last_author() {
  gh pr view "$PR_NUMBER" --json comments 2>/dev/null | jq -r --arg re "$AUTHORS_PATTERN" '((.comments | map(select(.author.login|test($re))) | last | .author.login) // "")' || echo ""
}

extract_tasks() {
  # Extract GitHub-style checkbox tasks (unchecked and checked)
  awk '/^- \[ ?[xX ] ?\]/ {print}'
}

post_summary_comment() {
  local ts="$1"
  local tasks_block="$2"
  local tmpfile
  tmpfile="$(mktemp)"
  {
    echo "CodeRabbit update ($ts)"
    echo
    echo "Extracted tasks:"
    echo
    if [[ -n "$tasks_block" ]]; then
      printf "%s\n" "$tasks_block"
    else
      echo "(no checkbox tasks parsed)"
    fi
    echo
    echo "Backlog file updated: $BACKLOG"
  } > "$tmpfile"
  gh pr comment "$PR_NUMBER" --body-file "$tmpfile" || true
  rm -f "$tmpfile"
}

# Initialize last count to current, so we only append on new comments after start.
init_count="$(get_count || echo 0)"
echo "$init_count" > "$LAST_COUNT_FILE"

echo "Polling PR #$PR_NUMBER for comments by [$AUTHORS] every $INTERVAL seconds; writing to $BACKLOG" >&2

while true; do
  sleep "$INTERVAL"
  new_count="$(get_count || echo 0)"
  last_count="$(cat "$LAST_COUNT_FILE" 2>/dev/null || echo 0)"
  if [[ "$new_count" =~ ^[0-9]+$ ]] && [[ "$last_count" =~ ^[0-9]+$ ]]; then
    if (( new_count > last_count )); then
      body="$(get_last_body)"
      author="$(get_last_author)"
      ts="$(date -u +"%Y-%m-%d %H:%M:%SZ")"
      tasks_block="$(printf "%s\n" "$body" | extract_tasks || true)"
      {
        echo ""
        echo "## $ts $author"
        echo ""
        echo "Raw comment excerpt:"
        echo ""
        echo '```'
        printf "%s\n" "$body" | sed 's/\r$//'
        echo '```'
        echo ""
        echo "Extracted tasks:"
        echo ""
        if [[ -n "$tasks_block" ]]; then
          printf "%s\n" "$tasks_block" | sed 's/^/ - /'
        else
          echo "(none parsed)"
        fi
      } >> "$BACKLOG"

      # Auto-commit backlog update
      if [[ "$AUTO_COMMIT" == "1" ]]; then
        git add "$BACKLOG" || true
        if ! git diff --cached --quiet -- "$BACKLOG"; then
          git commit -m "chore(backlog): append CodeRabbit update to PR #$PR_NUMBER at $ts" || true
          git push || true
        fi
      fi

      # Post a summary comment to the PR
      if [[ "$POST_COMMENT" == "1" ]]; then
post_summary_comment "$ts" "$tasks_block"
      fi

      echo "$new_count" > "$LAST_COUNT_FILE"
    fi
  fi
done

