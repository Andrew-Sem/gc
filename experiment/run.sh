#!/usr/bin/env bash
# Usage: AGENT=codex|claude experiment/run.sh <run-dir>
# Runs one implementer in <run-dir>/workspace with the fixed prompt and records the trace.
set -euo pipefail
run=$(cd "$1" && pwd)
agent=${AGENT:-codex}
date -u +%FT%TZ > "$run/started"
set +e
case $agent in
codex)
  echo "codex gpt-6.1-sol medium $(codex --version)" > "$run/agent"
  codex exec --skip-git-repo-check --ephemeral --ignore-user-config --disable memories \
    -m gpt-6.1-sol -c model_reasoning_effort=medium \
    -s workspace-write -c sandbox_workspace_write.network_access=true \
    --add-dir "$(go env GOCACHE)" --add-dir "$(go env GOMODCACHE)" \
    -C "$run/workspace" --json -o "$run/last-message.md" \
    - < "$run/prompt.md" > "$run/trace.jsonl" 2> "$run/stderr.txt"
  ;;
claude)
  echo "claude claude-opus-5-5 medium $(claude --version)" > "$run/agent"
  (cd "$run/workspace" && claude -p "$(cat "$run/prompt.md")" \
    --model claude-opus-5-5 --effort medium \
    --setting-sources project --strict-mcp-config --no-session-persistence \
    --dangerously-skip-permissions --output-format stream-json --verbose \
    < /dev/null > "$run/trace.jsonl" 2> "$run/stderr.txt")
  ;;
*) echo "unknown agent $agent" >&2; exit 2 ;;
esac
echo $? > "$run/exit-code"
set -e
if [ "$agent" = claude ]; then
  python3 -I -c 'import json,sys
for l in open(sys.argv[1]):
    try: e=json.loads(l)
    except ValueError: continue
    if e.get("type")=="result": open(sys.argv[2],"w").write(str(e.get("result","")))' "$run/trace.jsonl" "$run/last-message.md"
fi
date -u +%FT%TZ > "$run/finished"
echo "finished with exit code $(cat "$run/exit-code")"
