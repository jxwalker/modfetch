#!/usr/bin/env bash
set -euo pipefail

PR_NUMBER="${1:-18}"
INTERVAL="${INTERVAL:-120}"
STATE_DIR=".pr_monitor"
BACKLOG="docs/backlog/pr-${PR_NUMBER}.md"

mkdir -p "$STATE_DIR" "$(dirname "$BACKLOG")"
LAST_COUNT_FILE="${STATE_DIR}/pr_${PR_NUMBER}_coderabbit_count.txt"

get_count() {
  gh pr view "$PR_NUMBER" --json comments --jq '((.comments | map(select(.author.login=="coderabbitai")) | length) // 0)' 2>/dev/null || echo "0"
}

get_last_body() {
  gh pr view "$PR_NUMBER" --json comments --jq '((.comments | map(select(.author.login=="coderabbitai")) | last | .body) // "")' 2>/dev/null || echo ""
}

# Initialize last count to current, so we only append on new comments after start.
init_count="$(get_count || echo 0)"
echo "$init_count" > "$LAST_COUNT_FILE"

echo "Polling PR #$PR_NUMBER for CodeRabbit comments every $INTERVAL seconds; writing to $BACKLOG" >&2

while true; do
  sleep "$INTERVAL"
  new_count="$(get_count || echo 0)"
  last_count="$(cat "$LAST_COUNT_FILE" 2>/dev/null || echo 0)"
  if [[ "$new_count" =~ ^[0-9]+$ ]] && [[ "$last_count" =~ ^[0-9]+$ ]]; then
    if (( new_count > last_count )); then
      body="$(get_last_body)"
      ts="$(date -u +"%Y-%m-%d %H:%M:%SZ")"
      {
        echo ""
        echo "## $ts CodeRabbit"
        echo ""
        echo "Raw comment excerpt:"
        echo ""
        echo '```'
        printf "%s\n" "$body" | sed 's/\r$//'
        echo '```'
        echo ""
        echo "Extracted tasks:"
        echo ""
        # Extract checkbox lines if present
        printf "%s\n" "$body" | awk '/^- \[ \]/ {print " - " $0}'
      } >> "$BACKLOG"
      echo "$new_count" > "$LAST_COUNT_FILE"
    fi
  fi
done

