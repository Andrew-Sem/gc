#!/usr/bin/env bash
# Usage: experiment/run.sh <run-dir>
# Runs Codex once in <run-dir>/workspace with the fixed prompt and records the trace.
set -euo pipefail
run=$(cd "$1" && pwd)
model=${MODEL:-gpt-6.1-sol}
effort=${EFFORT:-medium}
echo "$model $effort $(codex --version)" > "$run/agent"
date -u +%FT%TZ > "$run/started"
set +e
codex exec --skip-git-repo-check --ephemeral --disable memories \
  -m "$model" -c model_reasoning_effort="$effort" \
  -s workspace-write -c sandbox_workspace_write.network_access=true \
  --add-dir "$(go env GOCACHE)" --add-dir "$(go env GOMODCACHE)" \
  -C "$run/workspace" --json -o "$run/last-message.md" \
  - < "$run/prompt.md" > "$run/trace.jsonl" 2> "$run/stderr.txt"
echo $? > "$run/exit-code"
set -e
date -u +%FT%TZ > "$run/finished"
echo "finished with exit code $(cat "$run/exit-code")"
