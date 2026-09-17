#!/usr/bin/env bash
set -eo pipefail

PROJECT_DIR="$HOME/.claude/projects/$(pwd | tr '/' '-')"

if [ ! -d "$PROJECT_DIR" ]; then
  echo "No sessions found for this project ($PROJECT_DIR does not exist)."
  exit 1
fi

query="${1:-}"

mapfile -t rows < <(python3 - "$PROJECT_DIR" <<'PY'
import json, sys, pathlib
d = pathlib.Path(sys.argv[1])
for f in sorted(d.glob("*.jsonl"), key=lambda p: p.stat().st_mtime, reverse=True):
    title, sid = None, f.stem
    with f.open() as fh:
        for line in fh:
            try:
                obj = json.loads(line)
            except ValueError:
                continue
            if obj.get("type") == "ai-title":
                title = obj.get("aiTitle")
            elif title is None and obj.get("type") == "user":
                msg = obj.get("message", {})
                content = msg.get("content")
                if isinstance(content, str):
                    title = content[:60]
                elif isinstance(content, list) and content and isinstance(content[0], dict):
                    title = str(content[0].get("text", ""))[:60]
    print(f"{sid}\t{title or '(untitled)'}\t{f.stat().st_mtime}")
PY
)

if [ "${#rows[@]}" -eq 0 ]; then
  echo "No sessions found for this project."
  exit 1
fi

match_ids=()
match_titles=()

if [ -n "$query" ]; then
  low_query=$(echo "$query" | tr '[:upper:]' '[:lower:]')
fi

for row in "${rows[@]}"; do
  IFS=$'\t' read -r sid title mtime <<< "$row"
  if [ -z "$query" ]; then
    match_ids+=("$sid")
    match_titles+=("$title")
    continue
  fi
  low_title=$(echo "$title" | tr '[:upper:]' '[:lower:]')
  if [[ "$low_title" == *"$low_query"* ]] || [[ "$sid" == *"$query"* ]]; then
    match_ids+=("$sid")
    match_titles+=("$title")
  fi
done

if [ "${#match_ids[@]}" -eq 0 ]; then
  echo "No session found matching \"$query\""
  exit 1
fi

if [ "${#match_ids[@]}" -gt 1 ] || [ -z "$query" ]; then
  echo "Sessions for this project:"
  for i in "${!match_ids[@]}"; do
    mark=""
    if [ "${match_ids[$i]}" = "${CLAUDE_CODE_SESSION_ID:-}" ]; then
      mark=" (current)"
    fi
    printf "  %d. %s [%s]%s\n" "$((i+1))" "${match_titles[$i]}" "${match_ids[$i]}" "$mark"
  done
  read -rp "Select a session to delete (1-${#match_ids[@]}, or Enter to cancel): " choice
  if [ -z "$choice" ]; then
    echo "Cancelled."
    exit 0
  fi
  if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "${#match_ids[@]}" ]; then
    echo "Invalid selection, aborting."
    exit 1
  fi
  idx=$((choice-1))
else
  idx=0
fi

sid="${match_ids[$idx]}"
title="${match_titles[$idx]}"
file="$PROJECT_DIR/$sid.jsonl"

read -rp "Delete session \"$title\" ($file)? (y/N) " confirm
if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
  echo "Cancelled."
  exit 0
fi

rm -- "$file"
echo "Session deleted."

if [ "$sid" = "${CLAUDE_CODE_SESSION_ID:-}" ]; then
  echo "This was the active session. Exit it (/exit or Ctrl-D) and start a new 'claude' to fully leave it."
fi
