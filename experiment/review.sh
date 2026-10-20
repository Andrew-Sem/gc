#!/usr/bin/env bash
# Usage: experiment/review.sh <run-dir> codex|claude
# Runs one reviewer in a scratch directory and writes <run-dir>/review-<family>.md.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
run=$(cd "$1" && pwd)
family=$2
cp "$here/review-checklist.md" "$run/review-checklist.md"
scratch=$(mktemp -d "${TMPDIR:-/tmp}/gc-review-XXXXXX")
prompt=$(sed "s#RUN_DIR#$run#" "$here/review-prompt.md")
case $family in
codex)
  codex exec --skip-git-repo-check --ephemeral --ignore-user-config --disable memories \
    -m gpt-6.1-sol -c model_reasoning_effort=medium \
    -s workspace-write -c sandbox_workspace_write.network_access=true \
    --add-dir "$(go env GOCACHE)" --add-dir "$(go env GOMODCACHE)" \
    -C "$scratch" --json -o "$run/review-codex.md" \
    "$prompt" < /dev/null > "$run/review-codex.trace.jsonl" 2> "$run/review-codex.stderr.txt"
  ;;
claude)
  (cd "$scratch" && claude -p "$prompt" \
    --model claude-opus-5-5 --effort medium \
    --setting-sources project --strict-mcp-config --no-session-persistence \
    --dangerously-skip-permissions --output-format stream-json --verbose \
    < /dev/null > "$run/review-claude.trace.jsonl" 2> "$run/review-claude.stderr.txt")
  python3 -I -c 'import json,sys
for l in open(sys.argv[1]):
    try: e=json.loads(l)
    except ValueError: continue
    if e.get("type")=="result": open(sys.argv[2],"w").write(str(e.get("result","")))' "$run/review-claude.trace.jsonl" "$run/review-claude.md"
  ;;
*) echo "unknown reviewer $family" >&2; exit 2 ;;
esac
rm -rf "$scratch"
echo "$(basename "$run") $family: $(head -1 "$run/review-$family.md" 2>/dev/null)"
